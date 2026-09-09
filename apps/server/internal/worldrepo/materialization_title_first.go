package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type DevelopmentTitleCandidate struct {
	BoardID     string `json:"board_id"`
	Candidate   int    `json:"candidate"`
	Original    string `json:"original"`
	Subject     string `json:"subject"`
	Author      string `json:"author"`
	EventID     string `json:"event_id"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	EraStatus   string `json:"era_status,omitempty"`
	EraReason   string `json:"era_reason,omitempty"`
	EraEvidence string `json:"era_evidence,omitempty"`
}
type developmentTitleFirstState struct {
	history         []world.Post
	attempted       bool
	result          map[string]developmentSparseSituation
	err             error
	rows            []DevelopmentTitleCandidate
	eraResearchUsed int
}

var developmentTitleFirst sync.Map

func (r *Repository) EnableDevelopmentTitleFirstPoC(prior []world.Post) {
	developmentTitleFirst.Store(r, &developmentTitleFirstState{history: append([]world.Post(nil), prior...)})
}
func developmentTitleFirstEnabled(r *Repository) bool {
	_, ok := developmentTitleFirst.Load(r)
	return ok
}
func (r *Repository) DevelopmentTitleCandidates() []DevelopmentTitleCandidate {
	if v, ok := developmentTitleFirst.Load(r); ok {
		return append([]DevelopmentTitleCandidate(nil), v.(*developmentTitleFirstState).rows...)
	}
	return nil
}
func titleFirstSubject(facts []string) string {
	for _, f := range facts {
		if strings.HasPrefix(f, "title_first_subject=") {
			return strings.TrimPrefix(f, "title_first_subject=")
		}
	}
	return ""
}

func titleFirstReviewDecisionMalformed(reason string) bool {
	return strings.HasPrefix(strings.TrimSpace(reason), "検査結果不備：")
}

// Only the isolated conversation Lab calls this. Candidate wording is generated
// independently, but each root already has a cheap canonical sparse situation
// selected by the world layer. The title matcher may realize that situation; it
// may not replace it with a new experience or world occurrence.
func (r *Repository) developmentPlanTitleFirst(host world.Host, window []developmentWindowShell, personas []world.Persona) (result map[string]developmentSparseSituation, err error) {
	stateValue, _ := developmentTitleFirst.Load(r)
	state := stateValue.(*developmentTitleFirstState)
	if state.attempted {
		return state.result, state.err
	}
	state.attempted = true
	defer func() { state.result = result; state.err = err }()
	var m LLMMaterializer
	switch x := r.Materializer.(type) {
	case LLMMaterializer:
		m = x
	case *LLMMaterializer:
		m = *x
	default:
		return nil, fmt.Errorf("title-first requires LLMMaterializer")
	}
	planner, ok := m.Renderer.(llm.BBSTitleCandidatePlanner)
	if !ok {
		return nil, fmt.Errorf("renderer does not support title candidates")
	}
	eraValidator, ok := m.Renderer.(llm.BBSTitleEraValidator)
	if !ok {
		return nil, fmt.Errorf("renderer does not support title era validation")
	}
	facts := r.existingPersonaFactsByID(personas)
	boards := []world.Board{}
	events := map[string][]llm.BBSWorldWindowEvent{}
	canonicalSituations := map[string]developmentSparseSituation{}
	// Include already-canonical history plus lightweight pseudo roots as each
	// situation is chosen. This preserves the sparse selector's novelty behavior
	// across the same generated window before any title has been accepted.
	situationHistory := append([]world.Post(nil), state.history...)
	for _, item := range window {
		s := item.shell
		if s.action != "thread_start" || s.parentIndex != 0 || s.sourceIndex != 0 {
			continue
		}
		if len(events[item.board.ID]) == 0 {
			boards = append(boards, item.board)
		}
		fs := []string{}
		for _, f := range facts[s.persona.ID] {
			if f.MaterializedAt.IsZero() || !f.MaterializedAt.After(s.createdAt) {
				fs = append(fs, f.Key+"="+f.Value)
			}
		}
		situation := developmentSituationForShell(host, item.board, s, situationHistory, nil)
		canonicalSituations[item.eventID] = situation
		situationHistory = append(situationHistory, world.Post{
			BoardID:         item.board.ID,
			AuthorPersonaID: s.persona.ID,
			CreatedAt:       s.createdAt,
			Intent: world.PostIntent{
				SituationKind:    situation.kind,
				SituationSummary: situation.summary,
				SituationFacts:   append([]string(nil), situation.facts...),
			},
		})
		events[item.board.ID] = append(events[item.board.ID], llm.BBSWorldWindowEvent{
			EventID:          item.eventID,
			BoardID:          item.board.ID,
			BoardName:        item.board.Name,
			AuthorHandle:     s.persona.Handle,
			CreatedAt:        s.createdAt.Format(time.RFC3339),
			Action:           s.action,
			AnchorKey:        s.anchorKey,
			CauseKind:        s.causeKind,
			CauseSummary:     s.causeSummary,
			DiscourseMode:    s.discourseMode,
			PersonaProfile:   personaSummary(s.persona),
			ExistingFacts:    fs,
			SituationKind:    situation.kind,
			SituationSummary: situation.summary,
			SituationFacts:   append([]string(nil), situation.facts...),
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	out := map[string]developmentSparseSituation{}
	usage := GenerationUsage{}
	addUsage := func(u llm.TokenUsage) {
		usage = addDevelopmentGenerationUsage(usage, GenerationUsage{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, OutputTokens: u.OutputTokens, ReasoningTokens: u.ReasoningTokens, TotalTokens: u.TotalTokens, Model: u.Model})
	}
	defer func() { storeDevelopmentPlanningUsage(r, host.ID, "title-first", usage) }()
	for boardIndex, board := range boards {
		generationSituations := make([]llm.BBSTitleGenerationSituation, 0, len(events[board.ID]))
		for i, event := range events[board.ID] {
			generationSituations = append(generationSituations, llm.BBSTitleGenerationSituation{
				Index:   i + 1,
				Kind:    event.SituationKind,
				Summary: event.SituationSummary,
				Facts:   append([]string(nil), event.SituationFacts...),
			})
		}
		pool, err := planner.GenerateBBSTitleCandidates(ctx, llm.BBSTitleGenerationRequest{
			WorldDate:  r.WorldDate,
			BoardName:  board.Name,
			Situations: generationSituations,
		})
		if err != nil {
			return nil, err
		}
		addUsage(pool.Usage)
		offset := len(state.rows)
		for i, title := range pool.Titles {
			state.rows = append(state.rows, DevelopmentTitleCandidate{BoardID: board.ID, Candidate: i + 1, Original: title, Status: "unreviewed", Reason: "検査未完了"})
		}
		earliest, _ := time.Parse(time.RFC3339, events[board.ID][0].CreatedAt)
		asOf := earliest.Format("2006-01-02")
		eligibleTitles, originalCandidates, eraUsage, eraErr := r.developmentRouteTitleEra(ctx, board, asOf, pool, state, offset, eraValidator)
		addUsage(eraUsage)
		if eraErr != nil || len(eligibleTitles) == 0 {
			continue
		}
		prior := []world.Post{}
		for _, post := range state.history {
			if post.CreatedAt.Before(earliest) {
				prior = append(prior, post)
			}
		}
		boardsRemaining := len(boards) - boardIndex
		if err := r.developmentAssignTitleFirstBoard(ctx, host, board, asOf, eligibleTitles, originalCandidates, events[board.ID], canonicalSituations, planningBBSState(prior, 48), state, offset, boardsRemaining, planner, addUsage, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) developmentAssignTitleFirstBoard(ctx context.Context, host world.Host, board world.Board, asOf string, eligibleTitles []string, originalCandidates []int, boardEvents []llm.BBSWorldWindowEvent, canonicalSituations map[string]developmentSparseSituation, recentBBSState string, state *developmentTitleFirstState, offset int, boardsRemaining int, planner llm.BBSTitleCandidatePlanner, addUsage func(llm.TokenUsage), out map[string]developmentSparseSituation) error {
	remainingTitles := append([]string(nil), eligibleTitles...)
	remainingCandidates := append([]int(nil), originalCandidates...)
	remainingEvents := append([]llm.BBSWorldWindowEvent(nil), boardEvents...)
	eventByID := map[string]llm.BBSWorldWindowEvent{}
	for _, e := range boardEvents {
		eventByID[e.EventID] = e
	}
	acceptedCandidates := map[int]bool{}
	acceptedEvents := map[string]bool{}
	blockedCandidates := map[int]bool{}
	lastReasons := map[int]string{}
	researchAllowance := developmentTitleEraResearchAllowance(state.eraResearchUsed, boardsRemaining)
	boardResearchUsed := 0
	maxPasses := len(boardEvents) + 1
	for pass := 0; pass < maxPasses && len(remainingTitles) > 0 && len(remainingEvents) > 0; pass++ {
		req := llm.BBSTitleReviewRequest{BoardName: board.Name, Titles: remainingTitles, Events: remainingEvents, RecentBBSState: recentBBSState}
		review, err := planner.ReviewBBSTitleCandidates(ctx, req)
		addUsage(review.Usage)
		if err == nil {
			err = llm.ValidateBBSTitleReview(req, review)
		}
		if err != nil {
			for _, originalCandidate := range remainingCandidates {
				row := &state.rows[offset+originalCandidate-1]
				row.Status = "unreviewed"
				row.Reason = fmt.Sprintf("時代[%s]: %s / 人物割当検査失敗: %s", row.EraStatus, row.EraReason, err.Error())
			}
			return nil
		}
		selected := map[int]llm.BBSTitleDecision{}
		selectedOrder := make([]int, 0, len(review.Decisions))
		malformedThisPass := map[int]bool{}
		retry := false
		for _, d := range review.Decisions {
			if d.Candidate < 1 || d.Candidate > len(remainingCandidates) {
				return fmt.Errorf("invalid candidate index")
			}
			originalCandidate := remainingCandidates[d.Candidate-1]
			lastReasons[originalCandidate] = d.Reason
			if d.EventID == "" {
				if titleFirstReviewDecisionMalformed(d.Reason) {
					blockedCandidates[originalCandidate] = true
					malformedThisPass[originalCandidate] = true
					retry = true
				}
				continue
			}
			selected[originalCandidate] = d
			selectedOrder = append(selectedOrder, originalCandidate)
		}
		researchJobs := make([]developmentTitleEraResearchJob, 0)
		for _, originalCandidate := range selectedOrder {
			d := selected[originalCandidate]
			row := &state.rows[offset+originalCandidate-1]
			e := eventByID[d.EventID]
			row.EventID = d.EventID
			row.Author = e.AuthorHandle
			row.Subject = d.Subject
			if row.EraStatus != "research" {
				continue
			}
			if boardResearchUsed+len(researchJobs) >= researchAllowance || state.eraResearchUsed+len(researchJobs) >= developmentTitleEraResearchBudget {
				row.EraStatus = "unverified"
				row.EraReason = fmt.Sprintf("%s / 採用候補のWeb史料確認はrun最大%d件を板間で公平配分するため、この板の今回枠%d件を超えて未検証", row.EraReason, developmentTitleEraResearchBudget, researchAllowance)
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("時代検証未完了のため除外: %s / 人物仮割当: %s", row.EraReason, d.Reason)
				blockedCandidates[originalCandidate] = true
				retry = true
				continue
			}
			researchJobs = append(researchJobs, developmentTitleEraResearchJob{candidate: originalCandidate, title: d.Subject})
		}
		state.eraResearchUsed += len(researchJobs)
		boardResearchUsed += len(researchJobs)
		outcomes := r.developmentResearchTitleEraBatch(ctx, host, board, asOf, researchJobs)
		for originalCandidate, outcome := range outcomes {
			row := &state.rows[offset+originalCandidate-1]
			row.EraStatus = outcome.status
			row.EraReason = outcome.reason
			row.EraEvidence = outcome.evidence
		}
		for _, originalCandidate := range selectedOrder {
			d := selected[originalCandidate]
			row := &state.rows[offset+originalCandidate-1]
			if row.Status == "era_rejected" {
				continue
			}
			if _, ok := eventByID[d.EventID]; !ok {
				return fmt.Errorf("title assignment outside world slots")
			}
			switch row.EraStatus {
			case "ng":
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("Web史料検証で除外: %s / 人物仮割当: %s", row.EraReason, d.Reason)
				blockedCandidates[originalCandidate] = true
				retry = true
				continue
			case "unverified", "research":
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("時代検証未完了のため除外: %s / 人物仮割当: %s", row.EraReason, d.Reason)
				blockedCandidates[originalCandidate] = true
				retry = true
				continue
			case "ok", "verified":
			default:
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("不明な時代検証状態 %q / 人物仮割当: %s", row.EraStatus, d.Reason)
				blockedCandidates[originalCandidate] = true
				retry = true
				continue
			}
			if acceptedEvents[d.EventID] {
				return fmt.Errorf("duplicate title assignment")
			}
			canonical, ok := canonicalSituations[d.EventID]
			if !ok || strings.TrimSpace(canonical.kind) == "" {
				return fmt.Errorf("missing canonical world situation for %q", d.EventID)
			}
			acceptedCandidates[originalCandidate] = true
			acceptedEvents[d.EventID] = true
			row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, d.Reason)
			row.Status = "accepted"
			if d.Subject != row.Original {
				row.Status = "corrected"
			}
			mergedFacts := append([]string(nil), canonical.facts...)
			mergedFacts = append(mergedFacts,
				"title_first_subject="+d.Subject,
				"title_first_original="+row.Original,
				"title_first_review="+d.Reason,
				"title_first_summary="+d.Summary,
				"historical_check=title_era_"+row.EraStatus,
				"subject_contract=Keep the accepted title verbatim. Render only the canonical world situation and existing persona/BBS facts; do not invent a different event, possession, purchase, personal history or unsupported detail.",
			)
			out[d.EventID] = developmentSparseSituation{kind: canonical.kind, summary: canonical.summary, facts: mergedFacts}
		}
		for originalCandidate := range malformedThisPass {
			row := &state.rows[offset+originalCandidate-1]
			row.Status = "rejected"
			row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, lastReasons[originalCandidate])
		}
		if !retry {
			break
		}
		nextTitles := make([]string, 0, len(remainingTitles))
		nextCandidates := make([]int, 0, len(remainingCandidates))
		for i, originalCandidate := range remainingCandidates {
			if acceptedCandidates[originalCandidate] || blockedCandidates[originalCandidate] {
				continue
			}
			nextTitles = append(nextTitles, remainingTitles[i])
			nextCandidates = append(nextCandidates, originalCandidate)
		}
		nextEvents := make([]llm.BBSWorldWindowEvent, 0, len(remainingEvents))
		for _, e := range remainingEvents {
			if !acceptedEvents[e.EventID] {
				nextEvents = append(nextEvents, e)
			}
		}
		if len(nextTitles) == len(remainingTitles) && len(nextEvents) == len(remainingEvents) {
			break
		}
		remainingTitles = nextTitles
		remainingCandidates = nextCandidates
		remainingEvents = nextEvents
	}
	for _, originalCandidate := range originalCandidates {
		row := &state.rows[offset+originalCandidate-1]
		if row.Status == "accepted" || row.Status == "corrected" || row.Status == "era_rejected" {
			continue
		}
		if row.EraStatus == "research" {
			routingReason := row.EraReason
			row.EraStatus = "not_needed"
			row.EraReason = "人物割当に使われなかったためWeb史料確認なし / 振り分け理由: " + routingReason
		}
		reason := strings.TrimSpace(lastReasons[originalCandidate])
		if reason == "" {
			reason = "人物割当に採用されなかった"
		}
		row.Status = "rejected"
		row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, reason)
	}
	return nil
}
