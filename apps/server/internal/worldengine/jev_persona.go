package worldengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

var ErrPersonaProfileAdvisorUnavailable = errors.New("persona profile advisor unavailable")

type PersonaProfileAdviceItem struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
}

type PersonaProfileAdviceRequest struct {
	WorldDate string                     `json:"world_date"`
	Profiles  []PersonaProfileAdviceItem `json:"profiles"`
}

type PersonaProfileAdviceProbabilities struct {
	FutureInformation float64 `json:"future_information"`
	ExternalReview    float64 `json:"external_review"`
}

type PersonaProfileAdviceDecision struct {
	Model       string
	Profiles    map[string]PersonaProfileAdviceProbabilities
	InputTokens int
}

type PersonaProfileAdvisor interface {
	AdvisePersonaProfiles(context.Context, PersonaProfileAdviceRequest) (PersonaProfileAdviceDecision, error)
}

// AdvisePersonaProfiles is a Lab-only semantic audit. It never changes a
// persona or creates world facts; callers use the probabilities to measure
// whether an additional era gate appears useful after a tightly constrained
// presentation-only profile generation prompt.
func (a JevAdvisor) AdvisePersonaProfiles(ctx context.Context, req PersonaProfileAdviceRequest) (PersonaProfileAdviceDecision, error) {
	if strings.TrimSpace(a.APIKey) == "" {
		return PersonaProfileAdviceDecision{}, ErrPersonaProfileAdvisorUnavailable
	}
	if len(req.Profiles) == 0 {
		return PersonaProfileAdviceDecision{Profiles: map[string]PersonaProfileAdviceProbabilities{}}, nil
	}

	model := strings.TrimSpace(a.Model)
	if model == "" {
		model = "jev-latest"
	}
	endpoint := strings.TrimSpace(a.Endpoint)
	if endpoint == "" {
		endpoint = defaultJevEndpoint
	}
	client := a.Client
	if client == nil {
		client = &http.Client{}
	}

	stateProfiles := make([]map[string]any, 0, len(req.Profiles))
	seen := map[string]bool{}
	for _, item := range req.Profiles {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			return PersonaProfileAdviceDecision{}, fmt.Errorf("invalid/duplicate persona profile id %q", id)
		}
		seen[id] = true
		stateProfiles = append(stateProfiles, map[string]any{"id": id, "profile": item.Profile})
	}
	state := map[string]any{
		"world_date": req.WorldDate,
		"profiles":   stateProfiles,
		"policy": map[string]any{
			"future_information": "True only when the profile contains terminology, products, services, works, events, technologies, cultural framing, or other information that would not yet exist or would not reasonably be knowable to a Japanese BBS participant by world_date. Retrospective later-observer framing also counts. Generic timeless personality/participation descriptions do not.",
			"external_review": "True when the profile asserts a specific time-sensitive real-world fact, named product/service/work/event/version/standard, release/timing claim, or other external historical claim that should be verified. Generic terms such as パソコン通信, モデム, ソフトウェア, ゲーム, 音楽, 地域, ファイル, チャット are not by themselves review-worthy.",
			"authority": "This is an advisory audit only. Do not rewrite the profile and do not infer unmentioned biography.",
		},
	}

	type target struct {
		id   string
		kind string
	}
	questions := map[string]any{}
	targets := map[string]target{}
	for _, item := range req.Profiles {
		id := strings.TrimSpace(item.ID)
		keyBase := sanitizeJevQuestionKey(id)
		futureKey := "p_" + keyBase + "_future"
		reviewKey := "p_" + keyBase + "_review"
		targets[futureKey] = target{id: id, kind: "future"}
		targets[reviewKey] = target{id: id, kind: "review"}
		questions[futureKey] = map[string]any{
			"type":         "noul",
			"instructions": fmt.Sprintf("persona_id=%q; probability that state.policy.future_information is true for this profile", id),
			"criteria":     map[string]any{"true": "contains_future_information", "false": "no_future_information"},
		}
		questions[reviewKey] = map[string]any{
			"type":         "noul",
			"instructions": fmt.Sprintf("persona_id=%q; probability that state.policy.external_review is true for this profile", id),
			"criteria":     map[string]any{"true": "external_review_needed", "false": "no_external_review_needed"},
		}
	}

	payload, err := json.Marshal(map[string]any{"model": model, "state": state, "questions": questions})
	if err != nil {
		return PersonaProfileAdviceDecision{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return PersonaProfileAdviceDecision{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return PersonaProfileAdviceDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return PersonaProfileAdviceDecision{}, fmt.Errorf("jev systemone API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type string  `json:"type"`
			Noul float64 `json:"noul"`
		} `json:"answers"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return PersonaProfileAdviceDecision{}, err
	}

	out := PersonaProfileAdviceDecision{
		Model:       decoded.Model,
		Profiles:    make(map[string]PersonaProfileAdviceProbabilities, len(req.Profiles)),
		InputTokens: decoded.Usage.InputTokens,
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t := targets[key]
		answer, ok := decoded.Answers[key]
		if !ok {
			return PersonaProfileAdviceDecision{}, fmt.Errorf("jev response missing answer %q", key)
		}
		if answer.Type != "" && answer.Type != "noul" {
			return PersonaProfileAdviceDecision{}, fmt.Errorf("jev answer %q has unexpected type %q", key, answer.Type)
		}
		if answer.Noul < 0 || answer.Noul > 1 {
			return PersonaProfileAdviceDecision{}, fmt.Errorf("jev answer %q probability out of range: %f", key, answer.Noul)
		}
		p := out.Profiles[t.id]
		if t.kind == "future" {
			p.FutureInformation = answer.Noul
		} else {
			p.ExternalReview = answer.Noul
		}
		out.Profiles[t.id] = p
	}
	if out.Model == "" {
		out.Model = model
	}
	return out, nil
}
