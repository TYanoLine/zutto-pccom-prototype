package worldrepo

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// observationJob is process-local coordination only. Canonical observation
// results are the posts/bodies committed to the underlying world store. Keeping
// the waiter primitive out of world state prevents transport timing from becoming
// part of the simulated world.

type observationJob struct {
	done chan struct{}
	err  error
}

func observationBoardKey(hostID, boardID string) string {
	return hostID + "|" + boardID
}

// BeginHostObservation starts demanded work. If the same board is still waiting
// in the low-priority prefetch queue, demand promotion removes it from that queue
// and starts/joins it immediately. A different background board already running
// is allowed to continue in parallel.
func (r *Repository) BeginHostObservation(host world.Host, boards []world.Board) {
	if strings.TrimSpace(host.ID) == "" {
		return
	}
	seen := map[string]bool{}
	for _, board := range boards {
		if strings.TrimSpace(board.ID) == "" || seen[board.ID] {
			continue
		}
		seen[board.ID] = true
		r.promoteBoardFromPrefetch(host.ID, board.ID)
		r.beginBoardObservation(host, board)
	}
}

// BeginHostPrefetch appends speculative work to one ordered background queue per
// host. The background lane is serial (A -> B -> C -> D), while an explicit
// demand may promote C out of the waiting queue and run it in parallel with the
// currently executing background item A.
func (r *Repository) BeginHostPrefetch(host world.Host, boards []world.Board) {
	if strings.TrimSpace(host.ID) == "" || len(boards) == 0 {
		return
	}

	r.prefetchMu.Lock()
	queue := r.prefetchQueues[host.ID]
	queued := make(map[string]bool, len(queue))
	for _, board := range queue {
		queued[board.ID] = true
	}
	for _, board := range boards {
		if strings.TrimSpace(board.ID) == "" || queued[board.ID] {
			continue
		}
		queue = append(queue, board)
		queued[board.ID] = true
	}
	r.prefetchQueues[host.ID] = queue
	if r.prefetchRunning[host.ID] || len(queue) == 0 {
		r.prefetchMu.Unlock()
		return
	}
	r.prefetchRunning[host.ID] = true
	r.prefetchMu.Unlock()

	go r.runHostPrefetchQueue(host)
}

func (r *Repository) runHostPrefetchQueue(host world.Host) {
	for {
		r.prefetchMu.Lock()
		queue := r.prefetchQueues[host.ID]
		if len(queue) == 0 {
			r.prefetchRunning[host.ID] = false
			r.prefetchMu.Unlock()
			return
		}
		board := queue[0]
		r.prefetchQueues[host.ID] = append([]world.Board(nil), queue[1:]...)
		r.prefetchMu.Unlock()

		started := time.Now()
		job := r.beginBoardObservation(host, board)
		if job != nil {
			<-job.done
		}
		log.Printf("BBS timing: host=%s board=%s mode=prefetch phase=queue_item_total duration=%s", host.ID, board.ID, time.Since(started))
	}
}

func (r *Repository) promoteBoardFromPrefetch(hostID, boardID string) {
	r.prefetchMu.Lock()
	defer r.prefetchMu.Unlock()
	queue := r.prefetchQueues[hostID]
	if len(queue) == 0 {
		return
	}
	out := queue[:0]
	for _, board := range queue {
		if board.ID == boardID {
			continue
		}
		out = append(out, board)
	}
	r.prefetchQueues[hostID] = append([]world.Board(nil), out...)
}

func (r *Repository) beginBoardObservation(host world.Host, board world.Board) *observationJob {
	key := observationBoardKey(host.ID, board.ID)

	r.observationMu.Lock()
	if existing := r.observationBoardJobs[key]; existing != nil {
		select {
		case <-existing.done:
			if existing.err == nil {
				if !r.sharedBBSArticleEngineEnabled(host) || r.bbsArticles == nil || !r.bbsArticles.NeedsCatchUp(host, board) {
					r.observationMu.Unlock()
					return existing
				}
			}
			delete(r.observationBoardJobs, key)
		default:
			r.observationMu.Unlock()
			return existing
		}
	}
	job := &observationJob{done: make(chan struct{})}
	r.observationBoardJobs[key] = job
	r.observationMu.Unlock()

	go func() {
		job.err = r.materializeObservedBoardHeaders(host, board)
		if job.err != nil {
			log.Printf("BBS header observation failed: host=%s board=%s err=%v", host.ID, board.ID, job.err)
		}
		// Keep a failed job addressable until current waiters see its error. A
		// later CONNECT/read may then replace the failed lease and retry.
		close(job.done)
	}()
	return job
}

func (r *Repository) materializeObservedBoardHeaders(host world.Host, board world.Board) error {
	started := time.Now()
	defer func() {
		log.Printf("BBS timing: host=%s board=%s phase=header_materialize_total duration=%s", host.ID, board.ID, time.Since(started))
	}()
	// Real hosts share the World-owned on-demand BBS engine.

	if r.sharedBBSArticleEngineEnabled(host) && r.bbsArticles != nil {
		// If this board has never been materialized, realize the prose-free
		// activity state that already existed before the user opened the board.
		// This replaces the old HAKATA-only fixed 40-root evaluation batch.
		var err error
		ctx, traceDone := r.beginGenerationTrace(context.Background(), host, board, "headers", 0)
		existing := filterBoard(r.Base.ListPosts(host.ID), board.ID)
		if len(existing) == 0 {
			if state, ok := r.BoardActivity(host, board); ok && state.RetainedRoots > 0 {
				err = r.bbsArticles.CatchUpInitialBoardActivity(context.Background(), host, board, state)
			} else {
				err = r.bbsArticles.CatchUp(context.Background(), host, board)
			}
		} else {
			err = r.bbsArticles.CatchUp(context.Background(), host, board)
		}
		// Inspect the store after the attempt, including any posts actually saved
		// before a later slot failed. Never log uncommitted planner drafts.
		r.logNewBBSHeaders(host, board, existing)
		if err != nil {
			return fmt.Errorf("shared BBS catch-up host %s board %s: %w", host.ID, board.ID, err)
		}
		return nil
	}

	if len(filterBoard(r.Base.ListPosts(host.ID), board.ID)) > 0 {
		return nil
	}
	if err := r.ensureBoard(host, board.ID, board.Name); err != nil {
		return fmt.Errorf("observe host %s board %s: %w", host.ID, board.ID, err)
	}
	return nil
}

// WaitForBoardHeaders waits only for this board's CONNECT-triggered background
// job. Callers that bypass CONNECT (tests/tools) safely start that one board on
// demand.
func (r *Repository) WaitForBoardHeaders(ctx context.Context, host world.Host, board world.Board) ([]world.Post, error) {
	// Existing canonical history is immediately readable for every real host.
	// Background catch-up may add a new batch later, but host-program navigation
	// never waits merely because the shared article engine is extending history.
	if r.sharedBBSArticleEngineEnabled(host) {
		if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
			return existing, nil
		}
	}
	job := r.boardObservationJob(host.ID, board.ID)
	if job == nil {
		// Persisted canonical data from a previous process is already complete.
		// Only start a new observation when this board has never materialized.
		if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
			return existing, nil
		}
		job = r.beginBoardObservation(host, board)
	}
	if job != nil {
		select {
		case <-job.done:
			if job.err != nil {
				r.forgetFailedBoardObservation(host.ID, board.ID, job)
				return nil, job.err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return filterBoard(r.Base.ListPosts(host.ID), board.ID), nil
}

func (r *Repository) boardObservationJob(hostID, boardID string) *observationJob {
	key := observationBoardKey(hostID, boardID)
	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	return r.observationBoardJobs[key]
}

func (r *Repository) forgetFailedBoardObservation(hostID, boardID string, job *observationJob) {
	key := observationBoardKey(hostID, boardID)
	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	if r.observationBoardJobs[key] == job && job.err != nil {
		delete(r.observationBoardJobs, key)
	}
}

// WaitForArticleBody single-flights prose materialization by thread. If two users
// open different messages in the same thread simultaneously, one render runs
// first; the other waiter re-checks canonical state and then renders only what is
// still missing. This preserves causal root->reply prose order.
func (r *Repository) WaitForArticleBody(ctx context.Context, host world.Host, board world.Board, postID int64) (world.Post, bool, error) {
	if _, err := r.WaitForBoardHeaders(ctx, host, board); err != nil {
		return world.Post{}, false, err
	}

	for {
		selected, found := r.findMaterializationPost(host.ID, board.ID, postID)
		if !found {
			return world.Post{}, false, nil
		}
		if strings.TrimSpace(selected.Body) != "" {
			return selected, true, nil
		}

		postsByID := map[int64]world.Post{}
		for _, post := range r.Base.ListPosts(host.ID) {
			postsByID[post.ID] = post
		}
		rootID := threadRootID(postsByID, selected)
		key := fmt.Sprintf("%s|%s|%d", host.ID, board.ID, rootID)
		job, _ := r.getOrStartBodyObservationJob(key, host, board, postID)

		select {
		case <-job.done:
			if job.err != nil {
				return world.Post{}, true, job.err
			}
		case <-ctx.Done():
			return world.Post{}, true, ctx.Err()
		}
		// The shared thread job may have targeted an earlier message requested by
		// another session. Re-check and, if needed, start the next causal step.
	}
}

func (r *Repository) getOrStartBodyObservationJob(key string, host world.Host, board world.Board, postID int64) (*observationJob, bool) {
	r.observationMu.Lock()
	if existing := r.observationBodyJobs[key]; existing != nil {
		r.observationMu.Unlock()
		return existing, false
	}
	job := &observationJob{done: make(chan struct{})}
	r.observationBodyJobs[key] = job
	r.observationMu.Unlock()

	go func() {
		post, found, _, diagnostic := r.MaterializationArticleWithDebug(host, board, postID)
		switch {
		case !found:
			job.err = fmt.Errorf("article %d not found after header observation", postID)
		case strings.Contains(diagnostic, "error stage="):
			job.err = fmt.Errorf("article %d materialization failed: %s", postID, diagnostic)
		case strings.TrimSpace(post.Body) == "":
			job.err = fmt.Errorf("article %d materialization returned an empty body", postID)
		}
		if job.err != nil {
			log.Printf("BBS article body observation failed: host=%s board=%s post=%d err=%v", host.ID, board.ID, postID, job.err)
		}

		r.observationMu.Lock()
		if r.observationBodyJobs[key] == job {
			delete(r.observationBodyJobs, key)
		}
		r.observationMu.Unlock()
		close(job.done)
	}()
	return job, true
}


func (r *Repository) MaterializationObservationRunning(hostID string) bool {
	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	return r.observationRunningLocked(hostID)
}

func (r *Repository) observationRunningLocked(hostID string) bool {
	prefix := hostID + "|"
	for key, job := range r.observationBoardJobs {
		if !strings.HasPrefix(key, prefix) || job == nil {
			continue
		}
		select {
		case <-job.done:
		default:
			return true
		}
	}
	for key, job := range r.observationBodyJobs {
		if !strings.HasPrefix(key, prefix) || job == nil {
			continue
		}
		select {
		case <-job.done:
		default:
			return true
		}
	}
	return false
}

func (r *Repository) clearCompletedObservationJobsLocked(hostID string) {
	prefix := hostID + "|"
	for key, job := range r.observationBoardJobs {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if job == nil {
			delete(r.observationBoardJobs, key)
			continue
		}
		select {
		case <-job.done:
			delete(r.observationBoardJobs, key)
		default:
		}
	}
	for key, job := range r.observationBodyJobs {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if job == nil {
			delete(r.observationBodyJobs, key)
			continue
		}
		select {
		case <-job.done:
			delete(r.observationBodyJobs, key)
		default:
		}
	}
}
