package world

import (
	"testing"
	"time"
)

func TestDefaultLexiconAndBroaderPropagation(t *testing.T) {
	lex := DefaultLexicon()
	if lex == nil || len(lex.Topics) == 0 {
		t.Fatalf("DefaultLexicon returned empty lexicon")
	}

	// Property 6: Broader propagation for pc98 -> computers
	broader := lex.Broader("pc98")
	if len(broader) == 0 || broader[0] != "computers" {
		t.Fatalf("expected pc98 broader to include computers, got %v", broader)
	}

	weights := ArticleTopicWeights(nil, []string{"pc98"}, lex, nil, time.Date(1996, 5, 10, 12, 0, 0, 0, jstZone))
	wPC98 := weights["pc98"]
	wComputers := weights["computers"]
	if wPC98 <= 0 || wComputers <= 0 {
		t.Fatalf("expected positive weights for pc98 and computers, got %v", weights)
	}
	if wPC98 <= wComputers {
		t.Fatalf("expected direct topic pc98 (%f) > broader topic computers (%f)", wPC98, wComputers)
	}

	pPC98 := Persona{
		ID:        "p-pc98",
		Interests: map[string]float64{"pc98": 0.9},
	}
	pComp := Persona{
		ID:        "p-comp",
		Interests: map[string]float64{"computers": 0.9},
	}
	pFood := Persona{
		ID:        "p-food",
		Interests: map[string]float64{"food": 0.9},
	}

	relPC98 := Relevance(pPC98, nil, weights)
	relComp := Relevance(pComp, nil, weights)
	relFood := Relevance(pFood, nil, weights)
	if !(relPC98 > relComp && relComp > relFood) {
		t.Fatalf("expected rel(pc98)=%f > rel(computers)=%f > rel(food)=%f", relPC98, relComp, relFood)
	}
}

func TestOpenVocabularyUnknownTopicMatching(t *testing.T) {
	lex := DefaultLexicon()
	// Property 5: Open-vocabulary topic key not present in lexicon
	unknownTopic := "amiga_4000_toaster"
	if len(lex.Broader(unknownTopic)) != 0 {
		t.Fatalf("expected unknown topic to have no broader keys in DefaultLexicon")
	}

	weights := ArticleTopicWeights(nil, []string{unknownTopic}, lex, nil, time.Date(1996, 6, 1, 20, 0, 0, 0, jstZone))
	if weights[unknownTopic] <= 0 {
		t.Fatalf("expected positive weight for open-vocabulary topic %q, got %v", unknownTopic, weights)
	}

	pMatchInterest := Persona{
		ID:        "p-amiga-interest",
		Interests: map[string]float64{unknownTopic: 0.85},
	}
	pMatchFact := Persona{
		ID: "p-amiga-fact",
	}
	facts := []PersonaFact{
		{PersonaID: "p-amiga-fact", Topic: unknownTopic, Value: "Amiga 4000 user"},
	}
	pUnrelated := Persona{
		ID:        "p-unrelated",
		Interests: map[string]float64{"cars": 0.9},
	}

	relInterest := Relevance(pMatchInterest, nil, weights)
	relFact := Relevance(pMatchFact, facts, weights)
	relUnrelated := Relevance(pUnrelated, nil, weights)
	if relInterest <= relUnrelated {
		t.Fatalf("expected persona with unknown topic in Interests (%f) > unrelated (%f)", relInterest, relUnrelated)
	}
	if relFact <= relUnrelated {
		t.Fatalf("expected persona with unknown topic in PersonaFact (%f) > unrelated (%f)", relFact, relUnrelated)
	}
}

func TestSalienceDateWindow(t *testing.T) {
	lex := DefaultLexicon()
	boardPrior := map[string]float64{
		"games": 1.0,
		"food":  1.0,
	}
	topics := []string{"pokemon", "food"}

	inWindow := time.Date(1996, 3, 5, 21, 0, 0, 0, jstZone)
	outWindow := time.Date(1996, 10, 5, 21, 0, 0, 0, jstZone)

	wIn := ArticleTopicWeights(boardPrior, topics, lex, nil, inWindow)
	wOut := ArticleTopicWeights(boardPrior, topics, lex, nil, outWindow)
	if wIn["pokemon"] <= wOut["pokemon"] {
		t.Fatalf("expected pokemon weight inside salience window (%f) > outside (%f)", wIn["pokemon"], wOut["pokemon"])
	}
}

func TestLexiconMatchAndMerge(t *testing.T) {
	base := DefaultLexicon()
	matches := base.Match("昨日の23時からテレホーダイでPC-98のソフトをダウンロードしました")
	hasPC98 := false
	hasTeleho := false
	for _, m := range matches {
		if m == "pc98" {
			hasPC98 = true
		}
		if m == "telehodai" {
			hasTeleho = true
		}
	}
	if !hasPC98 || !hasTeleho {
		t.Fatalf("expected matches to contain pc98 and telehodai, got %v", matches)
	}

	extra, err := LoadLexiconYAML([]byte(`
topics:
  bebox:
    words: ["BeBox", "BeOS"]
    broader: ["computers"]
`))
	if err != nil {
		t.Fatalf("LoadLexiconYAML failed: %v", err)
	}
	merged := MergeLexicons(base, extra)
	m2 := merged.Match("秋葉原でBeBoxのデモを見ました")
	hasBeBox := false
	for _, m := range m2 {
		if m == "bebox" {
			hasBeBox = true
		}
	}
	if !hasBeBox {
		t.Fatalf("expected merged lexicon to match bebox, got %v", m2)
	}
}
