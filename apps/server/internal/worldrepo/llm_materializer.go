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
	Renderer                    llm.BoardPostRenderer
	Fallback                    Materializer
	HistoricalReferencesEnabled bool
	HistoricalTexture           []string
	CuratedHistoricalReferences bool
	// ModelHistoricalMemory is a fresh-Lab experiment: allow model knowledge without a referent dictionary.
	ModelHistoricalMemory bool
	// PreferConcreteHistoricalNames keeps model-memory dictionary-free while preferring a known real name over a generic label when it naturally fits.
	PreferConcreteHistoricalNames bool
	// SearchGroundedHistoricalReferences is a fresh-Lab experiment: real names may enter only through searched canonical Situation evidence.
	SearchGroundedHistoricalReferences bool
}

func (m LLMMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, error) {
	posts, _, err := m.GenerateBoardPostsWithUsage(ctx, req, decision)
	return posts, err
}

func (m LLMMaterializer) GenerateBoardPostsWithUsage(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post, GenerationUsage, error) {
	if m.HistoricalReferencesEnabled && decision.Level == historicalkb.EvidenceVerified && !decision.Knowledge.CanUse {
		return nil, GenerationUsage{}, fmt.Errorf("verified historical knowledge unavailable")
	}
	if m.Renderer == nil {
		return nil, GenerationUsage{}, fmt.Errorf("LLM board post renderer is not configured")
	}

	m = m.withPeriodReferents(req.WorldDate)
	facts := m.historicalFacts(decision)
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
		EraRules:         m.eraRules(),
		AuthorHandle:     author,
		PersonaProfile:   personaProfile,
		PostIntent:       intentSummary(req.Intent),
		CanonicalSubject: req.CanonicalSubject,
		Kind:             string(req.Kind),
		ParentSubject: func() string {
			if req.ParentPost != nil {
				return req.ParentPost.Subject
			}
			return ""
		}(),
		ParentBody: func() string {
			if req.ParentPost != nil {
				return req.ParentPost.Body
			}
			return ""
		}(),
		QuoteText: req.QuoteText, BodyMinChars: req.BodyMinChars, BodyMaxChars: req.BodyMaxChars,
	})
	if err != nil {
		return nil, GenerationUsage{}, fmt.Errorf("board post renderer failed: %w", err)
	}
	if req.Persona != nil && req.Persona.Handle != "" {
		draft.Author = req.Persona.Handle
	}
	if strings.TrimSpace(req.CanonicalSubject) != "" {
		// CanonicalSubject is already world-selected. The prose renderer may write
		// the body naturally, but it must never silently rename an accepted thread.
		draft.Subject = req.CanonicalSubject
	}
	if req.QuoteText != "" {
		draft.Body, err = llm.EnsureExactQuote(draft.Body, req.QuoteText)
		if err != nil {
			return nil, GenerationUsage{}, fmt.Errorf("quote validation failed: %w", err)
		}
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

func (m LLMMaterializer) historicalFacts(decision worldengine.EvidenceDecision) []string {
	out := make([]string, 0, len(m.HistoricalTexture)+4)
	if m.HistoricalReferencesEnabled {
		out = append(out, usableClaims(decision)...)
	}
	for _, fact := range m.HistoricalTexture {
		fact = strings.TrimSpace(fact)
		if fact != "" {
			out = append(out, fact)
		}
	}
	return out
}

func (m LLMMaterializer) eraRules() string {
	if m.SearchGroundedHistoricalReferences {
		return "HISTORICAL_REFERENCES=SEARCH_GROUNDED_EXPERIMENT. Do not introduce new real product/work/service/company/person/place/event names from model memory. A real name already present in canonical Situation/PostIntent was selected only after bounded historical search and is an allowed canonical referent: preserve it in subject/body instead of generalizing it away. Do not add release dates, prices, specifications, plot, popularity, ownership history or other details unless they are explicitly canonical. Unnamed situations should remain unnamed. Never use anything after the supplied world date.\n" + llm.DiegeticWorldFrame
	}
	if m.ModelHistoricalMemory {
		if m.PreferConcreteHistoricalNames {
			return "HISTORICAL_REFERENCES=MODEL_MEMORY_CONCRETE_NAME_EXPERIMENT. This fresh-Lab run intentionally supplies NO proper-noun dictionary or referent list. Use your own historical knowledge only for things you are confident existed and were knowable in Japan on or before the supplied world date. CONCRETE-NAME PREFERENCE: when the already-selected canonical situation naturally corresponds to a real product, work, service, company, person, place, event, news item, seasonal or cultural reference that you confidently know, prefer that concrete historical name over a generic label such as 'game', 'word processor', 'communication service' or 'music'. The Situation proposal is allowed to establish a modest NEW canonical occurrence in which the selected actor played, used, read, watched, heard, visited, or discussed that named thing; prior actor-use evidence is not required for the new occurrence itself. Do not extrapolate from that occurrence to persistent ownership, purchase history, long-term preference, compatibility, or unrelated biography. This is a concretization preference, NOT a quota or topic-selection rule: never redirect an event toward a remembered name, never insert a name as decoration, and do not repeat one favored name across unrelated posts. If timing, identity, availability, price, specifications, plot, event details or any other factual detail is uncertain, omit it or stay generic rather than guessing. Once canonical Situation facts contain a real name, preserve that canonical name in article prose instead of generalizing it away. Do not invent BBS-internal history, posts, logs or SYSOP actions. Keep fictional host facts separate from real history.\\n" + llm.DiegeticWorldFrame
		}
		return "HISTORICAL_REFERENCES=MODEL_MEMORY_EXPERIMENT. This fresh-Lab run intentionally supplies NO proper-noun dictionary. Use your own historical knowledge when it naturally makes an already-selected situation more concrete. You MAY introduce real product, work, service, company, person, place, event, news, seasonal or cultural names only when you are confident they existed and were knowable in Japan on or before the supplied world date. Never use anything from the future. Do not force a proper noun into every post and do not turn remembered names into topic quotas. If timing, identity, availability, ownership, compatibility, price, specifications, plot, event details or other factual details are uncertain, omit those details or stay generic rather than guessing. A real thing's existence does NOT establish that this persona owned, used, watched, bought or experienced it; such actor-specific facts still require canonical world state. Do not invent BBS-internal history, posts, logs or SYSOP actions. Keep fictional host facts separate from real history.\\n" + llm.DiegeticWorldFrame
	}
	if m.HistoricalReferencesEnabled || len(m.HistoricalTexture) > 0 {
		return "HISTORICAL_REFERENCES=ON. 世界時刻より未来の知識を使わない。新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースは、supplied historical facts / historical texture または明示された canonical historical evidence にあるものだけ使用し、モデル記憶から補完しない。supplied texture は話題リストではなく、その時点の世界に存在してよい背景語彙・参照対象である。必要な場合は曖昧な総称へ逃げず具体名を自然に使ってよいが、無関係な投稿へ時代小道具として挿入しない。局固有の架空設定と史実を混同しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよい。\n" + llm.DiegeticWorldFrame
	}
	return "HISTORICAL_REFERENCES=OFF. 世界時刻より未来の知識を使わない。生成する題名・本文・意味計画へ、新しい実在の製品名・作品名・サービス名・企業名・人物名・具体的地名・歴史上の出来事やニュースを導入しない。既に canonical world state として明示的に供給された固有名詞を消去する必要はないが、そこから別の実在情報を連想・補完しない。セーブ、モデム、回線、駅、店、ゲーム、通信ソフト等の一般語彙は自然に使ってよく、具体性まで抽象語に潰さない。\n" + llm.DiegeticWorldFrame
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
	parts := make([]string, 0, 20)
	if i.DiscourseMode != "" {
		parts = append(parts, "discourse_mode="+i.DiscourseMode)
	}
	if summary := strings.TrimSpace(i.SituationSummary); summary != "" {
		parts = append(parts, "canonical_event="+summary)
	} else {
		if i.Topic != "" {
			parts = append(parts, "topic="+i.Topic)
		}
		if i.Motivation != "" {
			parts = append(parts, "motivation="+i.Motivation)
		}
		if i.Stance != "" {
			parts = append(parts, "stance="+i.Stance)
		}
	}
	for _, fact := range i.SituationFacts {
		if workerRelevantSituationFact(fact) {
			parts = append(parts, strings.TrimSpace(fact))
		}
	}
	if i.Goal != "" {
		parts = append(parts, "goal="+i.Goal)
	}
	if len(i.Claims) > 0 {
		parts = append(parts, "claims="+strings.Join(i.Claims, " / "))
	}
	if len(i.RespondsToClaims) > 0 {
		parts = append(parts, "responds_to_claims="+strings.Join(i.RespondsToClaims, " / "))
	}
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
		parts = append(parts, "conversation_context:\n"+strings.TrimSpace(i.RenderContext))
	}
	return strings.Join(parts, "\n")
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
