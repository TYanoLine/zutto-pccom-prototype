package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
)

// OpenAIToolTurnModel adapts OpenAI / Azure OpenAI Chat Completions to ToolTurnModel.
type OpenAIToolTurnModel struct {
	Endpoint string
	APIKey   string
	Model    string
	Client   *http.Client
}

var _ ToolTurnModel = OpenAIToolTurnModel{}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCallItem `json:"tool_calls,omitempty"`
}

type openAIToolCallItem struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIFunctionCallItem `json:"function"`
}

type openAIFunctionCallItem struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolItem struct {
	Type     string                 `json:"type"`
	Function openAIFunctionItemDesc `json:"function"`
}

type openAIFunctionItemDesc struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Parameters  ToolParameterSchema `json:"parameters,omitempty"`
}

type openAIChatPayload struct {
	Model      string              `json:"model"`
	Messages   []openAIChatMessage `json:"messages"`
	Tools      []openAIToolItem    `json:"tools,omitempty"`
	ToolChoice string              `json:"tool_choice,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string               `json:"role"`
			Content   string               `json:"content"`
			ToolCalls []openAIToolCallItem `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete implements ToolTurnModel.
func (m OpenAIToolTurnModel) Complete(ctx context.Context, req ToolTurnRequest) (ToolTurnResponse, error) {
	if m.APIKey == "" {
		return ToolTurnResponse{}, errors.New("AZURE_OPENAI_API_KEY is not set")
	}
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	payload := openAIChatPayload{
		Model:    m.Model,
		Messages: make([]openAIChatMessage, 0, len(req.Messages)),
	}

	for _, msg := range req.Messages {
		cm := openAIChatMessage{Role: msg.Role, Content: msg.Content}
		if msg.Role == "tool" && msg.ToolResult != nil {
			cm.ToolCallID = msg.ToolResult.ToolCallID
			cm.Content = msg.ToolResult.Content
		} else if len(msg.ToolCalls) > 0 {
			cm.ToolCalls = make([]openAIToolCallItem, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				cm.ToolCalls = append(cm.ToolCalls, openAIToolCallItem{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFunctionCallItem{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				})
			}
		}
		payload.Messages = append(payload.Messages, cm)
	}

	if len(req.Tools) > 0 {
		payload.Tools = make([]openAIToolItem, 0, len(req.Tools))
		for _, td := range req.Tools {
			payload.Tools = append(payload.Tools, openAIToolItem{
				Type: "function",
				Function: openAIFunctionItemDesc{
					Name:        td.Name,
					Description: td.Description,
					Parameters:  td.Parameters,
				},
			})
		}
		payload.ToolChoice = "auto"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ToolTurnResponse{}, fmt.Errorf("marshal chat payload: %w", err)
	}

	endpoint, err := azureopenai.URL(m.Endpoint, "chat/completions")
	if err != nil {
		return ToolTurnResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ToolTurnResponse{}, err
	}
	if err := azureopenai.ApplyAPIKey(httpReq, m.APIKey); err != nil {
		return ToolTurnResponse{}, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return ToolTurnResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ToolTurnResponse{}, fmt.Errorf("Azure OpenAI chat API returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return ToolTurnResponse{}, fmt.Errorf("decode chat response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return ToolTurnResponse{}, errors.New("empty choices from chat completion")
	}

	choice := chatResp.Choices[0].Message
	out := ToolTurnResponse{
		Text: strings.TrimSpace(choice.Content),
	}
	if len(choice.ToolCalls) > 0 {
		out.ToolCalls = make([]ToolCall, 0, len(choice.ToolCalls))
		for _, tc := range choice.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: []byte(tc.Function.Arguments),
			})
		}
	}
	return out, nil
}
