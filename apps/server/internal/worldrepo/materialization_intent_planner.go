package worldrepo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type developmentTimelineShell struct {
	index       int
	persona     world.Persona
	createdAt   time.Time
	action      string
	parentIndex int
}

type developmentTimelinePlanEvent struct {
	index      int
	subject    string
	topic      string
	motivation string
	stance     string
	goal       string
	facts      []llm.BBSIntentFactDraft
}

type developmentTimelinePlan struct {
	events []developmentTimelinePlanEvent
	usage  GenerationUsage
}

type developmentTimelinePlanner interface {
	PlanDevelopmentTimeline(context.Context, world.Host, world.Board, string, []developmentTimelineShell, map[string][]world.PersonaFact, string) (developmentTimelinePlan, error)
}

// PlanDevelopmentTimeline delegates free-form semantic proposal to a renderer
// that explicitly implements BBSTimelineIntentPlanner. WorldRepository keeps
// ownership of actors, times and reply topology; the planner has no fixed topic
// catalog, subject bank, information slots or response-act enum to choose from.
func (m LLMMaterializer) PlanDevelopmentTimeline(ctx context.Context, host world.Host, board world.Board, worldDate string, shells []developmentTimelineShell, factsByPersona map[string][]world.PersonaFact, recentBBS string) (developmentTimelinePlan, error) {
	planner, ok := m.Renderer.(llm.BBSTimelineIntentPlanner)
	if !ok {
		return developmentTimelinePlan{}, errors.New("configured renderer does not implement BBS timeline intent planning")
	}
	events := make([]llm.BBSIntentEvent, 0, len(shells))
	for _, shell := range shells {
		existingFacts := make([]string, 0, len(factsByPersona[shell.persona.ID]))
		for _, fact := range factsByPersona[shell.persona.ID] {
			existingFacts = append(existingFacts, fact.Key+"="+fact.Value)
		}
		events = append(events, llm.BBSIntentEvent{
			Index:             shell.index,
			AuthorHandle:      shell.persona.Handle,
			CreatedAt:         shell.createdAt.Format(time.RFC3339),
			Action:            shell.action,
			ParentEventIndex:  shell.parentIndex,
			PersonaProfile:    personaSummary(shell.persona),
			ExistingFacts:     existingFacts,
		})
	}
	draft, err := planner.GenerateBBSTimelineIntent(ctx, llm.BBSTimelineIntentRequest{
		HostName:       host.Name,
		HostRegion:     host.Region,
		HostSoftware:   host.Software,
		BoardID:        board.ID,
		BoardName:      board.Name,
		WorldDate:      worldDate,
		EraRules:       "世界時刻より未来の知識を使わない。外部世界の具体的な歴史事実・製品仕様は根拠なしに確定しない。架空住人の個人的事実と史実を区別する。",
		RecentBBSState: recentBBS,
		Events:         events,
	})
	if err != nil {
		return developmentTimelinePlan{}, err
	}
	planned := make([]developmentTimelinePlanEvent, 0, len(draft.Events))
	for _, event := range draft.Events {
		planned = append(planned, developmentTimelinePlanEvent{
			index:      event.Index,
			subject:    strings.TrimSpace(event.Subject),
			topic:      strings.TrimSpace(event.Topic),
			motivation: strings.TrimSpace(event.Motivation),
			stance:     strings.TrimSpace(event.Stance),
			goal:       strings.TrimSpace(event.Goal),
			facts:      event.Facts,
		})
	}
	sort.SliceStable(planned, func(i, j int) bool { return planned[i].index < planned[j].index })
	return developmentTimelinePlan{
		events: planned,
		usage: GenerationUsage{
			InputTokens:       draft.Usage.InputTokens,
			CachedInputTokens: draft.Usage.CachedInputTokens,
			OutputTokens:      draft.Usage.OutputTokens,
			ReasoningTokens:   draft.Usage.ReasoningTokens,
			TotalTokens:       draft.Usage.TotalTokens,
			Model:             draft.Usage.Model,
		},
	}, nil
}

func (r *Repository) existingPersonaFactsByID(personas []world.Persona) map[string][]world.PersonaFact {
	out := map[string][]world.PersonaFact{}
	store, ok := r.Base.(world.PersonaFactStore)
	if !ok {
		return out
	}
	for _, persona := range personas {
		out[persona.ID] = store.ListPersonaFacts(persona.ID)
	}
	return out
}

// commitPlannedFacts accepts generic semantic keys proposed for this concrete
// event. Existing world facts always win. This removes the old predefined fact
// slot schema while preserving the important invariant that an observed durable
// fact cannot silently change later.
func (r *Repository) commitPlannedFacts(persona world.Persona, topic string, proposed []llm.BBSIntentFactDraft, at time.Time) []string {
	store, ok := r.Base.(world.PersonaFactStore)
	if !ok || len(proposed) == 0 {
		return nil
	}
	existing := map[string]world.PersonaFact{}
	for _, fact := range store.ListPersonaFacts(persona.ID) {
		existing[fact.Key] = fact
	}
	claims := make([]string, 0, len(proposed))
	seen := map[string]bool{}
	for _, draft := range proposed {
		key := strings.TrimSpace(strings.ToLower(draft.Key))
		value := strings.TrimSpace(draft.Value)
		if key == "" || value == "" || seen[key] {
			continue
		}
		seen[key] = true
		if fact, found := existing[key]; found {
			claims = append(claims, fact.Value)
			continue
		}
		fact := world.PersonaFact{
			PersonaID:      persona.ID,
			Key:            key,
			Topic:          topic,
			Value:          value,
			MaterializedAt: at,
			SourceKind:     "validated_semantic_proposal",
		}
		store.SavePersonaFact(fact)
		existing[key] = fact
		claims = append(claims, fact.Value)
	}
	return claims
}

func planningBBSState(posts []world.Post, limit int) string {
	if len(posts) == 0 {
		return ""
	}
	ordered := append([]world.Post(nil), posts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[len(ordered)-limit:]
	}
	var b strings.Builder
	for _, post := range ordered {
		fmt.Fprintf(&b, "MSG %04d %s %s: %s", post.ID, post.CreatedAt.Format("01/02 15:04"), post.Author, post.Subject)
		if post.Intent.Topic != "" {
			fmt.Fprintf(&b, " | topic=%s", post.Intent.Topic)
		}
		if post.Intent.Goal != "" {
			fmt.Fprintf(&b, " | goal=%s", post.Intent.Goal)
		}
		if len(post.Intent.Claims) > 0 {
			fmt.Fprintf(&b, " | claims=%s", strings.Join(post.Intent.Claims, " / "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
