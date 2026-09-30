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

type minimalPromptSlot struct {
	EventID        string   `json:"event_id"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	DiscourseMode  string   `json:"discourse_mode"`
	PersonaProfile string   `json:"persona_profile"`
	SituationKind  string   `json:"situation_kind"`
	SituationFacts []string `json:"situation_facts"`
}

func minimalPromptSlots(slots []worldrepo.DevelopmentMinimalRootSlot) []minimalPromptSlot {
	out := make([]minimalPromptSlot, 0, len(slots))
	for _, slot := range slots {
		out = append(out, minimalPromptSlot{
			EventID:        slot.EventID,
			AuthorHandle:   slot.AuthorHandle,
			CreatedAt:      slot.CreatedAt,
			DiscourseMode:  slot.DiscourseMode,
			PersonaProfile: slot.PersonaProfile,
			SituationKind:  slot.SituationKind,
			SituationFacts: append([]string(nil), slot.SituationFacts...),
		})
	}
	return out
}

func minimalSituationPrompt(host world.Host, board world.Board, slots []worldrepo.DevelopmentMinimalRootSlot) string {
	data, _ := json.MarshalIndent(minimalPromptSlots(slots), "", "  ")
	return fmt.Sprintf(`1996年前後の日本の草の根パソコン通信世界です。
以下はWorld Engineがすでに選んだ独立したroot投稿枠です。
各枠について、その人物が実際に書き込みたくなる直前の「具体的な現在のシチュエーション」を1件だけ作ってください。
これは本文ではなく世界事実です。

局: %s
地域: %s
掲示板: %s

守ること:
- actor、日時、板、discourse_mode、与えられたsituation facetの意味を変えない。
- 人間の日常行動として、何が起きたかが具体的に想像できる小さな出来事にする。
- 別root同士を同じ出来事として結び付けず、設定されていない恒久的な人物設定を足さない。
- このPoCでは新しい実在ゲーム名・製品名・人物名・地名を追加しない。固有名詞なしでも出来事自体は具体的にする。

WORLD SLOTS:
%s`, host.Name, host.Region, board.Name, string(data))
}

func minimalPostPrompt(host world.Host, board world.Board, slots []worldrepo.DevelopmentMinimalRootSlot, situations map[string]minimalBatchSituation) string {
	type row struct {
		Slot      minimalPromptSlot     `json:"slot"`
		Situation minimalBatchSituation `json:"situation"`
	}
	compact := minimalPromptSlots(slots)
	rows := make([]row, 0, len(compact))
	for _, slot := range compact {
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

func newMinimalSituationTitleBatchPoCHandler(repo *worldrepo.Repository, endpoint, apiKey, defaultModel string) http.HandlerFunc {
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
		offset := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 || n > 80 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "offset must be 0..80"})
				return
			}
			offset = n
		}
		lookbackDays := 365
		if raw := strings.TrimSpace(r.URL.Query().Get("lookback_days")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 30 || n > 730 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "lookback_days must be 30..730"})
				return
			}
			lookbackDays = n
		}

		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			phone = erikaKExperimentPhone
		}
		boardID := strings.TrimSpace(r.URL.Query().Get("board"))
		if boardID == "" {
			boardID = "4"
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

		needed := offset + count
		maxPosts := needed * 6
		if maxPosts < 120 {
			maxPosts = 120
		}
		allSlots := repo.DevelopmentMinimalRootSlotsWindow(host, board, needed, lookbackDays, maxPosts)
		if len(allSlots) <= offset {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "not enough world-selected root slots",
				"available": len(allSlots),
				"requested_offset": offset,
				"requested_count": count,
			})
			return
		}
		end := offset + count
		if end > len(allSlots) {
			end = len(allSlots)
		}
		slots := append([]worldrepo.DevelopmentMinimalRootSlot(nil), allSlots[offset:end]...)

		situationPrompt := minimalSituationPrompt(host, board, slots)
		stage1Ctx, stage1Cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer stage1Cancel()
		stage1, err := callMinimalAzureOpenAIJSON(stage1Ctx, endpoint, apiKey, model, situationPrompt, "minimal_bbs_situations", minimalBatchSituationSchema(slots), 5200)
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
		stage2, err := callMinimalAzureOpenAIJSON(stage2Ctx, endpoint, apiKey, model, postPrompt, "minimal_bbs_posts", minimalBatchPostSchema(slots), 7600)
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
			"offset": offset,
			"lookback_days": lookbackDays,
			"sample_available_through": len(allSlots),
			"situation_stage": map[string]any{"model": stage1.Model, "latency_ms": stage1.LatencyMS, "usage": stage1.Usage},
			"post_stage": map[string]any{"model": stage2.Model, "latency_ms": stage2.LatencyMS, "usage": stage2.Usage},
			"rows": rows,
		})
	}
}