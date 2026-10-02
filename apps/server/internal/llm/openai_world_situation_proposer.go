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

type bbsWorldSituationWire struct {
	ObjectClass      string   `json:"object_class"`
	ChangeClass      string   `json:"change_class"`
	Occurrence       string   `json:"occurrence"`
	Observation      string   `json:"observation,omitempty"`
	Experience       string   `json:"experience,omitempty"`
	Result           string   `json:"result,omitempty"`
	Stance           string   `json:"stance,omitempty"`
	Basis            string   `json:"basis,omitempty"`
	AttemptedActions string   `json:"attempted_actions,omitempty"`
	PracticalPoint   string   `json:"practical_point,omitempty"`
	Question         string   `json:"question,omitempty"`
	NoveltyKey       string   `json:"novelty_key"`
	MustNot          []string `json:"must_not"`
}

type bbsWorldSituationProposalWire struct {
	Situations map[string]bbsWorldSituationWire `json:"situations"`
}

// compactSituationEvent is the per-root material shown to the model. It
// deliberately has no topic/domain routing key: what may be posted is decided
// from the board name and scope alone.
type compactSituationEvent struct {
	EventID        string   `json:"event_id"`
	BoardID        string   `json:"board_id"`
	BoardName      string   `json:"board_name"`
	BoardScope     string   `json:"board_scope,omitempty"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	DiscourseMode  string   `json:"discourse_mode"`
	PostPurpose    string   `json:"post_purpose"`
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
			DiscourseMode:  event.DiscourseMode,
			PostPurpose:    bbsSituationPostPurpose(event.DiscourseMode),
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
あなたの出力は、World Engineがすでに選んだroot投稿枠について「投稿直前に世界で起きていたSituation」として正本化され、後段のBBS件名と記事本文を生成する材料になります。記事本文そのものではありません。

world-selected roots には、投稿者、日時、掲示板名（board_name）、掲示板の用途（board_scope）、投稿目的、人物情報、既存世界事実が材料として入っています。
board_name と board_scope は、その掲示板に投稿してよい話題の範囲を表します。board_scope は利用者には見えない内部の方向性です。各rootのSituationは、必ずこの板の範囲の内側で、小さく具体的に1件ずつ作ってください。
persona_profile は投稿者の暮らしぶりや語り口の材料であり、話題を決める根拠にはなりません。投稿者の関心が板の範囲の外にあっても、範囲外の話題は選ばないでください。
recent/avoid material は重複を避けるための材料です。避けるときも板の範囲の外へは出ず、範囲内の別の切り口を選んでください。
入力された世界事実は前提として扱い、別rootの出来事とは混ぜないでください。
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
		draft := BBSWorldSituationDraft{
			EventID:          id,
			ObjectClass:      strings.TrimSpace(value.ObjectClass),
			ChangeClass:      strings.TrimSpace(value.ChangeClass),
			Occurrence:       strings.TrimSpace(value.Occurrence),
			Observation:      strings.TrimSpace(value.Observation),
			Experience:       strings.TrimSpace(value.Experience),
			Result:           strings.TrimSpace(value.Result),
			Stance:           strings.TrimSpace(value.Stance),
			Basis:            strings.TrimSpace(value.Basis),
			AttemptedActions: strings.TrimSpace(value.AttemptedActions),
			PracticalPoint:   strings.TrimSpace(value.PracticalPoint),
			Question:         strings.TrimSpace(value.Question),
			NoveltyKey:       strings.TrimSpace(value.NoveltyKey),
			MustNot:          cleanStringList(value.MustNot),
		}
		// Compatibility projection for older development consumers.
		switch event.DiscourseMode {
		case "share_observation":
			draft.ActorObservation = draft.Observation
		case "share_experience":
			draft.ActorObservation = draft.Experience
			draft.Impact = draft.Result
		case "state_opinion":
			draft.ActorObservation = draft.Basis
			draft.Impact = draft.Stance
		case "share_tip":
			draft.ActorObservation = draft.AttemptedActions
			draft.Impact = draft.PracticalPoint
		case "ask_peers":
			draft.ActorObservation = draft.AttemptedActions
			draft.Uncertainty = draft.Question
		default:
			draft.ActorObservation = draft.Observation
		}
		out = append(out, draft)
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

func bbsWorldSituationProposalSchema(events []BBSWorldWindowEvent) map[string]any {
	properties := make(map[string]any, len(events))
	required := make([]string, 0, len(events))
	for _, event := range events {
		id := strings.TrimSpace(event.EventID)
		fields := map[string]any{
			"object_class": map[string]any{"type": "string"},
			"change_class": map[string]any{"type": "string"},
			"occurrence":   map[string]any{"type": "string"},
			"novelty_key":  map[string]any{"type": "string"},
			"must_not":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2},
		}
		eventRequired := []string{"object_class", "change_class", "occurrence", "novelty_key", "must_not"}
		add := func(name string) {
			fields[name] = map[string]any{"type": "string"}
			eventRequired = append(eventRequired, name)
		}
		switch strings.TrimSpace(event.DiscourseMode) {
		case "share_observation":
			add("observation")
		case "share_experience":
			add("experience")
			add("result")
		case "state_opinion":
			add("stance")
			add("basis")
		case "share_tip":
			add("attempted_actions")
			add("result")
			add("practical_point")
		case "ask_peers":
			add("attempted_actions")
			add("question")
		default:
			add("observation")
		}
		properties[id] = map[string]any{
			"type":                 "object",
			"properties":           fields,
			"required":             eventRequired,
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


func bbsSituationPostPurpose(mode string) string {
	switch strings.TrimSpace(mode) {
	case "share_observation":
		return "自分が観察したことを他の会員へ伝える"
	case "share_experience":
		return "自分に起きたことと結果を他の会員へ話す"
	case "state_opinion":
		return "自分の意見と、そのきっかけになったSituationを述べる"
	case "share_tip":
		return "自分で試して分かった小さな実用情報を共有する"
	case "ask_peers":
		return "自分で試したところまでを示し、未解決の点を他の会員へ聞く"
	default:
		return "このSituationについて他の会員へ伝える"
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
