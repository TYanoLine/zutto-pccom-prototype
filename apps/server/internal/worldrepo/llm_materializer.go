package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// LLMMaterializer turns world-selected, causally anchored semantic state into
// prose. Fallback remains as an inert compatibility field for older wiring; it
// is never called. Renderer failure leaves the article unmaterialized so a later
// observation can retry rather than committing canned prose.
type LLMMaterializer struct {
	Renderer llm.BoardPostRenderer
	Fallback Materializer
}

func (m LLMMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, error) {
	posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
	return posts, err
}

func (m LLMMaterializer) GenerateBoardPostsWithUsage(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, GenerationUsage, error) {
	if decision.Level == historicalkb.EvidenceVerified && !decision.Knowledge.CanUse {
		return nil, GenerationUsage{}, fmt.Errorf("verified historical knowledge unavailable")
	}
	if m.Renderer == nil {
		return nil, GenerationUsage{}, fmt.Errorf("LLM board post renderer is not configured")
	}

	facts := usableClaims(decision)
	author := ""
	personaProfile := ""
	if req.Persona != nil {
		author = req.Persona.Handle
		personaProfile = personaSummary(*req.Persona)
	}
	draft, err := m.Renderer.GenerateBoardPost(ctx, llm.BoardPostRequest{
		HostName:         req.Host.Name,
		HostRegion:       req.Host.Region,
		HostSoftware:     req.Host.Software,
		BoardID:          req.BoardID,
		BoardTopic:       req.BoardTopic,
		WorldDate:        req.WorldDate,
		HistoricalFacts:  facts,
		EraRules:         "世界時刻より未来の知識を使わない。具体的な歴史事実は supplied historical facts の範囲に限定する。局固有の架空設定と史実を混同しない。\n" + llm.DiegeticWorldFrame,
		AuthorHandle:     author,
		PersonaProfile:   personaProfile,
		PostIntent:       intentSummary(req.Intent),
		CanonicalSubject: req.CanonicalSubject,
	})
	if err != nil {
		return nil, GenerationUsage{}, fmt.Errorf("board post renderer failed: %w", err)
	}
	if req.Persona != nil && req.Persona.Handle != "" {
		draft.Author = req.Persona.Handle
	}
	if strings.TrimSpace(req.CanonicalSubject) != "" {
		draft.Subject = req.CanonicalSubject
	}
	usage := GenerationUsage{
		InputTokens:       draft.Usage.InputTokens,
		CachedInputTokens: draft.Usage.CachedInputTokens,
		OutputTokens:      draft.Usage.OutputTokens,
		ReasoningTokens:   draft.Usage.ReasoningTokens,
		TotalTokens:       draft.Usage.TotalTokens,
		Model:             draft.Usage.Model,
	}
	return []world.Post{{Author: draft.Author, Subject: draft.Subject, Body: draft.Body, CreatedAt: worldTime(req.WorldDate)}}, usage, nil
}

func personaSummary(p world.Persona) string {
	interestKeys := make([]string, 0, len(p.Interests))
	for key := range p.Interests {
		interestKeys = append(interestKeys, key)
	}
	sort.Strings(interestKeys)
	interests := make([]string, 0, len(interestKeys))
	for _, key := range interestKeys {
		interests = append(interests, fmt.Sprintf("%s=%.2f", key, p.Interests[key]))
	}
	opinionKeys := make([]string, 0, len(p.Opinions))
	for key := range p.Opinions {
		opinionKeys = append(opinionKeys, key)
	}
	sort.Strings(opinionKeys)
	opinions := make([]string, 0, len(opinionKeys))
	for _, key := range opinionKeys {
		opinions = append(opinions, fmt.Sprintf("%s=%.2f", key, p.Opinions[key]))
	}
	baseline := "(none supplied)"
	if len(p.EverydayContext) > 0 {
		baseline = strings.Join(p.EverydayContext, " / ")
	}
	return fmt.Sprintf("age=%d; gender=%s; occupation=%s; activity=%s; reply=%.2f; thread_start=%.2f; lurker=%.2f; newcomer_open=%.2f; argumentative=%.2f; writing=%s; everyday_baseline=[%s]; interests=[%s]; opinions=[%s]; IMPORTANT: everyday_baseline is ordinary already-established context, normally unspoken and never a novelty/topic by itself",
		p.Age, p.Gender, p.Occupation, p.ActivityPattern, p.ReplyTendency, p.ThreadStartTendency, p.LurkerTendency, p.NewcomerOpenness, p.Argumentativeness, p.WritingStyle, baseline, strings.Join(interests, ","), strings.Join(opinions, ","))
}

func intentSummary(i world.PostIntent) string {
	parts := make([]string, 0, 24)
	if i.Action != "" {
		parts = append(parts, "action="+i.Action)
	}
	if i.AnchorKey != "" {
		parts = append(parts, "internal_routing_domain="+i.AnchorKey)
	}
	if i.CauseKind != "" {
		parts = append(parts, "world_cause="+i.CauseKind)
	}
	if i.DiscourseMode != "" {
		parts = append(parts, "discourse_mode="+i.DiscourseMode)
	}
	if i.SourcePostID != 0 {
		parts = append(parts, fmt.Sprintf("source_post_id=%d", i.SourcePostID))
	}
	if i.Topic != "" {
		parts = append(parts, "topic="+i.Topic)
	}
	if i.Motivation != "" {
		parts = append(parts, "motivation="+i.Motivation)
	}
	if i.Stance != "" {
		parts = append(parts, "stance="+i.Stance)
	}
	if i.Goal != "" {
		parts = append(parts, "goal="+i.Goal)
	}
	if len(i.Claims) > 0 {
		parts = append(parts, "claims="+strings.Join(i.Claims, " / "))
	}
	if i.RespondsToPostID != 0 {
		parts = append(parts, fmt.Sprintf("responds_to_post_id=%d", i.RespondsToPostID))
	}
	if len(i.RespondsToClaims) > 0 {
		parts = append(parts, "responds_to_claims="+strings.Join(i.RespondsToClaims, " / "))
	}

	// Producer fields are canonical editorial instructions. The article renderer
	// is deliberately a worker: it may choose wording, line breaks and period-native
	// conversational texture, but it must not replace these facts with a different
	// event or invent missing story state.
	if i.ProducerEventID != "" {
		parts = append(parts, "producer_event_id="+i.ProducerEventID)
	}
	if i.ProducerEpisode != "" {
		parts = append(parts, "producer_episode="+i.ProducerEpisode)
	}
	if len(i.ProducerReferents) > 0 {
		parts = append(parts, "producer_referents="+strings.Join(i.ProducerReferents, " / "))
	}
	if len(i.ProducerActorKnowledge) > 0 {
		parts = append(parts, "producer_actor_knowledge="+strings.Join(i.ProducerActorKnowledge, " / "))
	}
	if len(i.ProducerAudienceContext) > 0 {
		parts = append(parts, "producer_audience_context="+strings.Join(i.ProducerAudienceContext, " / "))
	}
	if len(i.ProducerContribution) > 0 {
		parts = append(parts, "producer_required_contribution="+strings.Join(i.ProducerContribution, " / "))
	}
	if len(i.ProducerMustNot) > 0 {
		parts = append(parts, "producer_must_not="+strings.Join(i.ProducerMustNot, " / "))
	}
	if strings.TrimSpace(i.RenderContext) != "" {
		parts = append(parts, "bbs_context:\n"+strings.TrimSpace(i.RenderContext))
	}
	return strings.Join(parts, "; ")
}

func usableClaims(decision worldengine.EvidenceDecision) []string {
	if !decision.Knowledge.CanUse {
		return nil
	}
	out := make([]string, 0, len(decision.Knowledge.Facts))
	for _, f := range decision.Knowledge.Facts {
		if f.Status == historicalkb.FactRejected || strings.TrimSpace(f.Claim) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(f.Claim))
	}
	return out
}
