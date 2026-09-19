package worldrepo

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type developmentBehaviorAdvisor interface {
	AdviseBehavior(context.Context, worldengine.BehaviorAdviceRequest) (worldengine.BehaviorAdviceDecision, error)
}

type developmentBehaviorAdvice struct {
	Visit       map[string]float64
	Write       map[string]float64
	Reply       map[string]float64
	Model       string
	InputTokens int
	Fallback    bool
}

func (a developmentBehaviorAdvice) value(values map[string]float64, personaID, boardID string) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	value, ok := values[worldengine.BehaviorPairKey(personaID, boardID)]
	return value, ok
}

func (a developmentBehaviorAdvice) visit(personaID, boardID string) (float64, bool) {
	return a.value(a.Visit, personaID, boardID)
}

func (a developmentBehaviorAdvice) write(personaID, boardID string) (float64, bool) {
	return a.value(a.Write, personaID, boardID)
}

func (a developmentBehaviorAdvice) reply(personaID, boardID string) (float64, bool) {
	return a.value(a.Reply, personaID, boardID)
}

func (a developmentBehaviorAdvice) boardPairCount(boardID string, personas []world.Persona) int {
	count := 0
	for _, persona := range personas {
		if _, ok := a.write(persona.ID, boardID); ok {
			count++
		}
	}
	return count
}

func stabilizeJevProbability(value float64) float64 {
	// Jev is probabilistic. Bucket the advisory prior before it reaches the
	// deterministic sampler so tiny provider jitter does not routinely change
	// canonical action counts on retry.
	return math.Round(clamp01(value)*20) / 20
}

func (r *Repository) developmentJevBehaviorAdvice(host world.Host, boards []world.Board, personas []world.Persona) developmentBehaviorAdvice {
	if r == nil || r.Engine == nil || len(boards) == 0 || len(personas) == 0 {
		return developmentBehaviorAdvice{}
	}
	advisor, ok := r.Engine.(developmentBehaviorAdvisor)
	if !ok {
		return developmentBehaviorAdvice{}
	}

	sortedPersonas := append([]world.Persona(nil), personas...)
	sort.SliceStable(sortedPersonas, func(i, j int) bool { return sortedPersonas[i].ID < sortedPersonas[j].ID })
	sortedBoards := append([]world.Board(nil), boards...)
	sort.SliceStable(sortedBoards, func(i, j int) bool { return sortedBoards[i].ID < sortedBoards[j].ID })

	request := worldengine.BehaviorAdviceRequest{
		WorldDate: r.WorldDate,
		HostID:    host.ID,
		HostName:  host.Name,
		Personas:  make([]worldengine.BehaviorPersona, 0, len(sortedPersonas)),
		Boards:    make([]worldengine.BehaviorBoard, 0, len(sortedBoards)),
		Affinities: make([]worldengine.BehaviorAffinity, 0, len(sortedPersonas)*len(sortedBoards)),
	}
	for _, persona := range sortedPersonas {
		request.Personas = append(request.Personas, worldengine.BehaviorPersona{
			ID:                  persona.ID,
			Handle:              persona.Handle,
			Age:                 persona.Age,
			Occupation:          persona.Occupation,
			ActivityPattern:     persona.ActivityPattern,
			ReplyTendency:       persona.ReplyTendency,
			ThreadStartTendency: persona.ThreadStartTendency,
			LurkerTendency:      persona.LurkerTendency,
			NewcomerOpenness:    persona.NewcomerOpenness,
			Interests:           persona.Interests,
		})
	}
	for _, board := range sortedBoards {
		request.Boards = append(request.Boards, worldengine.BehaviorBoard{ID: board.ID, Name: board.Name})
		for _, persona := range sortedPersonas {
			request.Affinities = append(request.Affinities, worldengine.BehaviorAffinity{
				PersonaID: persona.ID,
				BoardID:   board.ID,
				Value:     demoBoardAffinity(persona, board),
			})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	decision, err := advisor.AdviseBehavior(ctx, request)
	if errors.Is(err, worldengine.ErrBehaviorAdvisorUnavailable) {
		return developmentBehaviorAdvice{}
	}
	if err != nil {
		return developmentBehaviorAdvice{Fallback: true}
	}

	visit := make(map[string]float64, len(decision.Probabilities))
	write := make(map[string]float64, len(decision.Probabilities))
	reply := make(map[string]float64, len(decision.Probabilities))
	for key, value := range decision.Probabilities {
		visit[key] = stabilizeJevProbability(value.Visit)
		write[key] = stabilizeJevProbability(value.Write)
		reply[key] = stabilizeJevProbability(value.Reply)
	}
	return developmentBehaviorAdvice{
		Visit:       visit,
		Write:       write,
		Reply:       reply,
		Model:       decision.Model,
		InputTokens: decision.InputTokens,
	}
}
