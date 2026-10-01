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
		BoardID:   req.BoardID,
		Author:    "NPC",
		Subject:   "materialized subject",
		Body:      body,
		CreatedAt: time.Date(1996, 8, 26, 20, 0, 0, 0, time.Local),
	}}, nil
}

func TestHostLookupDoesNotObserveOrGenerate(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &blockingObservationMaterializer{}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	repo.SetArticleDetailPlanner(emptyArticleDetailPlanner{})

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
	repo.SetArticleDetailPlanner(emptyArticleDetailPlanner{})
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
	repo.SetArticleDetailPlanner(emptyArticleDetailPlanner{})
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

func TestCompletedBoardObservationCanBeSafelyRearmed(t *testing.T) {
	base:=world.NewMemoryStore()
	materializer:=&blockingObservationMaterializer{}
	repo:=New(base,observationTestEvidence{},materializer,"1996-08-26")
	host,err:=repo.HostByPhone("0450000001");if err!=nil{t.Fatal(err)}
	board:=world.Board{ID:"main",Name:"フリートーク"}
	repo.BeginHostObservation(host,[]world.Board{board})
	first,err:=repo.WaitForBoardHeaders(context.Background(),host,board)
	if err!=nil||len(first)!=1{t.Fatalf("initial observation: posts=%v err=%v",first,err)}
	repo.observationMu.Lock()
	if repo.observationRunningLocked(host.ID) {repo.observationMu.Unlock();t.Fatal("completed worker still running")}
	repo.clearCompletedObservationJobsLocked(host.ID)
	removed:=base.ClearHostPosts(host.ID)
	repo.observationMu.Unlock()
	// Generic-host fallback uses a one-time realization marker as well as
	// observation leases. Re-arm both here, as the debug reset path does.
	repo.mu.Lock()
	delete(repo.materialized,host.ID+"|"+board.ID)
	repo.mu.Unlock()
	if removed!=1 {t.Fatalf("cleared %d posts, want 1",removed)}
	again,err:=repo.WaitForBoardHeaders(context.Background(),host,board)
	if err!=nil||len(again)!=1||materializer.calls.Load()!=2 {
		t.Fatalf("fresh observation failed: posts=%v err=%v calls=%d",again,err,materializer.calls.Load())
	}
}

func TestRunningBoardObservationBlocksCanonicalReset(t *testing.T) {
	base:=world.NewMemoryStore()
	materializer:=&blockingObservationMaterializer{started:make(chan struct{}),release:make(chan struct{})}
	repo:=New(base,observationTestEvidence{},materializer,"1996-08-26")
	host,err:=repo.HostByPhone("0450000001");if err!=nil{t.Fatal(err)}
	board:=world.Board{ID:"main",Name:"フリートーク"}
	repo.BeginHostObservation(host,[]world.Board{board})
	select { case <-materializer.started: case <-time.After(time.Second): t.Fatal("observation never started") }
	repo.observationMu.Lock()
	blocked:=repo.observationRunningLocked(host.ID)
	repo.observationMu.Unlock()
	if !blocked {t.Fatal("running worker did not block reset")}
	close(materializer.release)
	if _,err:=repo.WaitForBoardHeaders(context.Background(),host,board);err!=nil{t.Fatal(err)}
	repo.observationMu.Lock()
	stillRunning:=repo.observationRunningLocked(host.ID)
	repo.observationMu.Unlock()
	if stillRunning {t.Fatal("completed worker still blocks reset")}
}

func TestPrefetchQueuePromotesDemandedBoardWithoutStoppingCurrentBackgroundItem(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &perBoardObservationMaterializer{
		started: make(chan string, 8),
		release: map[string]chan struct{}{
			"a": make(chan struct{}),
			"b": make(chan struct{}),
			"c": make(chan struct{}),
			"d": make(chan struct{}),
		},
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}

	boardA := world.Board{ID: "a", Name: "A"}
	boardB := world.Board{ID: "b", Name: "B"}
	boardC := world.Board{ID: "c", Name: "C"}
	boardD := world.Board{ID: "d", Name: "D"}
	repo.BeginHostPrefetch(host, []world.Board{boardA, boardB, boardC, boardD})

	select {
	case got := <-materializer.started:
		if got != "a" {
			t.Fatalf("first background board=%q, want a", got)
		}
	case <-time.After(time.Second):
		t.Fatal("background queue did not start board a")
	}

	// C is waiting behind A/B. Demand should remove C from that waiting queue
	// and start it immediately without canceling A.
	repo.BeginHostObservation(host, []world.Board{boardC})
	select {
	case got := <-materializer.started:
		if got != "c" {
			t.Fatalf("demanded board start=%q, want c", got)
		}
	case <-time.After(time.Second):
		t.Fatal("demanded board c did not start in parallel with background a")
	}

	close(materializer.release["c"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := repo.WaitForBoardHeaders(ctx, host, boardC); err != nil {
		t.Fatal(err)
	}

	// A is still the background item. Releasing it should advance the background
	// queue to B, then D; C must not reappear because demand promoted it out.
	close(materializer.release["a"])
	select {
	case got := <-materializer.started:
		if got != "b" {
			t.Fatalf("background after a=%q, want b", got)
		}
	case <-time.After(time.Second):
		t.Fatal("background queue did not advance to b")
	}

	close(materializer.release["b"])
	select {
	case got := <-materializer.started:
		if got != "d" {
			t.Fatalf("background after b=%q, want d (c should be removed)", got)
		}
	case <-time.After(time.Second):
		t.Fatal("background queue did not advance to d")
	}
	close(materializer.release["d"])

	select {
	case got := <-materializer.started:
		t.Fatalf("unexpected extra board generation after queue drain: %q", got)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestDemandJoinsSameBoardAlreadyRunningInPrefetch(t *testing.T) {
	base := world.NewMemoryStore()
	materializer := &perBoardObservationMaterializer{
		started: make(chan string, 4),
		release: map[string]chan struct{}{"c": make(chan struct{})},
	}
	repo := New(base, observationTestEvidence{}, materializer, "1996-08-26")
	host, err := repo.HostByPhone("0450000001")
	if err != nil {
		t.Fatal(err)
	}
	boardC := world.Board{ID: "c", Name: "C"}
	repo.BeginHostPrefetch(host, []world.Board{boardC})
	select {
	case got := <-materializer.started:
		if got != "c" {
			t.Fatalf("prefetch started %q, want c", got)
		}
	case <-time.After(time.Second):
		t.Fatal("prefetch c did not start")
	}

	repo.BeginHostObservation(host, []world.Board{boardC})
	select {
	case got := <-materializer.started:
		t.Fatalf("same demanded board started duplicate job: %q", got)
	case <-time.After(80 * time.Millisecond):
	}

	close(materializer.release["c"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := repo.WaitForBoardHeaders(ctx, host, boardC); err != nil {
		t.Fatal(err)
	}
}
