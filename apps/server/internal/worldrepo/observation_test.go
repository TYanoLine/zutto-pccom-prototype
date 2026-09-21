package worldrepo

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type observationTestEvidence struct{}

func (observationTestEvidence) ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error) {
	return worldengine.EvidenceDecision{}, nil
}

type blockingObservationMaterializer struct {
	calls   atomic.Int32
	once    sync.Once
	started chan struct{}
	release chan struct{}
	body    string
}

func (m *blockingObservationMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	m.calls.Add(1)
	if m.started != nil {
		m.once.Do(func() { close(m.started) })
	}
	if m.release != nil {
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	body := m.body
	if body == "" {
		body = "materialized body"
	}
	return []world.Post{{
		BoardID:  req.BoardID,
		Author:   "NPC",
		Subject:  "materialized subject",
		Body:     body,
		CreatedAt: time.Date(1996, 8, 26, 20, 0, 0, 0, time.Local),
	}}, nil
}

func TestHostLookupDoesNotObserveOrGenerate(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")

	if _, err := repo.HostByPhone("0450000001"); err != nil {
		t.Fatal(err)
	}
	if got := materializer.calls.Load(); got != 0 {
		t.Fatalf("HostByPhone generated content: calls=%d", got)
	}
	if posts := base.ListPosts("quiet-test"); len(posts) != 0 {
		t.Fatalf("host metadata lookup created posts: %+v", posts)
	}
}

func TestHostObservationStartsInBackgroundAndBoardReadWaits(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}

	repo.BeginHostObservation(host, []world.Board{board})
	select {
	case <-materializer.started:
	case <-time.After(time.Second):
		t.Fatal("background observation did not start")
	}

	type result struct {
		posts []world.Post
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		posts, err := repo.WaitForBoardHeaders(context.Background(), host, board)
		resultCh <- result{posts: posts, err: err}
	}()

	select {
	case got := <-resultCh:
		t.Fatalf("board read returned before background observation completed: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	close(materializer.release)
	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if len(got.posts) != 1 || got.posts[0].Subject != "materialized subject" {
			t.Fatalf("unexpected observed headers: %+v", got.posts)
		}
	case <-time.After(time.Second):
		t.Fatal("board read did not resume after observation completed")
	}

	repo.BeginHostObservation(host, []world.Board{board})
	if _, err := repo.WaitForBoardHeaders(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if got := materializer.calls.Load(); got != 1 {
		t.Fatalf("observation was not single-flighted: calls=%d", got)
	}
}

func TestArticleBodyWaitSingleFlightsConcurrentReaders(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{
		started: make(chan struct{}),
		release: make(chan struct{}),
		body:    "generated article body",
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}
	post := base.AddPost(host.ID, world.Post{
		BoardID:   board.ID,
		Author:    "NPC",
		Subject:   "header only",
		CreatedAt: time.Date(1996, 8, 26, 19, 0, 0, 0, time.Local),
	})
	repo.BeginHostObservation(host, []world.Board{board})

	type result struct {
		post  world.Post
		found bool
		err   error
	}
	ch := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			p, found, err := repo.WaitForArticleBody(context.Background(), host, board, post.ID)
			ch <- result{post: p, found: found, err: err}
		}()
	}

	select {
	case <-materializer.started:
	case <-time.After(time.Second):
		t.Fatal("body materialization did not start")
	}
	time.Sleep(30 * time.Millisecond)
	if got := materializer.calls.Load(); got != 1 {
		t.Fatalf("concurrent readers started duplicate body generation: calls=%d", got)
	}

	close(materializer.release)
	for i := 0; i < 2; i++ {
		select {
		case got := <-ch:
			if got.err != nil || !got.found || got.post.Body != "generated article body" {
				t.Fatalf("unexpected body result: %+v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("reader did not resume after body materialization")
		}
	}
	if got := materializer.calls.Load(); got != 1 {
		t.Fatalf("body materialization was duplicated: calls=%d", got)
	}
}


type perBoardObservationMaterializer struct {
	started chan string
	release map[string]chan struct{}
}

func (m *perBoardObservationMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, _ worldengine.EvidenceDecision) ([]world.Post, error) {
	m.started <- req.BoardID
	if ch := m.release[req.BoardID]; ch != nil {
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []world.Post{{
		BoardID: req.BoardID, Author: "NPC", Subject: "board " + req.BoardID,
		Body: "body", CreatedAt: time.Date(1996, 8, 26, 20, 0, 0, 0, time.Local),
	}}, nil
}

func TestBoardObservationWaitDoesNotBlockOnUnrelatedBoard(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &perBoardObservationMaterializer{
		started: make(chan string, 2),
		release: map[string]chan struct{}{
			"a": make(chan struct{}),
			"b": make(chan struct{}),
		},
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	boardA := world.Board{ID: "a", Name: "A"}
	boardB := world.Board{ID: "b", Name: "B"}

	repo.BeginHostObservation(host, []world.Board{boardA, boardB})
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-materializer.started:
			seen[id] = true
		case <-time.After(time.Second):
			t.Fatalf("both board jobs did not start independently: %+v", seen)
		}
	}

	close(materializer.release["b"])
	resultCh := make(chan error, 1)
	go func() {
		posts, err := repo.WaitForBoardHeaders(context.Background(), host, boardB)
		if err == nil && (len(posts) != 1 || posts[0].BoardID != "b") {
			err = fmt.Errorf("unexpected board B posts: %+v", posts)
		}
		resultCh <- err
	}()

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("board B waited for unrelated board A")
	}

	select {
	case <-materializer.release["a"]:
		t.Fatal("test setup unexpectedly released board A")
	default:
	}
	close(materializer.release["a"])
}


func TestResetRearmsCompletedBoardObservation(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}

	repo.BeginHostObservation(host, []world.Board{board})
	posts, err := repo.WaitForBoardHeaders(context.Background(), host, board)
	if err != nil || len(posts) != 1 {
		t.Fatalf("initial observation failed: posts=%+v err=%v", posts, err)
	}
	if got := materializer.calls.Load(); got != 1 {
		t.Fatalf("initial materialization calls=%d", got)
	}

	cleared, _, ok := repo.ResetMaterializationConversation(host)
	if !ok || cleared != 1 {
		t.Fatalf("reset failed: cleared=%d ok=%v", cleared, ok)
	}
	if repo.boardObservationJob(host.ID, board.ID) != nil {
		t.Fatal("completed board observation marker survived reset")
	}

	posts, err = repo.WaitForBoardHeaders(context.Background(), host, board)
	if err != nil || len(posts) != 1 {
		t.Fatalf("post-reset observation failed: posts=%+v err=%v", posts, err)
	}
	if got := materializer.calls.Load(); got != 2 {
		t.Fatalf("reset did not start a fresh observation: calls=%d", got)
	}
}

func TestResetRefusesWhileBoardObservationIsRunning(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	board := world.Board{ID: "main", Name: "フリートーク"}

	repo.BeginHostObservation(host, []world.Board{board})
	select {
	case <-materializer.started:
	case <-time.After(time.Second):
		t.Fatal("observation did not start")
	}
	if !repo.MaterializationObservationRunning(host.ID) {
		t.Fatal("running observation was not reported")
	}
	if cleared, facts, ok := repo.ResetMaterializationConversation(host); ok || cleared != 0 || facts != 0 {
		t.Fatalf("reset should be refused while worker runs: cleared=%d facts=%d ok=%v", cleared, facts, ok)
	}

	close(materializer.release)
	if _, err := repo.WaitForBoardHeaders(context.Background(), host, board); err != nil {
		t.Fatal(err)
	}
	if repo.MaterializationObservationRunning(host.ID) {
		t.Fatal("completed observation still reported running")
	}
	if _, _, ok := repo.ResetMaterializationConversation(host); !ok {
		t.Fatal("reset should succeed after observation completes")
	}
}

