package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// observationJob is process-local coordination only. Canonical observation
// results are the posts/bodies committed to the underlying world store. Keeping
// the waiter primitive out of world state prevents transport timing from becoming
// part of the simulated world.
type observationJob struct {
	done chan struct{}
	err  error
}

const (
	erikaKCatchupAction  = "world-catchup"
	erikaKCatchupCadence = 6 * time.Hour
)

func observationBoardKey(hostID, boardID string) string {
	return hostID + "|" + boardID
}

// BeginHostObservation is called after a successful CONNECT, never by HostByPhone
// or directory/catalog reads. CONNECT starts independent board-header jobs in the
// background. A later board read waits only for its own board job, never for
// unrelated boards on the same host.
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
		r.beginBoardObservation(host, board)
	}
}

func (r *Repository) beginBoardObservation(host world.Host, board world.Board) *observationJob {
	key := observationBoardKey(host.ID, board.ID)

	r.observationMu.Lock()
	if existing := r.observationBoardJobs[key]; existing != nil {
		select {
		case <-existing.done:
			if existing.err == nil {
				// Normal board observation is one-shot. Erika-K experiment boards
				// are different: a completed job can be re-armed after the world
				// clock has advanced far enough to justify another background post.
				if host.SoftwareID != "erika-k" || !r.erikaKBoardNeedsCatchup(host.ID, board.ID, r.currentWorldTime()) {
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
		// Keep a failed job addressable until current waiters see its error. A
		// later CONNECT/read may then replace the failed lease and retry.
		close(job.done)
	}()
	return job
}

func (r *Repository) materializeObservedBoardHeaders(host world.Host, board world.Board) error {
	if host.SoftwareID == "erika-k" {
		return r.materializeErikaKCatchup(host, board)
	}
	if len(filterBoard(r.Base.ListPosts(host.ID), board.ID)) > 0 {
		return nil
	}

	// The interactive development host uses the board-local title-first planner.
	// This is deliberately different from the Fresh Lab's host-wide planner:
	// opening board 2 must not wait for title planning on boards 1, 3, ... 16.
	if host.SoftwareID == "materialization-demo" && developmentInteractiveTitleFirstEnabled(r) {
		r.materializeInteractiveConversationBoardWindow(host, board)
		if errText := strings.TrimSpace(r.MaterializationPlanningDiagnostic(host.ID, board.ID)); strings.Contains(errText, "planning_error=") {
			return fmt.Errorf("observe host %s board %s: %s", host.ID, board.ID, errText)
		}
		return nil
	}

	if err := r.ensureBoard(host, board.ID, board.Name); err != nil {
		return fmt.Errorf("observe host %s board %s: %w", host.ID, board.ID, err)
	}
	return nil
}

func (r *Repository) erikaKBoardNeedsCatchup(hostID, boardID string, now time.Time) bool {
	posts := filterBoard(r.Base.ListPosts(hostID), boardID)
	if len(posts) == 0 {
		return true
	}
	cursor := latestErikaKBoardCursor(posts)
	if cursor.IsZero() || !cursor.Before(now) {
		return false
	}
	return now.Sub(cursor) >= erikaKCatchupCadence
}

func latestErikaKBoardCursor(posts []world.Post) time.Time {
	var latestRoot time.Time
	var latestCatchup time.Time
	for _, post := range posts {
		if post.ParentID != 0 || post.CreatedAt.IsZero() {
			continue
		}
		if post.CreatedAt.After(latestRoot) {
			latestRoot = post.CreatedAt
		}
		if post.Intent.Action == erikaKCatchupAction && post.CreatedAt.After(latestCatchup) {
			latestCatchup = post.CreatedAt
		}
	}
	if !latestCatchup.IsZero() {
		return latestCatchup
	}
	return latestRoot
}

func (r *Repository) materializeErikaKCatchup(host world.Host, board world.Board) error {
	now := r.currentWorldTime()
	if !r.erikaKBoardNeedsCatchup(host.ID, board.ID, now) {
		return nil
	}
	if r.Engine == nil || r.Materializer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	worldDate := now.Format(time.DateOnly)
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     board.Name,
		WorldDate:   worldDate,
		Region:      host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボードに、経過した世界時間に応じた新しい投稿を1件追加するための時代背景", host.Name, board.Name),
		Persistence: true,
		Importance:  .25,
		Specificity: .25,
	})
	if err != nil {
		return fmt.Errorf("catch up Erika-K host %s board %s: %w", host.ID, board.ID, err)
	}
	posts, err := r.Materializer.GenerateBoardPosts(ctx, BoardMaterializationRequest{
		Host:       host,
		BoardID:    board.ID,
		BoardTopic: board.Name,
		WorldDate:  worldDate,
	}, decision)
	if err != nil {
		return fmt.Errorf("catch up Erika-K host %s board %s: %w", host.ID, board.ID, err)
	}
	for _, post := range posts {
		if post.ParentID != 0 {
			continue
		}
		post.BoardID = board.ID
		post.CreatedAt = now
		post.Intent.Action = erikaKCatchupAction
		r.Base.AddPost(host.ID, post)
		break // one new root per cadence/window; never backfill a burst at once
	}
	return nil
}

// WaitForBoardHeaders waits only for this board's CONNECT-triggered background
// job. Callers that bypass CONNECT (tests/tools) safely start that one board on
// demand.
func (r *Repository) WaitForBoardHeaders(ctx context.Context, host world.Host, board world.Board) ([]world.Post, error) {
	// Erika-K board growth is intentionally background-only. Existing index data
	// is immediately usable and article-body reads must never wait for an unrelated
	// catch-up post that happens to be generating for the same board.
	if host.SoftwareID == "erika-k" {
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
