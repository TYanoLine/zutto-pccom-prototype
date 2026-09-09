package llm

import "testing"

func TestBBSWorldWindowProductionSchemaKeysBriefsByExactEventID(t *testing.T) {
	events := []BBSWorldWindowEvent{
		{EventID: "board-1:event-0001"},
		{EventID: "board-2:event-0002"},
	}
	schema := bbsWorldWindowProductionSchema(events)
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing")
	}
	briefs, ok := properties["briefs"].(map[string]any)
	if !ok {
		t.Fatal("briefs schema missing")
	}
	if got := briefs["type"]; got != "object" {
		t.Fatalf("briefs type=%v want object", got)
	}
	if got := briefs["additionalProperties"]; got != false {
		t.Fatalf("briefs additionalProperties=%v want false", got)
	}
	required, ok := briefs["required"].([]string)
	if !ok {
		t.Fatalf("briefs required has type %T", briefs["required"])
	}
	if len(required) != 2 || required[0] != events[0].EventID || required[1] != events[1].EventID {
		t.Fatalf("required=%v want request event ids in order", required)
	}
	briefProperties, ok := briefs["properties"].(map[string]any)
	if !ok || len(briefProperties) != 2 {
		t.Fatalf("brief properties=%v", briefProperties)
	}
	for _, event := range events {
		briefSchema, ok := briefProperties[event.EventID].(map[string]any)
		if !ok {
			t.Fatalf("missing schema for %s", event.EventID)
		}
		fields, ok := briefSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("brief fields missing for %s", event.EventID)
		}
		if _, exists := fields["event_id"]; exists {
			t.Fatalf("event_id must be owned by object key, not worker brief: %v", fields)
		}
	}
}

func TestBBSWorldWindowProductionSchemaForbidsAudienceContextOnStandaloneRoot(t *testing.T) {
	events := []BBSWorldWindowEvent{
		{EventID: "root", Action: "thread_start"},
		{EventID: "reply", Action: "reply", ParentEventID: "root", SourceEventID: "root"},
	}
	schema := bbsWorldWindowProductionSchema(events)
	briefs := schema["properties"].(map[string]any)["briefs"].(map[string]any)
	briefProperties := briefs["properties"].(map[string]any)
	rootFields := briefProperties["root"].(map[string]any)["properties"].(map[string]any)
	replyFields := briefProperties["reply"].(map[string]any)["properties"].(map[string]any)
	if got := rootFields["audience_context"].(map[string]any)["maxItems"]; got != 0 {
		t.Fatalf("root audience_context maxItems=%v want 0", got)
	}
	if got := replyFields["audience_context"].(map[string]any)["maxItems"]; got != 4 {
		t.Fatalf("reply audience_context maxItems=%v want 4", got)
	}
}

func TestBBSWorldWindowDraftFromWireUsesCanonicalKeysAndRequestOrder(t *testing.T) {
	req := BBSWorldWindowProductionRequest{Events: []BBSWorldWindowEvent{
		{EventID: "board-1:event-0001"},
		{EventID: "board-2:event-0002"},
	}}
	wire := bbsWorldWindowProductionWire{Briefs: map[string]bbsArticleBriefWire{
		"board-2:event-0002": validBriefWire("second"),
		"board-1:event-0001": validBriefWire("first"),
	}}

	draft, err := bbsWorldWindowDraftFromWire(req, wire)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Briefs) != 2 {
		t.Fatalf("briefs=%d want 2", len(draft.Briefs))
	}
	if draft.Briefs[0].EventID != req.Events[0].EventID || draft.Briefs[0].Subject != "first" {
		t.Fatalf("first brief=%+v", draft.Briefs[0])
	}
	if draft.Briefs[1].EventID != req.Events[1].EventID || draft.Briefs[1].Subject != "second" {
		t.Fatalf("second brief=%+v", draft.Briefs[1])
	}
}

func TestBBSWorldWindowDraftFromWireRejectsMissingOrUnknownKey(t *testing.T) {
	req := BBSWorldWindowProductionRequest{Events: []BBSWorldWindowEvent{
		{EventID: "board-1:event-0001"},
		{EventID: "board-2:event-0002"},
	}}
	_, err := bbsWorldWindowDraftFromWire(req, bbsWorldWindowProductionWire{Briefs: map[string]bbsArticleBriefWire{
		"board-1:event-0001": validBriefWire("first"),
		"board-9:event-9999": validBriefWire("unknown"),
	}})
	if err == nil {
		t.Fatal("unknown key should fail")
	}
}

func TestValidateBBSWorldWindowProductionRejectsReplySemanticsOnStandaloneRoot(t *testing.T) {
	req := BBSWorldWindowProductionRequest{Events: []BBSWorldWindowEvent{{EventID: "root", Action: "thread_start"}}}

	draft := BBSWorldWindowProductionDraft{Briefs: []BBSArticleBriefDraft{validProductionBrief("root", "Re: unrelated", nil, nil)}}
	if err := validateBBSWorldWindowProduction(req, draft); err == nil {
		t.Fatal("standalone root with Re: subject should fail")
	}

	draft = BBSWorldWindowProductionDraft{Briefs: []BBSArticleBriefDraft{validProductionBrief("root", "standalone", nil, []string{"another selected post"})}}
	if err := validateBBSWorldWindowProduction(req, draft); err == nil {
		t.Fatal("standalone root with audience_context should fail")
	}
}

func TestValidateBBSWorldWindowProductionRejectsReferentReuseAcrossUnrelatedRoots(t *testing.T) {
	req := BBSWorldWindowProductionRequest{Events: []BBSWorldWindowEvent{
		{EventID: "root-a", Action: "thread_start"},
		{EventID: "root-b", Action: "thread_start"},
	}}
	draft := BBSWorldWindowProductionDraft{Briefs: []BBSArticleBriefDraft{
		validProductionBrief("root-a", "a", []string{"13日夜の接続切れ"}, nil),
		validProductionBrief("root-b", "b", []string{"13日夜の接続切れ"}, nil),
	}}
	if err := validateBBSWorldWindowProduction(req, draft); err == nil {
		t.Fatal("same referent across unrelated roots should fail")
	}
}

func TestValidateBBSWorldWindowProductionAllowsReferentReuseInsideExplicitThread(t *testing.T) {
	req := BBSWorldWindowProductionRequest{Events: []BBSWorldWindowEvent{
		{EventID: "root", Action: "thread_start"},
		{EventID: "reply", Action: "reply", ParentEventID: "root", SourceEventID: "root"},
	}}
	draft := BBSWorldWindowProductionDraft{Briefs: []BBSArticleBriefDraft{
		validProductionBrief("root", "root subject", []string{"13日夜の接続切れ"}, nil),
		validProductionBrief("reply", "reply subject", []string{"13日夜の接続切れ"}, []string{"root context"}),
	}}
	if err := validateBBSWorldWindowProduction(req, draft); err != nil {
		t.Fatalf("explicit thread should allow stable referent reuse: %v", err)
	}
}

func TestPublicProductIdentityCanRecurWithoutSharingPrivateIncident(t *testing.T) {
	req := BBSWorldWindowProductionRequest{PublicReferents: []string{"架空ゲームA"}, Events: []BBSWorldWindowEvent{
		{EventID: "a", Action: "thread_start"}, {EventID: "b", Action: "thread_start"},
	}}
	draft := BBSWorldWindowProductionDraft{Briefs: []BBSArticleBriefDraft{
		validProductionBrief("a", "架空ゲームAの感想", []string{"架空ゲームA"}, nil),
		validProductionBrief("b", "架空ゲームAの相談", []string{"架空ゲームA"}, nil),
	}}
	if err := validateBBSWorldWindowProduction(req, draft); err != nil {
		t.Fatal(err)
	}
	draft.Briefs[0].Referents = append(draft.Briefs[0].Referents, "同じ日の同じ故障")
	draft.Briefs[1].Referents = append(draft.Briefs[1].Referents, "同じ日の同じ故障")
	if err := validateBBSWorldWindowProduction(req, draft); err == nil {
		t.Fatal("public product must not authorize private incident reuse")
	}
}

func TestValidateBBSWorldWindowEventIDsRejectsUnknownTopologyReference(t *testing.T) {
	events := []BBSWorldWindowEvent{{EventID: "reply", Action: "reply", ParentEventID: "missing"}}
	if err := validateBBSWorldWindowEventIDs(events); err == nil {
		t.Fatal("unknown parent event should fail")
	}
}

func validBriefWire(subject string) bbsArticleBriefWire {
	return bbsArticleBriefWire{
		Subject:    subject,
		Episode:    "episode",
		Topic:      "topic",
		Motivation: "motivation",
		Stance:     "stance",
		Goal:       "goal",
	}
}

func validProductionBrief(id, subject string, referents, audienceContext []string) BBSArticleBriefDraft {
	return BBSArticleBriefDraft{
		EventID:         id,
		Subject:         subject,
		Episode:         "episode",
		Referents:       referents,
		AudienceContext: audienceContext,
		Topic:           "topic",
		Motivation:      "motivation",
		Stance:          "stance",
		Goal:            "goal",
	}
}
