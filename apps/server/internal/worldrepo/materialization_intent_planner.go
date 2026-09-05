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

const developmentPlanningBatchSize = 6

type developmentTimelineShell struct {
	index        int
	persona      world.Persona
	createdAt    time.Time
	action       string
	parentIndex  int
	anchorKey    string
	causeKind    string
	causeSummary string
	sourceIndex  int
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

// PlanDevelopmentTimeline realizes free-form semantics for events whose causal
// existence has already been selected by the world layer. Actor, time,
// root-vs-reply topology, causal anchor, and cause kind are fixed before the LLM
// is called. Persona facts are supplied only as contradiction guards/background;
// their mere presence must never be interpreted as a reason to post about them.
//
// A full two-week board can still contain multiple selected write events. We
// realize them in bounded chronological batches for latency/cost control. Earlier
// accepted semantics and newly proposed persona facts are available as transient
// consistency context, but nothing is committed until every batch succeeds.
func (m LLMMaterializer) PlanDevelopmentTimeline(ctx context.Context, host world.Host, board world.Board, worldDate string, shells []developmentTimelineShell, factsByPersona map[string][]world.PersonaFact, recentBBS string) (developmentTimelinePlan, error) {
	planner, ok := m.Renderer.(llm.BBSTimelineIntentPlanner)
	if !ok {
		return developmentTimelinePlan{}, errors.New("configured renderer does not implement BBS timeline intent planning")
	}
	if len(shells) == 0 {
		return developmentTimelinePlan{}, nil
	}

	workingFacts := clonePersonaFactsByID(factsByPersona)
	shellByIndex := make(map[int]developmentTimelineShell, len(shells))
	for _, shell := range shells {
		shellByIndex[shell.index] = shell
	}

	planned := make([]developmentTimelinePlanEvent, 0, len(shells))
	usage := GenerationUsage{}
	for start := 0; start < len(shells); start += developmentPlanningBatchSize {
		end := start + developmentPlanningBatchSize
		if end > len(shells) {
			end = len(shells)
		}
		batchShells := shells[start:end]
		events := make([]llm.BBSIntentEvent, 0, len(batchShells))
		for _, shell := range batchShells {
			existingFacts := make([]string, 0, len(workingFacts[shell.persona.ID]))
			for _, fact := range workingFacts[shell.persona.ID] {
				existingFacts = append(existingFacts, "BACKGROUND ONLY: "+fact.Key+"="+fact.Value)
			}
			events = append(events, llm.BBSIntentEvent{
				Index:            shell.index,
				AuthorHandle:     shell.persona.Handle,
				CreatedAt:        shell.createdAt.Format(time.RFC3339),
				Action:           shell.action,
				ParentEventIndex: shell.parentIndex,
				SourceEventIndex: shell.sourceIndex,
				AnchorKey:        shell.anchorKey,
				CauseKind:        shell.causeKind,
				CauseSummary:     shell.causeSummary,
				PersonaProfile:   personaSummary(shell.persona),
				ExistingFacts:    existingFacts,
			})
		}

		draft, err := planner.GenerateBBSTimelineIntent(ctx, llm.BBSTimelineIntentRequest{
			HostName:       host.Name,
			HostRegion:     host.Region,
			HostSoftware:   host.Software,
			BoardID:        board.ID,
			BoardName:      board.Name,
			WorldDate:      worldDate,
			EraRules:       "世界時刻より未来の知識を使わない。外部世界の具体的な歴史事実・製品仕様は根拠なしに確定しない。架空住人の個人的事実と史実を区別する。\n" + llm.DiegeticWorldFrame,
			RecentBBSState: planningTimelineContext(recentBBS, planned, shellByIndex, 12),
			Events:         events,
		})
		if err != nil {
			return developmentTimelinePlan{}, fmt.Errorf("planning batch %d-%d: %w", start+1, end, err)
		}
		if len(draft.Events) != len(batchShells) {
			return developmentTimelinePlan{}, fmt.Errorf("planning batch %d-%d returned %d events, want %d", start+1, end, len(draft.Events), len(batchShells))
		}

		batchPlanned := make([]developmentTimelinePlanEvent, 0, len(draft.Events))
		for _, event := range draft.Events {
			semantic := developmentTimelinePlanEvent{
				index:      event.Index,
				subject:    strings.TrimSpace(event.Subject),
				topic:      strings.TrimSpace(event.Topic),
				motivation: strings.TrimSpace(event.Motivation),
				stance:     strings.TrimSpace(event.Stance),
				goal:       strings.TrimSpace(event.Goal),
				facts:      event.Facts,
			}
			batchPlanned = append(batchPlanned, semantic)
			if shell, found := shellByIndex[event.Index]; found {
				addTransientPlannedFacts(workingFacts, shell.persona.ID, semantic)
			}
		}
		sort.SliceStable(batchPlanned, func(i, j int) bool { return batchPlanned[i].index < batchPlanned[j].index })
		planned = append(planned, batchPlanned...)
		addPlanningUsage(&usage, draft.Usage)
	}

	sort.SliceStable(planned, func(i, j int) bool { return planned[i].index < planned[j].index })
	return developmentTimelinePlan{events: planned, usage: usage}, nil
}

func clonePersonaFactsByID(src map[string][]world.PersonaFact) map[string][]world.PersonaFact {
	out := make(map[string][]world.PersonaFact, len(src))
	for personaID, facts := range src {
		out[personaID] = append([]world.PersonaFact(nil), facts...)
	}
	return out
}

func addTransientPlannedFacts(factsByPersona map[string][]world.PersonaFact, personaID string, event developmentTimelinePlanEvent) {
	existing := make(map[string]bool, len(factsByPersona[personaID]))
	for _, fact := range factsByPersona[personaID] {
		existing[strings.TrimSpace(strings.ToLower(fact.Key))] = true
	}
	for _, draft := range event.facts {
		key := strings.TrimSpace(strings.ToLower(draft.Key))
		value := strings.TrimSpace(draft.Value)
		if key == "" || value == "" || existing[key] {
			continue
		}
		factsByPersona[personaID] = append(factsByPersona[personaID], world.PersonaFact{
			PersonaID:  personaID,
			Key:        key,
			Topic:      event.topic,
			Value:      value,
			SourceKind: "transient_semantic_proposal",
		})
		existing[key] = true
	}
}

func planningTimelineContext(recentBBS string, planned []developmentTimelinePlanEvent, shellByIndex map[int]developmentTimelineShell, limit int) string {
	var b strings.Builder
	if strings.TrimSpace(recentBBS) != "" {
		b.WriteString(strings.TrimSpace(recentBBS))
		b.WriteByte('\n')
	}
	if len(planned) == 0 {
		return strings.TrimSpace(b.String())
	}
	start := 0
	if limit > 0 && len(planned) > limit {
		start = len(planned) - limit
	}
	b.WriteString("PLANNED EARLIER EVENTS IN THIS SAME TIMELINE:\n")
	for _, event := range planned[start:] {
		shell := shellByIndex[event.index]
		fmt.Fprintf(&b, "EVENT %04d %s %s action=%s", event.index, shell.createdAt.Format("01/02 15:04"), shell.persona.Handle, shell.action)
		if shell.parentIndex != 0 {
			fmt.Fprintf(&b, " parent=%04d", shell.parentIndex)
		}
		if shell.sourceIndex != 0 && shell.sourceIndex != shell.parentIndex {
			fmt.Fprintf(&b, " source=%04d", shell.sourceIndex)
		}
		if shell.anchorKey != "" {
			fmt.Fprintf(&b, " routing_domain=%s", shell.anchorKey)
		}
		if shell.causeKind != "" {
			fmt.Fprintf(&b, " cause=%s", shell.causeKind)
		}
		if event.subject != "" {
			fmt.Fprintf(&b, " subject=%s", event.subject)
		}
		if event.topic != "" {
			fmt.Fprintf(&b, " | topic=%s", event.topic)
		}
		if event.goal != "" {
			fmt.Fprintf(&b, " | goal=%s", event.goal)
		}
		if len(event.facts) > 0 {
			parts := make([]string, 0, len(event.facts))
			for _, fact := range event.facts {
				if strings.TrimSpace(fact.Key) != "" && strings.TrimSpace(fact.Value) != "" {
					parts = append(parts, strings.TrimSpace(fact.Key)+"="+strings.TrimSpace(fact.Value))
				}
			}
			if len(parts) > 0 {
				fmt.Fprintf(&b, " | facts=%s", strings.Join(parts, " / "))
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func addPlanningUsage(total *GenerationUsage, usage llm.TokenUsage) {
	total.InputTokens += usage.InputTokens
	total.CachedInputTokens += usage.CachedInputTokens
	total.OutputTokens += usage.OutputTokens
	total.ReasoningTokens += usage.ReasoningTokens
	total.TotalTokens += usage.TotalTokens
	if usage.Model == "" {
		return
	}
	if total.Model == "" {
		total.Model = usage.Model
	} else if total.Model != usage.Model {
		total.Model = "mixed"
	}
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

// commitPlannedFacts accepts generic semantic keys that genuinely became
// necessary while realizing this already-causal event. Existing world facts
// always win. Persona facts are persistence/consistency state, never future topic
// suggestions.
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
		if post.Intent.AnchorKey != "" {
			fmt.Fprintf(&b, " | routing_domain=%s", post.Intent.AnchorKey)
		}
		if post.Intent.CauseKind != "" {
			fmt.Fprintf(&b, " | cause=%s", post.Intent.CauseKind)
		}
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
