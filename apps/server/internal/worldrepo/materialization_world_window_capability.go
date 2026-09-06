package worldrepo

import "zutto-pccom/apps/server/internal/llm"

// developmentWorldWindowAvailable distinguishes a materializer that merely has
// the LLMMaterializer method set from one whose configured renderer can actually
// perform the producer pass. This keeps older test/fake renderers on the legacy
// board-local planner while production StructuredOpenAIProvider uses the new path.
func developmentWorldWindowAvailable(materializer Materializer) bool {
	switch m := materializer.(type) {
	case LLMMaterializer:
		_, ok := m.Renderer.(llm.BBSWorldWindowProducer)
		return ok
	case *LLMMaterializer:
		_, ok := m.Renderer.(llm.BBSWorldWindowProducer)
		return ok
	default:
		_, ok := materializer.(developmentWorldWindowPlanner)
		return ok
	}
}
