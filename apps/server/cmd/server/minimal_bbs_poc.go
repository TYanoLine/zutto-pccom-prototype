package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
)

type minimalBBSPoCOutput struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func minimalBBSPoCPrompt() string {
	return `1996年の日本の草の根パソコン通信BBSで、以下の人物がこの瞬間に書くroot記事を1件作ってください。

以下の情報は世界エンジンによって確定済みの事実です。

【世界】
日時: 1996年
局: HAKATA CANAL NET
地域: 福岡県福岡市
掲示板: GAME
この記事はroot。親記事や先行スレッド文脈はない。

【投稿者】
handle: AKI
activity: active
BBS上の行動傾向:
- reply_tendency: 0.55
- thread_start_tendency: 0.19
- lurker_tendency: 0.28
- newcomer_openness: 0.56
- argumentativeness: 0.27
関心:
- games: 0.86
- hardware: 0.78
- food: 0.76
- daily_life: 0.38
- music: 0.35
年齢・性別・職業・固定された文体については世界側ではまだ設定されていない。設定されていないプロフィールを補う必要はない。

【現在のシチュエーション】
AKIは複数のPCエンジン用Huカードを手元に持っている。
Huカードの保管場所が統一されておらず、ケースに入ったものは棚に、裸のものは机の引き出しに入っていた。
今回、遊びたいHuカードを1本探したところ、保管場所が分かれているため探すのに少し手間取った。
そこでAKIは手持ちのHuカードをいったん全部机の上に出した。
現在はまず「ケースに入っているもの」「裸のもの」の2種類に分けている。
ただ、整理している途中で、ケースの有無よりタイトル順に並べた方が次から探しやすいかもしれない、とも思っている。
今回の投稿理由は、このちょっとした出来事をGAME板で話すこと。
誰かに解決方法を質問することが主目的ではない。
大事件でもトラブルでもない。単なる日常的なゲーム趣味の一場面である。

【生成時の制約】
- 上記の世界事実を変更せず、書かれていない新しい出来事を作らない。
- 1996年を現在として生きている本人として書く。
- root件名は36文字以内。Re:は付けない。
- 件名と本文だけをJSONで返す。`
}

func newMinimalBBSPoCHandler(endpoint, apiKey, defaultModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(apiKey) == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "AZURE_OPENAI_API_KEY is not configured"})
			return
		}
		model := strings.TrimSpace(r.URL.Query().Get("model"))
		if model == "" { model = strings.TrimSpace(defaultModel) }
		if model == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "AZURE_OPENAI_MODEL is not configured"})
			return
		}

		prompt := minimalBBSPoCPrompt()
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subject": map[string]any{"type": "string"},
				"body": map[string]any{"type": "string"},
			},
			"required": []string{"subject", "body"},
			"additionalProperties": false,
		}
		payload := map[string]any{
			"model": model,
			"input": prompt,
			"reasoning": map[string]any{"effort": "low"},
			"text": map[string]any{
				"verbosity": "low",
				"format": map[string]any{
					"type": "json_schema",
					"name": "minimal_bbs_post",
					"strict": true,
					"schema": schema,
				},
			},
			"max_output_tokens": 700,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, `{"error":"encode request"}`, http.StatusInternalServerError)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		url, err := azureopenai.URL(endpoint, "responses")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			http.Error(w, `{"error":"build request"}`, http.StatusInternalServerError)
			return
		}
		if err := azureopenai.ApplyAPIKey(req, apiKey); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		started := time.Now()
		resp, err := http.DefaultClient.Do(req)
		latency := time.Since(started)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "latency_ms": latency.Milliseconds()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("Azure OpenAI returned %s", resp.Status), "latency_ms": latency.Milliseconds()})
			return
		}

		var decoded struct {
			Model string `json:"model"`
			Output []struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
			Usage map[string]any `json:"usage"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "decode Azure OpenAI response: " + err.Error(), "latency_ms": latency.Milliseconds()})
			return
		}
		var text string
		for _, out := range decoded.Output {
			for _, c := range out.Content {
				if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
					text += c.Text
				}
			}
		}
		if text == "" {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": errors.New("no output_text").Error(), "latency_ms": latency.Milliseconds()})
			return
		}
		var post minimalBBSPoCOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &post); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "decode generated post: " + err.Error(), "raw": text, "latency_ms": latency.Milliseconds()})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": decoded.Model,
			"requested_model": model,
			"prompt": prompt,
			"post": post,
			"latency_ms": latency.Milliseconds(),
			"usage": decoded.Usage,
			"persisted": false,
		})
	}
}
