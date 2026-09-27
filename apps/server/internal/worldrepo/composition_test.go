package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

func TestPrepareBoardCompositionUsesSemanticReplySource(t *testing.T) {
	store := world.NewMemoryStore()
	host, err := store.HostByPhone("0451234567")
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
