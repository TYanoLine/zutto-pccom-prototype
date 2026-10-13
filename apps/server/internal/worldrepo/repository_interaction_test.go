package worldrepo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

type countingLLMClient struct {
	callCount int
}

func (c *countingLLMClient) GenerateBoardPost(_ context.Context, _ llm.BoardPostRequest) (llm.BoardPostDraft, error) {
	c.callCount++
	return llm.BoardPostDraft{}, nil
}

func (c *countingLLMClient) MaterializeBBSTitleArticleDetails(_ context.Context, _ llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {
	c.callCount++
	return llm.BBSTitleArticleDetailDraft{}, nil
}

func (c *countingLLMClient) GenerateBBSTimelineIntent(_ context.Context, _ llm.BBSTimelineIntentRequest) (llm.BBSTimelineIntentDraft, error) {
	c.callCount++
	return llm.BBSTimelineIntentDraft{}, nil
}

func TestAddPost_RecordsInteractionEvent(t *testing.T) {
	store := newTestStore()
	llmClient := &countingLLMClient{}
	mat := LLMMaterializer{Renderer: llmClient}
	repo := New(store, nil, mat, "1996-08-29")

	host, err := store.HostByPhone(genericTestPhone)
	if err != nil {
		t.Fatal(err)
	}

	// 1. NPC の新規投稿 (board_root)
	npcPost := world.Post{
		BoardID:         "1",
		Author:          "MARI",
		AuthorPersonaID: "persona-mari-01",
		Subject:         "こんにちは",
		Body:            "最近のパソコン通信について。",
		CreatedAt:       time.Date(1996, 8, 29, 10, 0, 0, 0, time.Local),
	}
	savedNPC := repo.AddPost(host.ID, npcPost)

	// 台帳イベントの確認
	expectedNPCEventID := fmt.Sprintf("post:%s:%d", host.ID, savedNPC.ID)
	ev1, ok := store.GetInteractionEvent(expectedNPCEventID)
	if !ok {
		t.Fatalf("expected interaction event %s to exist in store", expectedNPCEventID)
	}
	if ev1.HostID != host.ID {
		t.Errorf("HostID = %q, want %q", ev1.HostID, host.ID)
	}
	if ev1.Channel != "board_root" {
		t.Errorf("Channel = %q, want board_root", ev1.Channel)
	}
	if ev1.Actor.Kind != "persona" || ev1.Actor.ID != "persona-mari-01" {
		t.Errorf("Actor = %+v, want persona:persona-mari-01", ev1.Actor)
	}
	if ev1.SourceKind != "npc_world_selected" {
		t.Errorf("SourceKind = %q, want npc_world_selected", ev1.SourceKind)
	}
	if len(ev1.AddressedTo) != 0 {
		t.Errorf("AddressedTo should be empty, got %+v", ev1.AddressedTo)
	}
	if ev1.RespondsToEventID != "" {
		t.Errorf("RespondsToEventID should be empty, got %q", ev1.RespondsToEventID)
	}

	// 2. 人間の新規投稿 (board_root)
	memberPost := world.Post{
		BoardID:        "1",
		Author:         "TARO",
		AuthorMemberID: "member-taro-99",
		Subject:        "はじめまして",
		Body:           "今日入会しました。",
		CreatedAt:      time.Date(1996, 8, 29, 10, 5, 0, 0, time.Local),
	}
	savedMember := repo.AddPost(host.ID, memberPost)

	expectedMemberEventID := fmt.Sprintf("post:%s:%d", host.ID, savedMember.ID)
	ev2, ok := store.GetInteractionEvent(expectedMemberEventID)
	if !ok {
		t.Fatalf("expected interaction event %s to exist in store", expectedMemberEventID)
	}
	if ev2.Channel != "board_root" {
		t.Errorf("Channel = %q, want board_root", ev2.Channel)
	}
	if ev2.Actor.Kind != "member" || ev2.Actor.ID != "member-taro-99" {
		t.Errorf("Actor = %+v, want member:member-taro-99", ev2.Actor)
	}
	if ev2.SourceKind != "human_authored" {
		t.Errorf("SourceKind = %q, want human_authored", ev2.SourceKind)
	}

	// 3. LLM プロバイダの呼び出しが 0 回であること
	if llmClient.callCount != 0 {
		t.Errorf("expected 0 LLM calls, got %d", llmClient.callCount)
	}
}

func TestAddPost_UpdatesRelationshipEdge(t *testing.T) {
	store := newTestStore()
	llmClient := &countingLLMClient{}
	mat := LLMMaterializer{Renderer: llmClient}
	repo := New(store, nil, mat, "1996-08-29")

	host, err := store.HostByPhone(genericTestPhone)
	if err != nil {
		t.Fatal(err)
	}

	// 1. 親記事 (NPC 作成)
	rootPost := repo.AddPost(host.ID, world.Post{
		BoardID:         "1",
		Author:          "MARI",
		AuthorPersonaID: "persona-mari-01",
		Subject:         "モデムの速度について",
		Body:            "28800bpsは快適ですね。",
		CreatedAt:       time.Date(1996, 8, 29, 12, 0, 0, 0, time.Local),
	})

	mariActor := world.ActorRef{Kind: "persona", ID: "persona-mari-01"}
	// Residents know a human by the login ID on the post (spec 019).
	taroActor := world.ActorRef{Kind: world.ActorKindLogin, ID: "member-taro-99"}

	// 2. 人間が親記事へ返信 (ParentID 指定)
	reply1 := repo.AddPost(host.ID, world.Post{
		BoardID:        "1",
		ParentID:       rootPost.ID,
		Author:         "TARO",
		AuthorMemberID: "member-taro-99",
		Subject:        "Re: モデムの速度について",
		Body:           "私も買い替えを検討しています。",
		CreatedAt:      time.Date(1996, 8, 29, 12, 10, 0, 0, time.Local),
	})

	// 返信台帳イベントの検証
	replyEventID := fmt.Sprintf("post:%s:%d", host.ID, reply1.ID)
	evReply, ok := store.GetInteractionEvent(replyEventID)
	if !ok {
		t.Fatalf("expected interaction event %s to exist", replyEventID)
	}
	if evReply.Channel != "board_reply" {
		t.Errorf("Channel = %q, want board_reply", evReply.Channel)
	}
	if len(evReply.AddressedTo) != 1 || evReply.AddressedTo[0] != mariActor {
		t.Errorf("AddressedTo = %+v, want [%+v]", evReply.AddressedTo, mariActor)
	}
	expectedParentEventID := fmt.Sprintf("post:%s:%d", host.ID, rootPost.ID)
	if evReply.RespondsToEventID != expectedParentEventID {
		t.Errorf("RespondsToEventID = %q, want %q", evReply.RespondsToEventID, expectedParentEventID)
	}

	// 関係エッジの検証: TARO -> MARI (BoardReplyCount == 1)
	edge, ok := store.GetRelationship(host.ID, taroActor, mariActor)
	if !ok {
		t.Fatal("expected relationship edge from TARO to MARI to exist")
	}
	if edge.BoardReplyCount != 1 {
		t.Errorf("BoardReplyCount = %d, want 1", edge.BoardReplyCount)
	}
	if !edge.LastInteractionAt.Equal(reply1.CreatedAt) {
		t.Errorf("LastInteractionAt = %v, want %v", edge.LastInteractionAt, reply1.CreatedAt)
	}
	if edge.LastEventID != replyEventID {
		t.Errorf("LastEventID = %q, want %q", edge.LastEventID, replyEventID)
	}

	// 3. 同じ人間が再度親記事へ返信 (BoardReplyCount == 2)
	reply2Time := time.Date(1996, 8, 29, 12, 20, 0, 0, time.Local)
	reply2 := repo.AddPost(host.ID, world.Post{
		BoardID:        "1",
		ParentID:       rootPost.ID,
		Author:         "TARO",
		AuthorMemberID: "member-taro-99",
		Subject:        "追記です",
		Body:           "おすすめのメーカーはありますか？",
		CreatedAt:      reply2Time,
	})

	edge2, ok := store.GetRelationship(host.ID, taroActor, mariActor)
	if !ok {
		t.Fatal("expected relationship edge after 2nd reply")
	}
	if edge2.BoardReplyCount != 2 {
		t.Errorf("BoardReplyCount = %d, want 2", edge2.BoardReplyCount)
	}
	if !edge2.LastInteractionAt.Equal(reply2Time) {
		t.Errorf("LastInteractionAt = %v, want %v", edge2.LastInteractionAt, reply2Time)
	}
	expectedReply2EventID := fmt.Sprintf("post:%s:%d", host.ID, reply2.ID)
	if edge2.LastEventID != expectedReply2EventID {
		t.Errorf("LastEventID = %q, want %q", edge2.LastEventID, expectedReply2EventID)
	}

	// 4. 冪等性検証: 同じイベントを再度適用しても二重加算されない
	repo.recordPostInteraction(host.ID, reply2)
	edgeIdempotent, _ := store.GetRelationship(host.ID, taroActor, mariActor)
	if edgeIdempotent.BoardReplyCount != 2 {
		t.Errorf("BoardReplyCount after re-recording = %d, want 2 (idempotent)", edgeIdempotent.BoardReplyCount)
	}

	// 5. LLM プロバイダの呼び出しが 0 回であること
	if llmClient.callCount != 0 {
		t.Errorf("expected 0 LLM calls, got %d", llmClient.callCount)
	}
}

// Spec 019: a human's post is filed under the human ID with the login ID it was
// written under; residents' relationships are kept by login ID, and a guest is
// nobody they know.
func TestHumanPostsAreFiledByHumanIDAndKnownByLoginID(t *testing.T) {
	store := newTestStore()
	repo := New(store, nil, LLMMaterializer{Renderer: &countingLLMClient{}}, "1996-08-29")
	host, err := store.HostByPhone(genericTestPhone)
	if err != nil {
		t.Fatal(err)
	}
	root := repo.AddPost(host.ID, world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "persona-mari-01", Subject: "モデム", CreatedAt: time.Date(1996, 8, 29, 12, 0, 0, 0, time.Local)})
	mari := world.ActorRef{Kind: world.ActorKindPersona, ID: "persona-mari-01"}

	reply := func(login, humanID string, minute int) world.Post {
		return repo.AddPost(host.ID, world.Post{BoardID: "1", ParentID: root.ID, Author: login, AuthorMemberID: login, AuthorHumanID: humanID,
			Subject: "Re: モデム", CreatedAt: time.Date(1996, 8, 29, 12, minute, 0, 0, time.Local)})
	}
	p := reply("TARO", "u1", 10)
	ev, ok := store.GetInteractionEvent(fmt.Sprintf("post:%s:%d", host.ID, p.ID))
	if !ok {
		t.Fatal("no ledger event")
	}
	if ev.Actor != (world.ActorRef{Kind: world.ActorKindMember, ID: "u1"}) || ev.ActorLogin != "TARO" {
		t.Errorf("ledger actor = %+v login %q", ev.Actor, ev.ActorLogin)
	}
	if len(ev.AddressedTo) != 1 || ev.AddressedTo[0] != mari || len(ev.AddressedLogins) != 1 || ev.AddressedLogins[0] != "" {
		t.Errorf("addressed = %+v logins %q", ev.AddressedTo, ev.AddressedLogins)
	}
	if _, ok := store.GetRelationship(host.ID, world.ActorRef{Kind: world.ActorKindLogin, ID: "TARO"}, mari); !ok {
		t.Error("no relationship between login TARO and MARI")
	}

	// The same human writing as GUEST, and a second guest, leave no relationship.
	reply("GUEST", "u1", 20)
	reply("GUEST", "u2", 30)
	if _, ok := store.GetRelationship(host.ID, world.ActorRef{Kind: world.ActorKindLogin, ID: "GUEST"}, mari); ok {
		t.Error("a relationship was kept for GUEST")
	}
	// Another human writing as TARO adds to the same person to residents.
	reply("TARO", "u3", 40)
	edge, _ := store.GetRelationship(host.ID, world.ActorRef{Kind: world.ActorKindLogin, ID: "TARO"}, mari)
	if edge.BoardReplyCount != 2 {
		t.Errorf("replies counted for TARO = %d, want 2", edge.BoardReplyCount)
	}
	// A reply to a guest's post is no relationship either.
	g := reply("GUEST", "u2", 50)
	repo.AddPost(host.ID, world.Post{BoardID: "1", ParentID: g.ID, Author: "MARI", AuthorPersonaID: "persona-mari-01", Subject: "Re: Re: モデム", CreatedAt: time.Date(1996, 8, 29, 12, 55, 0, 0, time.Local)})
	if _, ok := store.GetRelationship(host.ID, mari, world.ActorRef{Kind: world.ActorKindLogin, ID: "GUEST"}); ok {
		t.Error("a relationship was kept toward GUEST")
	}
}

// Chat lines and telegrams of a human are filed under the human ID with the
// login ID shown.
func TestChatRecordsKeepTheLoginID(t *testing.T) {
	repo, store, host, at := chatTestRepo(t, true)
	human := world.ActorRef{Kind: world.ActorKindMember, ID: "u1"}
	u, err := repo.RecordChatUtterance(host, world.ChatUtterance{Room: 0, Actor: human, Handle: "TARO", Body: "どうも", At: at})
	if err != nil {
		t.Fatal(err)
	}
	m, err := repo.SendLineMessage(host, world.LineMessage{From: human, FromHandle: "TARO", To: world.ActorRef{Kind: world.ActorKindPersona, ID: "p1"}, ToLine: 2, Body: "やあ", SentAt: at, DeliverAt: at})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{world.ChannelChatUtterance + ":" + u.ID, world.ChannelLineMessage + ":" + m.ID} {
		ev, ok := store.GetInteractionEvent(id)
		if !ok || ev.Actor != human || ev.ActorLogin != "TARO" {
			t.Errorf("%s: %+v", id, ev)
		}
	}
}
