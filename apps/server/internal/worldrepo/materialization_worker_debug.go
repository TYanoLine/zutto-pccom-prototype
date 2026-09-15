package worldrepo

import "zutto-pccom/apps/server/internal/world"

func DebugIntentSummary(intent world.PostIntent) string { return intentSummary(intent) }
