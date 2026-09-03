package worldrepo

import (
	"context"
	"errors"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// FallbackMaterializer remains only so older wiring/tests compile while the
// prototype migrates. It intentionally contains no canned content and never
// commits synthetic fallback prose.
type FallbackMaterializer struct{}

func (FallbackMaterializer) GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error) {
	return nil, errors.New("canned fallback materialization has been removed")
}
