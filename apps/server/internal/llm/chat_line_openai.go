package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var _ ChatLineWriter = StructuredOpenAIProvider{}

// chatLineKinds tells the model what the World selected, in the speaker's
// terms. The keys are chatslot's kinds.
var chatLineKinds = map[string]string{
	"greet":    "部屋に入ったところで、中にいる人たちに軽くあいさつする",
	"welcome":  "いま部屋に入ってきた addressee を迎える",
	"reply":    "addressee の直前の発言（transcript の該当行）に応じる",
	"talk":     "部屋の流れの中で、自分から何か一言言う",
	"farewell": "そろそろ部屋を出るので、そう告げる",
	"sendoff":  "抜けると言った addressee を見送る",
	"news":     "event に書かれた addressee の最近の出来事を、知り合いとして話題にする",
}

func chatLineSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"line": map[string]any{"type": "string"},
		},
		"required":             []string{"line"},
		"additionalProperties": false,
	}
}

// BuildChatLinePrompt is the prompt of one chat line: purpose, materials and
// the era frame.
func BuildChatLinePrompt(req ChatLineRequest) string {
	max := req.MaxCells
	if max <= 0 {
		max = DefaultChatLineCells
	}
	req.MaxCells = max
	materials, _ := json.MarshalIndent(req, "", "  ")
	act := chatLineKinds[req.Kind]
	if act == "" {
		act = req.Kind
	}
	weekdays := []string{"日", "月", "火", "水", "木", "金", "土"}
	when := fmt.Sprintf("%d年%d月%d日(%s) %02d:%02d", req.WorldTime.Year(), int(req.WorldTime.Month()), req.WorldTime.Day(),
		weekdays[req.WorldTime.Weekday()], req.WorldTime.Hour(), req.WorldTime.Minute())

	var b strings.Builder
	b.WriteString("あなたはパソコン通信のホスト局をシミュレートする世界の中で、チャット室にいる一人の人物（speaker）が次に打ち込む 1 行を書く。\n")
	b.WriteString("誰がいつ何に対して話すかは世界がすでに決めている。あなたはその 1 行の言葉だけを決める。\n\n")
	fmt.Fprintf(&b, "世界の日時: %s。これが speaker にとっての現在で、これより後の出来事・製品・言葉は存在しない。\n", when)
	fmt.Fprintf(&b, "この 1 行ですること: %s。\n\n", act)
	b.WriteString("材料（JSON）:\n")
	b.Write(materials)
	b.WriteString("\n\n書き方:\n")
	b.WriteString("- speaker.summary の人物と口調で、当時のパソコン通信のチャットらしく短く打つ。顔文字や (笑) は人によって使うが、使いすぎない。\n")
	b.WriteString("- relation は addressee との距離。初対面ならそれなりに、常連仲間なら砕けて。\n")
	b.WriteString("- transcript はこの部屋で speaker が見てきた発言（古い順）。同じことを繰り返さない。\n")
	b.WriteString("- event があるときは、そこに書かれたこと（掲示板と題）だけを根拠にする。内容を作らない。\n")
	fmt.Fprintf(&b, "- 改行しない。ハンドル名を行頭に付けない（画面が付ける）。全角 1 文字を 2 として %d 以内。\n", max)
	b.WriteString("- 出力は JSON の {\"line\": \"...\"} だけ。\n\n")
	b.WriteString(DiegeticWorldFrame)
	return b.String()
}

// WriteChatLine writes one chat line with a strict JSON schema call.
func (p StructuredOpenAIProvider) WriteChatLine(ctx context.Context, req ChatLineRequest) (string, error) {
	prompt := BuildChatLinePrompt(req)
	finish := beginDebugTrace(ctx, "チャットの発言", prompt)
	result, err := p.responseTextWithJSONSchemaReasoning(ctx, prompt, "low", "low", 1200, "chat_line", chatLineSchema())
	if err != nil {
		finish("", err)
		return "", err
	}
	finish(result.Text, nil)
	var out struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &out); err != nil {
		return "", fmt.Errorf("chat line: decode response: %w", err)
	}
	return out.Line, nil
}
