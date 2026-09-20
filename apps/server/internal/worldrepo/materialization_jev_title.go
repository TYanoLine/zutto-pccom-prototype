package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	developmentJevTitleEraSafeThreshold       = 0.80
	developmentJevTitleEraImpossibleThreshold = 0.80
	developmentJevTitleFitThreshold           = 0.35
)

type developmentTitleCandidateAdvisor interface {
	AdviseTitleCandidates(context.Context, worldengine.TitleCandidateAdviceRequest) (worldengine.TitleCandidateAdviceDecision, error)
}

type developmentJevTitlePlanner struct {
	titles      []string
	advice      worldengine.TitleCandidateAdviceDecision
	fitFloor    float64
	rankingOnly bool
}

func (p developmentJevTitlePlanner) GenerateBBSTitleCandidates(context.Context, string, string) (llm.BBSTitleCandidates, error) {
	return llm.BBSTitleCandidates{}, fmt.Errorf("Jev title planner does not generate candidate wording")
}

func (p developmentJevTitlePlanner) advicePairPresent(candidate int, eventID string) bool {
	_, ok := p.advice.Fit[worldengine.TitleCandidatePairKey(candidate, eventID)]
	return ok
}

func (p developmentJevTitlePlanner) ReviewBBSTitleCandidates(_ context.Context, req llm.BBSTitleReviewRequest) (llm.BBSTitleReview, error) {
	originalIndexes := make([]int, len(req.Titles))
	usedOriginal := map[int]bool{}
	for i, title := range req.Titles {
		for original, candidateTitle := range p.titles {
			idx := original + 1
			if usedOriginal[idx] || candidateTitle != title {
				continue
			}
			originalIndexes[i] = idx
			usedOriginal[idx] = true
			break
		}
		if originalIndexes[i] == 0 {
			return llm.BBSTitleReview{}, fmt.Errorf("Jev title review could not map candidate %q to original pool", title)
		}
	}

	type pair struct {
		localCandidate int
		original       int
		eventIndex     int
		eventID        string
		score          float64
	}
	pairs := make([]pair, 0, len(req.Titles)*len(req.Events))
	for local, original := range originalIndexes {
		title := req.Titles[local]
		if strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 36 || strings.ContainsAny(title, "\r\n") {
			continue
		}
		for ei, event := range req.Events {
			score := p.advice.Fit[worldengine.TitleCandidatePairKey(original, event.EventID)]
			floor := p.fitFloor
			if floor == 0 && !p.rankingOnly {
				floor = developmentJevTitleFitThreshold
			}
			if p.rankingOnly && !p.advicePairPresent(original, event.EventID) {
				continue
			}
			if score < floor {
				continue
			}
			pairs = append(pairs, pair{
				localCandidate: local + 1,
				original: original,
				eventIndex: ei,
				eventID: event.EventID,
				score: score,
			})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		if pairs[i].localCandidate != pairs[j].localCandidate {
			return pairs[i].localCandidate < pairs[j].localCandidate
		}
		return pairs[i].eventIndex < pairs[j].eventIndex
	})

	assignedCandidates := map[int]pair{}
	assignedEvents := map[string]bool{}
	for _, pair := range pairs {
		if _, ok := assignedCandidates[pair.localCandidate]; ok || assignedEvents[pair.eventID] {
			continue
		}
		assignedCandidates[pair.localCandidate] = pair
		assignedEvents[pair.eventID] = true
	}

	decisions := make([]llm.BBSTitleDecision, 0, len(req.Titles))
	for i, title := range req.Titles {
		local := i + 1
		original := originalIndexes[i]
		pair, ok := assignedCandidates[local]
		if !ok {
			best := 0.0
			for _, event := range req.Events {
				if score := p.advice.Fit[worldengine.TitleCandidatePairKey(original, event.EventID)]; score > best {
					best = score
				}
			}
			decisions = append(decisions, llm.BBSTitleDecision{
				Candidate: local,
				Reason: func() string {
					floor := p.fitFloor
					if floor == 0 && !p.rankingOnly {
						floor = developmentJevTitleFitThreshold
					}
					return fmt.Sprintf("Jev人物/投稿枠適合 %.2f（採用床 %.2f 未満または高得点枠が他候補に割当済み）", best, floor)
				}(),
			})
			continue
		}
		decisions = append(decisions, llm.BBSTitleDecision{
			Candidate: local,
			EventID: pair.eventID,
			Subject: title,
			Reason: fmt.Sprintf("Jev人物/投稿枠適合 %.2f; World側の決定的マッチングで採用", pair.score),
			Summary: "この人物が「" + title + "」を話題にする",
			Details: []string{},
		})
	}
	return llm.BBSTitleReview{Decisions: decisions}, nil
}

type developmentJevTitleEraValidator struct {
	advice      worldengine.TitleCandidateAdviceDecision
	observeOnly bool
}

func (v developmentJevTitleEraValidator) ValidateBBSTitleEra(_ context.Context, req llm.BBSTitleEraRequest) (llm.BBSTitleEraReview, error) {
	decisions := make([]llm.BBSTitleEraDecision, 0, len(req.Titles))
	for i := range req.Titles {
		candidate := i + 1
		p := v.advice.Era[candidate]
		status := llm.BBSTitleEraResearch
		reason := fmt.Sprintf("Jev一次振り分け: safe_without_research=%.2f logically_impossible=%.2f", p.SafeWithoutResearch, p.LogicallyImpossible)
		switch {
		case p.LogicallyImpossible >= developmentJevTitleEraImpossibleThreshold:
			status = llm.BBSTitleEraNG
		case p.SafeWithoutResearch >= developmentJevTitleEraSafeThreshold:
			status = llm.BBSTitleEraOK
		default:
			status = llm.BBSTitleEraResearch
		}
		if v.observeOnly {
			reason = fmt.Sprintf("LAB observe-only: original=%s; %s", status, reason)
			status = llm.BBSTitleEraOK
		}
		decisions = append(decisions, llm.BBSTitleEraDecision{Candidate: candidate, Status: status, Reason: reason})
	}
	return llm.BBSTitleEraReview{Decisions: decisions}, nil
}

func developmentTitleEraObserveOnly(renderer llm.BoardPostRenderer) bool {
	marker, ok := renderer.(interface{ TitleEraObserveOnly() bool })
	return ok && marker.TitleEraObserveOnly()
}

func (r *Repository) developmentJevTitleAdvice(
	ctx context.Context,
	host world.Host,
	board world.Board,
	asOf string,
	titles []string,
	events []llm.BBSWorldWindowEvent,
	recentBBSState string,
) (worldengine.TitleCandidateAdviceDecision, bool, error) {
	advisor, ok := r.Engine.(developmentTitleCandidateAdvisor)
	if !ok {
		return worldengine.TitleCandidateAdviceDecision{}, false, nil
	}
	adviceEvents := make([]worldengine.TitleEvaluationEvent, 0, len(events))
	for _, event := range events {
		adviceEvents = append(adviceEvents, worldengine.TitleEvaluationEvent{
			EventID: event.EventID,
			AuthorHandle: event.AuthorHandle,
			CreatedAt: event.CreatedAt,
			CauseKind: event.CauseKind,
			CauseSummary: event.CauseSummary,
			DiscourseMode: event.DiscourseMode,
			PersonaProfile: event.PersonaProfile,
			ExistingFacts: append([]string(nil), event.ExistingFacts...),
		})
	}
	decision, err := advisor.AdviseTitleCandidates(ctx, worldengine.TitleCandidateAdviceRequest{
		WorldDate: asOf,
		HostID: host.ID,
		HostName: host.Name,
		BoardID: board.ID,
		BoardName: board.Name,
		Titles: append([]string(nil), titles...),
		Events: adviceEvents,
		RecentBBSState: recentBBSState,
	})
	if err != nil {
		return decision, true, err
	}
	if len(decision.Era) != len(titles) {
		return decision, true, fmt.Errorf("Jev title advice omitted era candidates: got %d want %d", len(decision.Era), len(titles))
	}
	return decision, true, nil
}
