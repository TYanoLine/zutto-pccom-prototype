package worldrepo

import "zutto-pccom/apps/server/internal/world"

// ResetBBSGeneratedArticles is a debug-only reset for the shared article engine.
// It removes only engine-generated history (and descendants of removed roots).
// Seed history, user posts, boards, personas and host-program configuration stay.
func (r *Repository) ResetBBSGeneratedArticles(host world.Host) (removed int, kept int, ok bool) {
	if r == nil || r.bbsArticles == nil || !r.sharedBBSArticleEngineEnabled(host) {
		return 0, 0, false
	}

	// Do not clear underneath a running catch-up/body worker: that worker could
	// commit stale history after reset. Holding this lock also prevents a new
	// observation job from starting until the canonical post replacement ends.
	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	if r.observationRunningLocked(host.ID) {
		return 0, 0, false
	}
	r.clearCompletedObservationJobsLocked(host.ID)
	return r.bbsArticles.ResetGenerated(host.ID)
}

type debugHostPostClearer interface {
	ClearHostPosts(hostID string) int
}

// PrepareDebugBBSConnection clears the experiment station's entire article state
// and marks it so each board's first observation gets one immediate fresh batch,
// regardless of the normal world-time cadence. During this temporary evaluation
// mode there is intentionally no article seed and no cross-call user-post history.
func (r *Repository) PrepareDebugBBSConnection(host world.Host) (removed int, kept int, ok bool) {
	if r == nil || r.bbsArticles == nil || !r.sharedBBSArticleEngineEnabled(host) {
		return 0, 0, false
	}
	clearer, supported := r.Base.(debugHostPostClearer)
	if !supported {
		return 0, 0, false
	}

	r.observationMu.Lock()
	defer r.observationMu.Unlock()
	if r.observationRunningLocked(host.ID) {
		return 0, 0, false
	}
	r.clearCompletedObservationJobsLocked(host.ID)
	removed = clearer.ClearHostPosts(host.ID)

	r.mu.Lock()
	r.debugImmediateBBS[host.ID] = true
	r.mu.Unlock()
	return removed, 0, true
}

func (r *Repository) debugImmediateBBSHost(hostID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.debugImmediateBBS[hostID]
}
