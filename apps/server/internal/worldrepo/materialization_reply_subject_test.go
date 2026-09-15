package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestRepairDevelopmentPendingReplySubjectUsesCanonicalParentTitle(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	hostID := "reply-repair-host"
	root := base.AddPost(hostID, world.Post{BoardID: "2", Author: "SYSOP", Subject: "ISDNに移行した方、感想を教えてください"})
	reply := base.AddPost(hostID, world.Post{BoardID: "2", ParentID: root.ID, Author: "TAKA", Subject: "Re: " + developmentConversationPendingSubject})

	repaired := repo.repairDevelopmentPendingReplySubjects(hostID, filterBoard(base.ListPosts(hostID), "2"))
	want := "Re: " + root.Subject
	found := false
	for _, post := range repaired {
		if post.ID != reply.ID {
			continue
		}
		found = true
		if post.Subject != want {
			t.Fatalf("reply subject=%q, want %q", post.Subject, want)
		}
	}
	if !found {
		t.Fatal("repaired reply not returned")
	}

	stored := filterBoard(base.ListPosts(hostID), "2")
	for _, post := range stored {
		if post.ID == reply.ID && post.Subject != want {
			t.Fatalf("repair was not persisted: subject=%q want=%q", post.Subject, want)
		}
	}
}

func TestDevelopmentReplySubjectNeverWaitsForBodyWhenParentTitleKnown(t *testing.T) {
	parent := "ログ取りに便利な通信ソフト"
	if got, want := developmentReplySubject(parent), "Re: "+parent; got != want {
		t.Fatalf("reply subject=%q, want %q", got, want)
	}
}

func TestRepositoryListPostsRepairsPersistedPendingReplySubjects(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	hostID := "reply-list-repair-host"
	root := base.AddPost(hostID, world.Post{BoardID: "2", Author: "SYSOP", Subject: "ISDNに移行した方、感想を教えてください"})
	reply := base.AddPost(hostID, world.Post{BoardID: "2", ParentID: root.ID, Author: "NORI", Subject: "Re: " + developmentConversationPendingSubject})

	posts := repo.ListPosts(hostID)
	want := "Re: " + root.Subject
	for _, post := range posts {
		if post.ID == reply.ID && post.Subject != want {
			t.Fatalf("ListPosts reply subject=%q, want %q", post.Subject, want)
		}
	}
	stored := base.ListPosts(hostID)
	for _, post := range stored {
		if post.ID == reply.ID && post.Subject != want {
			t.Fatalf("ListPosts repair was not persisted: subject=%q want=%q", post.Subject, want)
		}
	}
}
