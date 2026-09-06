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
