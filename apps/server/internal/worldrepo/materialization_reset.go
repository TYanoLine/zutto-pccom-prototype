package worldrepo

import (
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

// ResetMaterializationConversation clears only development conversation history
// and lazily materialized persona facts. The host profile, board catalog,
// memberships and sparse core persona skeletons remain intact so a tester can
// regenerate the same cast and compare semantic-planning changes.
func (r *Repository) ResetMaterializationConversation(host world.Host) (postsCleared int, personaFactsCleared int, ok bool) {
	resetter, ok := r.Base.(world.DevelopmentConversationResetStore)
	if !ok {
		return 0, 0, false
	}

	// RESET must invalidate the process-local observation completion markers as
	// well as canonical posts. Otherwise a completed board job survives the DB
	// clear and WaitForBoardHeaders immediately returns an empty "stored reuse"
	// result instead of starting a fresh observation. Do not reset underneath a
	// still-running observation/body job; that worker could commit stale results
	// after the clear.
	r.observationMu.Lock()
	if r.observationRunningLocked(host.ID) {
		r.observationMu.Unlock()
		return 0, 0, false
	}
	r.clearCompletedObservationJobsLocked(host.ID)

	personaIDs := make([]string, 0)
	if personas, supported := r.Base.(world.PersonaStore); supported {
		for _, persona := range personas.ListHostPersonas(host.ID) {
			personaIDs = append(personaIDs, persona.ID)
		}
	}
	postsCleared = resetter.ClearHostPosts(host.ID)
	personaFactsCleared = resetter.ClearPersonaFacts(personaIDs)
	r.observationMu.Unlock()

	// Generic ensureBoard also keeps a process-local materialized marker. RESET
	// means "generate again", so invalidate those host/board markers too.
	r.mu.Lock()
	prefix := host.ID + "|"
	for key := range r.materialized {
		if strings.HasPrefix(key, prefix) {
			delete(r.materialized, key)
		}
	}
	r.mu.Unlock()

	developmentGenerationUsage.Range(func(key, _ any) bool {
		usageKey, keyOK := key.(generationUsageKey)
		if keyOK && usageKey.repo == r {
			developmentGenerationUsage.Delete(key)
		}
		return true
	})
	clearDevelopmentPlanningTelemetry(r)
	if developmentTitleFirstEnabled(r) {
		// RESET starts a genuinely new title-first planning pass instead of
		// reusing the cached candidate/assignment result from the prior run.
		r.EnableDevelopmentTitleFirstPoC(nil)
	}
	return postsCleared, personaFactsCleared, true
}
