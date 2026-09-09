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

// Only the isolated conversation Lab calls this. Candidate wording is generated
// first. Era routing is cheap; expensive historical research runs only after the
// persona/slot matcher has tentatively selected a candidate for an actual post.
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
		events[item.board.ID] = append(events[item.board.ID], llm.BBSWorldWindowEvent{EventID: item.eventID, BoardID: item.board.ID, BoardName: item.board.Name, AuthorHandle: s.persona.Handle, CreatedAt: s.createdAt.Format(time.RFC3339), Action: s.action, AnchorKey: s.anchorKey, CauseKind: s.causeKind, CauseSummary: s.causeSummary, DiscourseMode: s.discourseMode, PersonaProfile: personaSummary(s.persona), ExistingFacts: fs})
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
		// Minimal first pass deliberately receives no personas, slots or style rules.
		pool, err := planner.GenerateBBSTitleCandidates(ctx, r.WorldDate, board.Name)
		if err != nil {
			return nil, err
		}
		addUsage(pool.Usage)
		offset := len(state.rows)
		for i, title := range pool.Titles {
			state.rows = append(state.rows, DevelopmentTitleCandidate{BoardID: board.ID, Candidate: i + 1, Original: title, Status: "unreviewed", Reason: "検査未完了"})
		}

		// Use the earliest eligible root on the board. If a real referent existed by
		// this date it is safe for every later slot in the same generated window.
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
		req := llm.BBSTitleReviewRequest{BoardName: board.Name, Titles: eligibleTitles, Events: events[board.ID], RecentBBSState: planningBBSState(prior, 48)}
		review, err := planner.ReviewBBSTitleCandidates(ctx, req)
		addUsage(review.Usage)
		if err == nil {
			err = llm.ValidateBBSTitleReview(req, review)
		}
		if err != nil {
			for _, originalCandidate := range originalCandidates {
				row := &state.rows[offset+originalCandidate-1]
				row.Status = "unreviewed"
				row.Reason = fmt.Sprintf("時代[%s]: %s / 人物割当検査失敗: %s", row.EraStatus, row.EraReason, err.Error())
			}
			continue
		}

		// Only titles that both require historical evidence and were tentatively
		// selected for a real event slot spend Web research budget.
		boardsRemaining := len(boards) - boardIndex
		researchAllowance := developmentTitleEraResearchAllowance(state.eraResearchUsed, boardsRemaining)
		researchJobs := make([]developmentTitleEraResearchJob, 0)
		selectedResearch := map[int]bool{}
		for _, d := range review.Decisions {
			if d.Candidate < 1 || d.Candidate > len(eligibleTitles) || d.EventID == "" {
				continue
			}
			originalCandidate := originalCandidates[d.Candidate-1]
			row := &state.rows[offset+originalCandidate-1]
			if row.EraStatus != "research" {
				continue
			}
			if len(researchJobs) >= researchAllowance || state.eraResearchUsed+len(researchJobs) >= developmentTitleEraResearchBudget {
				row.EraStatus = "unverified"
				row.EraReason = fmt.Sprintf("%s / 採用候補のWeb史料確認はrun最大%d件を板間で公平配分するため、この板の今回枠%d件を超えて未検証", row.EraReason, developmentTitleEraResearchBudget, researchAllowance)
				continue
			}
			selectedResearch[originalCandidate] = true
			researchJobs = append(researchJobs, developmentTitleEraResearchJob{candidate: originalCandidate, title: d.Subject})
		}
		state.eraResearchUsed += len(researchJobs)
		outcomes := r.developmentResearchTitleEraBatch(ctx, host, board, asOf, researchJobs)
		for originalCandidate, outcome := range outcomes {
			row := &state.rows[offset+originalCandidate-1]
			row.EraStatus = outcome.status
			row.EraReason = outcome.reason
			row.EraEvidence = outcome.evidence
		}

		eventByID := map[string]llm.BBSWorldWindowEvent{}
		for _, e := range events[board.ID] {
			eventByID[e.EventID] = e
		}
		for _, d := range review.Decisions {
			if d.Candidate < 1 || d.Candidate > len(eligibleTitles) {
				return nil, fmt.Errorf("invalid candidate index")
			}
			originalCandidate := originalCandidates[d.Candidate-1]
			row := &state.rows[offset+originalCandidate-1]
			row.Status = "rejected"
			if d.EventID == "" {
				if row.EraStatus == "research" {
					routingReason := row.EraReason
					row.EraStatus = "not_needed"
					row.EraReason = "人物割当に使われなかったためWeb史料確認なし / 振り分け理由: " + routingReason
				}
				row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, d.Reason)
				continue
			}
			e, ok := eventByID[d.EventID]
			if !ok {
				return nil, fmt.Errorf("title assignment outside world slots")
			}
			row.EventID = d.EventID
			row.Author = e.AuthorHandle
			row.Subject = d.Subject

			if selectedResearch[originalCandidate] {
				// The batch outcome has replaced the temporary "research" state above.
			}
			switch row.EraStatus {
			case "ng":
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("Web史料検証で除外: %s / 人物仮割当: %s", row.EraReason, d.Reason)
				continue
			case "unverified", "research":
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("時代検証未完了のため除外: %s / 人物仮割当: %s", row.EraReason, d.Reason)
				continue
			case "ok", "verified":
				// eligible
			default:
				row.Status = "era_rejected"
				row.Reason = fmt.Sprintf("不明な時代検証状態 %q / 人物仮割当: %s", row.EraStatus, d.Reason)
				continue
			}

			if _, exists := out[d.EventID]; exists {
				return nil, fmt.Errorf("duplicate title assignment")
			}
			row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, d.Reason)
			row.Status = "accepted"
			if d.Subject != row.Original {
				row.Status = "corrected"
			}
			out[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: []string{"title_first_subject=" + d.Subject, "title_first_original=" + row.Original, "title_first_review=" + d.Reason, "historical_check=title_era_" + row.EraStatus, "subject_contract=Keep the accepted title verbatim. Write only its matter within this actor's existing facts. Do not invent new possessions, purchases, personal history or unsupported game/technical details."}}
		}
	}
	return out, nil
}
