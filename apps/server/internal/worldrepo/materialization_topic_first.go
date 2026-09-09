package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

var developmentTopicFirst sync.Map

// EnableDevelopmentTopicFirstPoC selects researched public subjects before the
// Situation proposal. It affects only this isolated development repository.
func (r *Repository) EnableDevelopmentTopicFirstPoC() {
	developmentTopicFirst.Store(r, true)
	r.EnableDevelopmentBatchSituationPoC()
}

func developmentTopicFirstEnabled(r *Repository) bool {
	_, ok := developmentTopicFirst.Load(r)
	return ok
}

func topicTargetFact(facts []string) string {
	for _, fact := range facts {
		if strings.HasPrefix(fact, "topic_target=") {
			return strings.TrimPrefix(fact, "topic_target=")
		}
	}
	return ""
}

// TopicTargetInSubject is shared by commit validation and read-only Lab diagnostics.
// The selected search name is already a short contemporary name. We allow width,
// punctuation and case differences, not arbitrary model-invented aliases.
func TopicTargetInSubject(subject, target string) bool {
	normalize := func(s string) string {
		s = strings.Map(func(r rune) rune {
			if r >= '！' && r <= '～' {
				return r - 0xfee0
			}
			return r
		}, s)
		return developmentNormalizeSituationKey(s)
	}
	return normalize(target) != "" && strings.Contains(normalize(subject), normalize(target))
}

// At most six researched pools, with three concurrent requests. Pools share only
// public historical evidence; actors' experiences are proposed independently.
// Date is part of the pool key: no later availability may leak into earlier posts.
func (r *Repository) developmentSelectTopicTargets(host world.Host, roots []developmentWindowShell) ([]developmentWindowShell, error) {
	if r.Engine == nil {
		return nil, fmt.Errorf("topic-first requires historical knowledge resolver")
	}
	out := append([]developmentWindowShell(nil), roots...)
	type pool struct {
		request worldengine.EvidenceRequest
		options []developmentGroundingCandidate
		status  string
	}
	pools := map[string]*pool{}
	keys := make([]string, len(out))
	earliest := map[string]string{}
	for _, root := range out {
		key := root.board.ID + "|" + root.shell.anchorKey
		date := root.shell.createdAt.Format("2006-01-02")
		if earliest[key] == "" || date < earliest[key] {
			earliest[key] = date
		}
	}
	for i, root := range out {
		// Keep everyday/general roots available rather than force names into every post.
		switch root.shell.anchorKey {
		case "games", "music", "software":
		default:
			out[i].topicTargetStatus = "general_topic"
			continue
		}
		key := root.board.ID + "|" + root.shell.anchorKey
		date := earliest[key]
		if _, exists := pools[key]; !exists {
			if len(pools) >= 6 {
				out[i].topicTargetStatus = "search_budget_exhausted"
				continue
			}
			kind := historicalkb.KnowledgeProductAvailability
			if root.shell.anchorKey == "music" {
				kind = historicalkb.KnowledgeCulturalSignal
			}
			pools[key] = &pool{request: worldengine.EvidenceRequest{
				Kind: kind, Subject: "topic-first-v1/" + root.board.Name + "/" + root.shell.anchorKey,
				WorldDate: date, Region: "JP", Audience: []string{"Japanese PC communication users"},
				Persistence: true, Importance: .7, Specificity: .9, HasProductModel: true,
				Need: fmt.Sprintf("%s時点の日本のBBS、板=%s、関心領域=%s。投稿者・日時・投稿する行動は既にworld engineが決定済み。出来事を作る前に、会話の対象にできる実在作品・製品の候補を2〜5件、公式資料または当時資料の出典で確認してください。後のworld engineが対象を選び、架空人物の感想・相談などの用件を確定します。記事や人物の経験は作らないでください。各候補は独立行で CANDIDATE: 当時の短い名称 || 出典URL、存在・利用可能時期、日本での機種/版、確認できる事実と未確認の範囲 の形式。新旧・種類を適度に混ぜ、未来の作品を除外する。発売前なら当時の公表の証拠と未発売を明記し、発売済み扱いしない。確認済み候補のみ返し、それ以外の候補を埋め合わせない。NPCの所有や購入は検索対象外。", date, root.board.Name, root.shell.anchorKey),
			}}
		}
		keys[i] = key
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for _, p := range pools {
		wg.Add(1)
		go func(p *pool) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				p.status = "search_timeout"
				return
			}
			defer func() { <-sem }()
			decision, err := r.Engine.ResolveEvidence(ctx, p.request)
			if err != nil {
				p.status = "search_failed"
				return
			}
			p.options = developmentGroundingCandidatesFromKnowledge(decision.Knowledge)
			p.status = "no_verified_candidate"
			if len(p.options) > 0 {
				p.status = "selected"
			}
		}(p)
	}
	wg.Wait()
	counts := map[string]int{}
	for i, root := range out {
		p := pools[keys[i]]
		if p == nil {
			continue
		}
		out[i].topicTargetStatus = p.status
		if selected, ok := developmentSelectGroundingCandidate(host.ID, root, p.options, r.Base.ListPosts(host.ID), counts); ok {
			out[i].topicTarget = &selected
			counts[developmentNormalizeReferentName(selected.Name)]++
		}
	}
	return out, nil
}
