package llm

import "context"

// DebugTraceStart is diagnostic-only. It is supplied by the caller for one
// Evaluation operation; production calls without it do not retain
// prompts or responses. Never add provider secrets or HTTP headers here.
type DebugTraceStart func(stage, prompt string) func(response string, err error)

type debugTraceContextKey struct{}

func WithDebugTrace(ctx context.Context, start DebugTraceStart) context.Context {
	if start == nil {
		return ctx
	}
	return context.WithValue(ctx, debugTraceContextKey{}, start)
}

func beginDebugTrace(ctx context.Context, stage, prompt string) func(string, error) {
	if start, ok := ctx.Value(debugTraceContextKey{}).(DebugTraceStart); ok && start != nil {
		return start(stage, prompt)
	}
	return func(string, error) {}
}
