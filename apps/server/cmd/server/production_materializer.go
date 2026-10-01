package main

import (
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/worldrepo"
)

// Live world generation receives World facts and board context, not a
// preselected catalog of historical topics. Historical evidence experiments
// remain explicitly selectable in Materialization Lab.
func newProductionMaterializer(renderer llm.BoardPostRenderer) worldrepo.LLMMaterializer {
	return worldrepo.LLMMaterializer{
		Renderer:              renderer,
		Fallback:              worldrepo.FallbackMaterializer{},
		ModelHistoricalMemory: true,
	}
}
