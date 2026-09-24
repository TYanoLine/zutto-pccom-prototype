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
	done     chan struct{}
	err      error
	cancel   context.CancelFunc
	prefetch bool
	started  time.Time
}

func observationBoardKey(hostID, boardID string) string {
	return hostID + "|" + boardID
}

// BeginHostObservation is valid only after a successful CONNECT observation
// boundary, never from HostByPhone or directory/catalog reads. A connected runtime
// may call it at login or during navigation to prefetch a narrowly-scoped board.
// A later board read waits only for its own board job, never for unrelated boards.
func (r *Repository) BeginHostObservation(host world.Host, boards []world.Board) {
	r.beginHostObservation(host, boards, false)
}

// BeginHostPrefetch starts speculative low-priority work. If the user later
// demands another board, Repository cancels unrelated unfinished prefetch jobs
// before starting/joining the demanded board.
func (r *Repository) BeginHostPrefetch(host world.Host, boards []world.Board) {
	r.beginHostObservation(host, boards, true)
}

func (r *Repository) beginHostObservation(host world.Host, boards []world.Board, prefetch bool) {
	if strings.TrimSpace(host.ID) == "" {
		return
	}
	seen := map[string]bool{}
	for _, board := range boards {
		if strings.TrimSpace(board.ID) == "" || seen[board.ID] {
			continue
		}
		seen[board.ID] = true
		r.beginBoardObservation(host, board, prefetch)
	}
}

func (r *Repository) beginBoardObservation(host world.Host, board world.Board, prefetch bool) *observationJob {
	key := observationBoardKey(host.ID, board.ID)

	r.observationMu.Lock()
	if !prefetch {
		r.cancelUnrelatedPrefetchLocked(host.ID, board.ID)
	}
	if existing := r.observationBoardJobs[key]; existing != nil {
		if !prefetch {
			existing.prefetch = false
		}
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
	ctx, cancel := context.WithCancel(context.Background())
	job := &observationJob{
		done:     make(chan struct{}),
		cancel:   cancel,
		prefetch: prefetch,
		started:  time.Now(),
	}
	r.observationBoardJobs[key] = job
	r.observationMu.Unlock()

	go func() {
		mode := "demand"
		if job.prefetch {
			mode = "prefetch"
		}
		job.err = r.materializeObservedBoardHeaders(ctx, host, board, mode)
		elapsed := time.Since(job.started)
		if job.err != nil {
			if ctx.Err() != nil && prefetch {
				log.Printf("BBS header prefetch canceled: host=%s board=%s elapsed=%s", host.ID, board.ID, elapsed)
			} else {
				log.Printf("BBS header observation failed: host=%s board=%s mode=%s elapsed=%s err=%v", host.ID, board.ID, mode, elapsed, job.err)
			}
		} else {
			log.Printf("BBS timing: host=%s board=%s mode=%s phase=header_total duration=%s", host.ID, board.ID, mode, elapsed)
		}
		cancel()
		// Keep a failed job addressable until current waiters see its error. A
		// later CONNECT/read may then replace the failed lease and retry.
		close(job.done)
	}()
	return job
}

func (r *Repository) cancelUnrelatedPrefetchLocked(hostID, demandedBoardID string) {
	prefix := hostID + "|"
	for key, job := range r.observationBoardJobs {
		if job == nil || !job.prefetch || key == observationBoardKey(hostID, demandedBoardID) || !strings.HasPrefix(key, prefix) {
			continue
		}
		select {
		case <-job.done:
		default:
			if job.cancel != nil {
				job.cancel()
			}
		}
	}
}

func (r *Repository) materializeObservedBoardHeaders(ctx context.Context, host world.Host, board world.Board, mode string) error {
	// The isolated materialization-demo host remains a diagnostic harness for the
	// older title-first experiments. Real host runtimes all use the same shared
	// BBS article engine below; host software only controls how canonical posts
	// are presented to callers.
	if host.SoftwareID == "materialization-demo" && developmentInteractiveTitleFirstEnabled(r) {
		r.materializeInteractiveConversationBoardWindow(host, board)
		if errText := strings.TrimSpace(r.MaterializationPlanningDiagnostic(host.ID, board.ID)); strings.Contains(errText, "planning_error=") {
			return fmt.Errorf("observe host %s board %s: %s", host.ID, board.ID, errText)
		}
		return nil
	}

	if r.sharedBBSArticleEngineEnabled(host) && r.bbsArticles != nil {
		// Predictive prefetch and a user's demanded board may overlap in wall-clock
		// time. Keep only one expensive header-materialization pipeline active per
		// host so the "minimal range" policy cannot turn into parallel LLM bursts.
		// Board jobs remain distinct: callers still wait only for their requested
		// board, but its worker may queue briefly behind the host's current prefetch.
		lock := r.sharedBBSHostMaterializationLock(host.ID)
		queueStarted := time.Now()
		lock.Lock()
		queueWait := time.Since(queueStarted)
		defer lock.Unlock()
		log.Printf("BBS timing: host=%s board=%s mode=%s phase=queue_wait duration=%s", host.ID, board.ID, mode, queueWait)

		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		if r.debugImmediateBBSHost(host.ID) {
			err = r.bbsArticles.CatchUpInitial(ctx, host, board)
		} else {
			err = r.bbsArticles.CatchUp(ctx, host, board)
		}
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
			return r.repairDevelopmentPendingReplySubjects(host.ID, existing), nil
		}
		job = r.beginBoardObservation(host, board, false)
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
	return r.repairDevelopmentPendingReplySubjects(host.ID, filterBoard(r.Base.ListPosts(host.ID), board.ID)), nil
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

		rootID := selected.ID
		if selected.ParentID != 0 {
			rootID = selected.ParentID
		}
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
