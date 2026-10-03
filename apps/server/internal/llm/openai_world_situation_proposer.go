package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var _ BBSWorldSituationProposer = StructuredOpenAIProvider{}

// ErrBBSWorldSituationOutputTruncated identifies invalid structured JSON whose
// response consumed its entire output-token budget. The planner can retry
// fewer shells without relaxing any world or persistence invariants.
var ErrBBSWorldSituationOutputTruncated = errors.New("BBS Situation output token budget exhausted")

// bbsWorldSituationWire is the same for every root. The world layer does not
// pick a post type, so there are no type-dependent fields.
type bbsWorldSituationWire struct {
	ObjectClass string   `json:"object_class"`
	Occurrence  string   `json:"occurrence"`
	PostContent string   `json:"post_content"`
	NoveltyKey  string   `json:"novelty_key"`
	MustNot     []string `json:"must_not"`
}

type bbsWorldSituationProposalWire struct {
	Situations map[string]bbsWorldSituationWire `json:"situations"`
}

// compactSituationEvent is the per-root material shown to the model. It
// deliberately has no topic/domain routing key and no post type: what may be
// posted, and in what form, is decided from the board name and scope alone.
type compactSituationEvent struct {
	EventID        string   `json:"event_id"`
	BoardID        string   `json:"board_id"`
	BoardName      string   `json:"board_name"`
	BoardScope     string   `json:"board_scope,omitempty"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	PersonaProfile string   `json:"persona_profile,omitempty"`
	ExistingFacts  []string `json:"existing_facts,omitempty"`
}

func compactSituationEvents(events []BBSWorldWindowEvent) []compactSituationEvent {
	out := make([]compactSituationEvent, 0, len(events))
	for _, event := range events {
		out = append(out, compactSituationEvent{
			EventID:        event.EventID,
			BoardID:        event.BoardID,
			BoardName:      event.BoardName,
			BoardScope:     event.BoardScope,
			AuthorHandle:   event.AuthorHandle,
			CreatedAt:      event.CreatedAt,
			PersonaProfile: event.PersonaProfile,
			ExistingFacts:  append([]string(nil), event.ExistingFacts...),
		})
	}
	return out
}

func (p StructuredOpenAIProvider) GenerateBBSWorldSituationProposals(ctx context.Context, req BBSWorldSituationProposalRequest) (BBSWorldSituationProposalDraft, error) {
	if err := validateBBSWorldWindowEventIDs(req.Events); err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	if len(req.Events) == 0 {
		return BBSWorldSituationProposalDraft{}, nil
	}
	for _, event := range req.Events {
		if !isStandaloneWorldWindowRoot(event) {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer received non-standalone root %q", event.EventID)
		}
	}

	eventsJSON, err := json.Marshal(compactSituationEvents(req.Events))
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	avoidJSON, err := json.Marshal(req.AvoidSituations)
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	historicalFacts := "(none supplied)"
	if len(req.HistoricalFacts) > 0 {
		historicalFacts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	historicalPolicy := bbsWorldSituationHistoricalPolicy(req)
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(none supplied)"
	}

	prompt := fmt.Sprintf(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界を、利用者が見ていない間も続いている永続世界としてシミュレーションしています。
あなたの出力は、World Engineがすでに選んだroot投稿枠について「この投稿が書かれる元になるSituation」として正本化され、後段のBBS件名と記事本文を生成する材料になります。記事本文そのものではありません。

world-selected roots には、投稿者、日時、掲示板名（board_name）、掲示板の用途（board_scope）、人物情報、既存世界事実が材料として入っています。
board_name と board_scope は、その掲示板に投稿してよい話題の範囲を表します。board_scope は利用者には見えない内部の方向性です。各rootのSituationは、必ずこの板の範囲の内側で、小さく具体的に1件ずつ作ってください。
投稿の形（名乗り、挨拶、歓迎、連絡、報告、問いかけ、意見、体験談など）はWorldが決めていません。その板に実際に載る自然な形を、board_name と board_scope から判断してください。出来事や体験談を無理に作る必要はありません。rootどうしが同じ形（すべて問いかけ、すべて体験談など）に偏らないよう、板の範囲の中で形と話題を散らしてください。
persona_profile は投稿者の暮らしぶりや語り口の材料であり、話題を決める根拠にはなりません。投稿者の関心が板の範囲の外にあっても、範囲外の話題は選ばないでください。
recent/avoid material は重複を避けるための材料です。避けるときも板の範囲の外へは出ず、範囲内の別の切り口を選んでください。
入力された世界事実は前提として扱い、別rootの出来事とは混ぜないでください。

各rootの出力項目:
- object_class: 投稿が扱う対象を短く。
- occurrence: この投稿が書かれる状況。出来事でなくてよい。
- post_content: 投稿で実際に述べる中身（名乗る、挨拶する、伝える、尋ねる、など）を1〜2文で。本文そのものは書かない。
- novelty_key: 他のrootと重ならない識別子。
- must_not: 本文で踏み外したくないことを最大2件。

historical material policy: %s

世界日付: %s
局: %s
地域: %s
期間: %s .. %s

historical material:
%s

recent BBS state（文脈・重複回避の材料）:
%s

world-selected roots:
%s

recent/avoid material:
%s`, historicalPolicy, req.WorldDate, req.HostName, req.HostRegion, req.WindowStart, req.WindowEnd, historicalFacts, recent, string(eventsJSON), string(avoidJSON))

	maxTokens := bbsWorldSituationMaxTokens(len(req.Events))
	traceFinish := beginDebugTrace(ctx, "Situation", prompt)
	producer := p.withWorldWindowHTTPTimeout()
	result, err := producer.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_world_situations", bbsWorldSituationProposalSchema(req.Events))
	traceFinish(result.Text, err)
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	var wire bbsWorldSituationProposalWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		decodeErr := fmt.Errorf(
			"decode BBS world-situation JSON: %w (output_chars=%d output_tokens=%d max_tokens=%d)",
			err, len([]rune(result.Text)), result.Usage.OutputTokens, maxTokens,
		)
		if result.Usage.OutputTokens >= maxTokens {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("%w: %w", ErrBBSWorldSituationOutputTruncated, decodeErr)
		}
		return BBSWorldSituationProposalDraft{}, decodeErr
	}
	if len(wire.Situations) != len(req.Events) {
		return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer returned %d situations, want %d", len(wire.Situations), len(req.Events))
	}

	out := make([]BBSWorldSituationDraft, 0, len(req.Events))
	for _, event := range req.Events {
		id := strings.TrimSpace(event.EventID)
		value, ok := wire.Situations[id]
		if !ok {
			return BBSWorldSituationProposalDraft{}, fmt.Errorf("situation proposer omitted required event key %q", id)
		}
		out = append(out, BBSWorldSituationDraft{
			EventID:     id,
			ObjectClass: strings.TrimSpace(value.ObjectClass),
			Occurrence:  strings.TrimSpace(value.Occurrence),
			PostContent: strings.TrimSpace(value.PostContent),
			NoveltyKey:  strings.TrimSpace(value.NoveltyKey),
			MustNot:     cleanStringList(value.MustNot),
		})
	}
	return BBSWorldSituationProposalDraft{Situations: out, Usage: result.Usage}, nil
}

func bbsWorldSituationMaxTokens(eventCount int) int {
	if eventCount < 1 {
		return 1200
	}
	// Structured JSON and model reasoning share one completion-token budget.
	// Reserve room for both without asking the model to produce more prose:
	// 1 event: 4k; 4 events: 8k; the normal 6-event chunk: 12k.
	// The planner still splits pending chunks on budget truncation. This is a
	// request ceiling, not a target output length or a fixed token charge.
	maxTokens := eventCount * 2000
	if maxTokens < 4000 {
		maxTokens = 4000
	}
	if maxTokens > 12000 {
		maxTokens = 12000
	}
	return maxTokens
}

// bbsWorldSituationProposalSchema gives every root the same fields. The post
// type is not selected by the world layer, so nothing here depends on one.
func bbsWorldSituationProposalSchema(events []BBSWorldWindowEvent) map[string]any {
	properties := make(map[string]any, len(events))
	required := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		properties[id] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"object_class": map[string]any{"type": "string"},
				"occurrence":   map[string]any{"type": "string"},
				"post_content": map[string]any{"type": "string"},
				"novelty_key":  map[string]any{"type": "string"},
				"must_not":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2},
			},
			"required":             []string{"object_class", "occurrence", "post_content", "novelty_key", "must_not"},
			"additionalProperties": false,
		}
		required = append(required, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"situations": map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
		"required":             []string{"situations"},
		"additionalProperties": false,
	}
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
