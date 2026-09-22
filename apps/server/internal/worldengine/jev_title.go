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

var ErrTitleCandidateAdvisorUnavailable = errors.New("title candidate advisor unavailable")

type TitleEvaluationEvent struct {
	EventID        string   `json:"event_id"`
	AuthorHandle   string   `json:"author_handle"`
	CreatedAt      string   `json:"created_at"`
	CauseKind      string   `json:"cause_kind,omitempty"`
	CauseSummary   string   `json:"cause_summary,omitempty"`
	DiscourseMode  string   `json:"discourse_mode,omitempty"`
	PersonaProfile string   `json:"persona_profile,omitempty"`
	ExistingFacts  []string `json:"existing_facts,omitempty"`
}

type TitleCandidateAdviceRequest struct {
	WorldDate      string                 `json:"world_date"`
	HostID         string                 `json:"host_id"`
	HostName       string                 `json:"host_name"`
	BoardID        string                 `json:"board_id"`
	BoardName      string                 `json:"board_name"`
	Titles         []string               `json:"titles"`
	Events         []TitleEvaluationEvent `json:"events"`
	RecentBBSState string                 `json:"recent_bbs_state,omitempty"`
}

type TitleEraProbabilities struct {
	SafeWithoutResearch float64 `json:"safe_without_research"`
	LogicallyImpossible float64 `json:"logically_impossible"`
}

type TitleCandidateAdviceDecision struct {
	Model         string
	Era           map[int]TitleEraProbabilities
	Fit           map[string]float64
	InputTokens   int
}

type TitleCandidateAdvisor interface {
	AdviseTitleCandidates(context.Context, TitleCandidateAdviceRequest) (TitleCandidateAdviceDecision, error)
}

func TitleCandidatePairKey(candidate int, eventID string) string {
	return fmt.Sprintf("%d\x1f%s", candidate, eventID)
}

func (e Engine) AdviseTitleCandidates(ctx context.Context, req TitleCandidateAdviceRequest) (TitleCandidateAdviceDecision, error) {
	if e.TitleAdvisor == nil {
		return TitleCandidateAdviceDecision{}, ErrTitleCandidateAdvisorUnavailable
	}
	return e.TitleAdvisor.AdviseTitleCandidates(ctx, req)
}

// AdviseTitleCandidates uses System One only as a semantic classifier. It does
// not choose or persist a world event. The caller converts the probabilities
// into deterministic era routing and title-slot matching, and verified
// historical research remains a separate WorldEngine/HistoricalKnowledge step.
func (a JevAdvisor) AdviseTitleCandidates(ctx context.Context, req TitleCandidateAdviceRequest) (TitleCandidateAdviceDecision, error) {
	if strings.TrimSpace(a.APIKey) == "" {
		return TitleCandidateAdviceDecision{}, ErrTitleCandidateAdvisorUnavailable
	}
	if len(req.Titles) == 0 {
		return TitleCandidateAdviceDecision{Era: map[int]TitleEraProbabilities{}, Fit: map[string]float64{}}, nil
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

	recent := strings.TrimSpace(req.RecentBBSState)
	if runes := []rune(recent); len(runes) > 6000 {
		recent = string(runes[:6000])
	}

	titleState := make([]map[string]any, 0, len(req.Titles))
	for i, title := range req.Titles {
		titleState = append(titleState, map[string]any{"candidate": i + 1, "title": title})
	}
	eventState := make([]map[string]any, 0, len(req.Events))
	for _, event := range req.Events {
		eventState = append(eventState, map[string]any{
			"event_id": event.EventID,
			"author_handle": event.AuthorHandle,
			"created_at": event.CreatedAt,
			"cause_kind": event.CauseKind,
			"cause_summary": event.CauseSummary,
			"discourse_mode": event.DiscourseMode,
			"persona_profile": event.PersonaProfile,
			"existing_facts": event.ExistingFacts,
		})
	}
	state := map[string]any{
		"world_date": req.WorldDate,
		"host": map[string]any{"id": req.HostID, "name": req.HostName},
		"board": map[string]any{"id": req.BoardID, "name": req.BoardName},
		"titles": titleState,
		"events": eventState,
		"recent_bbs_state": recent,
		"policy": map[string]any{
			"era": "Classify only whether an external historical lookup is needed. Named products, works, services, standards or time-dependent real-world claims are not safe without research merely because they seem familiar. Only explicit contradictions derivable from world_date alone are logically impossible.",
			"fit": "Estimate semantic compatibility between an uncommitted title candidate and an already-selected world event slot. Board name/id are a hard placement constraint: a title that would normally belong to another board/category should receive low fit even if its era and author are plausible. Do not invent a different topic or event. A title may establish the minimal experience or opinion directly expressed by the title when it does not contradict existing persona facts. Respect board scope, author role, cause, discourse mode, recent BBS state, and SYSOP role competence.",
			"authority": "Probabilities are advisory only. Deterministic World code performs matching and persistence. Historical verification remains separate.",
		},
	}

	type target struct {
		kind      string
		candidate int
		eventID   string
	}
	questions := map[string]any{}
	targets := map[string]target{}
	for i := range req.Titles {
		candidate := i + 1
		safeKey := fmt.Sprintf("c%d_era_safe", candidate)
		targets[safeKey] = target{kind: "safe", candidate: candidate}
		questions[safeKey] = map[string]any{
			"type": "noul",
			"instructions": fmt.Sprintf("candidate=%d; probability that state.policy.era classifies it safe_without_research", candidate),
			"criteria": map[string]any{"true": "safe_without_research", "false": "research_needed"},
		}
		impossibleKey := fmt.Sprintf("c%d_era_impossible", candidate)
		targets[impossibleKey] = target{kind: "impossible", candidate: candidate}
		questions[impossibleKey] = map[string]any{
			"type": "noul",
			"instructions": fmt.Sprintf("candidate=%d; probability that state.policy.era classifies it logically_impossible", candidate),
			"criteria": map[string]any{"true": "logically_impossible", "false": "not_logically_impossible"},
		}
		for _, event := range req.Events {
			key := fmt.Sprintf("c%d_e_%s_fit", candidate, sanitizeJevQuestionKey(event.EventID))
			targets[key] = target{kind: "fit", candidate: candidate, eventID: event.EventID}
			questions[key] = map[string]any{
				"type": "noul",
				"instructions": fmt.Sprintf("candidate=%d event_id=%q; probability that state.policy.fit is satisfied", candidate, event.EventID),
				"criteria": map[string]any{"true": "fit", "false": "not_fit"},
			}
		}
	}

	payload, err := json.Marshal(map[string]any{"model": model, "state": state, "questions": questions})
	if err != nil {
		return TitleCandidateAdviceDecision{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return TitleCandidateAdviceDecision{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return TitleCandidateAdviceDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return TitleCandidateAdviceDecision{}, fmt.Errorf("jev systemone API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Model string `json:"model"`
		Answers map[string]struct {
			Type string  `json:"type"`
			Noul float64 `json:"noul"`
		} `json:"answers"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return TitleCandidateAdviceDecision{}, err
	}

	out := TitleCandidateAdviceDecision{
		Model: decoded.Model,
		Era: make(map[int]TitleEraProbabilities, len(req.Titles)),
		Fit: make(map[string]float64, len(req.Titles)*len(req.Events)),
		InputTokens: decoded.Usage.InputTokens,
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, question := range keys {
		t := targets[question]
		answer, ok := decoded.Answers[question]
		if !ok {
			return TitleCandidateAdviceDecision{}, fmt.Errorf("jev response missing answer %q", question)
		}
		if answer.Type != "" && answer.Type != "noul" {
			return TitleCandidateAdviceDecision{}, fmt.Errorf("jev answer %q has unexpected type %q", question, answer.Type)
		}
		if answer.Noul < 0 || answer.Noul > 1 {
			return TitleCandidateAdviceDecision{}, fmt.Errorf("jev answer %q probability out of range: %f", question, answer.Noul)
		}
		switch t.kind {
		case "safe":
			p := out.Era[t.candidate]
			p.SafeWithoutResearch = answer.Noul
			out.Era[t.candidate] = p
		case "impossible":
			p := out.Era[t.candidate]
			p.LogicallyImpossible = answer.Noul
			out.Era[t.candidate] = p
		case "fit":
			out.Fit[TitleCandidatePairKey(t.candidate, t.eventID)] = answer.Noul
		}
	}
	if out.Model == "" {
		out.Model = model
	}
	return out, nil
}

func sanitizeJevQuestionKey(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "event"
	}
	return b.String()
}
