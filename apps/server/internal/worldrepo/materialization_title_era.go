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

const developmentTitleEraResearchBudget = 12
const developmentTitleEraResearchConcurrency = 3

type developmentTitleEraOutcome struct {
	status   string
	reason   string
	evidence string
}

type developmentTitleEraResearchJob struct {
	candidate int
	title     string
}

func (r *Repository) developmentValidateTitleEra(
	ctx context.Context,
	host world.Host,
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

	researchJobs := make([]developmentTitleEraResearchJob, 0)
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
		case llm.BBSTitleEraResearch:
			if state.eraResearchUsed >= developmentTitleEraResearchBudget {
				row.EraStatus = "unverified"
				row.EraReason = fmt.Sprintf("%s / Web史料確認はrun上限%d件に達したため未検証", d.Reason, developmentTitleEraResearchBudget)
				row.Status = "era_rejected"
				row.Reason = "時代検証未完了のため除外: " + row.EraReason
				continue
			}
			state.eraResearchUsed++
			row.EraStatus = "research"
			row.EraReason = d.Reason
			researchJobs = append(researchJobs, developmentTitleEraResearchJob{candidate: idx, title: title})
		}
	}

	outcomes := r.developmentResearchTitleEraBatch(ctx, host, board, asOf, researchJobs)
	for candidate, outcome := range outcomes {
		row := &state.rows[offset+candidate-1]
		row.EraStatus = outcome.status
		row.EraReason = outcome.reason
		row.EraEvidence = outcome.evidence
		if outcome.status != "verified" {
			row.Status = "era_rejected"
			if outcome.status == "ng" {
				row.Reason = "Web史料検証で除外: " + outcome.reason
			} else {
				row.Reason = "時代検証未完了のため除外: " + outcome.reason
			}
		}
	}

	for candidate, title := range pool.Titles {
		row := &state.rows[offset+candidate]
		if row.EraStatus != "ok" && row.EraStatus != "verified" {
			continue
		}
		eligibleTitles = append(eligibleTitles, title)
		originalCandidates = append(originalCandidates, candidate+1)
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
			result := r.developmentResearchTitleEra(ctx, host, board, asOf, job.title)
			mu.Lock()
			out[job.candidate] = result
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (r *Repository) developmentResearchTitleEra(ctx context.Context, host world.Host, board world.Board, asOf, title string) developmentTitleEraOutcome {
	if r.Engine == nil {
		return developmentTitleEraOutcome{status: "unverified", reason: "Historical Knowledge Engineが利用できない"}
	}
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
