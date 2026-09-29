package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

var _ BBSSituationTitlePlanner = StructuredOpenAIProvider{}

type bbsSituationTitleWire struct {
	Titles map[string]string `json:"titles"`
}

// GenerateBBSSituationTitles performs wording only. The world event and Situation
// are already canonical; this pass must not invent what the post is about.
func (p StructuredOpenAIProvider) GenerateBBSSituationTitles(ctx context.Context, req BBSSituationTitleRequest) (BBSSituationTitleDraft, error) {
	if len(req.Articles) == 0 {
		return BBSSituationTitleDraft{}, nil
	}
	input, err := json.Marshal(req.Articles)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	recent, _ := json.Marshal(req.RecentSubjects)
	prompt := fmt.Sprintf(`1996年前後の日本の草の根パソコン通信BBSです。
以下の各記事について、すでに確定しているSituationを表す自然なroot件名だけを書いてください。

局: %s
地域: %s
掲示板: %s
板の範囲: %s
世界日付: %s

ルール:
- Situationと人物プロフィールは事実です。件名のために新しい出来事・対象・評価・固有名詞を足さない。
- 本人が件名欄へ普通に入力しそうな日本語にする。説明文や検索見出しに整える必要はない。
- 36文字以内、1行、Re:なし。
- 同じバッチや直近件名と同じ言い回しを機械的に繰り返さない。

直近のroot件名:
%s

確定済み記事:
%s`, req.HostName, req.HostRegion, req.BoardName, req.BoardScope, req.WorldDate, string(recent), string(input))

	properties := make(map[string]any, len(req.Articles))
	required := make([]string, 0, len(req.Articles))
	for _, article := range req.Articles {
		id := strings.TrimSpace(article.EventID)
		if id == "" {
			return BBSSituationTitleDraft{}, fmt.Errorf("situation title seed has empty event id")
		}
		properties[id] = map[string]any{"type": "string"}
		required = append(required, id)
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"titles": map[string]any{
				"type": "object",
				"properties": properties,
				"required": required,
				"additionalProperties": false,
			},
		},
		"required": []string{"titles"},
		"additionalProperties": false,
	}
	maxTokens := 300 + len(req.Articles)*80
	if maxTokens > 3000 {
		maxTokens = 3000
	}
	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_situation_titles", schema)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	var wire bbsSituationTitleWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		return BBSSituationTitleDraft{}, fmt.Errorf("decode situation title JSON: %w", err)
	}
	if len(wire.Titles) != len(req.Articles) {
		return BBSSituationTitleDraft{}, fmt.Errorf("situation title planner returned %d titles, want %d", len(wire.Titles), len(req.Articles))
	}
	seen := map[string]bool{}
	out := make([]BBSSituationTitle, 0, len(req.Articles))
	for _, article := range req.Articles {
		subject, ok := wire.Titles[article.EventID]
		subject = strings.TrimSpace(subject)
		if !ok || subject == "" {
			return BBSSituationTitleDraft{}, fmt.Errorf("situation title planner omitted %q", article.EventID)
		}
		if utf8.RuneCountInString(subject) > 36 || strings.ContainsAny(subject, "\r\n") || hasReplySubjectPrefix(subject) {
			return BBSSituationTitleDraft{}, fmt.Errorf("invalid situation title for %q: %q", article.EventID, subject)
		}
		key := strings.ToLower(subject)
		if seen[key] {
			return BBSSituationTitleDraft{}, fmt.Errorf("duplicate situation title %q", subject)
		}
		seen[key] = true
		out = append(out, BBSSituationTitle{EventID: article.EventID, Subject: subject})
	}
	return BBSSituationTitleDraft{Titles: out, Usage: result.Usage}, nil
}
