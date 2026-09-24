package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// This is a Lab safety/cost ceiling. Research now runs only for tentatively
// assigned titles, so 16 covers the maximum independent roots in the standard
// four-board Lab without spending searches on unused members of the 20-title pools.
const developmentTitleEraResearchBudget = 16
const developmentTitleEraResearchConcurrency = 3

type developmentTitleEraOutcome struct {
	status   string
	reason   string
	evidence string
}

type developmentTitleEraResearchJob struct {
	candidate int
	title     string
	claims    []llm.BBSTitleHistoricalClaim
}

func developmentTitleEraResearchAllowance(used, boardsRemaining int) int {
	remaining := developmentTitleEraResearchBudget - used
	if remaining <= 0 || boardsRemaining <= 0 {
		return 0
	}
	// Reserve a fair share for every board that has not been processed yet.
	// Ceil division lets unused earlier shares flow to later boards without
	// exceeding the run-wide budget.
	return (remaining + boardsRemaining - 1) / boardsRemaining
}

// developmentRouteTitleEra is intentionally cheap. It only classifies whether
// a title is safe without historical lookup, requires research if selected, or
// is logically impossible from the world date alone. RESEARCH candidates remain
// eligible for persona/slot matching; Web research is deferred until after that.
func (r *Repository) developmentRouteTitleEra(
	ctx context.Context,
	board world.Board,
	asOf string,
	pool llm.BBSTitleCandidates,
	state *developmentTitleFirstState,
	offset int,
	validator llm.BBSTitleEraValidator,
) (eligibleTitles []string, originalCandidates []int, usage llm.TokenUsage, err error) {
	req := llm.BBSTitleEraRequest{WorldDate: asOf, BoardName: board.Name, Titles: pool.Titles}
	review, err := validator.ValidateBBSTitleEra(ctx, req)
	usage = review.Usage
	if err == nil {
		err = llm.ValidateBBSTitleEraReview(req, review)
	}
	if err != nil {
		for i := offset; i < offset+len(pool.Titles); i++ {
			state.rows[i].EraStatus = "unverified"
			state.rows[i].EraReason = "時代振り分け失敗: " + err.Error()
			state.rows[i].Status = "era_rejected"
			state.rows[i].Reason = state.rows[i].EraReason
		}
		return nil, nil, usage, err
	}

	decisions := make(map[int]llm.BBSTitleEraDecision, len(review.Decisions))
	for _, d := range review.Decisions {
		decisions[d.Candidate] = d
	}

	for candidate, title := range pool.Titles {
		idx := candidate + 1
		row := &state.rows[offset+candidate]
		d := decisions[idx]
		switch d.Status {
		case llm.BBSTitleEraOK:
			row.EraStatus = "ok"
			row.EraReason = d.Reason
		case llm.BBSTitleEraNG:
			row.EraStatus = "ng"
			row.EraReason = d.Reason
			row.Status = "era_rejected"
			row.Reason = "時代検証で除外: " + d.Reason
			continue
		case llm.BBSTitleEraResearch:
			row.EraStatus = "research"
			row.EraReason = d.Reason
			if developmentInteractiveTitleFirstEnabled(r) {
				row.Status = "era_rejected"
				row.Reason = "対話UIの記事一覧ではWeb史料確認を同期実行しないため候補外: " + d.Reason
				continue
			}
		}
		eligibleTitles = append(eligibleTitles, title)
		originalCandidates = append(originalCandidates, idx)
	}
	return eligibleTitles, originalCandidates, usage, nil
}

func (r *Repository) developmentResearchTitleEraBatch(ctx context.Context, host world.Host, board world.Board, asOf string, jobs []developmentTitleEraResearchJob) map[int]developmentTitleEraOutcome {
	out := make(map[int]developmentTitleEraOutcome, len(jobs))
	if len(jobs) == 0 {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, developmentTitleEraResearchConcurrency)
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result := r.developmentResearchTitleEra(ctx, host, board, asOf, job.title, job.claims)
			mu.Lock()
			out[job.candidate] = result
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (r *Repository) developmentResearchTitleEra(ctx context.Context, host world.Host, board world.Board, asOf, title string, claims []llm.BBSTitleHistoricalClaim) developmentTitleEraOutcome {
	if r.Engine == nil {
		return developmentTitleEraOutcome{status: "unverified", reason: "Historical Knowledge Engineが利用できない"}
	}
	if len(claims) == 0 {
		return r.developmentResearchLegacyTitleEra(ctx, host, board, asOf, title)
	}

	evidence := make([]string, 0, len(claims))
	for _, claim := range claims {
		outcome := r.developmentResearchHistoricalClaim(ctx, asOf, title, claim)
		if outcome.status == "ng" {
			return outcome
		}
		if outcome.status != "verified" {
			return outcome
		}
		if strings.TrimSpace(outcome.evidence) != "" {
			evidence = append(evidence, strings.TrimSpace(outcome.evidence))
		}
	}
	return developmentTitleEraOutcome{
		status:   "verified",
		reason:   "再利用可能なHistorical KB claimを検証済み",
		evidence: strings.Join(evidence, " / "),
	}
}

func (r *Repository) developmentResearchLegacyTitleEra(ctx context.Context, host world.Host, board world.Board, asOf, title string) developmentTitleEraOutcome {
	need := fmt.Sprintf("Web検索で、次のBBS記事タイトルに含まれる現実世界の年代事実だけを検証してください。投稿者が実際に購入・所有・利用・視聴・プレイしたかは架空世界側の別判定なので検証対象外です。タイトル=%q。基準日は%s、日本のパソコン通信利用者がその日までに自然に知り得る内容かを確認してください。製品・作品・サービス・規格・機種・人物・番組・曲・イベント等について、発売、発表、サービス開始、利用可能時期が基準日より後なら不適合です。ProvisionalAnswerは必ず先頭を ERA_OK: または ERA_NG: のどちらかにしてください。ERA_OKはタイトル内の時点依存する現実世界の要素がすべて基準日までに成立すると信頼できる史料で確認できた場合だけ。史料不足・同名曖昧・版や機種を確認できない場合も保守的にERA_NGとしてください。理由は短く、確認した時期を含めてください。", title, asOf)
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:            historicalkb.KnowledgeGeneral,
		Subject:         "bbs-title-era:" + asOf + ":" + strings.TrimSpace(title),
		WorldDate:       asOf,
		Region:          "JP",
		Audience:        []string{"Japanese PC communication users"},
		Need:            need,
		Persistence:     true,
		Importance:      .85,
		Specificity:     .98,
		HasExactDate:    true,
		HasProductModel: true,
	})
	if err != nil {
		return developmentTitleEraOutcome{status: "unverified", reason: "Web史料確認失敗: " + compactTitleEraError(err)}
	}
	return developmentTitleEraOutcomeFromEvidence(decision)
}

func (r *Repository) developmentResearchHistoricalClaim(ctx context.Context, asOf, title string, claim llm.BBSTitleHistoricalClaim) developmentTitleEraOutcome {
	subject := strings.TrimSpace(claim.Subject)
	needText := strings.TrimSpace(claim.Need)
	if subject == "" || needText == "" {
		return developmentTitleEraOutcome{status: "unverified", reason: "Historical claim metadataが不完全"}
	}
	kind := developmentHistoricalClaimKind(claim.Kind)
	need := fmt.Sprintf("Web検索で、BBS件名候補に必要な次の現実世界claimだけを検証してください。claim subject=%q。確認内容=%s。基準日は%s、日本でその日までに成立していたことだけを確認してください。元の候補タイトル=%q。投稿者の所有・購入・利用・嗜好は検証対象外です。ProvisionalAnswerは先頭を ERA_OK: または ERA_NG: にしてください。ERA_OKはこのclaimが基準日までに信頼できる資料で成立すると確認できた場合だけ。史料不足・同名曖昧・版や機種を特定できない場合はERA_NGとしてください。", subject, needText, asOf, title)
	req := worldengine.EvidenceRequest{
		Kind:        kind,
		Subject:     subject,
		WorldDate:   asOf,
		Region:      "JP",
		Audience:    []string{"Japanese PC communication users"},
		Need:        need,
		Persistence: true,
		Importance:  .85,
		Specificity: .98,
		HasExactDate: true,
	}
	switch kind {
	case historicalkb.KnowledgeProductAvailability:
		req.HasProductModel = true
	case historicalkb.KnowledgeTechnicalCapability:
		req.HasProductModel = true
		req.HasTechnicalSpec = true
	}
	decision, err := r.Engine.ResolveEvidence(ctx, req)
	if err != nil {
		return developmentTitleEraOutcome{status: "unverified", reason: "Web史料確認失敗: " + compactTitleEraError(err)}
	}
	return developmentHistoricalClaimOutcomeFromEvidence(decision)
}

func developmentHistoricalClaimKind(kind string) historicalkb.KnowledgeKind {
	switch strings.TrimSpace(kind) {
	case "product_availability":
		return historicalkb.KnowledgeProductAvailability
	case "technical_capability":
		return historicalkb.KnowledgeTechnicalCapability
	case "terminology":
		return historicalkb.KnowledgeTerminology
	case "historical_event":
		return historicalkb.KnowledgeHistoricalEvent
	default:
		return historicalkb.KnowledgeGeneral
	}
}

func developmentHistoricalClaimOutcomeFromEvidence(decision worldengine.EvidenceDecision) developmentTitleEraOutcome {
	if !decision.Knowledge.CanUse {
		reason := "検証済みHistorical Factが不足"
		if decision.Knowledge.ResearchPending {
			reason = "同一史料調査が実行中のため未検証"
		}
		return developmentTitleEraOutcome{status: "unverified", reason: reason}
	}
	var verifiedEvidence string
	for _, fact := range decision.Knowledge.Facts {
		if fact.Status != historicalkb.FactVerified && fact.Status != historicalkb.FactOperatorVerified && fact.Status != historicalkb.FactCanonical {
			continue
		}
		claim := strings.TrimSpace(fact.Claim)
		upper := strings.ToUpper(claim)
		if strings.Contains(upper, "ERA_NG:") {
			return developmentTitleEraOutcome{status: "ng", reason: stripTitleEraPrefix(claim, "ERA_NG:"), evidence: claim}
		}
		if strings.Contains(upper, "ERA_OK:") {
			return developmentTitleEraOutcome{status: "verified", reason: stripTitleEraPrefix(claim, "ERA_OK:"), evidence: claim}
		}
		if verifiedEvidence == "" {
			verifiedEvidence = claim
		}
	}
	if verifiedEvidence != "" {
		return developmentTitleEraOutcome{status: "verified", reason: "既存の検証済みHistorical Factを再利用", evidence: verifiedEvidence}
	}
	return developmentTitleEraOutcome{status: "unverified", reason: "Historical Factに検証済みclaimがない"}
}

func developmentTitleEraOutcomeFromEvidence(decision worldengine.EvidenceDecision) developmentTitleEraOutcome {
	if !decision.Knowledge.CanUse {
		reason := "検証済みHistorical Factが不足"
		if decision.Knowledge.ResearchPending {
			reason = "同一史料調査が実行中のため未検証"
		}
		return developmentTitleEraOutcome{status: "unverified", reason: reason}
	}
	for _, fact := range decision.Knowledge.Facts {
		if fact.Status != historicalkb.FactVerified && fact.Status != historicalkb.FactOperatorVerified && fact.Status != historicalkb.FactCanonical {
			continue
		}
		claim := strings.TrimSpace(fact.Claim)
		upper := strings.ToUpper(claim)
		if strings.Contains(upper, "ERA_NG:") {
			return developmentTitleEraOutcome{status: "ng", reason: stripTitleEraPrefix(claim, "ERA_NG:"), evidence: claim}
		}
		if strings.Contains(upper, "ERA_OK:") {
			return developmentTitleEraOutcome{status: "verified", reason: stripTitleEraPrefix(claim, "ERA_OK:"), evidence: claim}
		}
	}
	return developmentTitleEraOutcome{status: "unverified", reason: "Historical FactにERA_OK/ERA_NG判定がない"}
}

func stripTitleEraPrefix(claim, marker string) string {
	upper := strings.ToUpper(claim)
	idx := strings.Index(upper, marker)
	if idx < 0 {
		return strings.TrimSpace(claim)
	}
	reason := strings.TrimSpace(claim[idx+len(marker):])
	if reason == "" {
		return strings.TrimSpace(claim)
	}
	return reason
}

func compactTitleEraError(err error) string {
	if err == nil {
		return "unknown error"
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	runes := []rune(message)
	if len(runes) > 180 {
		message = string(runes[:180]) + "…"
	}
	return message
}
