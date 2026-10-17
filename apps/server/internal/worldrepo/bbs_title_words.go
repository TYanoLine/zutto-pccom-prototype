package worldrepo

import (
	"fmt"

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

// selectDraftTitles selects the final subject for each root draft deterministically.
// When variants are enabled, it picks the variant most differently shaped from
// recently taken titles. Duplicate counts are recorded in diagnostics.
func (p repositoryBBSBatchPlanner) selectDraftTitles(
	req bbsengine.BatchRequest,
	orderedEventIDs []string,
	drafts map[string]llm.BBSRootDraft,
	recent []string,
) (map[string]string, titleShapeDiagnostics) {
	diag := titleShapeDiagnostics{Variants: p.repo != nil && p.repo.titleVariants}
	titleByEvent := make(map[string]string, len(orderedEventIDs))
	taken := append([]string(nil), recent...)
	pickedInBatch := make(map[string]string, len(orderedEventIDs))

	for i, eventID := range orderedEventIDs {
		draft := drafts[eventID]
		seed := fmt.Sprintf("%s|%s|%d|%s", req.Host.ID, req.Board.ID, i, eventID)
		subject := draft.Titles[0]
		if diag.Variants && len(draft.Titles) > 1 {
			subject = pickTitleVariant(seed, draft.Titles, taken, pickedInBatch)
		}
		if isDuplicateOfAny(subject, taken) {
			diag.DuplicateAccepted++
		}
		pickedInBatch[eventID] = subject
		titleByEvent[eventID] = subject
		taken = append(taken, subject)
	}
	return titleByEvent, diag
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
