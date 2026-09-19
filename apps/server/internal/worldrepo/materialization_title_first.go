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
	BoardID     string   `json:"board_id"`
	Candidate   int      `json:"candidate"`
	Original    string   `json:"original"`
	Subject     string   `json:"subject"`
	Author      string   `json:"author"`
	EventID     string   `json:"event_id"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason"`
	EraStatus   string   `json:"era_status,omitempty"`
	EraReason   string   `json:"era_reason,omitempty"`
	EraEvidence string   `json:"era_evidence,omitempty"`
	Details     []string `json:"details,omitempty"`
}
type DevelopmentTitleFirstTiming struct {
	CandidateGenerationMS    int64 `json:"candidate_generation_ms"`
	EraRoutingMS             int64 `json:"era_routing_ms"`
	AssignmentReviewMS       int64 `json:"assignment_review_ms"`
	EraResearchMS            int64 `json:"era_research_ms"`
	ArticleDetailMS          int64 `json:"article_detail_ms"`
	TitleEvaluationMS        int64 `json:"title_evaluation_ms"`
	TotalPlanningMS          int64 `json:"total_planning_ms"`
	CandidateGenerationCalls int   `json:"candidate_generation_calls"`
	EraRoutingCalls          int   `json:"era_routing_calls"`
	AssignmentReviewCalls    int   `json:"assignment_review_calls"`
	EraResearchBatches       int   `json:"era_research_batches"`
	ArticleDetailCalls       int   `json:"article_detail_calls"`
}

type developmentTitleFirstState struct {
	history         []world.Post
	attempted       bool
	result          map[string]developmentSparseSituation
	err             error
	rows            []DevelopmentTitleCandidate
	eraResearchUsed int
	timing          DevelopmentTitleFirstTiming
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

func (r *Repository) DevelopmentTitleFirstTiming() DevelopmentTitleFirstTiming {
	if v, ok := developmentTitleFirst.Load(r); ok {
		return v.(*developmentTitleFirstState).timing
	}
	return DevelopmentTitleFirstTiming{}
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
// first. Era routing is cheap; expensive historical research runs only after the
// persona/slot matcher has tentatively selected a candidate for an actual post.
func (r *Repository) developmentPlanTitleFirst(host world.Host, window []developmentWindowShell, personas []world.Persona) (result map[string]developmentSparseSituation, err error) {
	stateValue, _ := developmentTitleFirst.Load(r)
	state := stateValue.(*developmentTitleFirstState)
	if state.attempted {
		return state.result, state.err
	}
	state.attempted = true
	planningStarted := time.Now()
	defer func() {
		state.timing.TotalPlanningMS = time.Since(planningStarted).Milliseconds()
		state.timing.TitleEvaluationMS = state.timing.EraRoutingMS + state.timing.AssignmentReviewMS
		state.result = result
		state.err = err
	}()
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
	detailPlanner, ok := m.Renderer.(llm.BBSTitleArticleDetailPlanner)
	if !ok {
		return nil, fmt.Errorf("renderer does not support title article details")
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
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	out := map[string]developmentSparseSituation{}
	usage := GenerationUsage{}
	addUsage := func(u llm.TokenUsage) {
		usage = addDevelopmentGenerationUsage(usage, GenerationUsage{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, OutputTokens: u.OutputTokens, ReasoningTokens: u.ReasoningTokens, TotalTokens: u.TotalTokens, Model: u.Model})
	}
	defer func() { storeDevelopmentPlanningUsage(r, host.ID, "title-first", usage) }()
	for boardIndex, board := range boards {
		stageStarted := time.Now()
		pool, err := planner.GenerateBBSTitleCandidates(ctx, r.WorldDate, board.Name)
		state.timing.CandidateGenerationMS += time.Since(stageStarted).Milliseconds()
		state.timing.CandidateGenerationCalls++
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
		stageStarted = time.Now()
		eligibleTitles, originalCandidates, eraUsage, eraErr := r.developmentRouteTitleEra(ctx, board, asOf, pool, state, offset, eraValidator)
		state.timing.EraRoutingMS += time.Since(stageStarted).Milliseconds()
		state.timing.EraRoutingCalls++
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
		if err := r.developmentAssignTitleFirstBoard(ctx, host, board, asOf, eligibleTitles, originalCandidates, events[board.ID], planningBBSState(prior, 48), state, offset, boardsRemaining, planner, detailPlanner, addUsage, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) developmentAssignTitleFirstBoard(ctx context.Context, host world.Host, board world.Board, asOf string, eligibleTitles []string, originalCandidates []int, boardEvents []llm.BBSWorldWindowEvent, recentBBSState string, state *developmentTitleFirstState, offset int, boardsRemaining int, planner llm.BBSTitleCandidatePlanner, detailPlanner llm.BBSTitleArticleDetailPlanner, addUsage func(llm.TokenUsage), out map[string]developmentSparseSituation) error {
	remainingTitles := append([]string(nil), eligibleTitles...)
	remainingCandidates := append([]int(nil), originalCandidates...)
	remainingEvents := append([]llm.BBSWorldWindowEvent(nil), boardEvents...)
	eventByID := map[string]llm.BBSWorldWindowEvent{}
	for _, e := range boardEvents {
		eventByID[e.EventID] = e
	}
	acceptedCandidates := map[int]bool{}
	acceptedEvents := map[string]bool{}
	acceptedCandidateByEvent := map[string]int{}
	acceptedDetailSeeds := map[string]llm.BBSTitleArticleDetailSeed{}
	blockedCandidates := map[int]bool{}
	lastReasons := map[int]string{}
	researchAllowance := developmentTitleEraResearchAllowance(state.eraResearchUsed, boardsRemaining)
	boardResearchUsed := 0
	maxPasses := len(boardEvents) + 1
	for pass := 0; pass < maxPasses && len(remainingTitles) > 0 && len(remainingEvents) > 0; pass++ {
		req := llm.BBSTitleReviewRequest{BoardName: board.Name, Titles: remainingTitles, Events: remainingEvents, RecentBBSState: recentBBSState}
		stageStarted := time.Now()
		review, err := planner.ReviewBBSTitleCandidates(ctx, req)
		state.timing.AssignmentReviewMS += time.Since(stageStarted).Milliseconds()
		state.timing.AssignmentReviewCalls++
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
		stageStarted = time.Now()
		outcomes := r.developmentResearchTitleEraBatch(ctx, host, board, asOf, researchJobs)
		if len(researchJobs) > 0 {
			state.timing.EraResearchMS += time.Since(stageStarted).Milliseconds()
			state.timing.EraResearchBatches++
		}
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
			acceptedCandidates[originalCandidate] = true
			acceptedEvents[d.EventID] = true
			acceptedCandidateByEvent[d.EventID] = originalCandidate
			e := eventByID[d.EventID]
			acceptedDetailSeeds[d.EventID] = llm.BBSTitleArticleDetailSeed{
				EventID: d.EventID, Subject: d.Subject, Summary: d.Summary,
				AuthorHandle: e.AuthorHandle, CreatedAt: e.CreatedAt, DiscourseMode: e.DiscourseMode,
				PersonaProfile: e.PersonaProfile, ExistingFacts: append([]string(nil), e.ExistingFacts...),
			}
			row.Reason = fmt.Sprintf("時代[%s]: %s / 人物: %s", row.EraStatus, row.EraReason, d.Reason)
			row.Status = "accepted"
			if d.Subject != row.Original {
				row.Status = "corrected"
			}
			situationFacts := []string{
				"title_first_subject=" + d.Subject,
				"title_first_original=" + row.Original,
				"title_first_review=" + d.Reason,
				"world_adoption=title_candidate",
				"world_adopted_summary=" + d.Summary,
				"historical_check=title_era_" + row.EraStatus,
				"subject_contract=Keep the accepted title verbatim. The accepted title and world_adopted_summary are canonical world facts for this post. Article-local specifics will be added only by the post-adoption Article Detail Materializer.",
			}
			out[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: situationFacts}
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
	if len(acceptedDetailSeeds) > 0 && !developmentInteractiveTitleFirstEnabled(r) {
		seeds := make([]llm.BBSTitleArticleDetailSeed, 0, len(acceptedDetailSeeds))
		for _, event := range boardEvents {
			if seed, ok := acceptedDetailSeeds[event.EventID]; ok {
				seeds = append(seeds, seed)
			}
		}
		detailStarted := time.Now()
		detailDraft, detailErr := detailPlanner.MaterializeBBSTitleArticleDetails(ctx, llm.BBSTitleArticleDetailRequest{
			BoardName: board.Name, WorldDate: asOf, RecentBBSState: recentBBSState, Articles: seeds,
		})
		state.timing.ArticleDetailMS += time.Since(detailStarted).Milliseconds()
		state.timing.ArticleDetailCalls++
		addUsage(detailDraft.Usage)
		if detailErr != nil {
			for _, originalCandidate := range acceptedCandidateByEvent {
				row := &state.rows[offset+originalCandidate-1]
				// Detail enrichment is optional. Once title/persona/Era adoption has
				// succeeded, a secondary detail call must not erase the whole article.
				row.Reason += " / 記事detail具体化失敗（採用記事は保持）: " + detailErr.Error()
				row.Details = nil
			}
		} else {
			for _, article := range detailDraft.Articles {
				originalCandidate, ok := acceptedCandidateByEvent[article.EventID]
				if !ok {
					continue
				}
				row := &state.rows[offset+originalCandidate-1]
				situation := out[article.EventID]
				row.Details = nil
				for _, detail := range article.Details {
					encoded := strings.TrimSpace(detail.Kind) + ":" + strings.TrimSpace(detail.Fact)
					row.Details = append(row.Details, encoded)
					situation.facts = append(situation.facts, "article_detail="+encoded)
				}
				situation.facts = append(situation.facts,
					"article_detail_contract=The article_detail facts are canonical article-local specifics selected after title/persona/Era adoption. Materially express at least two distinct supplied details. A detail must add information beyond the title/summary; never collapse it back into vague wording. Do not add external historical/product/game facts, durable biography, or unexplained causes beyond canonical context.",
				)
				out[article.EventID] = situation
			}
		}
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
