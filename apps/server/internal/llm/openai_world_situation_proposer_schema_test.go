package llm

import "testing"

func TestBBSWorldSituationProposalSchemaKeysExactRoots(t *testing.T) {
	events := []BBSWorldWindowEvent{{EventID: "board-1:event-0001", Action: "thread_start"}, {EventID: "board-3:event-0002", Action: "thread_start"}}
	schema := bbsWorldSituationProposalSchema(events)
	props := schema["properties"].(map[string]any)["situations"].(map[string]any)["properties"].(map[string]any)
	if len(props) != 2 || props["board-1:event-0001"] == nil || props["board-3:event-0002"] == nil {
		t.Fatalf("unexpected properties: %#v", props)
	}
}
