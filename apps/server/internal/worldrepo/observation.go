package worldrepo

import (
	"context"
	"fmt"
	"strings"

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

// BeginHostObservation is called after a successful CONNECT, never by HostByPhone
// or directory/catalog reads. It starts header catch-up in the background and is
// idempotent for concurrent sessions observing the same host.
func (r *Repository) BeginHostObservation(host world.Host, boards []world.Board) {
	if strings.TrimSpace(host.ID) == "" {
		return
	}

	r.observationMu.Lock()
	if _, ok := r.observationHostJobs[host.ID]; ok {
		r.observationMu.Unlock()
		return
	}
	job := &observationJob{done: make(chan struct{})}
	r.observationHostJobs[host.ID] = job
	r.observationMu.Unlock()

	boardCopy := append([]world.Board(nil), boards...)
	go func() {
		job.err = r.materializeObservedHostHeaders(host, boardCopy)
		close(job.done)
	}()
}

func (r *Repository) materializeObservedHostHeaders(host world.Host, boards []world.Board) error {
	// Existing canonical posts mean this prototype host has already been observed.
	// We do not manufacture missing boards on a later process-local observation,
	// because silence/absence may itself be part of the stored world history.
	if len(r.Base.ListPosts(host.ID)) > 0 {
		return nil
	}

	// The development observation host has a host-wide title-first planner. Run it
	// once here so CONNECT, rather than the first board-index request, becomes the
	// observation trigger. No article bodies are rendered in this phase.
	if host.SoftwareID == "materialization-demo" && developmentConversationViewPoCEnabled(r) {
		_, _ = r.materializeConversationWorldWindow(host)
		return nil
	}

	// Other runtimes retain their own board topology. The host program supplies
	// the catalog; the repository only fills content for those already-known boards.
	seen := map[string]bool{}
	for _, board := range boards {
		if board.ID == "" || seen[board.ID] {
			continue
		}
		seen[board.ID] = true
		if len(filterBoard(r.Base.ListPosts(host.ID), board.ID)) > 0 {
			continue
		}
		if err := r.ensureBoard(host, board.ID, board.Name); err != nil {
			return fmt.Errorf("observe host %s board %s: %w", host.ID, board.ID, err)
		}
	}
	return nil
}

// WaitForBoardHeaders blocks only when the CONNECT-triggered host observation is
// still producing the canonical header set. Callers that bypass CONNECT (tests,
// tools) safely start a one-board observation on demand.
func (r *Repository) WaitForBoardHeaders(ctx context.Context, host world.Host, board world.Board) ([]world.Post, error) {
	job := r.hostObservationJob(host.ID)
	if job == nil {
		r.BeginHostObservation(host, []world.Board{board})
		job = r.hostObservationJob(host.ID)
	}
	if job != nil {
		select {
		case <-job.done:
			if job.err != nil {
				return nil, job.err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.repairDevelopmentPendingReplySubjects(host.ID, filterBoard(r.Base.ListPosts(host.ID), board.ID)), nil
}

func (r *Repository) hostObservationJob(hostID string) *observationJob {
	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	return r.observationHostJobs[hostID]
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
		job, started := r.getOrStartBodyObservationJob(key, host, board, postID)
		if !started {
			// Another session is already materializing this thread.
		}

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
