package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/llm"
)

func TestPersonaHistoryPocProfilesAreSparseFivePeople(t *testing.T) {
	profiles := personaHistoryPocProfiles()
	if len(profiles) != 5 {
		t.Fatalf("profiles = %d, want 5", len(profiles))
	}
	for _, profile := range profiles {
		if profile.ID == "" || profile.Handle == "" || profile.Age == 0 || profile.Occupation == "" {
			t.Fatalf("incomplete profile: %#v", profile)
		}
		if len(profile.Interests) == 0 {
			t.Fatalf("profile %s has no interests", profile.ID)
		}
	}
}

func TestApplyPersonaHistoryCandidateRequiresPostEvidenceAndReinforces(t *testing.T) {
	history := map[string][]llm.PersonaHistoryEntry{"P01": {}}
	body := "平日はだいたい23時ごろから接続してます。仕事が遅い日は読んで終わりですね。"
	candidate := llm.PersonaHistoryCandidate{
		PersonaID: "P01",
		Kind: "self_fact",
		Key: "routine.connect_time",
		Value: "平日は23時ごろから接続することが多い",
		Evidence: "平日はだいたい23時ごろから接続してます",
		Confidence: .92,
	}
	ok, reason := applyPersonaHistoryCandidate(history, 1, "R1-P01", body, candidate)
	if !ok {
		t.Fatalf("candidate rejected: %s", reason)
	}
	if got := len(history["P01"]); got != 1 {
		t.Fatalf("history len = %d, want 1", got)
	}
	if history["P01"][0].Observations != 1 {
		t.Fatalf("observations = %d", history["P01"][0].Observations)
	}

	ok, reason = applyPersonaHistoryCandidate(history, 2, "R2-P01", body, candidate)
	if !ok {
		t.Fatalf("reinforcement rejected: %s", reason)
	}
	if history["P01"][0].Observations != 2 || history["P01"][0].LastRound != 2 {
		t.Fatalf("reinforcement did not update entry: %#v", history["P01"][0])
	}

	bad := candidate
	bad.Evidence = "本文に存在しない根拠"
	if ok, _ := applyPersonaHistoryCandidate(history, 3, "R3-P01", body, bad); ok {
		t.Fatal("unsupported evidence was accepted")
	}
}

func TestApplyPersonaHistoryCandidateRejectsConflictingSelfFact(t *testing.T) {
	history := map[string][]llm.PersonaHistoryEntry{
		"P01": {{
			Kind: "self_fact", Key: "routine.connect_time", Value: "平日は23時ごろ",
			Confidence: .9, FirstRound: 1, LastRound: 1, Observations: 1,
		}},
	}
	body := "平日は夕方6時にはいつも接続してます。"
	candidate := llm.PersonaHistoryCandidate{
		PersonaID: "P01", Kind: "self_fact", Key: "routine.connect_time",
		Value: "平日は18時ごろ", Evidence: "平日は夕方6時にはいつも接続してます", Confidence: .9,
	}
	ok, reason := applyPersonaHistoryCandidate(history, 2, "R2-P01", body, candidate)
	if ok {
		t.Fatal("conflicting self_fact was accepted")
	}
	if !strings.Contains(reason, "矛盾") {
		t.Fatalf("unexpected rejection reason: %s", reason)
	}
}

func TestPersonaHistoryPocViewerHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/poc/persona-history", nil)
	rr := httptest.NewRecorder()
	newPersonaHistoryPocViewerHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"5人が「投稿した結果」で人物史を持ちはじめる",
		"/api/debug/persona-history?action=start",
		"2-PASS",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("viewer missing %q", want)
		}
	}
}
