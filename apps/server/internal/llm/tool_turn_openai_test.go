package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIToolTurnModelTextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "secret" {
			t.Errorf("api-key header=%q", r.Header.Get("api-key"))
		}
		var payload openAIChatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "こんにちは" {
			t.Errorf("messages=%+v", payload.Messages)
		}
		if len(payload.Tools) != 0 {
			t.Errorf("tools should be empty, got %d", len(payload.Tools))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"こんにちは！"}}]}`)
	}))
	defer server.Close()

	model := OpenAIToolTurnModel{
		Endpoint: server.URL,
		APIKey:   "secret",
		Model:    "test-model",
		Client:   server.Client(),
	}

	resp, err := model.Complete(context.Background(), ToolTurnRequest{
		Messages: []ToolTurnMessage{{Role: "user", Content: "こんにちは"}},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if resp.Text != "こんにちは！" {
		t.Errorf("resp.Text = %q, want %q", resp.Text, "こんにちは！")
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("resp.ToolCalls = %+v, want empty", resp.ToolCalls)
	}
}

func TestOpenAIToolTurnModelWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload openAIChatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "get_my_recent_posts" {
			t.Errorf("tools=%+v", payload.Tools)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_123",
						"type": "function",
						"function": {
							"name": "get_my_recent_posts",
							"arguments": "{\"limit\":3}"
						}
					}]
				}
			}]
		}`)
	}))
	defer server.Close()

	model := OpenAIToolTurnModel{
		Endpoint: server.URL,
		APIKey:   "secret",
		Model:    "test-model",
		Client:   server.Client(),
	}

	resp, err := model.Complete(context.Background(), ToolTurnRequest{
		Messages: []ToolTurnMessage{{Role: "user", Content: "最近の投稿を調べて"}},
		Tools: []ToolDefinition{{
			Name:        "get_my_recent_posts",
			Description: "最近の投稿を取得",
		}},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	call := resp.ToolCalls[0]
	if call.ID != "call_123" || call.Name != "get_my_recent_posts" || string(call.Arguments) != `{"limit":3}` {
		t.Errorf("call = %+v", call)
	}
}

func TestOpenAIToolTurnModelToolResultPassesThrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload openAIChatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Messages) != 3 {
			t.Fatalf("messages count=%d, want 3", len(payload.Messages))
		}
		toolMsg := payload.Messages[2]
		if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_123" || toolMsg.Content != "投稿1件" {
			t.Errorf("tool message = %+v", toolMsg)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"確認できました。"}}]}`)
	}))
	defer server.Close()

	model := OpenAIToolTurnModel{
		Endpoint: server.URL,
		APIKey:   "secret",
		Model:    "test-model",
		Client:   server.Client(),
	}

	resp, err := model.Complete(context.Background(), ToolTurnRequest{
		Messages: []ToolTurnMessage{
			{Role: "user", Content: "最近の投稿は？"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_123", Name: "get_my_recent_posts"}}},
			{Role: "tool", ToolResult: &ToolResult{ToolCallID: "call_123", Content: "投稿1件"}},
		},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if resp.Text != "確認できました。" {
		t.Errorf("resp.Text = %q", resp.Text)
	}
}

func TestOpenAIToolTurnModelErrors(t *testing.T) {
	// APIKey missing
	model := OpenAIToolTurnModel{Endpoint: "http://example.com"}
	_, err := model.Complete(context.Background(), ToolTurnRequest{})
	if err == nil || !strings.Contains(err.Error(), "AZURE_OPENAI_API_KEY is not set") {
		t.Errorf("expected API key error, got %v", err)
	}

	// 500 error from server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "internal server error")
	}))
	defer server.Close()

	model2 := OpenAIToolTurnModel{
		Endpoint: server.URL,
		APIKey:   "secret",
		Client:   server.Client(),
	}
	_, err = model2.Complete(context.Background(), ToolTurnRequest{})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got %v", err)
	}
}
