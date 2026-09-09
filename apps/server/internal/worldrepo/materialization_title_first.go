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
	BoardID   string `json:"board_id"`
	Candidate int    `json:"candidate"`
	Original  string `json:"original"`
	Subject   string `json:"subject"`
	Author    string `json:"author"`
	EventID   string `json:"event_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
type developmentTitleFirstState struct {
	history   []world.Post
	attempted bool
	result    map[string]developmentSparseSituation
	err       error
	rows      []DevelopmentTitleCandidate
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

// Only the isolated conversation Lab calls this. Candidate assignment is proposed
// against immutable eligible slots and checked before anything is committed.
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
	for _, board := range boards {
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
		earliest, _ := time.Parse(time.RFC3339, events[board.ID][0].CreatedAt)
		prior := []world.Post{}
		for _, post := range state.history {
			if post.CreatedAt.Before(earliest) {
				prior = append(prior, post)
			}
		}
		req := llm.BBSTitleReviewRequest{BoardName: board.Name, Titles: pool.Titles, Events: events[board.ID], RecentBBSState: planningBBSState(prior, 48)}
		review, err := planner.ReviewBBSTitleCandidates(ctx, req)
		if err != nil {
			return nil, err
		}
		addUsage(review.Usage)
		if err := llm.ValidateBBSTitleReview(req, review); err != nil {
			return nil, err
		}
		eventByID := map[string]llm.BBSWorldWindowEvent{}
		for _, e := range events[board.ID] {
			eventByID[e.EventID] = e
		}
		for _, d := range review.Decisions {
			if d.Candidate < 1 || d.Candidate > len(pool.Titles) {
				return nil, fmt.Errorf("invalid candidate index")
			}
			row := &state.rows[offset+d.Candidate-1]
			row.Reason = d.Reason
			row.Status = "rejected"
			if d.EventID == "" {
				continue
			}
			e, ok := eventByID[d.EventID]
			if !ok {
				return nil, fmt.Errorf("title assignment outside world slots")
			}
			if _, exists := out[d.EventID]; exists {
				return nil, fmt.Errorf("duplicate title assignment")
			}
			row.EventID = d.EventID
			row.Author = e.AuthorHandle
			row.Subject = d.Subject
			row.Status = "accepted"
			if d.Subject != row.Original {
				row.Status = "corrected"
			}
			out[d.EventID] = developmentSparseSituation{kind: "title_first", summary: d.Summary, facts: []string{"title_first_subject=" + d.Subject, "title_first_original=" + row.Original, "title_first_review=" + d.Reason, "historical_check=model_memory_provisional_not_source_verified", "subject_contract=Keep the accepted title verbatim. Write only its matter within this actor's existing facts. Do not invent new possessions, purchases, personal history or unsupported game/technical details."}}
		}
	}
	return out, nil
}
