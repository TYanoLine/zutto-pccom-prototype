package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestPrepareBoardCompositionUsesSemanticReplySource(t *testing.T) {
	store := newTestStore()
	host, err := store.HostByPhone(genericTestPhone)
	if err != nil {
		t.Fatal(err)
	}
	root := store.AddPost(host.ID, world.Post{BoardID: "1", Author: "MARI", Subject: "親記事", Body: "引用される一行"})
	notSource := store.AddPost(host.ID, world.Post{BoardID: "1", Author: "TAKA", Subject: "別記事", Body: "別の内容"})
	reply := world.Post{ID: 9876, BoardID: "1", Subject: "返信", Intent: world.PostIntent{Action: "reply", RespondsToPostID: root.ID, SourcePostID: notSource.ID}}
	repo := New(store, nil, nil, "1996-08-29")
	got := repo.prepareBoardComposition(BoardMaterializationRequest{Host: host, BoardID: "1", WorldDate: "1996-08-29", QuoteText: "引用される一行"}, reply)
	if got.Kind != worldengine.PostKindReply {
		t.Fatalf("kind=%q, want reply", got.Kind)
	}
	if got.ParentPost == nil || got.ParentPost.ID != root.ID {
		t.Fatalf("resolved parent=%#v, want semantic reply source %d", got.ParentPost, root.ID)
	}
	if got.BodyMinChars < 1 || got.BodyMaxChars < got.BodyMinChars {
		t.Fatalf("invalid length band %d-%d", got.BodyMinChars, got.BodyMaxChars)
	}
}


func TestPrepareBoardCompositionTreatsFlatSemanticResponseAsReply(t *testing.T) {
	store := newTestStore()
	host, err := store.HostByPhone(genericTestPhone)
	if err != nil {
		t.Fatal(err)
	}
	root := store.AddPost(host.ID, world.Post{
		BoardID: "1",
		Author:  "MARI",
		Subject: "フロッピーの保管場所",
		Body:    "湿気も気になるので、みなさんはどんな入れ物にまとめていますか。机の引き出しでいいのかな。",
	})
	reply := world.Post{
		ID:      9877,
		BoardID: "1",
		Intent: world.PostIntent{
			Action:           "bbs-world-catchup",
			RespondsToPostID: root.ID,
			SourcePostID:     root.ID,
		},
	}
	repo := New(store, nil, nil, "1996-08-29")
	got := repo.prepareBoardComposition(BoardMaterializationRequest{
		Host:      host,
		BoardID:   "1",
		WorldDate: "1996-08-29",
	}, reply)
	if got.Kind != worldengine.PostKindReply {
		t.Fatalf("kind=%q, want reply for semantic response without host-level parent linkage", got.Kind)
	}
	if got.ParentPost == nil || got.ParentPost.ID != root.ID {
		t.Fatalf("resolved parent=%#v, want %d", got.ParentPost, root.ID)
	}
}

func TestReplyQuoteFragmentPrefersFocusedFinalSentence(t *testing.T) {
	body := "フロッピーの置き場所に困っています。湿気も気になるので、みなさんはどんな入れ物にまとめていますか。机の引き出しでいいのかな。"
	got := replyQuoteFragment(body)
	if got != "机の引き出しでいいのかな。" {
		t.Fatalf("quote=%q", got)
	}
	if got == body {
		t.Fatal("short multi-sentence parent was quoted in full")
	}
}
