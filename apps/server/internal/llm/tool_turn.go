package llm

import (
	"context"
	"errors"
	"fmt"
)

// ToolTurnMessage is one message in a tool-capable generation turn.
type ToolTurnMessage struct {
	Role       string // "system" | "user" | "assistant" | "tool"
	Content    string
	ToolCalls  []ToolCall
	ToolResult *ToolResult
}

// ToolTurnRequest is one model call. An empty Tools slice means the model must
// answer in text (the equivalent of tool_choice="none").
type ToolTurnRequest struct {
	Messages []ToolTurnMessage
	Tools    []ToolDefinition
}

// ToolTurnResponse is the model's reply: final text, tool requests, or both.
type ToolTurnResponse struct {
	Text      string
	ToolCalls []ToolCall
}

// ToolTurnModel is the provider-neutral model surface used by RunToolTurn.
// Adapters for concrete providers live outside this file.
type ToolTurnModel interface {
	Complete(ctx context.Context, req ToolTurnRequest) (ToolTurnResponse, error)
}

// ToolTurnOutcome reports the final text and how much work the turn took.
type ToolTurnOutcome struct {
	Text              string
	ModelCalls        int
	ToolCallsExecuted int
}

// ErrNoFinalText is returned when the model still has not produced text after
// the single permitted tool round-trip.
var ErrNoFinalText = errors.New("model returned no final text")

// RunToolTurn runs one generation request with at most
// policy.MaxToolCallsPerTurn tool round-trips. With the default policy that is
// one: round 1 offers the tools; if the model asks for one, the first request is
// executed and round 2 is made without tools so the model must answer.
func RunToolTurn(ctx context.Context, model ToolTurnModel, reg *ToolRegistry, policy ToolExecutionPolicy,
	actorID string, messages []ToolTurnMessage) (ToolTurnOutcome, error) {
	if model == nil {
		return ToolTurnOutcome{}, errors.New("tool turn model is nil")
	}
	history := append([]ToolTurnMessage(nil), messages...)

	var tools []ToolDefinition
	if reg != nil && policy.MaxToolCallsPerTurn > 0 {
		tools = reg.Definitions()
	}

	out := ToolTurnOutcome{}
	resp, err := model.Complete(ctx, ToolTurnRequest{Messages: history, Tools: tools})
	out.ModelCalls++
	if err != nil {
		return out, fmt.Errorf("tool turn round 1: %w", err)
	}
	if len(tools) == 0 || len(resp.ToolCalls) == 0 {
		out.Text = resp.Text
		return out, nil
	}

	call := resp.ToolCalls[0]
	result := reg.Execute(ctx, actorID, call)
	out.ToolCallsExecuted++
	history = append(history,
		ToolTurnMessage{Role: "assistant", Content: resp.Text, ToolCalls: []ToolCall{call}},
		ToolTurnMessage{Role: "tool", ToolResult: &result},
	)

	resp, err = model.Complete(ctx, ToolTurnRequest{Messages: history})
	out.ModelCalls++
	if err != nil {
		return out, fmt.Errorf("tool turn round 2: %w", err)
	}
	if resp.Text == "" {
		return out, ErrNoFinalText
	}
	out.Text = resp.Text
	return out, nil
}
