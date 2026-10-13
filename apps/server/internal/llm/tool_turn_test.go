package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type scriptedToolModel struct {
	responses []ToolTurnResponse
	requests  []ToolTurnRequest
	err       error
}

func (m *scriptedToolModel) Complete(_ context.Context, req ToolTurnRequest) (ToolTurnResponse, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return ToolTurnResponse{}, m.err
	}
	i := len(m.requests) - 1
	if i >= len(m.responses) {
		return ToolTurnResponse{Text: "overflow"}, nil
	}
	return m.responses[i], nil
}

func newTestToolRegistry(executed *[]string) *ToolRegistry {
	exec := &MockToolExecutor{Handlers: map[string]func(context.Context, string, json.RawMessage) (string, error){
		ToolGetMyRecentPosts: func(_ context.Context, actorID string, _ json.RawMessage) (string, error) {
			*executed = append(*executed, ToolGetMyRecentPosts+"@"+actorID)
			return "#1001 [1] 件名", nil
		},
		ToolGetMyProfileFacts: func(_ context.Context, actorID string, _ json.RawMessage) (string, error) {
			*executed = append(*executed, ToolGetMyProfileFacts+"@"+actorID)
			return "なし", nil
		},
	}}
	return NewToolRegistry(StandardActorTools(), exec)
}

func TestRunToolTurnNoToolRequestUsesOneModelCall(t *testing.T) {
	var executed []string
	model := &scriptedToolModel{responses: []ToolTurnResponse{{Text: "そのまま書きます"}}}
	out, err := RunToolTurn(context.Background(), model, newTestToolRegistry(&executed), DefaultToolExecutionPolicy(),
		"persona:p1", []ToolTurnMessage{{Role: "user", Content: "書いて"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelCalls != 1 || out.ToolCallsExecuted != 0 || out.Text != "そのまま書きます" {
		t.Fatalf("outcome=%+v", out)
	}
	if len(model.requests[0].Tools) != len(StandardActorTools()) {
		t.Fatalf("round 1 offered %d tools, want %d", len(model.requests[0].Tools), len(StandardActorTools()))
	}
	if len(executed) != 0 {
		t.Fatalf("executed=%v", executed)
	}
}

func TestRunToolTurnOneRoundTripWithoutToolsInRoundTwo(t *testing.T) {
	var executed []string
	model := &scriptedToolModel{responses: []ToolTurnResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: ToolGetMyRecentPosts, Arguments: json.RawMessage(`{}`)}}},
		{Text: "さっきの件ですね"},
	}}
	out, err := RunToolTurn(context.Background(), model, newTestToolRegistry(&executed), DefaultToolExecutionPolicy(),
		"persona:p1", []ToolTurnMessage{{Role: "user", Content: "さっきの件は？"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelCalls != 2 || out.ToolCallsExecuted != 1 || out.Text != "さっきの件ですね" {
		t.Fatalf("outcome=%+v", out)
	}
	if len(model.requests[1].Tools) != 0 {
		t.Fatalf("round 2 must not offer tools, got %d", len(model.requests[1].Tools))
	}
	last := model.requests[1].Messages
	if len(last) != 3 || last[1].Role != "assistant" || last[2].Role != "tool" || last[2].ToolResult == nil {
		t.Fatalf("round 2 messages=%+v", last)
	}
	if last[2].ToolResult.Content != "#1001 [1] 件名" || last[2].ToolResult.ToolCallID != "c1" {
		t.Fatalf("tool result=%+v", last[2].ToolResult)
	}
	if len(executed) != 1 || executed[0] != ToolGetMyRecentPosts+"@persona:p1" {
		t.Fatalf("executed=%v", executed)
	}
}

func TestRunToolTurnExecutesOnlyFirstOfSeveralToolCalls(t *testing.T) {
	var executed []string
	model := &scriptedToolModel{responses: []ToolTurnResponse{
		{ToolCalls: []ToolCall{
			{ID: "c1", Name: ToolGetMyRecentPosts},
			{ID: "c2", Name: ToolGetMyProfileFacts},
		}},
		{Text: "完了"},
	}}
	out, err := RunToolTurn(context.Background(), model, newTestToolRegistry(&executed), DefaultToolExecutionPolicy(),
		"persona:p1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.ToolCallsExecuted != 1 || len(executed) != 1 {
		t.Fatalf("outcome=%+v executed=%v", out, executed)
	}
	if got := model.requests[1].Messages[0].ToolCalls; len(got) != 1 || got[0].ID != "c1" {
		t.Fatalf("assistant message must carry only the executed call, got %+v", got)
	}
}

func TestRunToolTurnStopsAfterTwoModelCallsWhenToolRequestedAgain(t *testing.T) {
	var executed []string
	model := &scriptedToolModel{responses: []ToolTurnResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: ToolGetMyRecentPosts}}},
		{ToolCalls: []ToolCall{{ID: "c2", Name: ToolGetMyProfileFacts}}},
		{Text: "呼ばれてはいけない"},
	}}
	out, err := RunToolTurn(context.Background(), model, newTestToolRegistry(&executed), DefaultToolExecutionPolicy(),
		"persona:p1", nil)
	if !errors.Is(err, ErrNoFinalText) {
		t.Fatalf("err=%v, want ErrNoFinalText", err)
	}
	if out.ModelCalls != 2 || len(model.requests) != 2 {
		t.Fatalf("model calls=%d requests=%d, want 2", out.ModelCalls, len(model.requests))
	}
	if len(executed) != 1 {
		t.Fatalf("second tool request must not run, executed=%v", executed)
	}
}

func TestRunToolTurnZeroPolicyOffersNoTools(t *testing.T) {
	var executed []string
	model := &scriptedToolModel{responses: []ToolTurnResponse{{Text: "ツールなし"}}}
	out, err := RunToolTurn(context.Background(), model, newTestToolRegistry(&executed), ToolExecutionPolicy{},
		"persona:p1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelCalls != 1 || len(model.requests[0].Tools) != 0 || out.Text != "ツールなし" {
		t.Fatalf("outcome=%+v tools=%d", out, len(model.requests[0].Tools))
	}
}

func TestRunToolTurnReturnsModelErrors(t *testing.T) {
	boom := errors.New("boom")
	model := &scriptedToolModel{err: boom}
	out, err := RunToolTurn(context.Background(), model, nil, DefaultToolExecutionPolicy(), "persona:p1", nil)
	if !errors.Is(err, boom) || out.ModelCalls != 1 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestRunToolTurnNilModel(t *testing.T) {
	if _, err := RunToolTurn(context.Background(), nil, nil, DefaultToolExecutionPolicy(), "persona:p1", nil); err == nil {
		t.Fatal("expected error for nil model")
	}
}
