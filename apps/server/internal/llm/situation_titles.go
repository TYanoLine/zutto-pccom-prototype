package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

var _ BBSSituationTitlePlanner = StructuredOpenAIProvider{}

const maxTitleVariants = 3

type bbsSituationTitleWire struct {
	Titles map[string]string `json:"titles"`
}

type bbsSituationTitleVariantsWire struct {
	Titles map[string][]string `json:"titles"`
}

// situationTitleMaterial is how one article is shown to the title call: the
// writer comes first, then the background. Field order is the JSON order.
type situationTitleMaterial struct {
	EventID    string                 `json:"event_id"`
	CreatedAt  string                 `json:"created_at"`
	Author     situationTitleAuthor   `json:"author"`
	Background situationTitleBackdrop `json:"background"`
}

type situationTitleAuthor struct {
	Handle         string `json:"handle"`
	PersonaProfile string `json:"persona_profile,omitempty"`
}

type situationTitleBackdrop struct {
	Kind    string   `json:"kind"`
	Summary string   `json:"summary"`
	Facts   []string `json:"facts"`
}

func situationTitleMaterials(seeds []BBSSituationTitleSeed) []situationTitleMaterial {
	out := make([]situationTitleMaterial, 0, len(seeds))
	for _, s := range seeds {
		out = append(out, situationTitleMaterial{
			EventID:    s.EventID,
			CreatedAt:  s.CreatedAt,
			Author:     situationTitleAuthor{Handle: s.AuthorHandle, PersonaProfile: s.PersonaProfile},
			Background: situationTitleBackdrop{Kind: s.SituationKind, Summary: s.SituationSummary, Facts: s.SituationFacts},
		})
	}
	return out
}

// buildSituationTitlePrompt follows purpose + materials + output format. The
// background is said once to be context for what the writer did, not wording to
// copy; no list of things to avoid is added.
func buildSituationTitlePrompt(req BBSSituationTitleRequest) (string, error) {
	articles, err := json.Marshal(situationTitleMaterials(req.Articles))
	if err != nil {
		return "", err
	}
	covered, _ := json.Marshal(req.RecentSubjects)
	forms, _ := json.Marshal(selectTitleFormNotes(req.FormSeed))

	var b strings.Builder
	b.WriteString(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界で、すでに正本化されたSituationからBBSの記事一覧に表示するroot件名を作ります。
各人物が、確定済みSituationの話題をBBS一覧で伝える短い件名を、自分の投稿として付ける題名の形で書いてください。
題名は、一覧を眺める人がこの投稿を開くか決める短い言葉です。何の話かを題名だけで伝える必要があるとき（複数の対象が混在する板など）は、対象の名前を、題名の好きな位置に入れてください。

局: `)
	b.WriteString(req.HostName)
	fmt.Fprintf(&b, "\n地域: %s\n掲示板: %s\n板の範囲: %s\n世界日付: %s\n", req.HostRegion, req.BoardName, req.BoardScope, req.WorldDate)
	fmt.Fprintf(&b, "\n題名を付ける記事（author は書き手。background は、書き手が何をしたかを知る背景であり、語順や言い回しの手本ではありません）:\n%s\n", articles)
	fmt.Fprintf(&b, "\nすでに扱った題材（重複を避けるための一覧。順序は入れ替えてあり、語順や言い回しの手本ではありません）:\n%s\n", covered)
	if len(req.RecentFormFacts) > 0 {
		b.WriteString("\n直近の題名について測った事実:\n")
		for _, fact := range req.RecentFormFacts {
			b.WriteString("- " + fact + "\n")
		}
	}
	fmt.Fprintf(&b, "\n題名の書き方の幅を示す例（別の分野の題名で、話題の手本ではありません）:\n%s\n", forms)
	b.WriteString("\n上の例は、題名の形に幅があることを示すだけです。選ぶものでも真似るものでもなく、言葉の運びや長さを写す必要はありません。この記事の話にいちばん合う運びを、記事ごとに自分で考えてください。\n")
	if req.MultiVariant {
		fmt.Fprintf(&b, "\n出力は、記事ごとに構成の異なる案を最大%d案、配列で返します。各案は36文字以内・1行・Re:なしです。", maxTitleVariants)
	} else {
		b.WriteString("\n出力する件名は36文字以内・1行・Re:なしです。")
	}
	return b.String(), nil
}

func situationTitleSchema(req BBSSituationTitleRequest) (map[string]any, error) {
	properties := make(map[string]any, len(req.Articles))
	required := make([]string, 0, len(req.Articles))
	for _, article := range req.Articles {
		id := strings.TrimSpace(article.EventID)
		if id == "" {
			return nil, fmt.Errorf("situation title seed has empty event id")
		}
		if req.MultiVariant {
			// Strict schemas do not reliably support item-count limits, so the
			// cap of maxTitleVariants is applied when the response is read.
			properties[id] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
		} else {
			properties[id] = map[string]any{"type": "string"}
		}
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"titles": map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"titles"},
		"additionalProperties": false,
	}, nil
}

func validSituationTitle(subject string) bool {
	return subject != "" && utf8.RuneCountInString(subject) <= 36 && !strings.ContainsAny(subject, "\r\n") && !hasReplySubjectPrefix(subject)
}

// GenerateBBSSituationTitles performs wording only. The world event and Situation
// are already canonical; this pass must not invent what the post is about.
func (p StructuredOpenAIProvider) GenerateBBSSituationTitles(ctx context.Context, req BBSSituationTitleRequest) (BBSSituationTitleDraft, error) {
	if len(req.Articles) == 0 {
		return BBSSituationTitleDraft{}, nil
	}
	prompt, err := buildSituationTitlePrompt(req)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	schema, err := situationTitleSchema(req)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	maxTokens := 300 + len(req.Articles)*80
	if req.MultiVariant {
		maxTokens = 300 + len(req.Articles)*80*maxTitleVariants
	}
	if maxTokens > 3000 {
		maxTokens = 3000
	}
	traceFinish := beginDebugTrace(ctx, "件名", prompt)
	result, err := p.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_situation_titles", schema)
	traceFinish(result.Text, err)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	candidates, err := decodeSituationTitles(result.Text, req)
	if err != nil {
		return BBSSituationTitleDraft{}, err
	}
	out := make([]BBSSituationTitle, 0, len(req.Articles))
	for _, article := range req.Articles {
		list := candidates[article.EventID]
		title := BBSSituationTitle{EventID: article.EventID, Subject: list[0]}
		if req.MultiVariant {
			title.Candidates = list
		}
		out = append(out, title)
	}
	return BBSSituationTitleDraft{Titles: out, Usage: result.Usage}, nil
}

// decodeSituationTitles returns, per event, the valid titles in response order
// (at least one). A single-title response with MultiVariant off must be fully
// valid; with MultiVariant on, invalid variants are dropped and only an event
// with no valid variant is an error.
func decodeSituationTitles(text string, req BBSSituationTitleRequest) (map[string][]string, error) {
	raw := strings.TrimSpace(text)
	byEvent := make(map[string][]string, len(req.Articles))
	if req.MultiVariant {
		var wire bbsSituationTitleVariantsWire
		if err := json.Unmarshal([]byte(raw), &wire); err != nil {
			return nil, fmt.Errorf("decode situation title JSON: %w", err)
		}
		if len(wire.Titles) != len(req.Articles) {
			return nil, fmt.Errorf("situation title planner returned %d titles, want %d", len(wire.Titles), len(req.Articles))
		}
		for _, article := range req.Articles {
			variants, ok := wire.Titles[article.EventID]
			if !ok {
				return nil, fmt.Errorf("situation title planner omitted %q", article.EventID)
			}
			if len(variants) > maxTitleVariants {
				variants = variants[:maxTitleVariants]
			}
			for _, v := range variants {
				if v = strings.TrimSpace(v); validSituationTitle(v) {
					byEvent[article.EventID] = append(byEvent[article.EventID], v)
				}
			}
			if len(byEvent[article.EventID]) == 0 {
				return nil, fmt.Errorf("invalid situation title for %q: %q", article.EventID, variants)
			}
		}
		return byEvent, nil
	}
	var wire bbsSituationTitleWire
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return nil, fmt.Errorf("decode situation title JSON: %w", err)
	}
	if len(wire.Titles) != len(req.Articles) {
		return nil, fmt.Errorf("situation title planner returned %d titles, want %d", len(wire.Titles), len(req.Articles))
	}
	for _, article := range req.Articles {
		subject, ok := wire.Titles[article.EventID]
		subject = strings.TrimSpace(subject)
		if !ok || subject == "" {
			return nil, fmt.Errorf("situation title planner omitted %q", article.EventID)
		}
		if !validSituationTitle(subject) {
			return nil, fmt.Errorf("invalid situation title for %q: %q", article.EventID, subject)
		}
		byEvent[article.EventID] = []string{subject}
	}
	return byEvent, nil
}
