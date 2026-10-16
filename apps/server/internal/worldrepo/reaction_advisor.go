package worldrepo

import (
	"context"
	"strings"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// TopicTaggerFunc is an optional function that extracts up to 5 open-vocabulary
// topic keys from an untagged human post.
type TopicTaggerFunc func(ctx context.Context, post world.Post) ([]string, error)

// BehaviorReactionAdvisor adapts worldengine.BehaviorAdvisor (such as Jev) to
// world.ReactionAdvisor (spec 008, plan 4.7). It is disabled (nil) by default.
// When enabled, it queries candidate reply priors in a single batch call and
// quantizes scores to 0.05 increments. It never creates events, chooses targets,
// or commits world facts.
type BehaviorReactionAdvisor struct {
	Behavior    worldengine.BehaviorAdvisor
	TopicTagger TopicTaggerFunc
}

func (a BehaviorReactionAdvisor) TagTopics(ctx context.Context, post world.Post) ([]string, error) {
	if a.TopicTagger == nil {
		return nil, nil
	}
	raw, err := a.TopicTagger(ctx, post)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, t := range raw {
		nk := world.NormalizeTopicKey(t)
		if nk == "" || seen[nk] {
			continue
		}
		seen[nk] = true
		out = append(out, nk)
		if len(out) >= 5 {
			break
		}
	}
	return out, nil
}

func (a BehaviorReactionAdvisor) AdviseReactors(ctx context.Context, req world.AdviceRequest) (map[string]float64, error) {
	if a.Behavior == nil || len(req.Candidates) == 0 || strings.TrimSpace(req.Board.ID) == "" {
		return nil, nil
	}
	candidates := req.Candidates
	if len(candidates) > world.ReactionCandidateCap {
		candidates = candidates[:world.ReactionCandidateCap]
	}
	personas := make([]worldengine.BehaviorPersona, 0, len(candidates))
	for _, p := range candidates {
		personas = append(personas, worldengine.BehaviorPersona{
			ID:                  p.ID,
			Handle:              p.Handle,
			Age:                 p.Age,
			Occupation:          p.Occupation,
			ActivityPattern:     p.ActivityPattern,
			ReplyTendency:       p.ReplyTendency,
			ThreadStartTendency: p.ThreadStartTendency,
			LurkerTendency:      p.LurkerTendency,
			NewcomerOpenness:    p.NewcomerOpenness,
			Interests:           p.Interests,
		})
	}
	decision, err := a.Behavior.AdviseBehavior(ctx, worldengine.BehaviorAdviceRequest{
		WorldDate: req.WorldDate,
		HostID:    req.Host.ID,
		HostName:  req.Host.Name,
		Personas:  personas,
		Boards: []worldengine.BehaviorBoard{
			{ID: req.Board.ID, Name: req.Board.Name},
		},
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(candidates))
	for _, p := range candidates {
		key := worldengine.BehaviorPairKey(p.ID, req.Board.ID)
		if prob, ok := decision.Probabilities[key]; ok {
			out[p.ID] = world.QuantizeAdvice(prob.Reply)
		}
	}
	return out, nil
}

// SetReactionAdvisor installs an optional world.ReactionAdvisor (nil disables advice).
func (r *Repository) SetReactionAdvisor(advisor world.ReactionAdvisor) {
	if r == nil {
		return
	}
	r.reactionAdvisorMu.Lock()
	r.reactionAdvisor = advisor
	r.reactionAdvisorMu.Unlock()
}

func (r *Repository) getReactionAdvisor() world.ReactionAdvisor {
	if r == nil {
		return nil
	}
	r.reactionAdvisorMu.RLock()
	adv := r.reactionAdvisor
	r.reactionAdvisorMu.RUnlock()
	return adv
}
