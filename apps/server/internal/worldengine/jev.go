package worldengine

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
)

const defaultJevEndpoint = "https://api.typesafe.ai/v1/systemone"

var ErrWriteAdvisorUnavailable = errors.New("write propensity advisor unavailable")
var ErrBehaviorAdvisorUnavailable = errors.New("behavior advisor unavailable")

type BehaviorPersona struct {
	ID                  string             `json:"id"`
	Handle              string             `json:"handle"`
	Age                 int                `json:"age,omitempty"`
	Occupation          string             `json:"occupation,omitempty"`
	ActivityPattern     string             `json:"activity_pattern,omitempty"`
	ReplyTendency       float64            `json:"reply_tendency"`
	ThreadStartTendency float64            `json:"thread_start_tendency"`
	LurkerTendency      float64            `json:"lurker_tendency"`
	NewcomerOpenness    float64            `json:"newcomer_openness"`
	Interests           map[string]float64 `json:"interests,omitempty"`
}

type BehaviorBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BehaviorAffinity struct {
	PersonaID string  `json:"persona_id"`
	BoardID   string  `json:"board_id"`
	Value     float64 `json:"value"`
}

type BehaviorAdviceRequest struct {
	WorldDate  string             `json:"world_date"`
	HostID     string             `json:"host_id"`
	HostName   string             `json:"host_name"`
	Personas   []BehaviorPersona  `json:"personas"`
	Boards     []BehaviorBoard    `json:"boards"`
	Affinities []BehaviorAffinity `json:"affinities"`
}

type BehaviorProbabilities struct {
	Visit float64 `json:"visit"`
	Write float64 `json:"write"`
	Reply float64 `json:"reply"`
}

type BehaviorAdviceDecision struct {
	Model         string
	Probabilities map[string]BehaviorProbabilities
	InputTokens   int
}

type BehaviorAdvisor interface {
	AdviseBehavior(context.Context, BehaviorAdviceRequest) (BehaviorAdviceDecision, error)
}

func BehaviorPairKey(personaID, boardID string) string {
	return personaID + "\x1f" + boardID
}


type WritePropensityPersona struct {
	ID                  string             `json:"id"`
	Handle              string             `json:"handle"`
	Age                 int                `json:"age,omitempty"`
	Occupation          string             `json:"occupation,omitempty"`
	ActivityPattern     string             `json:"activity_pattern,omitempty"`
	ReplyTendency       float64            `json:"reply_tendency"`
	ThreadStartTendency float64            `json:"thread_start_tendency"`
	LurkerTendency      float64            `json:"lurker_tendency"`
	NewcomerOpenness    float64            `json:"newcomer_openness"`
	BoardAffinity       float64            `json:"board_affinity"`
	Interests           map[string]float64 `json:"interests,omitempty"`
	VisitCount          int                `json:"visit_count"`
}

type WritePropensityRequest struct {
	WorldDate string                   `json:"world_date"`
	HostID    string                   `json:"host_id"`
	HostName  string                   `json:"host_name"`
	BoardID   string                   `json:"board_id"`
	BoardName string                   `json:"board_name"`
	Personas  []WritePropensityPersona `json:"personas"`
}

type WritePropensityDecision struct {
	Model         string
	Probabilities map[string]float64
	InputTokens   int
}

type WritePropensityAdvisor interface {
	AdviseWritePropensities(context.Context, WritePropensityRequest) (WritePropensityDecision, error)
}

func (e Engine) AdviseWritePropensities(ctx context.Context, req WritePropensityRequest) (WritePropensityDecision, error) {
	if e.WriteAdvisor == nil {
		return WritePropensityDecision{}, ErrWriteAdvisorUnavailable
	}
	return e.WriteAdvisor.AdviseWritePropensities(ctx, req)
}

func (e Engine) AdviseBehavior(ctx context.Context, req BehaviorAdviceRequest) (BehaviorAdviceDecision, error) {
	if e.BehaviorAdvisor == nil {
		return BehaviorAdviceDecision{}, ErrBehaviorAdvisorUnavailable
	}
	return e.BehaviorAdvisor.AdviseBehavior(ctx, req)
}

// JevAdvisor is deliberately narrow: it estimates coarse behavioral priors for
// board visits, write-vs-ROM decisions, and reply-vs-root topology. It never
// creates an event, chooses a topic or target, or commits world state. The World
// Engine remains authoritative.
type JevAdvisor struct {
	APIKey   string
	Model    string
	Endpoint string
	Client   *http.Client
}

func (a JevAdvisor) AdviseBehavior(ctx context.Context, req BehaviorAdviceRequest) (BehaviorAdviceDecision, error) {
	if strings.TrimSpace(a.APIKey) == "" {
		return BehaviorAdviceDecision{}, ErrBehaviorAdvisorUnavailable
	}
	if len(req.Personas) == 0 || len(req.Boards) == 0 {
		return BehaviorAdviceDecision{Probabilities: map[string]BehaviorProbabilities{}}, nil
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
		client = &http.Client{Timeout: 3 * time.Second}
	}

	state := map[string]any{
		"world_date": req.WorldDate,
		"host": map[string]any{
			"id": req.HostID,
			"name": req.HostName,
		},
		"personas": req.Personas,
		"boards": req.Boards,
		"board_affinities": req.Affinities,
		"decision_policy": map[string]any{
			"visit": "Whether this persona would plausibly visit/read this board on a representative eligible day. Visiting is not posting.",
			"write": "Conditional on a plausible visit, whether it naturally becomes a write opportunity instead of ROM/no-op. Ordinary interest, membership, equipment, or routine use is not itself a posting cause.",
			"reply": "Conditional on a write opportunity and at least one valid causal reply target already existing, whether replying is more natural than starting an independent root. This does not authorize a reply target or invent thread activity.",
			"authority": "These are behavioral priors only. The World Engine separately decides deterministic sampling, valid causes, thread topology, participation state, and persistence.",
		},
	}

	type answerTarget struct {
		pair      string
		dimension string
	}
	questions := make(map[string]any, len(req.Personas)*len(req.Boards)*3)
	targets := make(map[string]answerTarget, len(req.Personas)*len(req.Boards)*3)
	for pi, persona := range req.Personas {
		for bi, board := range req.Boards {
			pair := BehaviorPairKey(persona.ID, board.ID)
			prefix := fmt.Sprintf("p%d_b%d", pi, bi)
			specs := []struct {
				suffix       string
				instructions string
				trueText     string
				falseText    string
			}{
				{
					suffix: "visit",
					instructions: fmt.Sprintf("For persona id %q and board id %q, estimate whether this persona would naturally visit/read this board on a representative eligible day in the simulated window. Use activity pattern, interests, lurker tendency, and the supplied board affinity as priors.", persona.ID, board.ID),
					trueText: "A visit/read is behaviorally plausible on such a day.",
					falseText: "Skipping this board that day is more natural.",
				},
				{
					suffix: "write",
					instructions: fmt.Sprintf("For persona id %q and board id %q, conditional on an already-plausible visit, estimate whether it would naturally become a write opportunity rather than ROM/no-op. Do not invent a current event, purchase, problem, change, or surprise merely to justify writing.", persona.ID, board.ID),
					trueText: "Writing is behaviorally natural if the World Engine later has a separate valid cause or reply target.",
					falseText: "ROM/no-op is more natural for this visit.",
				},
				{
					suffix: "reply",
					instructions: fmt.Sprintf("For persona id %q and board id %q, conditional on a write opportunity and at least one valid causal reply target already existing, estimate whether replying is more natural than starting an independent root. Do not invent a thread or target.", persona.ID, board.ID),
					trueText: "Replying to a valid existing target is more natural than an independent root.",
					falseText: "Starting an independent root is more natural than replying, assuming a separate valid root cause exists.",
				},
			}
			for _, spec := range specs {
				key := prefix + "_" + spec.suffix
				targets[key] = answerTarget{pair: pair, dimension: spec.suffix}
				questions[key] = map[string]any{
					"type": "noul",
					"instructions": spec.instructions,
					"criteria": map[string]any{
						"true": spec.trueText,
						"false": spec.falseText,
					},
				}
			}
		}
	}

	payload, err := json.Marshal(map[string]any{
		"model": model,
		"state": state,
		"questions": questions,
	})
	if err != nil {
		return BehaviorAdviceDecision{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return BehaviorAdviceDecision{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return BehaviorAdviceDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return BehaviorAdviceDecision{}, fmt.Errorf("jev systemone API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
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
		return BehaviorAdviceDecision{}, err
	}

	out := BehaviorAdviceDecision{
		Model: decoded.Model,
		Probabilities: make(map[string]BehaviorProbabilities, len(req.Personas)*len(req.Boards)),
		InputTokens: decoded.Usage.InputTokens,
	}
	for question, target := range targets {
		answer, ok := decoded.Answers[question]
		if !ok {
			return BehaviorAdviceDecision{}, fmt.Errorf("jev response missing answer %q", question)
		}
		if answer.Type != "" && answer.Type != "noul" {
			return BehaviorAdviceDecision{}, fmt.Errorf("jev answer %q has unexpected type %q", question, answer.Type)
		}
		if answer.Noul < 0 || answer.Noul > 1 {
			return BehaviorAdviceDecision{}, fmt.Errorf("jev answer %q probability out of range: %f", question, answer.Noul)
		}
		probabilities := out.Probabilities[target.pair]
		switch target.dimension {
		case "visit":
			probabilities.Visit = answer.Noul
		case "write":
			probabilities.Write = answer.Noul
		case "reply":
			probabilities.Reply = answer.Noul
		}
		out.Probabilities[target.pair] = probabilities
	}
	if out.Model == "" {
		out.Model = model
	}
	return out, nil
}

func (a JevAdvisor) AdviseWritePropensities(ctx context.Context, req WritePropensityRequest) (WritePropensityDecision, error) {
	if strings.TrimSpace(a.APIKey) == "" {
		return WritePropensityDecision{}, ErrWriteAdvisorUnavailable
	}
	if len(req.Personas) == 0 {
		return WritePropensityDecision{Probabilities: map[string]float64{}}, nil
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
		client = &http.Client{Timeout: 2 * time.Second}
	}

	state := map[string]any{
		"world_date": req.WorldDate,
		"host": map[string]any{
			"id":   req.HostID,
			"name": req.HostName,
		},
		"board": map[string]any{
			"id":   req.BoardID,
			"name": req.BoardName,
		},
		"personas": req.Personas,
		"world_rule": "A plausible visit is not automatically a post. Ordinary interests and baseline context only route attention; a separate concrete cause or reply target is still required before any write can exist.",
	}
	questions := make(map[string]any, len(req.Personas))
	personaByQuestion := make(map[string]string, len(req.Personas))
	for i, persona := range req.Personas {
		key := fmt.Sprintf("persona_%d", i)
		personaByQuestion[key] = persona.ID
		questions[key] = map[string]any{
			"type": "noul",
			"instructions": fmt.Sprintf(
				"For persona id %q in the shared state, estimate whether one already-plausible visit to this board would naturally become a write opportunity (new root or reply) rather than ROM/no-op. Use the supplied behavioral tendencies, board affinity, and visit context as priors. Do not invent an event, product, purchase, problem, or surprising occurrence to justify writing; ordinary interest alone is not a cause.",
				persona.ID,
			),
			"criteria": map[string]any{
				"true":  "Writing is behaviorally natural for this plausible visit, assuming the World Engine later finds a separate valid root cause or reply target.",
				"false": "Reading/ROM/no-op is more natural for this plausible visit; ordinary interest or membership alone is insufficient.",
			},
		}
	}

	payload, err := json.Marshal(map[string]any{
		"model":     model,
		"state":     state,
		"questions": questions,
	})
	if err != nil {
		return WritePropensityDecision{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return WritePropensityDecision{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return WritePropensityDecision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return WritePropensityDecision{}, fmt.Errorf("jev systemone API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
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
		return WritePropensityDecision{}, err
	}

	out := WritePropensityDecision{
		Model:         decoded.Model,
		Probabilities: make(map[string]float64, len(req.Personas)),
		InputTokens:   decoded.Usage.InputTokens,
	}
	for question, personaID := range personaByQuestion {
		answer, ok := decoded.Answers[question]
		if !ok {
			return WritePropensityDecision{}, fmt.Errorf("jev response missing answer %q", question)
		}
		if answer.Type != "" && answer.Type != "noul" {
			return WritePropensityDecision{}, fmt.Errorf("jev answer %q has unexpected type %q", question, answer.Type)
		}
		if answer.Noul < 0 || answer.Noul > 1 {
			return WritePropensityDecision{}, fmt.Errorf("jev answer %q probability out of range: %f", question, answer.Noul)
		}
		out.Probabilities[personaID] = answer.Noul
	}
	if out.Model == "" {
		out.Model = model
	}
	return out, nil
}
