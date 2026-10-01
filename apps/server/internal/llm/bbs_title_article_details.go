package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	articleDetailMarkdownCitation = regexp.MustCompile(`\s*\(\[[^\]\r\n]+\]\(https?://[^\)\r\n]+\)\)\s*`)
	articleDetailBareURL = regexp.MustCompile(`https?://[^\s）)]+`)
)

var bbsArticleDetailKinds = map[string]bool{
	"referent":         true,
	"locator":          true,
	"timing":           true,
	"sequence":         true,
	"comparison":       true,
	"observation":      true,
	"question_scope":   true,
	"decision":         true,
	"reaction_context": true,
}

type BBSTitleArticleDetailSeed struct {
	EventID        string   `json:"event_id"`
	IsReply        bool     `json:"is_reply"`
	Subject        string   `json:"subject"`
	Summary        string   `json:"summary"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	DiscourseMode  string   `json:"discourse_mode"`
	PersonaProfile string   `json:"persona_profile,omitempty"`
	ExistingFacts  []string `json:"existing_facts,omitempty"`
	ThreadContext  string   `json:"thread_context,omitempty"`
	AuthorHistory  string   `json:"author_history,omitempty"`
}

type BBSTitleArticleDetailRequest struct {
	BoardName      string                      `json:"board_name"`
	WorldDate      string                      `json:"world_date"`
	RecentBBSState string                      `json:"recent_bbs_state,omitempty"`
	Articles       []BBSTitleArticleDetailSeed `json:"articles"`
}

type BBSArticleDetail struct {
	Kind string `json:"kind"`
	Fact string `json:"fact"`
}

type BBSTitleArticleDetailSet struct {
	EventID              string             `json:"event_id"`
	ReferentRequirement  string             `json:"referent_requirement"`
	ReferentStatus       string             `json:"referent_status"`
	ReferentGrounding    string             `json:"referent_grounding"`
	Details              []BBSArticleDetail `json:"details"`
}

type BBSTitleArticleDetailDraft struct {
	Articles             []BBSTitleArticleDetailSet `json:"articles"`
	Usage                TokenUsage                 `json:"-"`
	WebSearchCalls       int                        `json:"-"`
	WebSearchSources     []string                   `json:"-"`
	ForcedWebSearchRetry bool                       `json:"-"`
}

type BBSTitleArticleDetailPlanner interface {
	MaterializeBBSTitleArticleDetails(context.Context, BBSTitleArticleDetailRequest) (BBSTitleArticleDetailDraft, error)
}

func (p StructuredOpenAIProvider) MaterializeBBSTitleArticleDetails(ctx context.Context, req BBSTitleArticleDetailRequest) (BBSTitleArticleDetailDraft, error) {
	if len(req.Articles) == 0 {
		return BBSTitleArticleDetailDraft{}, nil
	}
	input, err := json.Marshal(req)
	if err != nil {
		return BBSTitleArticleDetailDraft{}, err
	}
	prompt := `既存記事またはreplyについて、本文を書くために不足している記事ローカルの世界事実だけを0〜2件補ってください。
これは文章構成やタイトル作成ではありません。subject/summary/persona/thread contextは変更しません。

判定:
- 返すメタデータは referent_requirement / referent_status / referent_grounding。本文へ書く世界事実ではなく生成制御用。
- referent_requirement=required: 一つの特定作品・製品・場所等を別対象へ替えると経験内容そのものが変わる。
- optional: 特定対象がなくても話題が成立する。
- none: 外部対象を同定する必要がない個人的・局内・日常話題。
- 既存contextに対象があるなら already_in_context。今回具体化したなら resolved。requiredだがまだ決められない外部対象だけ unresolved。
- 実在の外部対象は external_history、仮想世界内の匿名/私的対象は world_local、replyで親記事から継承する対象は inherited_context。

details:
- 0〜2件。本文に必要な小さな事実だけ。
- title/summaryの言い換え、編集指示、結論、教訓、読者への呼びかけは書かない。
- replyではThreadContextを読んだ上で、この返信者自身が今回足す観察・経験・条件を必要な場合だけ具体化する。他人の経験を本人へ移さない。
- ExistingFacts/PersonaProfile/AuthorHistoryと矛盾する所有歴・購入歴・職歴・家族事情・長期嗜好を作らない。
- MSG番号、投稿日時、板名などレンダリング情報をdetailにしない。
- 実在作品・製品・人物・場所・仕様など新しい外部史実をdetailへ入れる場合はWeb検索で投稿日時点の整合と、そのdetailで述べる具体命題を確認する。
- world_localな対象を検索で見つけた実在物へ置換しない。
- 確証がない外部仕様・攻略・価格・内容をモデル記憶で補わない。

使えるkind: referent, locator, timing, sequence, comparison, observation, question_scope, decision, reaction_context。
各event_idは入力と完全一致させてください。

入力データ:
` + string(input)
	kindEnum := []string{"referent", "locator", "timing", "sequence", "comparison", "observation", "question_scope", "decision", "reaction_context"}
	detailSchema := map[string]any{"type": "object", "properties": map[string]any{
		"kind": map[string]any{"type": "string", "enum": kindEnum},
		"fact": map[string]any{"type": "string"},
	}, "required": []string{"kind", "fact"}, "additionalProperties": false}
	articleSchema := map[string]any{"type": "object", "properties": map[string]any{
		"event_id": map[string]any{"type": "string"},
		"referent_requirement": map[string]any{"type": "string", "enum": []string{"required", "optional", "none"}},
		"referent_status": map[string]any{"type": "string", "enum": []string{"already_in_context", "resolved", "unresolved", "not_applicable"}},
		"referent_grounding": map[string]any{"type": "string", "enum": []string{"external_history", "world_local", "inherited_context", "not_applicable"}},
		"details":  map[string]any{"type": "array", "items": detailSchema, "minItems": 0, "maxItems": 2},
	}, "required": []string{"event_id", "referent_requirement", "referent_status", "referent_grounding", "details"}, "additionalProperties": false}
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"articles": map[string]any{"type": "array", "items": articleSchema, "minItems": len(req.Articles), "maxItems": len(req.Articles)},
	}, "required": []string{"articles"}, "additionalProperties": false}
	traceFinish := beginDebugTrace(ctx, "Article Detail", prompt)
	result, err := p.responseTextWithJSONSchemaWebSearch(ctx, prompt, "low", "medium", 4200, "bbs_title_article_details", schema)
	traceFinish(result.Text, err)
	if err != nil {
		return BBSTitleArticleDetailDraft{}, err
	}
	draft, err := decodeBBSTitleArticleDetailResult(req, result)
	if err != nil {
		return draft, err
	}
	if !articleDetailNeedsForcedWebSearch(req, draft) {
		return draft, nil
	}

	previousResult, _ := json.Marshal(draft.Articles)
	forcedPrompt := prompt + `

前回のstructured result:
` + string(previousResult) + `

REQUIRED REFERENT RETRY:
requiredな外部referentが未解決です。Web検索を最低1回使い、subject/summary/thread contextの意味を変えず、投稿日時点の日本で成立する具体的対象を解決してください。
- contextにすでに対象名があるなら、その対象を検証し、成立する限り置換しない。
- world_local/inherited_contextを実在対象へ置換しない。
- 最終結果でrequiredをunresolvedのまま返さない。外部対象ならreferent detailを明示する。
- 対象名の存在確認だけで未確認の仕様・攻略・ストーリー・数値を足さない。
- 元の記事意図・人物・日時を変えない。
`
	draft.ForcedWebSearchRetry = true
	forcedResult, err := p.responseTextWithJSONSchemaRequiredWebSearch(ctx, forcedPrompt, "low", "medium", 4200, "bbs_title_article_details", schema)
	if err != nil {
		return draft, nil
	}
	forcedDraft, err := decodeBBSTitleArticleDetailResult(req, forcedResult)
	if err != nil {
		return draft, nil
	}
	forcedDraft.ForcedWebSearchRetry = true
	forcedDraft.Usage = mergeTokenUsage(draft.Usage, forcedDraft.Usage)
	forcedDraft.WebSearchCalls += draft.WebSearchCalls
	forcedDraft.WebSearchSources = mergeWebSearchSources(draft.WebSearchSources, forcedDraft.WebSearchSources)
	return forcedDraft, nil
}

func ValidateBBSTitleArticleDetails(req BBSTitleArticleDetailRequest, draft BBSTitleArticleDetailDraft) error {
	if len(draft.Articles) != len(req.Articles) {
		return fmt.Errorf("article detail materializer returned %d articles, want %d", len(draft.Articles), len(req.Articles))
	}
	seeds := map[string]BBSTitleArticleDetailSeed{}
	for _, seed := range req.Articles {
		if strings.TrimSpace(seed.EventID) == "" || seeds[seed.EventID].EventID != "" {
			return fmt.Errorf("invalid/duplicate article detail seed %q", seed.EventID)
		}
		seeds[seed.EventID] = seed
	}
	seen := map[string]bool{}
	for _, article := range draft.Articles {
		seed, ok := seeds[article.EventID]
		if !ok || seen[article.EventID] {
			return fmt.Errorf("invalid/duplicate article detail event %q", article.EventID)
		}
		seen[article.EventID] = true
		requirement := strings.TrimSpace(article.ReferentRequirement)
		status := strings.TrimSpace(article.ReferentStatus)
		grounding := strings.TrimSpace(article.ReferentGrounding)
		if requirement != "" {
			switch requirement {
			case "required", "optional", "none":
			default:
				return fmt.Errorf("article %q has invalid referent_requirement %q", article.EventID, requirement)
			}
		}
		if status != "" {
			switch status {
			case "already_in_context", "resolved", "unresolved", "not_applicable":
			default:
				return fmt.Errorf("article %q has invalid referent_status %q", article.EventID, status)
			}
		}
		if grounding != "" {
			switch grounding {
			case "external_history", "world_local", "inherited_context", "not_applicable":
			default:
				return fmt.Errorf("article %q has invalid referent_grounding %q", article.EventID, grounding)
			}
		}
		if requirement == "required" && status == "not_applicable" {
			return fmt.Errorf("article %q cannot mark required referent as not_applicable", article.EventID)
		}
		if requirement == "none" && status != "" && status != "not_applicable" {
			return fmt.Errorf("article %q with no referent requirement must use not_applicable status", article.EventID)
		}
		if len(article.Details) > 2 {
			return fmt.Errorf("article %q needs 0-2 details", article.EventID)
		}
		seenFacts := map[string]bool{}
		hasReferent := false
		for _, detail := range article.Details {
			kind := strings.TrimSpace(detail.Kind)
			fact := strings.TrimSpace(detail.Fact)
			if !bbsArticleDetailKinds[kind] {
				return fmt.Errorf("article %q has invalid detail kind %q", article.EventID, kind)
			}
			if kind == "referent" {
				hasReferent = true
			}
			if fact == "" || utf8.RuneCountInString(fact) > 180 || strings.ContainsAny(fact, "\r\n") {
				return fmt.Errorf("article %q has invalid detail fact", article.EventID)
			}
			if fact == strings.TrimSpace(seed.Subject) || fact == strings.TrimSpace(seed.Summary) || articleDetailLooksEditorial(fact) {
				return fmt.Errorf("article %q detail is only a restatement/editorial instruction: %q", article.EventID, fact)
			}
			normalizedFact := strings.ToLower(strings.Join(strings.Fields(fact), " "))
			if seenFacts[normalizedFact] {
				return fmt.Errorf("article %q has duplicate detail fact %q", article.EventID, fact)
			}
			seenFacts[normalizedFact] = true
			if ArticleDetailFactIsRenderingMetadata(fact) {
				return fmt.Errorf("article %q detail leaked article-header/rendering metadata: %q", article.EventID, fact)
			}
			if ArticleDetailFactLeaksEvidenceMetadata(fact) {
				return fmt.Errorf("article %q detail leaked Web/evidence metadata: %q", article.EventID, fact)
			}
		}
		if hasReferent && status != "" && status != "resolved" && status != "already_in_context" {
			return fmt.Errorf("article %q has referent detail with incompatible status %q", article.EventID, status)
		}
		if status == "unresolved" && hasReferent {
			return fmt.Errorf("article %q cannot be unresolved while carrying a referent detail", article.EventID)
		}
		if status == "not_applicable" && hasReferent {
			return fmt.Errorf("article %q cannot be not_applicable while carrying a referent detail", article.EventID)
		}
		if grounding == "not_applicable" && hasReferent {
			return fmt.Errorf("article %q cannot use not_applicable grounding with a referent detail", article.EventID)
		}
		if grounding == "inherited_context" {
			if !seed.IsReply || status != "already_in_context" {
				return fmt.Errorf("article %q inherited_context requires a reply with already_in_context status", article.EventID)
			}
		}
		if grounding == "world_local" && status == "unresolved" {
			return fmt.Errorf("article %q world_local referent must be resolved locally, not left unresolved", article.EventID)
		}
		if grounding == "external_history" && status == "not_applicable" {
			return fmt.Errorf("article %q external_history grounding cannot be not_applicable", article.EventID)
		}
		if requirement == "none" && grounding != "" && grounding != "not_applicable" {
			return fmt.Errorf("article %q with no referent requirement must use not_applicable grounding", article.EventID)
		}
		if requirement == "required" && !seed.IsReply && (status == "resolved" || status == "already_in_context") && !hasReferent {
			return fmt.Errorf("article %q required root referent must be present in canonical details", article.EventID)
		}
	}
	return nil
}

func ValidateBBSTitleArticleDetailsForCommit(req BBSTitleArticleDetailRequest, draft BBSTitleArticleDetailDraft) error {
	if err := ValidateBBSTitleArticleDetails(req, draft); err != nil {
		return err
	}
	seeds := make(map[string]BBSTitleArticleDetailSeed, len(req.Articles))
	for _, seed := range req.Articles {
		seeds[seed.EventID] = seed
	}
	for _, article := range draft.Articles {
		if strings.TrimSpace(article.ReferentRequirement) != "required" {
			continue
		}
		seed := seeds[article.EventID]
		status := strings.TrimSpace(article.ReferentStatus)
		grounding := strings.TrimSpace(article.ReferentGrounding)
		if status == "unresolved" || status == "not_applicable" || status == "" {
			return fmt.Errorf("article %q required referent remained %q at commit", article.EventID, status)
		}
		if grounding == "" || grounding == "not_applicable" {
			return fmt.Errorf("article %q required referent has no commit grounding", article.EventID)
		}
		hasReferent := false
		for _, detail := range article.Details {
			if strings.TrimSpace(detail.Kind) != "referent" {
				continue
			}
			hasReferent = true
			if articleDetailReferentLooksPlaceholder(detail.Fact) {
				return fmt.Errorf("article %q required referent is only a placeholder: %q", article.EventID, detail.Fact)
			}
		}
		if seed.IsReply && status == "already_in_context" && grounding == "inherited_context" {
			continue
		}
		if !hasReferent {
			return fmt.Errorf("article %q required referent must be explicit before commit", article.EventID)
		}
	}
	return nil
}

func articleDetailReferentLooksPlaceholder(fact string) bool {
	value := strings.ToLower(strings.TrimSpace(fact))
	if value == "" {
		return true
	}
	for _, marker := range []string{
		"題名不詳", "題名不明", "作品名不詳", "作品名不明", "作品不詳", "作品不明",
		"タイトル不詳", "タイトル不明", "名称不詳", "名称不明", "名前不詳", "名前不明",
		"某作品", "ある作品",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func sanitizeArticleDetailEvidenceMetadata(fact string) string {
	value := articleDetailMarkdownCitation.ReplaceAllString(strings.TrimSpace(fact), "")
	value = articleDetailBareURL.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "()", "")
	value = strings.ReplaceAll(value, "（）", "")
	return strings.TrimSpace(value)
}

func ArticleDetailFactLeaksEvidenceMetadata(fact string) bool {
	value := strings.ToLower(strings.TrimSpace(fact))
	if value == "" {
		return false
	}
	for _, marker := range []string{"http://", "https://", "utm_source=", "web検索", "検索結果", "参照url", "source url"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func articleDetailLooksEditorial(fact string) bool {
	for _, marker := range []string{"話題にする", "共有する", "読者に", "参加者に", "紹介する", "紹介。", "報告する", "構成にする", "説明を求め", "情報提供を求め"} {
		if strings.Contains(fact, marker) {
			return true
		}
	}
	return false
}

// ArticleDetailFactIsRenderingMetadata identifies facts about the BBS record/header
// rather than facts inside the fictional article event. Such facts must never become
// canonical article_detail because prose workers can otherwise echo them verbatim.
func ArticleDetailFactIsRenderingMetadata(fact string) bool {
	value := strings.ToLower(strings.TrimSpace(fact))
	if value == "" {
		return false
	}
	for _, marker := range []string{
		"msg ", "msg#", "msg番号", "記事番号", "メッセージ番号", "投稿時刻", "投稿日時",
		"新規スレッド", "新スレ", "先頭投稿", "スレッドの先頭", "board id",
		"掲示されて", "掲示された", "投稿された", "書き込まれて", "書き込まれた",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}


func decodeBBSTitleArticleDetailResult(req BBSTitleArticleDetailRequest, result responseTextResult) (BBSTitleArticleDetailDraft, error) {
	var draft BBSTitleArticleDetailDraft
	if err := json.Unmarshal([]byte(result.Text), &draft); err != nil {
		return draft, err
	}
	for ai := range draft.Articles {
		for di := range draft.Articles[ai].Details {
			draft.Articles[ai].Details[di].Fact = sanitizeArticleDetailEvidenceMetadata(draft.Articles[ai].Details[di].Fact)
		}
	}
	draft.Usage = result.Usage
	draft.WebSearchCalls = result.WebSearchCalls
	draft.WebSearchSources = append([]string(nil), result.WebSearchSources...)
	if err := ValidateBBSTitleArticleDetails(req, draft); err != nil {
		return draft, err
	}
	return draft, nil
}

func articleDetailNeedsForcedWebSearch(req BBSTitleArticleDetailRequest, draft BBSTitleArticleDetailDraft) bool {
	articlesByEvent := make(map[string]BBSTitleArticleDetailSet, len(draft.Articles))
	for _, article := range draft.Articles {
		articlesByEvent[article.EventID] = article
	}
	for _, seed := range req.Articles {
		article, ok := articlesByEvent[seed.EventID]
		if !ok {
			continue
		}
		requirement := strings.TrimSpace(article.ReferentRequirement)
		status := strings.TrimSpace(article.ReferentStatus)
		grounding := strings.TrimSpace(article.ReferentGrounding)
		// A required root that is still unresolved must never silently bypass
		// the recovery pass merely because the first model mislabeled grounding
		// as not_applicable. Genuine world-local targets should have been
		// resolved locally in the first pass.
		if !seed.IsReply && requirement == "required" && status == "unresolved" &&
			grounding != "world_local" && grounding != "inherited_context" {
			return true
		}
		if grounding != "external_history" {
			continue
		}
		if status == "unresolved" {
			return true
		}
		if (status == "already_in_context" || status == "resolved") && draft.WebSearchCalls == 0 {
			return true
		}
	}
	return false
}

func mergeTokenUsage(a, b TokenUsage) TokenUsage {
	out := TokenUsage{
		InputTokens:       a.InputTokens + b.InputTokens,
		CachedInputTokens: a.CachedInputTokens + b.CachedInputTokens,
		OutputTokens:      a.OutputTokens + b.OutputTokens,
		ReasoningTokens:   a.ReasoningTokens + b.ReasoningTokens,
		TotalTokens:       a.TotalTokens + b.TotalTokens,
		Model:             b.Model,
	}
	if out.Model == "" {
		out.Model = a.Model
	}
	return out
}

func mergeWebSearchSources(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	seen := map[string]bool{}
	for _, source := range append(append([]string(nil), a...), b...) {
		source = strings.TrimSpace(source)
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		out = append(out, source)
	}
	return out
}