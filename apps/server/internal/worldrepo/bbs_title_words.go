package worldrepo

import (
	"context"
	"fmt"
	"log"
	"strings"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/titleshape"
)

// 調整値: プロンプトに渡す「すでに扱った題材」の最大件数。
const titleCoveredPromptLimit = 20

// titleShapeDiagnostics is operational telemetry about one board batch. It is
// logged, never used to accept or reject a title.
type titleShapeDiagnostics struct {
	Host              string            `json:"host"`
	Board             string            `json:"board"`
	Variants          bool              `json:"variants"`
	Retried           int               `json:"duplicate_retried"`
	DuplicateAccepted int               `json:"duplicate_accepted"`
	Report            titleshape.Report `json:"report"`
}

// wordRootTitles words the root titles in chunks. It prepares materials (a
// shuffled list of covered subjects and measured facts about recent titles),
// optionally picks one of several model variants deterministically, and gives
// exact duplicates one regeneration. A duplicate that survives is accepted.
func (p repositoryBBSBatchPlanner) wordRootTitles(
	ctx context.Context,
	titlePlanner llm.BBSSituationTitlePlanner,
	req bbsengine.BatchRequest,
	worldDate string,
	titleSeeds []llm.BBSSituationTitleSeed,
	recent []string,
) (map[string]string, titleShapeDiagnostics, error) {
	diag := titleShapeDiagnostics{Variants: p.repo != nil && p.repo.titleVariants}
	titleByEvent := make(map[string]string, len(titleSeeds))
	taken := append([]string(nil), recent...) // every title a new one must not repeat exactly

	for chunkStart := 0; chunkStart < len(titleSeeds); chunkStart += productionTitleChunkSize {
		chunkEnd := chunkStart + productionTitleChunkSize
		if chunkEnd > len(titleSeeds) {
			chunkEnd = len(titleSeeds)
		}
		chunk := titleSeeds[chunkStart:chunkEnd]
		seed := fmt.Sprintf("%s|%s|%d|%s", req.Host.ID, req.Board.ID, chunkStart/productionTitleChunkSize, chunk[0].CreatedAt)

		draft, err := p.requestTitles(ctx, titlePlanner, req, worldDate, chunk, taken, seed, diag.Variants, nil)
		if err != nil {
			return nil, diag, fmt.Errorf("word BBS Situation titles chunk %d..%d: %w", chunkStart, chunkEnd, err)
		}
		if len(draft.Titles) != len(chunk) {
			return nil, diag, fmt.Errorf("title chunk %d..%d returned %d titles, want %d", chunkStart, chunkEnd, len(draft.Titles), len(chunk))
		}

		picked := make(map[string]string, len(chunk))
		var order []string
		for _, title := range draft.Titles {
			eventID := strings.TrimSpace(title.EventID)
			subject := strings.TrimSpace(title.Subject)
			if eventID == "" || subject == "" {
				return nil, diag, fmt.Errorf("title chunk %d..%d returned empty event or subject", chunkStart, chunkEnd)
			}
			if _, exists := titleByEvent[eventID]; exists {
				return nil, diag, fmt.Errorf("title planner duplicated event %s", eventID)
			}
			if _, exists := picked[eventID]; exists {
				return nil, diag, fmt.Errorf("title planner duplicated event %s", eventID)
			}
			if len(title.Candidates) > 0 {
				subject = pickTitleVariant(seed+"|"+eventID, title.Candidates, taken, picked)
			}
			picked[eventID] = subject
			order = append(order, eventID)
		}

		// Exact duplicates of earlier titles, or of an earlier title in this
		// chunk, are regenerated once, alone.
		var dupSeeds []llm.BBSSituationTitleSeed
		seen := append([]string(nil), taken...)
		for _, eventID := range order {
			if isDuplicateOfAny(picked[eventID], seen) {
				for _, s := range chunk {
					if s.EventID == eventID {
						dupSeeds = append(dupSeeds, s)
					}
				}
			}
			seen = append(seen, picked[eventID])
		}
		if len(dupSeeds) > 0 {
			diag.Retried += len(dupSeeds)
			var dupTitles []string
			for _, s := range dupSeeds {
				dupTitles = append(dupTitles, picked[s.EventID])
			}
			retry, err := p.requestTitles(ctx, titlePlanner, req, worldDate, dupSeeds, taken, seed+"|retry", diag.Variants, dupTitles)
			if err != nil {
				log.Printf("BBS title duplicate regeneration failed; keeping originals: host=%s board=%s err=%v", req.Host.ID, req.Board.ID, err)
				diag.DuplicateAccepted += len(dupSeeds)
			} else {
				fresh := map[string]llm.BBSSituationTitle{}
				for _, t := range retry.Titles {
					fresh[strings.TrimSpace(t.EventID)] = t
				}
				for _, s := range dupSeeds {
					if t, ok := fresh[s.EventID]; ok && strings.TrimSpace(t.Subject) != "" {
						subject := strings.TrimSpace(t.Subject)
						if len(t.Candidates) > 0 {
							subject = pickTitleVariant(seed+"|retry|"+s.EventID, t.Candidates, taken, picked)
						}
						picked[s.EventID] = subject
					}
				}
				// Count what is still a duplicate after the one retry.
				seen = append([]string(nil), taken...)
				for _, eventID := range order {
					if isDuplicateOfAny(picked[eventID], seen) {
						diag.DuplicateAccepted++
					}
					seen = append(seen, picked[eventID])
				}
			}
		}

		for _, eventID := range order {
			titleByEvent[eventID] = picked[eventID]
			taken = append(taken, picked[eventID])
		}
	}
	return titleByEvent, diag, nil
}

func isDuplicateOfAny(title string, others []string) bool {
	for _, o := range others {
		if titleshape.IsDuplicate(title, o) {
			return true
		}
	}
	return false
}

// pickTitleVariant chooses among the model's variants. Variants that are exact
// duplicates of covered or already picked titles are set aside unless none
// remain; among the rest the most differently shaped one wins.
func pickTitleVariant(seed string, candidates, taken []string, pickedInChunk map[string]string) string {
	covered := append([]string(nil), taken...)
	for _, s := range pickedInChunk {
		covered = append(covered, s)
	}
	fresh := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if !isDuplicateOfAny(c, covered) {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) == 0 {
		fresh = candidates
	}
	return fresh[titleshape.PickVariant(seed, fresh, covered)]
}

func (p repositoryBBSBatchPlanner) requestTitles(
	ctx context.Context,
	titlePlanner llm.BBSSituationTitlePlanner,
	req bbsengine.BatchRequest,
	worldDate string,
	articles []llm.BBSSituationTitleSeed,
	taken []string,
	seed string,
	variants bool,
	justDuplicated []string,
) (llm.BBSSituationTitleDraft, error) {
	// Covered subjects are material for avoiding repeated topics, not examples
	// to imitate: the most recent ones, in an order unrelated to time.
	recent := taken
	if len(recent) > titleCoveredPromptLimit {
		recent = recent[len(recent)-titleCoveredPromptLimit:]
	}
	covered := titleshape.ShuffleSeeded(seed, recent)
	covered = append(covered, justDuplicated...)
	draft, err := titlePlanner.GenerateBBSSituationTitles(ctx, llm.BBSSituationTitleRequest{
		HostName:        req.Host.Name,
		HostRegion:      req.Host.Region,
		BoardID:         req.Board.ID,
		BoardName:       req.Board.Name,
		BoardScope:      req.Board.SemanticScope,
		WorldDate:       worldDate,
		RecentSubjects:  covered,
		Articles:        articles,
		FormSeed:        seed,
		RecentFormFacts: titleshape.FormFacts(recent),
		MultiVariant:    variants,
	})
	if err != nil {
		return draft, err
	}
	storeDevelopmentPlanningUsage(p.repo, req.Host.ID, "bbs-situation-title", GenerationUsage{
		InputTokens: draft.Usage.InputTokens, CachedInputTokens: draft.Usage.CachedInputTokens,
		OutputTokens: draft.Usage.OutputTokens, ReasoningTokens: draft.Usage.ReasoningTokens,
		TotalTokens: draft.Usage.TotalTokens, Model: draft.Usage.Model,
	})
	return draft, nil
}
