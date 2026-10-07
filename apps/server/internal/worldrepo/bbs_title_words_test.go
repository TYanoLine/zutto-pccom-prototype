package worldrepo

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// scriptedTitlePlanner answers from a function and records every request.
type scriptedTitlePlanner struct {
	requests []llm.BBSSituationTitleRequest
	answer   func(call int, req llm.BBSSituationTitleRequest) []llm.BBSSituationTitle
}

func (s *scriptedTitlePlanner) GenerateBBSSituationTitles(_ context.Context, req llm.BBSSituationTitleRequest) (llm.BBSSituationTitleDraft, error) {
	s.requests = append(s.requests, req)
	return llm.BBSSituationTitleDraft{Titles: s.answer(len(s.requests)-1, req)}, nil
}

func planTitles(t *testing.T, repo *Repository, host world.Host, recent []string, n int, titles llm.BBSSituationTitlePlanner) ([]bbsengine.PlannedPost, error) {
	t.Helper()
	at := time.Date(1996, 2, 17, 20, 30, 0, 0, time.FixedZone("JST", 9*3600))
	board := world.Board{ID: "20/1", Name: "ＧＡＭＥ", SemanticScope: "ゲームの話"}
	var posts []world.Post
	for _, s := range recent {
		posts = append(posts, world.Post{BoardID: board.ID, Subject: s, Intent: world.PostIntent{SituationKind: "open_topic"}})
	}
	var slots []bbsengine.Slot
	for i := 1; i <= n; i++ {
		slots = append(slots, bbsengine.Slot{Index: i, Author: fmt.Sprintf("A%d", i), CreatedAt: at.Add(time.Duration(i) * time.Minute)})
	}
	req := bbsengine.BatchRequest{Host: host, Board: board, RecentPosts: posts, Slots: slots}
	planner := repositoryBBSBatchPlanner{repo: repo}
	return planner.planSituationFirstRoots(context.Background(), LLMMaterializer{ModelHistoricalMemory: true},
		worldengine.EvidenceDecision{}, &openTopicSituationProposer{}, titles, req, slots, "1996-02-17")
}

func newTitleRepo() *Repository { return New(world.NewMemoryStore(), nil, nil, "1996-02-17") }

func TestDuplicateTitleIsRegeneratedAlone(t *testing.T) {
	planner := &scriptedTitlePlanner{answer: func(call int, req llm.BBSSituationTitleRequest) []llm.BBSSituationTitle {
		var out []llm.BBSSituationTitle
		for i, a := range req.Articles {
			subject := fmt.Sprintf("題名%d", i)
			if call == 0 && i == 2 {
				subject = "題名0" // same as the first article of the batch
			}
			if call == 1 {
				subject = "別の題名"
			}
			out = append(out, llm.BBSSituationTitle{EventID: a.EventID, Subject: subject})
		}
		return out
	}}
	posts, err := planTitles(t, newTitleRepo(), world.Host{ID: "h"}, nil, 3, planner)
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.requests) != 2 {
		t.Fatalf("calls = %d, want 2", len(planner.requests))
	}
	retry := planner.requests[1]
	if len(retry.Articles) != 1 || retry.Articles[0].EventID != "slot-3" {
		t.Fatalf("retry articles = %+v", retry.Articles)
	}
	if !strings.Contains(strings.Join(retry.RecentSubjects, "|"), "題名0") {
		t.Fatalf("retry does not tell the duplicate: %v", retry.RecentSubjects)
	}
	subjects := []string{}
	for _, p := range posts {
		subjects = append(subjects, p.Subject)
	}
	if want := []string{"題名0", "題名1", "別の題名"}; !reflect.DeepEqual(subjects, want) {
		t.Fatalf("subjects = %v, want %v", subjects, want)
	}
}

func TestDuplicateAfterRetryIsAcceptedAndNotAFailure(t *testing.T) {
	planner := &scriptedTitlePlanner{answer: func(_ int, req llm.BBSSituationTitleRequest) []llm.BBSSituationTitle {
		var out []llm.BBSSituationTitle
		for _, a := range req.Articles {
			out = append(out, llm.BBSSituationTitle{EventID: a.EventID, Subject: "同じ題名"})
		}
		return out
	}}
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(log.Writer())
	repo := newTitleRepo()
	repo.SetDebugLogGeneratedContent(true)
	posts, err := planTitles(t, repo, world.Host{ID: "h", Debug: hostcatalog.DebugFlags{ContentLog: true}}, []string{"同じ題名"}, 2, planner)
	if err != nil {
		t.Fatalf("duplicates must not fail generation: %v", err)
	}
	if len(posts) != 2 || posts[0].Subject != "同じ題名" || posts[1].Subject != "同じ題名" {
		t.Fatalf("posts = %+v", posts)
	}
	if len(planner.requests) != 2 {
		t.Fatalf("at most one regeneration: calls = %d", len(planner.requests))
	}
	if !strings.Contains(logs.String(), `"duplicate_accepted":2`) {
		t.Fatalf("duplicate_accepted not recorded:\n%s", logs.String())
	}
}

func uniqueTitles(_ int, req llm.BBSSituationTitleRequest) []llm.BBSSituationTitle {
	var out []llm.BBSSituationTitle
	for _, a := range req.Articles {
		out = append(out, llm.BBSSituationTitle{EventID: a.EventID, Subject: "題材" + a.EventID})
	}
	return out
}

func TestRecentFormFactsOnlyWhenBiased(t *testing.T) {
	biased := []string{"天神で集合", "天神の店", "天神の駅", "庭の花", "茶の話", "本の話", "旅の話", "夢の話", "山登り", "川遊び"}
	planner := &scriptedTitlePlanner{answer: uniqueTitles}
	if _, err := planTitles(t, newTitleRepo(), world.Host{ID: "h"}, biased, 1, planner); err != nil {
		t.Fatal(err)
	}
	facts := planner.requests[0].RecentFormFacts
	if len(facts) == 0 || !strings.Contains(facts[0], "「天神」") {
		t.Fatalf("facts = %v", facts)
	}
	for _, f := range facts {
		for _, bad := range []string{"しない", "禁止", "避け", "べき", "ください", "割"} {
			if strings.Contains(f, bad) {
				t.Fatalf("fact %q reads as an instruction", f)
			}
		}
	}
	quiet := &scriptedTitlePlanner{answer: uniqueTitles}
	varied := []string{"あいう", "えおか", "きくけ", "こさし", "すせそ", "たちつ", "てとな", "にぬね", "のはひ"}
	if _, err := planTitles(t, newTitleRepo(), world.Host{ID: "h"}, varied, 1, quiet); err != nil {
		t.Fatal(err)
	}
	if len(quiet.requests[0].RecentFormFacts) != 0 {
		t.Fatalf("unbiased recent titles must add no material: %v", quiet.requests[0].RecentFormFacts)
	}
}

func TestCoveredSubjectsAreShuffledDeterministicallyAndCapped(t *testing.T) {
	var recent []string
	for i := 0; i < 35; i++ {
		recent = append(recent, fmt.Sprintf("過去の題材%02d", i))
	}
	run := func() []string {
		planner := &scriptedTitlePlanner{answer: uniqueTitles}
		if _, err := planTitles(t, newTitleRepo(), world.Host{ID: "h"}, recent, 1, planner); err != nil {
			t.Fatal(err)
		}
		return planner.requests[0].RecentSubjects
	}
	a := run()
	if len(a) != 20 {
		t.Fatalf("covered subjects = %d, want 20", len(a))
	}
	if !reflect.DeepEqual(a, run()) {
		t.Fatal("covered order is not deterministic for a seed")
	}
	chronological := append([]string(nil), recent[15:]...)
	if reflect.DeepEqual(a, chronological) {
		t.Fatal("covered subjects were not reordered")
	}
}

func TestTitleShapeLogOnlyForOptedInHosts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		global  bool
		hostOpt bool
		want    bool
	}{
		{"opted in", true, true, true},
		{"host not opted in", true, false, false},
		{"global switch off", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log.SetOutput(&logs)
			defer log.SetOutput(log.Writer())
			repo := newTitleRepo()
			repo.SetDebugLogGeneratedContent(tc.global)
			planner := &scriptedTitlePlanner{answer: uniqueTitles}
			host := world.Host{ID: "h", Debug: hostcatalog.DebugFlags{ContentLog: tc.hostOpt}}
			if _, err := planTitles(t, repo, host, nil, 2, planner); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(logs.String(), "BBS title shape:"); got != tc.want {
				t.Fatalf("logged = %v, want %v\n%s", got, tc.want, logs.String())
			}
		})
	}
}

func TestVariantsPickDissimilarTitleAndAreOffByDefault(t *testing.T) {
	answer := func(_ int, req llm.BBSSituationTitleRequest) []llm.BBSSituationTitle {
		var out []llm.BBSSituationTitle
		for _, a := range req.Articles {
			title := llm.BBSSituationTitle{EventID: a.EventID, Subject: "ゼルダ貸せます"}
			if req.MultiVariant {
				title.Candidates = []string{"ゼルダ貸せます", "庭の話から始めます", "ゼルダ売ります"}
				title.Subject = title.Candidates[0]
			}
			out = append(out, title)
		}
		return out
	}
	recent := []string{"ゼルダ貸せます！", "ゼルダ売ります"}

	off := &scriptedTitlePlanner{answer: answer}
	posts, err := planTitles(t, newTitleRepo(), world.Host{ID: "h"}, recent, 1, off)
	if err != nil {
		t.Fatal(err)
	}
	if off.requests[0].MultiVariant || posts[0].Subject != "ゼルダ貸せます" {
		t.Fatalf("variants must be off by default: %+v %q", off.requests[0], posts[0].Subject)
	}

	repo := newTitleRepo()
	repo.SetTitleVariants(true)
	on := &scriptedTitlePlanner{answer: answer}
	posts, err = planTitles(t, repo, world.Host{ID: "h"}, recent, 1, on)
	if err != nil {
		t.Fatal(err)
	}
	if !on.requests[0].MultiVariant || posts[0].Subject != "庭の話から始めます" {
		t.Fatalf("variant pick = %q (multi=%v)", posts[0].Subject, on.requests[0].MultiVariant)
	}
}
