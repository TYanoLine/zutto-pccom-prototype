package main

import (
	"testing"

	"zutto-pccom/apps/server/internal/worldrepo"
)

func TestTypedSituationSchemaQuestionOnlyForAskPeers(t *testing.T) {
	slots := []worldrepo.DevelopmentMinimalRootSlot{
		{EventID: "ask", DiscourseMode: "ask_peers"},
		{EventID: "opinion", DiscourseMode: "state_opinion"},
		{EventID: "experience", DiscourseMode: "share_experience"},
		{EventID: "tip", DiscourseMode: "share_tip"},
		{EventID: "observation", DiscourseMode: "share_observation"},
	}
	schema := minimalTypedSituationSchema(slots)
	rootProps := schema["properties"].(map[string]any)
	situations := rootProps["situations"].(map[string]any)
	eventSchemas := situations["properties"].(map[string]any)

	for _, slot := range slots {
		event := eventSchemas[slot.EventID].(map[string]any)
		props := event["properties"].(map[string]any)
		_, hasQuestion := props["question"]
		if slot.DiscourseMode == "ask_peers" && !hasQuestion {
			t.Fatalf("ask_peers schema missing question")
		}
		if slot.DiscourseMode != "ask_peers" && hasQuestion {
			t.Fatalf("%s schema unexpectedly exposes question", slot.DiscourseMode)
		}
	}
}

func TestTypedSituationFieldsMatchDiscourseMode(t *testing.T) {
	cases := map[string][]string{
		"share_observation": {"observation"},
		"share_experience":  {"experience", "result"},
		"state_opinion":     {"stance", "basis"},
		"share_tip":         {"attempted_actions", "result", "practical_point"},
		"ask_peers":         {"attempted_actions", "question"},
	}
	for mode, wants := range cases {
		props, required := minimalTypedSituationFields(mode)
		requiredSet := map[string]bool{}
		for _, name := range required {
			requiredSet[name] = true
		}
		for _, want := range wants {
			if _, ok := props[want]; !ok {
				t.Fatalf("%s missing property %q", mode, want)
			}
			if !requiredSet[want] {
				t.Fatalf("%s property %q is not required", mode, want)
			}
		}
	}
}
