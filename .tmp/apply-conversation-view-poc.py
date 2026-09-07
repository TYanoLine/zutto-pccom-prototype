from pathlib import Path

ROOT = Path('.')

def replace_once(path, old, new):
    p = ROOT / path
    s = p.read_text()
    if old not in s:
        raise SystemExit(f'missing replacement target in {path}: {old[:80]!r}')
    p.write_text(s.replace(old, new, 1))

# Route only explicitly-enabled repositories around the host-wide semantic producer.
replace_once(
    'apps/server/internal/worldrepo/materialization_demo_persona.go',
    '''\tif hasProducerMaterialization(hostPosts) {\n''',
    '''\tif developmentConversationViewPoCEnabled(r) {\n\t\tposts, created := r.materializeConversationWorldWindow(host)\n\t\treturn filterBoard(posts, board.ID), created\n\t}\n\tif hasProducerMaterialization(hostPosts) {\n''',
)

# Use the reconstructed conversation view for rendering, and let the LLM choose
# the subject in this PoC. Replies are re-canonicalized to their root subject.
p = ROOT / 'apps/server/internal/worldrepo/materialization_article_debug.go'
s = p.read_text()
s = s.replace('r.materializationBBSRenderContext(host, board, selected)', 'r.materializationRenderContext(host, board, selected)')
old = '''\trenderIntent := selected.Intent\n\trenderIntent.RenderContext = renderContext\n\treq := BoardMaterializationRequest{\n\t\tHost:             host,\n\t\tBoardID:          board.ID,\n\t\tBoardTopic:       selected.Subject,\n\t\tWorldDate:        r.WorldDate,\n\t\tPersona:          persona,\n\t\tIntent:           renderIntent,\n\t\tCanonicalSubject: selected.Subject,\n\t}\n'''
new = '''\tboardTopic := selected.Subject\n\tcanonicalSubject := selected.Subject\n\tif developmentConversationViewPoCEnabled(r) {\n\t\tboardTopic = board.Name\n\t\tcanonicalSubject = ""\n\t}\n\trenderIntent := selected.Intent\n\trenderIntent.RenderContext = renderContext\n\treq := BoardMaterializationRequest{\n\t\tHost:             host,\n\t\tBoardID:          board.ID,\n\t\tBoardTopic:       boardTopic,\n\t\tWorldDate:        r.WorldDate,\n\t\tPersona:          persona,\n\t\tIntent:           renderIntent,\n\t\tCanonicalSubject: canonicalSubject,\n\t}\n'''
if old not in s:
    raise SystemExit('request construction target missing')
s = s.replace(old, new, 1)
old = '''\tselected.Body = posts[0].Body\n'''
new = '''\tif developmentConversationViewPoCEnabled(r) {\n\t\tselected.Subject = r.developmentConversationRenderedSubject(host.ID, selected, posts[0].Subject)\n\t}\n\tselected.Body = posts[0].Body\n'''
if old not in s:
    raise SystemExit('body assignment target missing')
s = s.replace(old, new, 1)
p.write_text(s)

# Enable the experimental mode only for the isolated fresh RESET -> ALLBODY lab.
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '''\trepo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)\n''',
    '''\trepo := worldrepo.New(base, l.engine, l.materializer, l.worldDate)\n\trepo.EnableDevelopmentConversationViewPoC()\n''',
)
replace_once(
    'apps/server/cmd/server/materialization_lab_fresh.go',
    '''// therefore includes the host-wide Producer call immediately followed by the\n// Article Worker burst, which the existing replay labs deliberately skip.\n''',
    '''// therefore exercises the experimental conversation-view path immediately\n// followed by the Article Worker burst, while ordinary runtime paths stay unchanged.\n''',
)

(ROOT / 'apps/server/internal/worldrepo/materialization_conversation_view.go').write_text(r'''package worldrepo

import (
    "fmt"
    "sort"
    "strings"
    "sync"

    "zutto-pccom/apps/server/internal/world"
)

const developmentConversationPendingSubject = "（本文生成時に決定）"

var developmentConversationViewPoC sync.Map

// EnableDevelopmentConversationViewPoC enables a development-only materialization
// experiment for this repository instance. The canonical database still owns
// topology, timing and actor identity, but the host-wide semantic producer is
// bypassed. Article prose is instead rendered from a transient conversation view
// reconstructed from canonical BBS records immediately before each write.
func (r *Repository) EnableDevelopmentConversationViewPoC() {
    developmentConversationViewPoC.Store(r, true)
}

func developmentConversationViewPoCEnabled(r *Repository) bool {
    _, ok := developmentConversationViewPoC.Load(r)
    return ok
}

// materializeConversationWorldWindow persists only cheap world-selected shells.
// It intentionally does not invent a producer episode/topic/goal graph. The
// broad routing domain and cause summary are retained as world-layer guardrails;
// natural subject/body wording is chosen later while looking at conversation
// history reconstructed from the DB.
func (r *Repository) materializeConversationWorldWindow(host world.Host) ([]world.Post, bool) {
    if existing := r.Base.ListPosts(host.ID); len(existing) > 0 {
        return existing, false
    }
    boards, _ := r.MaterializationBoards(host)
    personas, _ := r.MaterializationPersonas(host)
    if len(boards) == 0 || len(personas) == 0 {
        return nil, false
    }

    windowShells := make([]developmentWindowShell, 0, len(boards)*developmentWorldWindowPoCMaxShellsPerBoard)
    for _, board := range boards {
        visits := developmentVisitsForBoard(host, board, personas, r.WorldDate)
        shells, stats := selectDevelopmentTimelineShells(host, board, visits)
        shells, stats = limitDevelopmentShellsForProducer(shells, stats)
        storeDevelopmentSelectionStats(r, host.ID, board.ID, stats)
        clearDevelopmentPlanningError(r, host.ID, board.ID)
        for _, shell := range shells {
            windowShells = append(windowShells, developmentWindowShell{
                eventID: developmentWindowEventID(board.ID, shell.index),
                board:   board,
                shell:   shell,
            })
        }
    }
    sort.SliceStable(windowShells, func(i, j int) bool {
        if windowShells[i].shell.createdAt.Equal(windowShells[j].shell.createdAt) {
            return windowShells[i].eventID < windowShells[j].eventID
        }
        return windowShells[i].shell.createdAt.Before(windowShells[j].shell.createdAt)
    })

    committedByEventID := map[string]world.Post{}
    out := make([]world.Post, 0, len(windowShells))
    for _, item := range windowShells {
        shell := item.shell
        parentID := int64(0)
        sourcePostID := int64(0)
        respondsToID := int64(0)
        subject := developmentConversationPendingSubject

        if shell.parentIndex != 0 {
            parentEventID := developmentWindowEventID(item.board.ID, shell.parentIndex)
            parent, ok := committedByEventID[parentEventID]
            if !ok {
                continue
            }
            parentID = parent.ID
            subject = "Re: " + developmentConversationPendingSubject
            sourcePostID = parent.ID
            respondsToID = parent.ID
        }
        if shell.sourceIndex != 0 {
            sourceEventID := developmentWindowEventID(item.board.ID, shell.sourceIndex)
            if source, ok := committedByEventID[sourceEventID]; ok {
                sourcePostID = source.ID
                respondsToID = source.ID
            }
        }

        post := world.Post{
            BoardID:         item.board.ID,
            ParentID:        parentID,
            Author:          shell.persona.Handle,
            AuthorPersonaID: shell.persona.ID,
            Subject:         subject,
            Intent: world.PostIntent{
                Action:           shell.action,
                AnchorKey:        shell.anchorKey,
                CauseKind:        shell.causeKind,
                DiscourseMode:    shell.discourseMode,
                SourcePostID:     sourcePostID,
                Topic:            shell.anchorKey,
                Motivation:       shell.causeSummary,
                RespondsToPostID: respondsToID,
            },
            CreatedAt: shell.createdAt,
        }
        post = r.Base.AddPost(host.ID, post)
        committedByEventID[item.eventID] = post
        out = append(out, post)
    }
    return out, len(out) > 0
}

func (r *Repository) materializationRenderContext(host world.Host, board world.Board, selected world.Post) (string, demoRenderContextStats) {
    if !developmentConversationViewPoCEnabled(r) {
        return r.materializationBBSRenderContext(host, board, selected)
    }
    return r.materializationConversationViewContext(host, board, selected)
}

func (r *Repository) materializationConversationViewContext(host world.Host, board world.Board, selected world.Post) (string, demoRenderContextStats) {
    canonical, stats := r.materializationBBSRenderContext(host, board, selected)
    var b strings.Builder
    b.WriteString("CONVERSATION VIEW POC — transient rendering input rebuilt from canonical DB records.\n")
    b.WriteString("Do not treat this view as new world memory; write as the selected member inside the conversation already shown.\n")
    fmt.Fprintf(&b, "CURRENT WORLD SLOT: action=%s; routing_domain=%s; cause_kind=%s; discourse_mode=%s\n",
        selected.Intent.Action, selected.Intent.AnchorKey, selected.Intent.CauseKind, selected.Intent.DiscourseMode)
    if strings.TrimSpace(selected.Intent.Motivation) != "" {
        fmt.Fprintf(&b, "WORLD-LAYER CAUSE BOUNDARY: %s\n", strings.TrimSpace(selected.Intent.Motivation))
    }

    all := r.Base.ListPosts(host.ID)
    if selected.Intent.SourcePostID != 0 {
        if source, ok := developmentConversationFindPost(all, selected.Intent.SourcePostID); ok && postBefore(source, selected) {
            b.WriteString("\nEXPLICIT CANONICAL SOURCE FOR THIS WORLD SLOT:\n")
            writeDevelopmentConversationPost(&b, source)
        }
    }

    recent := developmentConversationRecentActorPosts(all, selected, 3)
    b.WriteString("\nTHIS ACTOR'S RECENT CANONICAL POSTS — continuity/style context, not mandatory topics:\n")
    if len(recent) == 0 {
        b.WriteString("(none)\n")
    }
    for _, post := range recent {
        writeDevelopmentConversationPost(&b, post)
    }

    b.WriteString("\nCANONICAL BOARD CONVERSATION:\n")
    b.WriteString(canonical)
    return b.String(), stats
}

func developmentConversationFindPost(posts []world.Post, id int64) (world.Post, bool) {
    for _, post := range posts {
        if post.ID == id {
            return post, true
        }
    }
    return world.Post{}, false
}

func developmentConversationRecentActorPosts(all []world.Post, selected world.Post, limit int) []world.Post {
    out := make([]world.Post, 0, limit)
    for _, post := range all {
        if post.ID == selected.ID || !postBefore(post, selected) {
            continue
        }
        sameActor := selected.AuthorPersonaID != "" && post.AuthorPersonaID == selected.AuthorPersonaID
        if !sameActor && !strings.EqualFold(post.Author, selected.Author) {
            continue
        }
        if post.ID == selected.Intent.SourcePostID || post.ID == selected.ParentID {
            continue
        }
        out = append(out, post)
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].CreatedAt.Equal(out[j].CreatedAt) {
            return out[i].ID > out[j].ID
        }
        return out[i].CreatedAt.After(out[j].CreatedAt)
    })
    if limit > 0 && len(out) > limit {
        out = out[:limit]
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].CreatedAt.Equal(out[j].CreatedAt) {
            return out[i].ID < out[j].ID
        }
        return out[i].CreatedAt.Before(out[j].CreatedAt)
    })
    return out
}

func writeDevelopmentConversationPost(b *strings.Builder, post world.Post) {
    fmt.Fprintf(b, "[MSG %04d board=%s %s %s] %s\n", post.ID, post.BoardID, post.CreatedAt.Format("01/02 15:04"), post.Author, post.Subject)
    if strings.TrimSpace(post.Body) != "" {
        b.WriteString(truncateDemoContext(strings.TrimSpace(post.Body), 420))
        b.WriteString("\n")
        return
    }
    b.WriteString("semantic shell: ")
    b.WriteString(demoPostSemanticSummary(post))
    b.WriteString("\n")
}

func (r *Repository) developmentConversationRenderedSubject(hostID string, selected world.Post, generated string) string {
    generated = strings.TrimSpace(generated)
    if selected.ParentID == 0 {
        if generated != "" {
            return generated
        }
        return selected.Subject
    }
    if parent, ok := developmentConversationFindPost(r.Base.ListPosts(hostID), selected.ParentID); ok {
        subject := strings.TrimSpace(parent.Subject)
        if subject != "" && subject != developmentConversationPendingSubject {
            return "Re: " + normalizeDemoSubject(subject)
        }
    }
    if generated != "" {
        return generated
    }
    return selected.Subject
}
''')

(ROOT / 'apps/server/internal/worldrepo/materialization_conversation_view_test.go').write_text(r'''package worldrepo

import (
    "context"
    "strings"
    "testing"

    "zutto-pccom/apps/server/internal/historicalkb"
    "zutto-pccom/apps/server/internal/llm"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldengine"
)

type conversationViewEvidenceEngine struct{}

func (conversationViewEvidenceEngine) ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
    return worldengine.EvidenceDecision{
        Level: historicalkb.EvidenceAtmospheric,
        Knowledge: historicalkb.KnowledgeResult{CanUse: true},
    }, nil
}

func TestConversationViewPoCStoresWorldShellsWithoutProducerBriefs(t *testing.T) {
    base := world.NewMemoryStore()
    repo := New(base, nil, nil, "1996-08-29")
    repo.EnableDevelopmentConversationViewPoC()
    host, err := repo.HostByPhone("0450000196")
    if err != nil {
        t.Fatal(err)
    }
    boards, _ := repo.MaterializationBoards(host)
    posts, created := repo.MaterializationPersonaArticleHeaders(host, boards[0])
    if !created || len(posts) == 0 {
        t.Fatalf("conversation view did not create board shells: created=%v posts=%d", created, len(posts))
    }
    all := base.ListPosts(host.ID)
    if len(all) < len(posts) {
        t.Fatalf("host window was not committed: host=%d board=%d", len(all), len(posts))
    }
    for _, post := range all {
        if post.Intent.ProducerEventID != "" || post.Intent.ProducerEpisode != "" || len(post.Intent.ProducerContribution) != 0 {
            t.Fatalf("producer brief leaked into conversation-view shell: %#v", post.Intent)
        }
        if post.Intent.Action == "" || post.Intent.AnchorKey == "" || post.Intent.CauseKind == "" || post.Intent.Motivation == "" {
            t.Fatalf("world shell missing immutable cause/topology data: %#v", post.Intent)
        }
        if post.Body != "" {
            t.Fatalf("header phase unexpectedly rendered body: msg=%d body=%q", post.ID, post.Body)
        }
    }
}

func TestConversationViewPoCLetsWorkerChooseRootSubjectFromConversationContext(t *testing.T) {
    base := world.NewMemoryStore()
    renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "自然に決めた件名", Body: "会話の流れで書いた本文です。"}}
    repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
    repo.EnableDevelopmentConversationViewPoC()
    host, err := repo.HostByPhone("0450000196")
    if err != nil {
        t.Fatal(err)
    }
    boards, _ := repo.MaterializationBoards(host)
    posts, _ := repo.MaterializationPersonaArticleHeaders(host, boards[0])
    var root world.Post
    for _, post := range posts {
        if post.ParentID == 0 {
            root = post
            break
        }
    }
    if root.ID == 0 {
        t.Fatal("no root post selected")
    }
    rendered, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, boards[0], root.ID)
    if !found || !created || rendered.Subject != "自然に決めた件名" {
        t.Fatalf("generated root subject was not committed: found=%v created=%v post=%#v diag=%q", found, created, rendered, diagnostic)
    }
    if renderer.req.CanonicalSubject != "" {
        t.Fatalf("conversation worker was still forced to canonical subject: %q", renderer.req.CanonicalSubject)
    }
    if renderer.req.BoardTopic != boards[0].Name {
        t.Fatalf("worker cue should be board conversation, got %q want %q", renderer.req.BoardTopic, boards[0].Name)
    }
    for _, want := range []string{"CONVERSATION VIEW POC", "CURRENT WORLD SLOT", "WORLD-LAYER CAUSE BOUNDARY", "CANONICAL BOARD CONVERSATION"} {
        if !strings.Contains(renderer.req.PostIntent, want) {
            t.Fatalf("conversation context missing %q: %s", want, renderer.req.PostIntent)
        }
    }
    if strings.Contains(renderer.req.PostIntent, "producer_event_id=") {
        t.Fatalf("producer worker contract remained active: %s", renderer.req.PostIntent)
    }
}

func TestConversationViewPoCReplyUsesRenderedParentAsChatHistory(t *testing.T) {
    base := world.NewMemoryStore()
    renderer := &fakeBoardRenderer{draft: llm.BoardPostDraft{Author: "WRONG", Subject: "会話の件", Body: "親から順番に生成された本文です。"}}
    repo := New(base, conversationViewEvidenceEngine{}, LLMMaterializer{Renderer: renderer}, "1996-08-29")
    repo.EnableDevelopmentConversationViewPoC()
    host, err := repo.HostByPhone("0450000196")
    if err != nil {
        t.Fatal(err)
    }
    boards, _ := repo.MaterializationBoards(host)
    _, _ = repo.MaterializationPersonaArticleHeaders(host, boards[0])
    all := base.ListPosts(host.ID)
    var reply world.Post
    var board world.Board
    for _, candidate := range all {
        if candidate.ParentID == 0 {
            continue
        }
        for _, b := range boards {
            if b.ID == candidate.BoardID {
                reply = candidate
                board = b
                break
            }
        }
        if reply.ID != 0 {
            break
        }
    }
    if reply.ID == 0 {
        t.Fatal("no reply selected in conversation window")
    }
    rendered, found, created, diagnostic := repo.MaterializationArticleWithDebug(host, board, reply.ID)
    if !found || !created {
        t.Fatalf("reply render failed: found=%v created=%v diag=%q", found, created, diagnostic)
    }
    if rendered.Subject != "Re: 会話の件" {
        t.Fatalf("reply subject not canonicalized from rendered root: %q", rendered.Subject)
    }
    if !strings.Contains(renderer.req.PostIntent, "THREAD SO FAR") || !strings.Contains(renderer.req.PostIntent, "親から順番に生成された本文です。") {
        t.Fatalf("reply did not receive prior prose as chat history: %s", renderer.req.PostIntent)
    }
    if renderer.req.CanonicalSubject != "" {
        t.Fatalf("reply worker should choose prose with subject canonicalized after render: %q", renderer.req.CanonicalSubject)
    }
}
''')

(ROOT / 'docs/CONVERSATION_VIEW_POC.md').write_text(r'''# Conversation-view materialization PoC

The isolated `fresh` materialization lab can run a development experiment that bypasses the host-wide semantic Producer.

## Goal

Test whether the naturalness of chat-style BBS roleplay can be recovered without making model chat history authoritative world state.

The database remains canonical for:

- actor identity
- timestamps
- board placement
- root/reply topology
- explicit source post
- world-selected routing domain/cause kind
- root discourse mode
- already-rendered BBS prose

The experimental header pass stores only those world-selected shells. It does **not** persist Producer `episode / referents / actor_knowledge / contribution / goal` briefs.

Immediately before prose rendering, the repository rebuilds a transient conversation view from canonical data:

- the exact thread so far
- the explicit source post selected by the world layer
- a few recent canonical posts by the same actor
- small related-post retrieval already used by the existing renderer
- the current shell's world-layer cause boundary

The article worker then chooses natural subject/body wording. Root subjects are committed from the worker result; reply subjects are canonicalized to `Re: <root subject>` after the root has been rendered. Existing chronological dependency rendering guarantees that a reply sees earlier thread prose first.

## Scope

`EnableDevelopmentConversationViewPoC()` is opt-in per repository instance. The server currently enables it only for the isolated fresh RESET-equivalent -> ALLBODY lab. Ordinary runtime materialization continues to use the existing Producer path.

This is intentionally an experiment, not a production architecture decision. If it materially improves naturalness, the next step is to replace the broad cause boundary with a typed, world-owned `WorldPostSituation` rather than re-expanding the semantic Producer.
''')
