package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var _ BBSWorldSituationProposer = StructuredOpenAIProvider{}

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

type compactSituationEvent struct {
	EventID        string   `json:"event_id"`
	BoardID        string   `json:"board_id"`
	BoardName      string   `json:"board_name"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	AnchorKey      string   `json:"anchor_key"`
	DiscourseMode  string   `json:"discourse_mode"`
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
			AuthorHandle:   event.AuthorHandle,
			CreatedAt:      event.CreatedAt,
			AnchorKey:      event.AnchorKey,
			DiscourseMode:  event.DiscourseMode,
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

	prompt := fmt.Sprintf(`1996年前後の日本の草の根パソコン通信世界について、すでに存在が決まったroot投稿ごとの「投稿直前の具体的Situation」を作ってください。
これは記事本文ではなく、本文より先に正本化する世界事実です。

固定事項:
- actor、日時、掲示板、routing domain、discourse_modeは入力どおり。
- existing_facts に Situation facet / scope boundary /人物の既存事実/世界が選んだ対象があれば、それを変えない。
- 各rootは独立。別rootの出来事を共有させない。
- 人間の日常行動として意味の通る、小さく具体的な出来事にする。
- 新しい恒久的な所有歴・購入歴・職歴・家族事情・長期嗜好を作らない。
- %s
- 同じbatchやavoid listで、同種の出来事・対象・distinctive detailを言い換えて繰り返さない。
- JSON形はdiscourse_modeごとに違う。その形に必要な世界事実だけを書く。ask_peers以外に質問を作らない。

discourse_modeの意味:
- share_observation: 観察したこと
- share_experience: 本人が経験したことと結果
- state_opinion: 本人の意見と、その根拠になったSituation
- share_tip: 本人が試したこと・結果・小さな実用ポイント
- ask_peers: 本人がすでに試したことと、他の会員に聞きたい未解決の問い

世界日付: %s
局: %s
地域: %s
期間: %s .. %s

allowed historical facts:
%s

直近BBS状態（重複回避の参考。ここから新事実を作らない）:
%s

world-selected roots:
%s

avoid:
%s`, historicalPolicy, req.WorldDate, req.HostName, req.HostRegion, req.WindowStart, req.WindowEnd, historicalFacts, recent, string(eventsJSON), string(avoidJSON))

	maxTokens := 600 + len(req.Events)*190
	if maxTokens > 8000 {
		maxTokens = 8000
	}
	producer := p.withWorldWindowHTTPTimeout()
	result, err := producer.responseTextWithJSONSchema(ctx, prompt, "low", maxTokens, "bbs_world_situations", bbsWorldSituationProposalSchema(req.Events))
	if err != nil {
		return BBSWorldSituationProposalDraft{}, err
	}
	var wire bbsWorldSituationProposalWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &wire); err != nil {
		return BBSWorldSituationProposalDraft{}, fmt.Errorf("decode BBS world-situation JSON: %w", err)
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


func bbsWorldSituationHistoricalPolicy(req BBSWorldSituationProposalRequest) string {
	if req.AllowModelHistoricalMemory {
		if req.PreferConcreteHistoricalNames {
			return "When the selected Situation naturally has a real contemporary target, PREFER that concrete historical name from your own historical knowledge only when confident it existed and was knowable in Japan by the event date. This concretization creates new canonical world state for this event; no earlier actor-use fact is required for the modest occurrence itself. This is not a quota. Do not change the Situation merely to insert a name. Do not put a generic 'do not name the title/product/device' rule in must_not."
		}
		return "You may use your own historical knowledge for a real contemporary name only when confident it existed and was knowable in Japan by the event date; otherwise stay generic. Do not add unsupported specifications, dates, prices or story/mechanic details."
	}
	return "Do not introduce a new real product/work/service/company/person/place/event name unless SUPPLIED HISTORICAL TEXTURE (allowed historical facts) or existing_facts explicitly permits it. Do not add unsupported specifications, dates, prices or content."
}
