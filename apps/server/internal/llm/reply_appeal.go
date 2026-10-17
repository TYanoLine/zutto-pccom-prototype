package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

type appealItemEvaluation struct {
	ID     int64  `json:"id"`
	Appeal int    `json:"appeal"`
	Reason string `json:"reason"`
}

type appealResponsePayload struct {
	Evaluations []appealItemEvaluation `json:"evaluations"`
}

func buildReplyAppealPrompt(req world.ReplyAppealRequest) string {
	var sb strings.Builder
	sb.WriteString("あなたは1996年の日本の草の根BBS（パソコン通信局）における、スレッドごとの返信（レス）の付きやすさ・盛り上がりを評価するシミュレータです。\n\n")
	sb.WriteString("各ルート記事について、レスの件数（数値）を決めるのではなく、話題や状況に応じた【盛り上がりの段階 (appeal: 0〜4)】を判断してください。\n")
	sb.WriteString("数値への換算と実際の返信生成は世界シミュレーション側が後から行います。\n\n")
	sb.WriteString("段階の定義:\n")
	sb.WriteString("  0: ほぼ返信を呼ばない（独白、一方的な告知、完結した個人的日記、形式的な挨拶など）\n")
	sb.WriteString("  1: やや返信を呼びにくい\n")
	sb.WriteString("  2: 中位・基準（通常の雑談・情報提供など。この板の基準率に相当）\n")
	sb.WriteString("  3: やや返信を呼びやすい（共感を呼ぶ話題、意見を求めやすい話題など）\n")
	sb.WriteString("  4: 長いやり取りになりやすい（切実な質問、トラブル・不具合の相談、議論が発展する提起など）\n\n")
	sb.WriteString("重要な指示:\n")
	sb.WriteString("- すべてのルートを中位（2）に偏らせず、質問や相談は高め（3〜4）、独白や告知は低め（0〜1）にメリハリをつけて判断してください。\n")
	sb.WriteString("- 投稿時刻や活動できる住民の数、板の直近のレス件数傾向も考慮してください（テレホーダイ等の混雑時は賑わいやすいなど）。\n")
	sb.WriteString("- 各ルートごとに 1 文の簡潔な理由（reason）を添えてください。\n\n")

	sb.WriteString(fmt.Sprintf("対象局: %s\n", req.Host))
	sb.WriteString(fmt.Sprintf("対象板: %s (基準返信率: %.2f)\n", req.Board, req.BaseRate))
	sb.WriteString(fmt.Sprintf("総会員数: %d\n", req.Members))
	if len(req.RecentCounts) > 0 {
		sb.WriteString(fmt.Sprintf("直近スレッドのレス件数傾向: %v\n", req.RecentCounts))
	}
	sb.WriteString("\n対象ルート記事一覧:\n")
	for _, root := range req.Roots {
		postedAt := req.PostedAt[root.ID]
		active := req.ActiveByRoot[root.ID]
		timeStr := ""
		if !postedAt.IsZero() {
			timeStr = postedAt.Format("2006-01-02 15:04 (Mon)")
		}
		sb.WriteString(fmt.Sprintf("- [ID: %d] 投稿時刻: %s, 活動可能住民数: %d\n", root.ID, timeStr, active))
		sb.WriteString(fmt.Sprintf("  件名: %s\n", root.Subject))
		if root.Summary != "" {
			sb.WriteString(fmt.Sprintf("  状況要約: %s\n", root.Summary))
		}
	}
	sb.WriteString("\n必ずJSON形式で、各ルートの id, appeal (0〜4), reason を返してください。\n")
	return sb.String()
}

func replyAppealSchema() map[string]any {
	item := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":     map[string]any{"type": "integer"},
			"appeal": map[string]any{"type": "integer", "minimum": 0, "maximum": 4},
			"reason": map[string]any{"type": "string"},
		},
		"required":             []string{"id", "appeal", "reason"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evaluations": map[string]any{
				"type":  "array",
				"items": item,
			},
		},
		"required":             []string{"evaluations"},
		"additionalProperties": false,
	}
}

// JudgeReplyAppeal judges the qualitative appeal level for a batch of root posts using LLM structured output.
func (p StructuredOpenAIProvider) JudgeReplyAppeal(ctx context.Context, req world.ReplyAppealRequest) (world.ReplyAppealResult, error) {
	result := world.ReplyAppealResult{
		Appeal: make(map[int64]int, len(req.Roots)),
		Reason: make(map[int64]string, len(req.Roots)),
	}
	for _, r := range req.Roots {
		result.Appeal[r.ID] = 2
		result.Reason[r.ID] = "fallback baseline appeal"
	}
	if len(req.Roots) == 0 {
		return result, nil
	}

	prompt := buildReplyAppealPrompt(req)
	schema := replyAppealSchema()

	resp, err := p.responseTextWithJSONSchema(ctx, prompt, "low", 2500, "bbs_reply_appeal", schema)
	if err != nil {
		log.Printf("BBS reply appeal evaluation failed: %v, falling back to baseline appeal=2", err)
		return result, err
	}

	var payload appealResponsePayload
	if err := json.Unmarshal([]byte(resp.Text), &payload); err != nil {
		log.Printf("BBS reply appeal unmarshal failed: %v, falling back to baseline appeal=2", err)
		return result, err
	}

	seen := make(map[int64]bool, len(req.Roots))
	validRoots := make(map[int64]bool, len(req.Roots))
	for _, r := range req.Roots {
		validRoots[r.ID] = true
	}

	for _, eval := range payload.Evaluations {
		if !validRoots[eval.ID] {
			continue
		}
		seen[eval.ID] = true
		if eval.Appeal < 0 || eval.Appeal > 4 {
			log.Printf("BBS reply appeal out of range for post %d: appeal=%d, falling back to 2", eval.ID, eval.Appeal)
			result.Appeal[eval.ID] = 2
			result.Reason[eval.ID] = "fallback baseline appeal (out of range)"
			continue
		}
		result.Appeal[eval.ID] = eval.Appeal
		result.Reason[eval.ID] = strings.TrimSpace(eval.Reason)
	}

	for _, r := range req.Roots {
		if !seen[r.ID] {
			log.Printf("BBS reply appeal missing for post %d, falling back to 2", r.ID)
			result.Appeal[r.ID] = 2
			result.Reason[r.ID] = "fallback baseline appeal (omitted from LLM response)"
		}
	}

	return result, nil
}
