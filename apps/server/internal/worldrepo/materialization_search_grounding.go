package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

var developmentSearchGroundingPoC sync.Map
var developmentSearchGroundingTelemetry sync.Map

type developmentSearchGroundingKey struct {
	repo   *Repository
	hostID string
}

type developmentSearchGroundingStats struct {
	Queries          int
	EvidenceHits     int
	CandidateOptions int
	Selections       int
	Refined          int
	Rejected         int
	SearchFailures   int
	RefineFailures   int
}

type developmentSearchGroundingRefiner interface {
	RefineDevelopmentWorldSituationsWithSearchGrounding(context.Context, world.Host, string, []developmentWindowShell, map[string][]world.PersonaFact, string, map[string]developmentSituationProposal, map[string][]string) (developmentWorldSituationPlan, error)
}

func (r *Repository) EnableDevelopmentSearchGroundingPoC() {
	developmentSearchGroundingPoC.Store(r, true)
}

func developmentSearchGroundingPoCEnabled(r *Repository) bool {
	_, ok := developmentSearchGroundingPoC.Load(r)
	return ok
}

func developmentSearchGroundingDiagnostic(r *Repository, hostID string) string {
	value, ok := developmentSearchGroundingTelemetry.Load(developmentSearchGroundingKey{repo: r, hostID: hostID})
	if !ok {
		return ""
	}
	stats := value.(developmentSearchGroundingStats)
	return fmt.Sprintf("search_queries=%d search_hits=%d candidate_options=%d grounded_selections=%d grounded_refinements=%d grounding_rejected=%d search_failures=%d refine_failures=%d", stats.Queries, stats.EvidenceHits, stats.CandidateOptions, stats.Selections, stats.Refined, stats.Rejected, stats.SearchFailures, stats.RefineFailures)
}

func (r *Repository) developmentSearchGroundSituations(host world.Host, roots []developmentWindowShell, accepted map[string]developmentSituationProposal, factsByPersona map[string][]world.PersonaFact) (map[string]developmentSituationProposal, GenerationUsage) {
	stats := developmentSearchGroundingStats{}
	storeStats := func() {
		developmentSearchGroundingTelemetry.Store(developmentSearchGroundingKey{repo: r, hostID: host.ID}, stats)
	}
	if r.Engine == nil {
		storeStats()
		return accepted, GenerationUsage{}
	}
	refiner, ok := r.Materializer.(developmentSearchGroundingRefiner)
	if !ok {
		stats.RefineFailures++
		storeStats()
		return accepted, GenerationUsage{}
	}

	candidates := developmentSearchGroundingCandidates(roots, accepted, 6)
	if len(candidates) == 0 {
		storeStats()
		return accepted, GenerationUsage{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	optionsByEvent := map[string][]developmentGroundingCandidate{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for _, item := range candidates {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			proposal := accepted[item.eventID]
			mu.Lock()
			stats.Queries++
			mu.Unlock()
			decision, err := r.Engine.ResolveEvidence(ctx, developmentSearchGroundingEvidenceRequest(item, proposal, factsByPersona[item.shell.persona.ID]))
			if err != nil {
				mu.Lock()
				stats.SearchFailures++
				mu.Unlock()
				return
			}
			options := developmentGroundingCandidatesFromKnowledge(decision.Knowledge)
			if len(options) == 0 {
				return
			}
			mu.Lock()
			stats.EvidenceHits++
			stats.CandidateOptions += len(options)
			optionsByEvent[item.eventID] = options
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(optionsByEvent) == 0 {
		storeStats()
		return accepted, GenerationUsage{}
	}

	grounding := map[string][]string{}
	selectedNames := map[string]int{}
	recentPosts := r.Base.ListPosts(host.ID)
	refineRoots := make([]developmentWindowShell, 0, len(optionsByEvent))
	for _, root := range roots {
		options := optionsByEvent[root.eventID]
		if len(options) == 0 {
			continue
		}
		selected, ok := developmentSelectGroundingCandidate(host.ID, root, options, recentPosts, selectedNames)
		if !ok {
			continue
		}
		selectedNames[developmentNormalizeReferentName(selected.Name)]++
		stats.Selections++
		grounding[root.eventID] = []string{developmentSelectedGroundingEvidence(selected)}
		refineRoots = append(refineRoots, root)
	}
	if len(refineRoots) == 0 {
		storeStats()
		return accepted, GenerationUsage{}
	}
	plan, err := refiner.RefineDevelopmentWorldSituationsWithSearchGrounding(ctx, host, r.WorldDate, refineRoots, factsByPersona, planningBBSState(r.Base.ListPosts(host.ID), 24), accepted, grounding)
	if err != nil {
		stats.RefineFailures++
		storeStats()
		return accepted, GenerationUsage{}
	}
	out := make(map[string]developmentSituationProposal, len(accepted))
	for id, proposal := range accepted {
		out[id] = proposal
	}
	for _, root := range refineRoots {
		original := accepted[root.eventID]
		refined, found := plan.proposals[root.eventID]
		selectedName := developmentSelectedGroundingName(grounding[root.eventID])
		if !found || !developmentSearchGroundingCompatible(original, refined) || !developmentRefinementUsesSelectedReferent(refined, selectedName) {
			stats.Rejected++
			continue
		}
		refined.groundingEvidence = append([]string(nil), grounding[root.eventID]...)
		out[root.eventID] = refined
		stats.Refined++
	}
	storeStats()
	return out, plan.usage
}

func developmentSearchGroundingEvidenceRequest(root developmentWindowShell, proposal developmentSituationProposal, personaFacts []world.PersonaFact) worldengine.EvidenceRequest {
	kind := historicalkb.KnowledgeGeneral
	switch strings.ToLower(strings.TrimSpace(root.shell.anchorKey)) {
	case "games", "software", "communications", "modem":
		kind = historicalkb.KnowledgeProductAvailability
	case "music":
		kind = historicalkb.KnowledgeCulturalSignal
	}
	worldDate := root.shell.createdAt.Format("2006-01-02")
	actorContext := developmentGroundingActorContext(root.shell.persona, personaFacts)
	need := fmt.Sprintf("次の架空BBS内の出来事の種類・行動・観察はworld engineが既に選択済みです。あなたの役割は、この出来事を%s時点の日本の現実世界へ自然に接地できる実在の製品・作品・サービス・機種・番組・曲などの候補集合をWeb検索で作ることです。重要: これは過去の正解を推理する検索ではありません。候補のうちどれが今回このNPCの世界事実になるかは、この後world engineが決定します。したがってoriginal Situationから一意に名前を推理できる必要はありません。出来事の意味を変えず、その日までに日本で利用・発売・稼働・放送・鑑賞・言及可能で、普通にこの出来事の対象になり得る候補を2〜5件返してください。候補はできるだけ公式資料・当時資料・信頼できる保存資料で存在時期を確認してください。候補名は当時の利用者が会話で自然に使う短い正式名または一般的名称にしてください。人気作だけに偏らず、actor contextやboard contextに自然に合う順で並べてください。ただしactorが所有・購入・視聴・プレイした事実は推測しないでください。候補はその可能性を提供するだけです。各候補はProvisionalAnswer内で必ず独立した行に CANDIDATE: 名前 || その日までの存在/利用可能性と、このgeneric Situationに意味を変えず適合する短い根拠 の厳密な形式で書いてください。候補が本当に作れない場合だけ NO_CANDIDATE: 理由 としてください。単に候補が複数あることはNO_CANDIDATEの理由ではありません。必要な歴史条件が裏付けられた候補だけを出し、その場合missingInfoは空配列にしてください。NPCの今回の関与、所有歴、長期嗜好、個別店舗在庫、主観的結果は検索スコープ外なのでmissingInfoへ入れないでください。board=%s; actor_context=%s; object=%s; occurrence=%s; actor_observation=%s", worldDate, root.board.Name, actorContext, proposal.objectClass, proposal.occurrence, proposal.actorObservation)
	return worldengine.EvidenceRequest{
		Kind:            kind,
		Subject:         proposal.objectClass + " / " + proposal.noveltyKey + " / candidate-pool-v1",
		WorldDate:       worldDate,
		Region:          "JP",
		Audience:        []string{"Japanese PC communication users"},
		Need:            need,
		Persistence:     true,
		Importance:      .7,
		Specificity:     .9,
		HasProductModel: true,
	}
}

func developmentUsableGroundingClaims(result historicalkb.KnowledgeResult) []string {
	if !result.CanUse {
		return nil
	}
	out := make([]string, 0, len(result.Facts))
	for _, fact := range result.Facts {
		if fact.Status != historicalkb.FactVerified && fact.Status != historicalkb.FactOperatorVerified && fact.Status != historicalkb.FactCanonical {
			continue
		}
		claim := strings.TrimSpace(fact.Claim)
		if claim == "" || developmentGroundingClaimSaysSkip(claim) {
			continue
		}
		out = append(out, claim)
	}
	return out
}

func developmentGroundingClaimSaysSkip(claim string) bool {
	lower := strings.ToLower(strings.TrimSpace(claim))
	for _, marker := range []string{"具体化不要", "特定不能", "確認できない", "該当なし", "insufficient evidence", "cannot verify"} {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func developmentSearchGroundingCandidates(roots []developmentWindowShell, accepted map[string]developmentSituationProposal, limit int) []developmentWindowShell {
	type ranked struct {
		root  developmentWindowShell
		score int
	}
	rankedRoots := make([]ranked, 0, len(roots))
	for _, root := range roots {
		if _, ok := accepted[root.eventID]; !ok {
			continue
		}
		score := developmentSearchGroundingScore(root)
		if score <= 0 {
			continue
		}
		rankedRoots = append(rankedRoots, ranked{root: root, score: score})
	}
	sort.SliceStable(rankedRoots, func(i, j int) bool {
		if rankedRoots[i].score != rankedRoots[j].score {
			return rankedRoots[i].score > rankedRoots[j].score
		}
		return rankedRoots[i].root.shell.createdAt.Before(rankedRoots[j].root.shell.createdAt)
	})
	if limit <= 0 || limit > len(rankedRoots) {
		limit = len(rankedRoots)
	}
	out := make([]developmentWindowShell, 0, limit)
	for _, item := range rankedRoots[:limit] {
		out = append(out, item.root)
	}
	return out
}

func developmentSearchGroundingScore(root developmentWindowShell) int {
	anchor := strings.ToLower(strings.TrimSpace(root.shell.anchorKey))
	switch anchor {
	case "games", "software", "music":
		return 3
	case "communications", "modem":
		return 2
	}
	switch root.board.ID {
	case "4", "5", "6":
		return 3
	}
	return 0
}

func developmentSearchGroundingCompatible(original, refined developmentSituationProposal) bool {
	if strings.TrimSpace(refined.objectClass) == "" || strings.TrimSpace(refined.occurrence) == "" || strings.TrimSpace(refined.actorObservation) == "" {
		return false
	}
	if developmentNormalizeSituationKey(original.noveltyKey) != developmentNormalizeSituationKey(refined.noveltyKey) {
		return false
	}
	if developmentSituationTextSimilarity(original.occurrence, refined.occurrence) < .18 {
		return false
	}
	if developmentSituationTextSimilarity(original.actorObservation, refined.actorObservation) < .12 {
		return false
	}
	return true
}

func (m LLMMaterializer) RefineDevelopmentWorldSituationsWithSearchGrounding(ctx context.Context, host world.Host, worldDate string, roots []developmentWindowShell, factsByPersona map[string][]world.PersonaFact, recentBBS string, originals map[string]developmentSituationProposal, grounding map[string][]string) (developmentWorldSituationPlan, error) {
	proposer, ok := m.Renderer.(llm.BBSWorldSituationProposer)
	if !ok {
		return developmentWorldSituationPlan{}, fmt.Errorf("configured renderer does not implement BBS world-situation proposals")
	}
	if len(roots) == 0 {
		return developmentWorldSituationPlan{proposals: map[string]developmentSituationProposal{}}, nil
	}
	windowStart := roots[0].shell.createdAt
	windowEnd := roots[0].shell.createdAt
	events := make([]llm.BBSWorldWindowEvent, 0, len(roots))
	for _, item := range roots {
		shell := item.shell
		if shell.createdAt.Before(windowStart) {
			windowStart = shell.createdAt
		}
		if shell.createdAt.After(windowEnd) {
			windowEnd = shell.createdAt
		}
		existingFacts := make([]string, 0, len(factsByPersona[shell.persona.ID])+len(grounding[item.eventID])+1)
		for _, fact := range factsByPersona[shell.persona.ID] {
			existingFacts = append(existingFacts, "BACKGROUND ONLY: "+fact.Key+"="+fact.Value)
		}
		original := originals[item.eventID]
		existingFacts = append(existingFacts, fmt.Sprintf("GROUNDING ORIGINAL SITUATION - PRESERVE SEMANTICS: object_class=%s; change_class=%s; occurrence=%s; actor_observation=%s; impact=%s; uncertainty=%s; novelty_key=%s", original.objectClass, original.changeClass, original.occurrence, original.actorObservation, original.impact, original.uncertainty, original.noveltyKey))
		for _, evidence := range grounding[item.eventID] {
			existingFacts = append(existingFacts, "GROUNDING VERIFIED HISTORICAL EVIDENCE - ALLOWED REAL REFERENTS: "+evidence)
		}
		events = append(events, llm.BBSWorldWindowEvent{
			EventID:        item.eventID,
			BoardID:        item.board.ID,
			BoardName:      item.board.Name,
			AuthorHandle:   shell.persona.Handle,
			CreatedAt:      shell.createdAt.Format(time.RFC3339),
			Action:         shell.action,
			AnchorKey:      shell.anchorKey,
			CauseKind:      shell.causeKind,
			CauseSummary:   shell.causeSummary,
			DiscourseMode:  shell.discourseMode,
			PersonaProfile: personaSummary(shell.persona),
			ExistingFacts:  existingFacts,
		})
	}
	draft, err := proposer.GenerateBBSWorldSituationProposals(ctx, llm.BBSWorldSituationProposalRequest{
		HostName:       host.Name,
		HostRegion:     host.Region,
		HostSoftware:   host.Software,
		WorldDate:      worldDate,
		WindowStart:    windowStart.Format(time.RFC3339),
		WindowEnd:      windowEnd.Format(time.RFC3339),
		EraRules:       m.planningEraRules(),
		RecentBBSState: recentBBS,
		Events:         events,
	})
	if err != nil {
		return developmentWorldSituationPlan{}, err
	}
	proposals := make(map[string]developmentSituationProposal, len(draft.Situations))
	for _, situation := range draft.Situations {
		proposals[situation.EventID] = developmentSituationProposal{
			eventID:          strings.TrimSpace(situation.EventID),
			objectClass:      strings.TrimSpace(situation.ObjectClass),
			changeClass:      strings.TrimSpace(situation.ChangeClass),
			occurrence:       strings.TrimSpace(situation.Occurrence),
			actorObservation: strings.TrimSpace(situation.ActorObservation),
			impact:           strings.TrimSpace(situation.Impact),
			uncertainty:      strings.TrimSpace(situation.Uncertainty),
			noveltyKey:       strings.TrimSpace(situation.NoveltyKey),
			mustNot:          append([]string(nil), situation.MustNot...),
		}
	}
	return developmentWorldSituationPlan{proposals: proposals, usage: GenerationUsage{
		InputTokens: draft.Usage.InputTokens, CachedInputTokens: draft.Usage.CachedInputTokens,
		OutputTokens: draft.Usage.OutputTokens, ReasoningTokens: draft.Usage.ReasoningTokens,
		TotalTokens: draft.Usage.TotalTokens, Model: draft.Usage.Model,
	}}, nil
}
