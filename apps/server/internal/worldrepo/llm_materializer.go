package worldrepo

import (
	"context"
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// LLMMaterializer turns WorldEngine-selected facts into prose. It does not
// decide whether an event happens or whether research is needed; those remain
// WorldEngine/HistoricalKnowledge responsibilities.
type LLMMaterializer struct {
	Renderer llm.BoardPostRenderer
	Fallback Materializer
}

func (m LLMMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, error) {
	posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
	return posts, err
}

// GenerateBoardPostsWithUsage is used only by development diagnostics. Token
// usage is operational metadata, not world state, so the normal Materializer
// interface deliberately does not require or persist it.
func (m LLMMaterializer) GenerateBoardPostsWithUsage(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, GenerationUsage, error) {
	if decision.Level == historicalkb.EvidenceVerified && !decision.Knowledge.CanUse {
		return nil, GenerationUsage{}, fmt.Errorf("verified historical knowledge unavailable")
	}
	if m.Renderer == nil {
		posts, err := m.fallback(ctx, req, decision, fmt.Errorf("LLM board post renderer is not configured"))
		return posts, GenerationUsage{}, err
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
		EraRules:         "世界時刻より未来の知識を使わない。具体的な歴史事実は supplied historical facts の範囲に限定する。局固有の架空設定と史実を混同しない。",
		AuthorHandle:     author,
		PersonaProfile:   personaProfile,
		PostIntent:       intentSummary(req.Intent),
		CanonicalSubject: req.CanonicalSubject,
	})
	if err != nil {
		posts, fallbackErr := m.fallback(ctx, req, decision, err)
		return posts, GenerationUsage{}, fallbackErr
	}

	// Actor and subject are world facts when already selected by WorldRepository.
	// Never let a prose renderer silently replace them.
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

func (m LLMMaterializer) fallback(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision, cause error) ([]world.Post, error) {
	// Verified materialization must not silently degrade into prose unsupported by
	// required evidence. For atmospheric/plausible content, deterministic fallback
	// keeps the world usable when the model endpoint is temporarily unavailable.
	if decision.Level == historicalkb.EvidenceVerified {
		return nil, cause
	}
	if m.Fallback == nil {
		return nil, cause
	}
	return m.Fallback.GenerateBoardPosts(ctx, req, decision)
}

func personaSummary(p world.Persona) string {
	return fmt.Sprintf("age=%d; occupation=%s; activity=%s; reply=%.2f; thread_start=%.2f; lurker=%.2f; newcomer_open=%.2f; argumentative=%.2f; writing=%s",
		p.Age, p.Occupation, p.ActivityPattern, p.ReplyTendency, p.ThreadStartTendency, p.LurkerTendency, p.NewcomerOpenness, p.Argumentativeness, p.WritingStyle)
}

func intentSummary(i world.PostIntent) string {
	parts := make([]string, 0, 12)
	if i.Action != "" {
		parts = append(parts, "action="+i.Action)
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
	if i.ResponseAct != "" {
		parts = append(parts, "response_act="+i.ResponseAct)
	}
	if len(i.InformationSlots) > 0 {
		parts = append(parts, "information_slots="+strings.Join(i.InformationSlots, ","))
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
	if i.RespondsToQuestion != "" {
		parts = append(parts, "responds_to_question="+i.RespondsToQuestion)
	}
	if i.FollowUpSlot != "" {
		parts = append(parts, "follow_up_slot="+i.FollowUpSlot)
	}
	if i.FollowUpQuestion != "" {
		parts = append(parts, "follow_up_question="+i.FollowUpQuestion)
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
