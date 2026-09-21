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
