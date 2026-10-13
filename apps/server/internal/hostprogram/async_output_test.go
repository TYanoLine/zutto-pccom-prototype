package hostprogram

import "zutto-pccom/apps/server/internal/hostprogram/erikak"

// Erika-K shows chat room joins and leaves without input (spec 016).
var _ AsyncOutput = (*erikak.Runtime)(nil)
