package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldrepo"
)

type minimalBatchSituation struct {
	Object           string `json:"object"`
	Occurrence       string `json:"occurrence"`
	ActorObservation string `json:"actor_observation"`
	Impact           string `json:"impact"`
	Uncertainty      string `json:"uncertainty"`
}

type minimalBatchPost struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type minimalBatchSituationWire struct {
	Situations map[string]minimalBatchSituation `json:"situations"`
}

type minimalBatchPostWire struct {
	Posts map[string]minimalBatchPost `json:"posts"`
}

func minimalBatchSituationSchema(slots []worldrepo.DevelopmentMinimalRootSlot) map[string]any {
	props := map[string]any{}
	required := make([]string, 0, len(slots))
	for _, slot := range slots {
		props[slot.EventID] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"object": map[string]any{"type": "string"},
				"occurrence": map[string]any{"type": "string"},
				"actor_observation": map[string]any{"type": "string"},
				"impact": map[string]any{"type": "string"},
				"uncertainty": map[string]any{"type": "string"},
			},
			"required": []string{"object", "occurrence", "actor_observation", "impact", "uncertainty"},
			"additionalProperties": false,
		}
		required = append(required, slot.EventID)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"situations": map[string]any{
				"type": "object",
				"properties": props,
				"required": required,
				"additionalProperties": false,
			},
		},
		"required": []string{"situations"},
		"additionalProperties": false,
	}
}

func minimalBatchPostSchema(slots []worldrepo.DevelopmentMinimalRootSlot) map[string]any {
	props := map[string]any{}
	required := make([]string, 0, len(slots))
	for _, slot := range slots {
		props[slot.EventID] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"subject": map[string]any{"type": "string"},
				"body": map[string]any{"type": "string"},
			},
			"required": []string{"subject", "body"},
			"additionalProperties": false,
		}
		required = append(required, slot.EventID)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"posts": map[string]any{
				"type": "object",
				"properties": props,
				"required": required,
				"additionalProperties": false,
			},
		},
		"required": []string{"posts"},
		"additionalProperties": false,
	}
}

func minimalSituationPrompt(host world.Host, board world.Board, slots []worldrepo.DevelopmentMinimalRootSlot) string {
	data, _ := json.MarshalIndent(slots, "", "  ")
	return fmt.Sprintf(`1996年前後の日本の草の根パソコン通信世界です。
以下はWorld Engineがすでに選んだ独立したroot投稿枠です。
各枠について、その人物が実際に書き込みたくなる直前の「具体的な現在のシチュエーション」を1件だけ作ってください。
これは本文ではなく世界事実です。

局: %s
地域: %s
掲示板: %s

守ること:
- actor、日時、板、cause、discourse_mode、与えられたsituation facetの意味を変えない。
- 人間の日常行動として、何が起きたかが具体的に想像できる小さな出来事にする。
- 別root同士を同じ出来事として結び付けず、設定されていない恒久的な人物設定を足さない。
- このPoCでは新しい実在ゲーム名・製品名・人物名・地名を追加しない。固有名詞なしでも出来事自体は具体的にする。

WORLD SLOTS:
%s`, host.Name, host.Region, board.Name, string(data))
}

func minimalPostPrompt(host world.Host, board world.Board, slots []worldrepo.DevelopmentMinimalRootSlot, situations map[string]minimalBatchSituation) string {
	type row struct {
		Slot      worldrepo.DevelopmentMinimalRootSlot `json:"slot"`
		Situation minimalBatchSituation                 `json:"situation"`
	}
	rows := make([]row, 0, len(slots))
	for _, slot := range slots {
		rows = append(rows, row{Slot: slot, Situation: situations[slot.EventID]})
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	return fmt.Sprintf(`以下は1996年前後の草の根パソコン通信BBSで、World Engineが確定した投稿者プロフィールと現在のシチュエーションです。
各人物がその瞬間に実際に書くroot記事の「件名」と「本文」を1件ずつ作ってください。

局: %s
地域: %s
掲示板: %s

守ること:
- 与えられた人物とSituationを変えず、書かれていない新しい出来事を作らない。
- 当時を現在として生きている本人として普通に書く。
- 件名はその人物が実際に入力しそうな自然な件名にし、36文字以内、Re:なし。
- 件名と本文だけを返す。

INPUT:
%s`, host.Name, host.Region, board.Name, string(data))
}

func newMinimalSituationTitleBatchPoCHandler(repo *worldrepo.Repository, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if strings.TrimSpace(apiKey) == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "OPENAI_API_KEY is not configured"})
			return
		}
		model := strings.TrimSpace(r.URL.Query().Get("model"))
		if model == "" {
			model = "gpt-6-luna"
		}
		if model != "gpt-6-luna" && model != "gpt-5.6-luna" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "model must be gpt-6-luna or gpt-5.6-luna"})
			return
		}
		count := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("count")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 20 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "count must be 1..20"})
				return
			}
			count = n
		}

		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			phone = erikaKExperimentPhone
		}
		boardID := strings.TrimSpace(r.URL.Query().Get("board"))
		if boardID == "" {
			boardID = "20/1"
		}
		host, err := repo.HostByPhone(phone)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		boards, _ := repo.MaterializationBoards(host)
		var board world.Board
		for _, candidate := range boards {
			if candidate.ID == boardID {
				board = candidate
				break
			}
		}
		if board.ID == "" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "board not found"})
			return
		}

		slots := repo.DevelopmentMinimalRootSlots(host, board, count)
		if len(slots) == 0 {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "no world-selected root slots"})
			return
		}

		situationPrompt := minimalSituationPrompt(host, board, slots)
		stage1Ctx, stage1Cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer stage1Cancel()
		stage1, err := callMinimalOpenAIJSON(stage1Ctx, apiKey, model, situationPrompt, "minimal_bbs_situations", minimalBatchSituationSchema(slots), 5200)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "situation generation: " + err.Error()})
			return
		}
		var situationWire minimalBatchSituationWire
		if err := json.Unmarshal([]byte(stage1.Text), &situationWire); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "decode situations: " + err.Error(), "raw": stage1.Text})
			return
		}

		postPrompt := minimalPostPrompt(host, board, slots, situationWire.Situations)
		stage2Ctx, stage2Cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer stage2Cancel()
		stage2, err := callMinimalOpenAIJSON(stage2Ctx, apiKey, model, postPrompt, "minimal_bbs_posts", minimalBatchPostSchema(slots), 7600)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "post generation: " + err.Error()})
			return
		}
		var postWire minimalBatchPostWire
		if err := json.Unmarshal([]byte(stage2.Text), &postWire); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "decode posts: " + err.Error(), "raw": stage2.Text})
			return
		}

		type resultRow struct {
			Slot      worldrepo.DevelopmentMinimalRootSlot `json:"slot"`
			Situation minimalBatchSituation                 `json:"situation"`
			Post      minimalBatchPost                      `json:"post"`
		}
		rows := make([]resultRow, 0, len(slots))
		for _, slot := range slots {
			rows = append(rows, resultRow{Slot: slot, Situation: situationWire.Situations[slot.EventID], Post: postWire.Posts[slot.EventID]})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"persisted": false,
			"model": model,
			"host": host.Name,
			"board": board,
			"count": len(rows),
			"situation_stage": map[string]any{"model": stage1.Model, "latency_ms": stage1.LatencyMS, "usage": stage1.Usage},
			"post_stage": map[string]any{"model": stage2.Model, "latency_ms": stage2.LatencyMS, "usage": stage2.Usage},
			"rows": rows,
		})
	}
}
