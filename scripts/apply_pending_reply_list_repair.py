from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old!r}")
    p.write_text(text.replace(old, new, 1))

replace_once(
    "apps/server/internal/worldrepo/repository.go",
    '''func (r *Repository) ListPosts(hostID string) []world.Post {\n\tif existing := r.Base.ListPosts(hostID); len(existing) > 0 {\n\t\treturn existing\n\t}\n''',
    '''func (r *Repository) ListPosts(hostID string) []world.Post {\n\tif existing := r.Base.ListPosts(hostID); len(existing) > 0 {\n\t\treturn r.repairDevelopmentPendingReplySubjects(hostID, existing)\n\t}\n''',
)

path = Path("apps/server/internal/worldrepo/materialization_reply_subject_test.go")
text = path.read_text()
text += r'''

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
'''
path.write_text(text)
print("patched Repository.ListPosts placeholder repair")
