package bbsengine

import (
	"context"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type situationStatePlanner struct{}

func (situationStatePlanner) PlanBBSBatch(_ context.Context, req BatchRequest) ([]PlannedPost, error) {
	out := make([]PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		if slot.ReplyToPostID != 0 || slot.ReplyToSlotIndex != 0 {
			out = append(out, PlannedPost{
				SlotIndex: slot.Index, DiscourseMode: "reply", SituationKind: "reply_to_existing",
				SituationSummary: "reply context",
			})
			continue
		}
		out = append(out, PlannedPost{
			SlotIndex: slot.Index,
			Subject: "手順を紙にメモ",
			Topic: "短い操作手順",
			Motivation: "world_selected_situation",
			DiscourseMode: "share_tip",
			SituationKind: "games_note_taking",
			SituationSummary: "繰り返し試す操作の順番を紙に書いた。",
			SituationFacts: []string{
				"world_fact_status=accepted_situation_before_subject",
				"object_class=短い操作手順",
				"occurrence=繰り返し試す操作の順番を紙に書いた。",
				"attempted_actions=何度か思い出しながら試した後で書き留めた。",
				"result=次の試行で見返せた。",
				"practical_point=実際に行った順番だけを短く書いた。",
			},
			ArticleDetailsMaterialized: true,
		})
	}
	return out, nil
}

func TestSituationFirstPlannerStatePersistsWithoutTitleFirstContract(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0920000196")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "4", Name: "ゲーム"}
	now := time.Date(1996, 8, 26, 23, 30, 0, 0, time.Local)
	engine := New(store, situationStatePlanner{}, func() time.Time { return now })
	if err := engine.CatchUpInitialCount(context.Background(), host, board, 1); err != nil {
		t.Fatal(err)
	}
	posts := filterBoard(store.ListPosts(host.ID), board.ID)
	if len(posts) != 1 {
		t.Fatalf("posts=%d, want 1", len(posts))
	}
	post := posts[0]
	if post.Intent.DiscourseMode != "share_tip" {
		t.Fatalf("discourse_mode=%q", post.Intent.DiscourseMode)
	}
	if post.Intent.SituationKind != "games_note_taking" {
		t.Fatalf("situation_kind=%q", post.Intent.SituationKind)
	}
	if !post.Intent.ArticleDetailsMaterialized {
		t.Fatal("situation-first root should skip legacy Article Detail materialization")
	}
	foundOccurrence := false
	for _, fact := range post.Intent.SituationFacts {
		if fact == "occurrence=繰り返し試す操作の順番を紙に書いた。" {
			foundOccurrence = true
		}
		if len(fact) >= len("title_first_subject=") && fact[:len("title_first_subject=")] == "title_first_subject=" {
			t.Fatalf("legacy title-first contract leaked into Situation-first post: %q", fact)
		}
	}
	if !foundOccurrence {
		t.Fatalf("typed Situation facts not persisted: %#v", post.Intent.SituationFacts)
	}
}
