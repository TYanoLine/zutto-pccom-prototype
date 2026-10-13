package llm

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStandardActorTools(t *testing.T) {
	tools := StandardActorTools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty standard tools")
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool definition must have a name")
		}
		if tool.Description == "" {
			t.Errorf("tool %s must have a description", tool.Name)
		}
		if tool.Parameters.Type != "object" {
			t.Errorf("tool %s parameters type must be object", tool.Name)
		}
		names[tool.Name] = true
	}

	expected := []string{
		ToolGetMyRecentPosts,
		ToolGetThreadContext,
		ToolGetInteractionHistoryWith,
		ToolSearchMyObservedEvents,
		ToolGetMyOpenLoops,
		ToolGetMyProfileFacts,
	}
	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("missing expected standard tool: %s", exp)
		}
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	mock := &MockToolExecutor{
		Handlers: map[string]func(ctx context.Context, actorID string, args json.RawMessage) (string, error){
			ToolGetMyRecentPosts: func(ctx context.Context, actorID string, args json.RawMessage) (string, error) {
				if actorID != "actor-123" {
					t.Errorf("expected actor-123, got %s", actorID)
				}
				return `[{"id": 100, "subject": "PC-9821のGA-98NB設定"}]`, nil
			},
		},
	}

	registry := NewToolRegistry(StandardActorTools(), mock)
	ctx := context.Background()

	// Valid tool call
	call := ToolCall{
		ID:        "call-1",
		Name:      ToolGetMyRecentPosts,
		Arguments: json.RawMessage(`{"limit": 2}`),
	}
	res := registry.Execute(ctx, "actor-123", call)
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if res.ToolCallID != "call-1" {
		t.Errorf("expected call-1, got %s", res.ToolCallID)
	}
	if res.Content == "" {
		t.Error("expected non-empty content")
	}

	// Unknown tool call
	unknown := ToolCall{
		ID:   "call-2",
		Name: "unknown_tool",
	}
	resUnknown := registry.Execute(ctx, "actor-123", unknown)
	if !resUnknown.IsError {
		t.Error("expected error for unknown tool")
	}
}

func TestDefaultToolExecutionPolicy(t *testing.T) {
	policy := DefaultToolExecutionPolicy()
	if policy.MaxToolCallsPerTurn != 1 {
		t.Errorf("expected MaxToolCallsPerTurn to be 1 for responsiveness, got %d", policy.MaxToolCallsPerTurn)
	}
}
