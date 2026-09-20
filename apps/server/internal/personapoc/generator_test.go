package personapoc

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateDeterministicAndUnique(t *testing.T) {
	a := Generate(12345, 120, "general", true)
	b := Generate(12345, 120, "general", true)
	if len(a.Personas) != 120 || len(b.Personas) != 120 {
		t.Fatalf("unexpected persona count: %d %d", len(a.Personas), len(b.Personas))
	}
	for i := range a.Personas {
		if a.Personas[i].AccountID != b.Personas[i].AccountID ||
			a.Personas[i].Handle != b.Personas[i].Handle ||
			a.Personas[i].Age != b.Personas[i].Age ||
			a.Personas[i].Occupation != b.Personas[i].Occupation ||
			a.Personas[i].ProfileSummary != b.Personas[i].ProfileSummary {
			t.Fatalf("generation is not deterministic at %d: %+v / %+v", i, a.Personas[i], b.Personas[i])
		}
	}
	if a.Quality.UniqueAccountIDRatio != 1 {
		t.Fatalf("account IDs collided: quality=%+v", a.Quality)
	}
	if a.Quality.UniqueHandleRatio != 1 {
		t.Fatalf("handles collided: quality=%+v", a.Quality)
	}
	if len(a.AccountIDPrefix) != 3 || !strings.HasPrefix(a.Personas[0].AccountID, a.AccountIDPrefix) {
		t.Fatalf("unexpected account namespace: prefix=%q first=%q scheme=%q", a.AccountIDPrefix, a.Personas[0].AccountID, a.AccountIDScheme)
	}
	if a.Quality.FutureTermHits != 0 || a.Quality.AgeOccupationWarnings != 0 {
		t.Fatalf("quality guard failed: %+v", a.Quality)
	}
}

func TestHandleCollisionVariantsAreNotOnlyNumericSuffixes(t *testing.T) {
	rng := rand.New(rand.NewSource(1995))
	variants := handleCollisionVariants(rng, "KAZU")
	if len(variants) < 5 {
		t.Fatalf("not enough collision variants: %v", variants)
	}
	hasDot, hasInitialHyphen, hasTechPrefix, hasMark, hasNumber := false, false, false, false, false
	for _, variant := range variants {
		switch {
		case strings.HasPrefix(variant, "KAZU."):
			hasDot = true
		case strings.HasPrefix(variant, "KAZU-"):
			hasInitialHyphen = true
		case strings.HasSuffix(variant, "-KAZU"):
			hasTechPrefix = true
		case variant == "KAZU☆":
			hasMark = true
		case strings.HasPrefix(variant, "KAZU") && variant != "KAZU":
			tail := strings.TrimPrefix(variant, "KAZU")
			hasNumber = len(tail) == 2 && tail[0] >= '0' && tail[0] <= '9' && tail[1] >= '0' && tail[1] <= '9'
		}
	}
	if !hasDot || !hasInitialHyphen || !hasTechPrefix || !hasMark || !hasNumber {
		t.Fatalf("collision strategies missing: dot=%v hyphen=%v tech=%v mark=%v number=%v variants=%v", hasDot, hasInitialHyphen, hasTechPrefix, hasMark, hasNumber, variants)
	}
}

func TestAccountIDsHaveGapsAndStayInOneStationNamespace(t *testing.T) {
	result := Generate(19960826, 100, "general", true)
	seen := map[string]bool{}
	for _, p := range result.Personas {
		if !strings.HasPrefix(p.AccountID, result.AccountIDPrefix) {
			t.Fatalf("account ID outside namespace: prefix=%q id=%q", result.AccountIDPrefix, p.AccountID)
		}
		if seen[p.AccountID] {
			t.Fatalf("duplicate account ID: %q", p.AccountID)
		}
		seen[p.AccountID] = true
	}
	if result.Personas[0].AccountID == result.AccountIDPrefix+"0001" &&
		result.Personas[1].AccountID == result.AccountIDPrefix+"0002" {
		t.Fatalf("account IDs look like display-order counters: %q %q", result.Personas[0].AccountID, result.Personas[1].AccountID)
	}
}

func TestHostProfileBiasesMembershipWithoutChangingGeneratorShape(t *testing.T) {
	games := Generate(777, 180, "games", true)
	tech := Generate(777, 180, "tech", true)
	gameInterestInGames := averageInterest(games.Personas, "games")
	gameInterestInTech := averageInterest(tech.Personas, "games")
	techInterestInTech := averageInterest(tech.Personas, "communications")
	techInterestInGames := averageInterest(games.Personas, "communications")
	if gameInterestInGames <= gameInterestInTech {
		t.Fatalf("games host did not select more game-oriented members: games=%.3f tech=%.3f", gameInterestInGames, gameInterestInTech)
	}
	if techInterestInTech <= techInterestInGames {
		t.Fatalf("tech host did not select more communications-oriented members: tech=%.3f games=%.3f", techInterestInTech, techInterestInGames)
	}
	if games.Quality.WildcardRatio < .07 {
		t.Fatalf("host fit selection eliminated wildcard members: %+v", games.Quality)
	}
}

func TestDetailTiersRemainSparseAtScale(t *testing.T) {
	result := Generate(999, 500, "general", true)
	core, active := 0, 0
	for _, p := range result.Personas {
		switch p.DetailTier {
		case "core-candidate":
			core++
		case "active":
			active++
		}
	}
	if core < 6 || core > 14 {
		t.Fatalf("unexpected core candidate count: %d", core)
	}
	if active < 100 || active > 150 {
		t.Fatalf("unexpected active tier count: %d", active)
	}
}

func averageInterest(personas []Identity, key string) float64 {
	if len(personas) == 0 {
		return 0
	}
	total := 0.0
	for _, p := range personas {
		total += p.Interests[key]
	}
	return total / float64(len(personas))
}
