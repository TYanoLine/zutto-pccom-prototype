package worldrepo

import "zutto-pccom/apps/server/internal/world"

// ResetMaterializationConversation clears only development conversation history
// and topic-triggered persona facts. The host profile, board catalog, memberships
// and sparse core persona skeletons remain intact so a tester can regenerate the
// same cast and compare the effect of conversation-materialization changes.
func (r *Repository) ResetMaterializationConversation(host world.Host) (postsCleared int, personaFactsCleared int, ok bool) {
	resetter, ok := r.Base.(world.DevelopmentConversationResetStore)
	if !ok {
		return 0, 0, false
	}

	personaIDs := make([]string, 0)
	if personas, supported := r.Base.(world.PersonaStore); supported {
		for _, persona := range personas.ListHostPersonas(host.ID) {
			personaIDs = append(personaIDs, persona.ID)
		}
	}
	postsCleared = resetter.ClearHostPosts(host.ID)
	personaFactsCleared = resetter.ClearPersonaFacts(personaIDs)

	// Token usage is operational debug state, not world state. Reset it together
	// with the demo conversation so before/after token totals are easy to compare.
	developmentGenerationUsage.Range(func(key, _ any) bool {
		usageKey, keyOK := key.(generationUsageKey)
		if keyOK && usageKey.repo == r {
			developmentGenerationUsage.Delete(key)
		}
		return true
	})
	return postsCleared, personaFactsCleared, true
}
