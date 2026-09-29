package main

import (
	"strings"
	"testing"
)

func TestMinimalBBSPoCPromptStaysMinimalAndSituationFirst(t *testing.T) {
	p := minimalBBSPoCPrompt()
	for _, want := range []string{
		"handle: AKI",
		"games: 0.86",
		"PCエンジン用Huカード",
		"ケースに入っているもの",
		"裸のもの",
		"タイトル順に並べた方が",
		"世界事実を変更せず",
		"1996年を現在として生きている本人",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
	for _, avoid := range []string{
		"referent_requirement",
		"historical_claims",
		"discourse_mode=",
		"DIEGETIC PRESENT / ERA NORMALITY",
		"article_detail_contract",
	} {
		if strings.Contains(p, avoid) {
			t.Fatalf("minimal prompt unexpectedly contains %q", avoid)
		}
	}
}
