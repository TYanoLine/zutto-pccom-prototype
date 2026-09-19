package worldrepo

import (
	"context"
	"errors"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type developmentWritePropensityAdvisor interface {
	AdviseWritePropensities(context.Context, worldengine.WritePropensityRequest) (worldengine.WritePropensityDecision, error)
}

func (r *Repository) developmentJevWriteProbabilities(host world.Host, board world.Board, visits []demoPostCandidate) (map[string]float64, string, error) {
	if r == nil || r.Engine == nil || len(visits) == 0 {
		return nil, "", nil
	}
	advisor, ok := r.Engine.(developmentWritePropensityAdvisor)
	if !ok {
		return nil, "", nil
	}

	type personaVisits struct {
		persona world.Persona
		visits  int
	}
	byPersona := map[string]personaVisits{}
	for _, candidate := range visits {
		value := byPersona[candidate.persona.ID]
		value.persona = candidate.persona
		value.visits++
		byPersona[candidate.persona.ID] = value
	}
	personaIDs := make([]string, 0, len(byPersona))
	for personaID := range byPersona {
		personaIDs = append(personaIDs, personaID)
	}
	sort.Strings(personaIDs)

	personas := make([]worldengine.WritePropensityPersona, 0, len(personaIDs))
	for _, personaID := range personaIDs {
		value := byPersona[personaID]
		p := value.persona
		personas = append(personas, worldengine.WritePropensityPersona{
			ID:                  p.ID,
			Handle:              p.Handle,
			Age:                 p.Age,
			Occupation:          p.Occupation,
			ActivityPattern:     p.ActivityPattern,
			ReplyTendency:       p.ReplyTendency,
			ThreadStartTendency: p.ThreadStartTendency,
			LurkerTendency:      p.LurkerTendency,
			NewcomerOpenness:    p.NewcomerOpenness,
			BoardAffinity:       demoBoardAffinity(p, board),
			Interests:           p.Interests,
			VisitCount:          value.visits,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	decision, err := advisor.AdviseWritePropensities(ctx, worldengine.WritePropensityRequest{
		WorldDate: r.WorldDate,
		HostID:    host.ID,
		HostName:  host.Name,
		BoardID:   board.ID,
		BoardName: board.Name,
		Personas:  personas,
	})
	if errors.Is(err, worldengine.ErrWriteAdvisorUnavailable) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return decision.Probabilities, decision.Model, nil
}
