package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func chatLineTestRequest() ChatLineRequest {
	return ChatLineRequest{
		WorldTime: time.Date(1996, 10, 15, 23, 41, 0, 0, time.FixedZone("JST", 9*3600)),
		Station:   "博多運河ネット", Room: 0, Kind: "news",
		Speaker:   ChatLinePerson{Handle: "NORI", Summary: "人物像: 28歳 会社員"},
		Addressee: &ChatLinePerson{Handle: "GUEST"}, Relation: "常連仲間",
		Present:    []string{"NORI", "GUEST", "KAZU"},
		Transcript: []ChatLineTurn{{Handle: "GUEST", Body: "こんばんは"}},
		Event:      &ChatLineEvent{Subject: "GUEST", Kind: "posted", Board: "雑談", Title: "モデム買いました", At: time.Date(1996, 10, 15, 23, 20, 0, 0, time.UTC)},
	}
}

func chatLineProvider(t *testing.T, text string, seen *map[string]any) StructuredOpenAIProvider {
	t.Helper()
	response, _ := json.Marshal(map[string]any{"model": "gpt-test", "output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": text}}}}})
	return StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com", APIKey: "test-key", Model: "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			if seen != nil {
				_ = json.Unmarshal(body, seen)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(response)))}, nil
		})},
	}}
}

func TestWriteChatLineSendsMaterialsAndSchema(t *testing.T) {
	var seen map[string]any
	p := chatLineProvider(t, `{"line":"GUESTさん、モデム買ったんだって？(^^)"}`, &seen)
	line, err := p.WriteChatLine(context.Background(), chatLineTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	if line != "GUESTさん、モデム買ったんだって？(^^)" {
		t.Fatalf("line = %q", line)
	}
	format := seen["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["name"] != "chat_line" || format["strict"] != true {
		t.Fatalf("format = %+v", format)
	}
	prompt, _ := seen["input"].(string)
	for _, want := range []string{"NORI", "人物像: 28歳 会社員", "GUEST", "KAZU", "常連仲間", "こんばんは", "雑談", "モデム買いました",
		"1996年10月15日(火) 23:41", "話題にする", "DIEGETIC PRESENT"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	for _, banned := range []string{"ユーザー", "ユーザ", "お客様", "プレイヤー", "主人公", "人間"} {
		if strings.Contains(prompt, banned) {
			t.Errorf("prompt singles out the caller with %q", banned)
		}
	}
}

func TestWriteChatLineRejectsBrokenJSON(t *testing.T) {
	p := chatLineProvider(t, `{"line":`, nil)
	if _, err := p.WriteChatLine(context.Background(), chatLineTestRequest()); err == nil {
		t.Fatal("broken JSON accepted")
	}
}
