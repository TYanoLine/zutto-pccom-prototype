package world

import (
	"testing"
	"time"
)

func TestInteractionLedgerStore(t *testing.T) {
	s := NewMemoryStore()

	// 1. 空の EventID はエラー
	err := s.AppendInteractionEvent(InteractionEvent{})
	if err == nil {
		t.Fatal("expected error for empty EventID, got nil")
	}

	actor1 := ActorRef{Kind: "persona", ID: "p001"}
	actor2 := ActorRef{Kind: "member", ID: "m002"}
	now := time.Now()

	e1 := InteractionEvent{
		EventID:       "ev-001",
		HostID:        "host-a",
		Channel:       "board_root",
		Actor:         actor1,
		AudienceScope: "public_board:host-a:main",
		ContentRef:    "1001",
		OccurredAt:    now.Add(-2 * time.Minute),
		SourceKind:    "npc_world_selected",
	}

	e2 := InteractionEvent{
		EventID:           "ev-002",
		HostID:            "host-a",
		Channel:           "board_reply",
		Actor:             actor2,
		AudienceScope:     "public_board:host-a:main",
		AddressedTo:       []ActorRef{actor1},
		RespondsToEventID: "ev-001",
		ContentRef:        "1002",
		OccurredAt:        now.Add(-1 * time.Minute),
		SourceKind:        "human_authored",
	}

	e3OtherHost := InteractionEvent{
		EventID:       "ev-003",
		HostID:        "host-b",
		Channel:       "board_root",
		Actor:         actor1,
		AudienceScope: "public_board:host-b:chat",
		ContentRef:    "2001",
		OccurredAt:    now,
		SourceKind:    "npc_world_selected",
	}

	// 2. 正常追加
	if err := s.AppendInteractionEvent(e1); err != nil {
		t.Fatalf("AppendInteractionEvent(e1) failed: %v", err)
	}
	if err := s.AppendInteractionEvent(e2); err != nil {
		t.Fatalf("AppendInteractionEvent(e2) failed: %v", err)
	}
	if err := s.AppendInteractionEvent(e3OtherHost); err != nil {
		t.Fatalf("AppendInteractionEvent(e3OtherHost) failed: %v", err)
	}

	// 3. 冪等性: 重複 EventID の追加はエラーにならず無視される
	if err := s.AppendInteractionEvent(e1); err != nil {
		t.Fatalf("AppendInteractionEvent duplicate should succeed (idempotent), got: %v", err)
	}

	// 4. GetInteractionEvent
	got1, ok := s.GetInteractionEvent("ev-001")
	if !ok {
		t.Fatal("expected ev-001 to exist")
	}
	if got1.EventID != "ev-001" || got1.Actor != actor1 {
		t.Errorf("unexpected event: %+v", got1)
	}

	_, ok = s.GetInteractionEvent("non-existent")
	if ok {
		t.Fatal("expected non-existent event to return false")
	}

	// 5. ListInteractionEvents (降順 & limit)
	eventsA := s.ListInteractionEvents("host-a", 10)
	if len(eventsA) != 2 {
		t.Fatalf("expected 2 events for host-a, got %d", len(eventsA))
	}
	// 降順: e2 が最初、e1 が次
	if eventsA[0].EventID != "ev-002" || eventsA[1].EventID != "ev-001" {
		t.Errorf("expected descending order [ev-002, ev-001], got [%s, %s]", eventsA[0].EventID, eventsA[1].EventID)
	}

	limit1 := s.ListInteractionEvents("host-a", 1)
	if len(limit1) != 1 || limit1[0].EventID != "ev-002" {
		t.Errorf("expected latest 1 event (ev-002), got %+v", limit1)
	}

	eventsB := s.ListInteractionEvents("host-b", 0)
	if len(eventsB) != 1 || eventsB[0].EventID != "ev-003" {
		t.Errorf("expected 1 event for host-b, got %+v", eventsB)
	}
}

func TestRelationshipStore(t *testing.T) {
	s := NewMemoryStore()

	from := ActorRef{Kind: "member", ID: "user1"}
	to := ActorRef{Kind: "persona", ID: "npc1"}
	to2 := ActorRef{Kind: "persona", ID: "npc2"}

	// 1. 初期状態
	_, ok := s.GetRelationship("host-a", from, to)
	if ok {
		t.Fatal("expected no relationship initially")
	}

	// 2. 不正なエッジの更新はエラー
	if err := s.UpdateRelationship(RelationshipEdge{}); err == nil {
		t.Fatal("expected error for empty edge, got nil")
	}
	if err := s.UpdateRelationship(RelationshipEdge{HostID: "host-a", FromActor: from}); err == nil {
		t.Fatal("expected error for zero ToActor, got nil")
	}

	// 3. 正常な更新
	now := time.Now()
	edge1 := RelationshipEdge{
		HostID:            "host-a",
		FromActor:         from,
		ToActor:           to,
		BoardReplyCount:   1,
		LastInteractionAt: now,
		LastEventID:       "ev-001",
	}
	if err := s.UpdateRelationship(edge1); err != nil {
		t.Fatalf("UpdateRelationship failed: %v", err)
	}

	got, ok := s.GetRelationship("host-a", from, to)
	if !ok {
		t.Fatal("expected relationship to exist")
	}
	if got.BoardReplyCount != 1 || got.LastEventID != "ev-001" {
		t.Errorf("unexpected edge: %+v", got)
	}

	// 4. 別のアクターへのエッジ追加
	edge2 := RelationshipEdge{
		HostID:            "host-a",
		FromActor:         from,
		ToActor:           to2,
		BoardReplyCount:   3,
		LastInteractionAt: now,
		LastEventID:       "ev-002",
	}
	if err := s.UpdateRelationship(edge2); err != nil {
		t.Fatalf("UpdateRelationship edge2 failed: %v", err)
	}

	// 5. ListRelationshipsFrom
	listFrom := s.ListRelationshipsFrom("host-a", from)
	if len(listFrom) != 2 {
		t.Fatalf("expected 2 edges from %v, got %d", from, len(listFrom))
	}

	// 存在しないホスト／アクターからのリスト
	emptyList := s.ListRelationshipsFrom("host-other", from)
	if len(emptyList) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(emptyList))
	}
}
