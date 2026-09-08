package worldrepo

import (
	"fmt"
	"hash/fnv"
	"strings"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
)

type developmentGroundingCandidate struct {
	Name     string
	Evidence string
	Rank     int
}

func developmentGroundingCandidatesFromKnowledge(result historicalkb.KnowledgeResult) []developmentGroundingCandidate {
	if !result.CanUse {
		return nil
	}
	out := make([]developmentGroundingCandidate, 0, 8)
	seen := map[string]struct{}{}
	for _, fact := range result.Facts {
		if fact.Status != historicalkb.FactVerified && fact.Status != historicalkb.FactOperatorVerified && fact.Status != historicalkb.FactCanonical {
			continue
		}
		for _, candidate := range developmentParseGroundingCandidates(fact.Claim) {
			key := developmentNormalizeReferentName(candidate.Name)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			candidate.Rank = len(out)
			seen[key] = struct{}{}
			out = append(out, candidate)
		}
	}
	return out
}

func developmentParseGroundingCandidates(claim string) []developmentGroundingCandidate {
	lines := strings.Split(strings.ReplaceAll(claim, "\r\n", "\n"), "\n")
	out := make([]developmentGroundingCandidate, 0, 5)
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		if !strings.HasPrefix(strings.ToUpper(line), "CANDIDATE:") {
			continue
		}
		rest := strings.TrimSpace(line[len("CANDIDATE:"):])
		parts := strings.SplitN(rest, "||", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		evidence := ""
		if len(parts) == 2 {
			evidence = strings.TrimSpace(parts[1])
		}
		out = append(out, developmentGroundingCandidate{Name: name, Evidence: evidence, Rank: len(out)})
	}
	return out
}

func developmentGroundingActorContext(persona world.Persona, facts []world.PersonaFact) string {
	parts := []string{personaSummary(persona)}
	for i, fact := range facts {
		if i >= 8 {
			break
		}
		key := strings.TrimSpace(fact.Key)
		value := strings.TrimSpace(fact.Value)
		if key != "" && value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "; ")
}

func developmentSelectGroundingCandidate(hostID string, root developmentWindowShell, options []developmentGroundingCandidate, recentPosts []world.Post, selectedNames map[string]int) (developmentGroundingCandidate, bool) {
	if len(options) == 0 {
		return developmentGroundingCandidate{}, false
	}
	best := options[0]
	bestScore := -1 << 30
	for _, option := range options {
		normalized := developmentNormalizeReferentName(option.Name)
		if normalized == "" {
			continue
		}
		score := 1000 - option.Rank*35
		score -= developmentReferentRecentCount(option.Name, recentPosts) * 300
		score -= selectedNames[normalized] * 700
		score += int(developmentStableGroundingHash(hostID+"|"+root.eventID+"|"+root.shell.persona.ID+"|"+option.Name) % 97)
		if score > bestScore {
			bestScore = score
			best = option
		}
	}
	return best, bestScore > (-1 << 30)
}

func developmentReferentRecentCount(name string, posts []world.Post) int {
	normalized := developmentNormalizeReferentName(name)
	if normalized == "" {
		return 0
	}
	count := 0
	for _, post := range posts {
		haystack := developmentNormalizeReferentName(post.Subject + " " + post.Body)
		if strings.Contains(haystack, normalized) {
			count++
		}
	}
	return count
}

func developmentStableGroundingHash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func developmentNormalizeReferentName(value string) string {
	replacer := strings.NewReplacer(" ", "", "　", "", "『", "", "』", "", "「", "", "」", "", "・", "", "-", "", "_", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func developmentSelectedGroundingEvidence(candidate developmentGroundingCandidate) string {
	return fmt.Sprintf("GROUNDING WORLD-SELECTED REFERENT - MUST USE: %s || VERIFIED SEARCH FIT: %s", candidate.Name, candidate.Evidence)
}

func developmentSelectedGroundingName(evidence []string) string {
	const prefix = "GROUNDING WORLD-SELECTED REFERENT - MUST USE:"
	for _, item := range evidence {
		if !strings.HasPrefix(item, prefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(item, prefix))
		if idx := strings.Index(rest, "||"); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
		}
		return rest
	}
	return ""
}

func developmentRefinementUsesSelectedReferent(refined developmentSituationProposal, selected string) bool {
	selected = developmentNormalizeReferentName(selected)
	if selected == "" {
		return false
	}
	combined := developmentNormalizeReferentName(strings.Join([]string{refined.objectClass, refined.occurrence, refined.actorObservation}, " "))
	return strings.Contains(combined, selected)
}
