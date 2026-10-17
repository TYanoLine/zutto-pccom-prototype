package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

var _ BBSRootDraftPlanner = StructuredOpenAIProvider{}

// ErrBBSWorldSituationOutputTruncated identifies invalid structured JSON whose
// response consumed its entire output-token budget. The planner can retry
// fewer shells without relaxing any world or persistence invariants.
var ErrBBSWorldSituationOutputTruncated = errors.New("BBS Situation output token budget exhausted")

// BBSRootDraft is the canonical micro-situation and title candidates behind one root post.
type BBSRootDraft struct {
	EventID           string   `json:"event_id"`
	ObjectClass       string   `json:"object_class"`
	Occurrence        string   `json:"occurrence"`
	PostContent       string   `json:"post_content"`
	NoveltyKey        string   `json:"novelty_key"`
	MustNot           []string `json:"must_not"`
	Topics            []string `json:"topics,omitempty"`
	Titles            []string `json:"titles"`
	ReplyAppeal       int      `json:"reply_appeal"`
	ReplyAppealReason string   `json:"reply_appeal_reason"`
	HasReplyAppeal    bool     `json:"has_reply_appeal"`
}

// BBSRootDraftResult contains all generated root drafts for a batch chunk.
type BBSRootDraftResult struct {
	Roots map[string]BBSRootDraft
	Usage TokenUsage
}

// BBSRootDraftRequest bundles inputs for both Situation planning and root title generation.
type BBSRootDraftRequest struct {
	HostName                      string
	HostRegion                    string
	HostSoftware                  string
	BoardID                       string
	BoardName                     string
	BoardScope                    string
	WorldDate                     string
	WindowStart                   string
	WindowEnd                     string
	AllowModelHistoricalMemory    bool
	PreferConcreteHistoricalNames bool
	HistoricalFacts               []string
	RecentBBSState                string
	Events                        []BBSWorldWindowEvent
	AvoidSituations               []string

	RecentSubjects  []string
	RecentFormFacts []string
	FormSeed        string
	MultiVariant    bool

	BaseReplyRate     float64
	Members           int
	RecentReplyCounts []int
}

// BBSRootDraftPlanner generates combined Situation and Title candidates in a single LLM call.
type BBSRootDraftPlanner interface {
	GenerateBBSRootDrafts(context.Context, BBSRootDraftRequest) (BBSRootDraftResult, error)
}

const maxTitleVariants = 3

type compactSituationEvent struct {
	EventID         string   `json:"event_id"`
	BoardID         string   `json:"board_id"`
	BoardName       string   `json:"board_name"`
	BoardScope      string   `json:"board_scope,omitempty"`
	AuthorHandle    string   `json:"author_handle"`
	CreatedAt       string   `json:"created_at"`
	ActiveResidents int      `json:"active_residents,omitempty"`
	PersonaProfile  string   `json:"persona_profile,omitempty"`
	ExistingFacts   []string `json:"existing_facts,omitempty"`
}

func compactSituationEvents(events []BBSWorldWindowEvent) []compactSituationEvent {
	out := make([]compactSituationEvent, 0, len(events))
	for _, event := range events {
		out = append(out, compactSituationEvent{
			EventID:         event.EventID,
			BoardID:         event.BoardID,
			BoardName:       event.BoardName,
			BoardScope:      event.BoardScope,
			AuthorHandle:    event.AuthorHandle,
			CreatedAt:       event.CreatedAt,
			ActiveResidents: event.ActiveResidents,
			PersonaProfile:  event.PersonaProfile,
			ExistingFacts:   append([]string(nil), event.ExistingFacts...),
		})
	}
	return out
}

func bbsWorldSituationMaxTokens(eventCount int) int {
	if eventCount < 1 {
		return 1200
	}
	maxTokens := eventCount * 2000
	if maxTokens < 4000 {
		maxTokens = 4000
	}
	if maxTokens > 12000 {
		maxTokens = 12000
	}
	return maxTokens
}

func bbsWorldSituationHistoricalPolicy(req BBSWorldSituationProposalRequest) string {
	if req.AllowModelHistoricalMemory {
		if req.PreferConcreteHistoricalNames {
			return "実在の対象が自然なら、世界日時点の日本で存在を確信できる具体名も材料として使えます。"
		}
		return "必要なら、世界日時点の日本で存在を確信できる実在名を材料として使えます。"
	}
	return "実在名は historical material または existing_facts に与えられたものを材料として使ってください。"
}

type bbsRootDraftWire struct {
	ObjectClass       string   `json:"object_class"`
	Occurrence        string   `json:"occurrence"`
	PostContent       string   `json:"post_content"`
	NoveltyKey        string   `json:"novelty_key"`
	MustNot           []string `json:"must_not"`
	Titles            []string `json:"titles"`
	ReplyAppeal       *int     `json:"reply_appeal"`
	ReplyAppealReason string   `json:"reply_appeal_reason"`
}

type bbsRootDraftProposalWire struct {
	Roots map[string]bbsRootDraftWire `json:"roots"`
}

func validSituationTitle(subject string) bool {
	return subject != "" && utf8.RuneCountInString(subject) <= 36 && !strings.ContainsAny(subject, "\r\n") && !hasReplySubjectPrefix(subject)
}


func buildRootDraftPrompt(req BBSRootDraftRequest) (string, error) {
	eventsJSON, err := json.Marshal(compactSituationEvents(req.Events))
	if err != nil {
		return "", err
	}
	avoidJSON, err := json.Marshal(req.AvoidSituations)
	if err != nil {
		return "", err
	}
	coveredJSON, _ := json.Marshal(req.RecentSubjects)
	formsJSON, _ := json.Marshal(selectTitleFormNotes(req.FormSeed))

	historicalFacts := "(none supplied)"
	if len(req.HistoricalFacts) > 0 {
		historicalFacts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	historicalPolicy := bbsWorldSituationHistoricalPolicy(BBSWorldSituationProposalRequest{
		AllowModelHistoricalMemory:    req.AllowModelHistoricalMemory,
		PreferConcreteHistoricalNames: req.PreferConcreteHistoricalNames,
	})
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(none supplied)"
	}

	var b strings.Builder
	b.WriteString(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界を、利用者が見ていない間も続いている永続世界としてシミュレーションしています。
あなたの出力は、World Engineがすでに選んだroot投稿枠について「この投稿が書かれる元になるSituation」と「BBSの記事一覧に表示するroot件名」を統合して出力するものです。記事本文そのものではありません。

world-selected roots には、投稿者、日時、掲示板名（board_name）、掲示板の用途（board_scope）、人物情報、既存世界事実が材料として入っています。
board_name と board_scope は、その掲示板に投稿してよい話題の範囲を表します。board_scope は利用者には見えない内部の方向性です。各rootのSituationは、必ずこの板の範囲の内側で、小さく具体的に1件ずつ作ってください。
投稿の形（名乗り、挨拶、歓迎、連絡、報告、問いかけ、意見、体験談など）はWorldが決めていません。その板に実際に載る自然な形を、board_name と board_scope から判断してください。出来事や体験談を無理に作る必要はありません。形や話題は、板の範囲の中で自然に混ざればよい。無理に散らさなくてよい。
persona_profile は投稿者の暮らしぶりや語り口の材料であり、話題を決める根拠にはなりません。投稿者の関心が板の範囲の外にあっても、範囲外の話題は選ばないでください。
recent/avoid material は文脈の参考です。同じ話題や続きの話が出てもかまいません。避けるときも板の範囲の外へは出ず、範囲内の別の切り口を選んでください。
入力された世界事実は前提として扱い、別rootの出来事とは混ぜないでください。

各rootについて、確定済みSituationの話題をBBS一覧で伝える短い件名（題名）を、自分の投稿として付ける題名の形で書いてください。
題名は、一覧を眺める人がこの投稿を開くか決める短い言葉です。何の話かが題名だけで分かるよう、対象の名前（作品名・機種名・場所など）を、先頭でなくてよいので題名のどこかに入れてください。

スレッドごとの返信（レス）の付きやすさ・盛り上がりについて、レスの件数（数値）を決めるのではなく、話題や状況に応じた【盛り上がりの段階 (appeal: 0〜4)】を判断してください。件名と状況要約は、あなたが先に書いた Situation と件名を指します。数値への換算と実際の返信生成は世界シミュレーション側が後から行います。

段階の定義:
  0: ほぼ返信を呼ばない（独白、一方的な告知、完結した個人的日記、形式的な挨拶など）
  1: やや返信を呼びにくい
  2: 中位・基準（通常の雑談・情報提供など。この板の基準率に相当）
  3: やや返信を呼びやすい（共感を呼ぶ話題、意見を求めやすい話題など）
  4: 長いやり取りになりやすい（切実な質問、トラブル・不具合の相談、議論が発展する提起など）

重要な指示:
- すべてのルートを中位（2）に偏らせず、質問や相談は高め（3〜4）、独白や告知は低め（0〜1）にメリハリをつけて判断してください。
- 投稿時刻や活動できる住民の数（active_residents）、板の直近のレス件数傾向も考慮してください（テレホーダイ等の混雑時は賑わいやすいなど）。

各rootの出力項目（必ずSituationの項目を書いてから、そのあとにtitlesを出力し、最後に盛り上がりの判断を出力してください）:
- object_class: 投稿が扱う対象を短く。
- occurrence: この投稿が書かれる状況。出来事でなくてよい。
- post_content: 投稿で実際に述べる中身（名乗る、挨拶する、伝える、尋ねる、など）を1〜2文で。本文そのものは書かない。
- novelty_key: 他のrootと重ならない識別子。
- must_not: 本文で踏み外したくないことを最大2件。
- titles: 確定済みSituationの話題をBBS一覧で伝える短い件名の配列。各案は36文字以内・1行・Re:なし。
- reply_appeal: スレッドごとの返信（レス）の付きやすさ・盛り上がりの段階 (appeal: 0〜4)。
- reply_appeal_reason: その段階と判断した 1 文の簡潔な理由。
`)

	if req.MultiVariant {
		fmt.Fprintf(&b, "\n出力は、記事ごとに構成の異なる案を最大%d案、配列で返します。案のうち少なくとも2案は対象の名前を入れ、そのうち1案以上は名前で始めない形にしてください。\n", maxTitleVariants)
	} else {
		b.WriteString("\n出力する件名は1案、36文字以内・1行・Re:なしです。\n")
	}

	fmt.Fprintf(&b, "\nhistorical material policy: %s\n", historicalPolicy)
	fmt.Fprintf(&b, "世界日付: %s\n局: %s\n地域: %s\n期間: %s .. %s\n掲示板: %s (基準返信率: %.2f)\n板の範囲: %s\n総会員数: %d\n",
		req.WorldDate, req.HostName, req.HostRegion, req.WindowStart, req.WindowEnd, req.BoardName, req.BaseReplyRate, req.BoardScope, req.Members)
	if len(req.RecentReplyCounts) > 0 {
		fmt.Fprintf(&b, "直近スレッドのレス件数傾向: %v\n", req.RecentReplyCounts)
	}

	fmt.Fprintf(&b, "\nhistorical material:\n%s\n", historicalFacts)
	fmt.Fprintf(&b, "\nrecent BBS state（文脈・参考の材料）:\n%s\n", recent)
	fmt.Fprintf(&b, "\nworld-selected roots:\n%s\n", string(eventsJSON))
	fmt.Fprintf(&b, "\nrecent/avoid material:\n%s\n", string(avoidJSON))
	fmt.Fprintf(&b, "\nすでに扱った題材（重複の参考一覧）:\n%s\n", string(coveredJSON))

	if len(req.RecentFormFacts) > 0 {
		b.WriteString("\n直近の題名について測った事実:\n")
		for _, fact := range req.RecentFormFacts {
			b.WriteString("- " + fact + "\n")
		}
	}
	fmt.Fprintf(&b, "\n題名の書き方の幅を示す例（別の分野の題名で、話題の手本ではありません）:\n%s\n", string(formsJSON))
	b.WriteString("\n上の例は、題名の形に幅があることを示すだけです。選ぶものでも真似るものでもなく、言葉の運びや長さを写す必要はありません。この記事の話にいちばん合う運びを、記事ごとに自分で考えてください。\n")

	return b.String(), nil
}

func bbsRootDraftSchema(events []BBSWorldWindowEvent) map[string]any {
	properties := make(map[string]any, len(events))
	required := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		properties[id] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"object_class":        map[string]any{"type": "string"},
				"occurrence":          map[string]any{"type": "string"},
				"post_content":        map[string]any{"type": "string"},
				"novelty_key":         map[string]any{"type": "string"},
				"must_not":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2},
				"titles":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"reply_appeal":        map[string]any{"type": "integer", "minimum": 0, "maximum": 4},
				"reply_appeal_reason": map[string]any{"type": "string"},
			},
			"required":             []string{"object_class", "occurrence", "post_content", "novelty_key", "must_not", "titles", "reply_appeal", "reply_appeal_reason"},
			"additionalProperties": false,
		}
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roots": map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"roots"},
		"additionalProperties": false,
	}
}

// GenerateBBSRootDrafts implements BBSRootDraftPlanner for StructuredOpenAIProvider.
func (p StructuredOpenAIProvider) GenerateBBSRootDrafts(ctx context.Context, req BBSRootDraftRequest) (BBSRootDraftResult, error) {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return BBSRootDraftResult{}, err
	}
	if len(req.Events) == 0 {
		return BBSRootDraftResult{Roots: make(map[string]BBSRootDraft)}, nil
	}
	for _, event := range req.Events {
		if !isStandaloneWorldWindowRoot(event) {
			return BBSRootDraftResult{}, fmt.Errorf("root draft planner received non-standalone root %q", event.EventID)
		}
	}

	prompt, err := buildRootDraftPrompt(req)
	if err != nil {
		return BBSRootDraftResult{}, err
	}

	maxTokens := bbsWorldSituationMaxTokens(len(req.Events)) + len(req.Events)*200
	traceFinish := beginDebugTrace(ctx, "Situation+件名", prompt)
	producer := p.withWorldWindowHTTPTimeout()
	result, err := producer.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_root_drafts", bbsRootDraftSchema(req.Events))
	traceFinish(result.Text, err)
	if err != nil {
		if strings.Contains(err.Error(), "max_tokens") {
			return BBSRootDraftResult{}, fmt.Errorf("%w: %w", ErrBBSWorldSituationOutputTruncated, err)
		}
		return BBSRootDraftResult{}, err
	}

	var wire bbsRootDraftProposalWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		decodeErr := fmt.Errorf(
			"decode BBS root draft JSON: %w (output_chars=%d output_tokens=%d max_tokens=%d)",
			err, len([]rune(result.Text)), result.Usage.OutputTokens, maxTokens,
		)
		if result.Usage.OutputTokens >= maxTokens {
			return BBSRootDraftResult{}, fmt.Errorf("%w: %w", ErrBBSWorldSituationOutputTruncated, decodeErr)
		}
		return BBSRootDraftResult{}, decodeErr
	}

	out := make(map[string]BBSRootDraft, len(req.Events))
	for _, event := range req.Events {
		id := strings.TrimSpace(event.EventID)
		value, ok := wire.Roots[id]
		if !ok {
			return BBSRootDraftResult{}, fmt.Errorf("root draft planner omitted required event key %q", id)
		}

		// Validate Situation components
		objClass := strings.TrimSpace(value.ObjectClass)
		occurrence := strings.TrimSpace(value.Occurrence)
		postContent := strings.TrimSpace(value.PostContent)
		noveltyKey := strings.TrimSpace(value.NoveltyKey)
		if objClass == "" || occurrence == "" || postContent == "" {
			return BBSRootDraftResult{}, fmt.Errorf("root draft %s returned incomplete situation fields", id)
		}
		if len(value.MustNot) > 2 {
			return BBSRootDraftResult{}, fmt.Errorf("root draft %s returned %d must_not constraints (max 2)", id, len(value.MustNot))
		}

		// Validate Titles
		validTitles := make([]string, 0, len(value.Titles))
		for _, t := range value.Titles {
			t = strings.TrimSpace(t)
			if validSituationTitle(t) {
				validTitles = append(validTitles, t)
			}
		}
		if len(validTitles) == 0 {
			return BBSRootDraftResult{}, fmt.Errorf("root draft %s returned no valid titles (all rejected by criteria)", id)
		}
		if req.MultiVariant && len(validTitles) > maxTitleVariants {
			validTitles = validTitles[:maxTitleVariants]
		}
		if !req.MultiVariant && len(validTitles) > 1 {
			validTitles = validTitles[:1]
		}

		// Validate Appeal
		appeal := 2
		reason := "fallback:no_appeal"
		hasAppeal := false
		if value.ReplyAppeal == nil {
			log.Printf("BBS root draft %s missing reply_appeal; fallback to 2", id)
			reason = "fallback:missing_appeal"
		} else if *value.ReplyAppeal < 0 || *value.ReplyAppeal > 4 {
			log.Printf("BBS root draft %s reply_appeal out of bounds (%d); fallback to 2", id, *value.ReplyAppeal)
			reason = "fallback:invalid_appeal"
		} else {
			appeal = *value.ReplyAppeal
			reason = strings.TrimSpace(value.ReplyAppealReason)
			hasAppeal = true
		}

		out[id] = BBSRootDraft{
			EventID:           id,
			ObjectClass:       objClass,
			Occurrence:        occurrence,
			PostContent:       postContent,
			NoveltyKey:        noveltyKey,
			MustNot:           cleanStringList(value.MustNot),
			Titles:            validTitles,
			ReplyAppeal:       appeal,
			ReplyAppealReason: reason,
			HasReplyAppeal:    hasAppeal,
		}
	}

	return BBSRootDraftResult{Roots: out, Usage: result.Usage}, nil
}
