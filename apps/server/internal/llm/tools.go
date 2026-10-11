package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ToolPropertySchema represents a property in JSON schema.
type ToolPropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolParameterSchema defines the JSON schema for tool arguments.
type ToolParameterSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]ToolPropertySchema `json:"properties,omitempty"`
	Required   []string                      `json:"required,omitempty"`
}

// ToolDefinition defines a generic callable tool presented to an LLM.
type ToolDefinition struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  ToolParameterSchema `json:"parameters"`
}

// ToolCall represents a request from the model to execute a tool.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the outcome of executing a ToolCall.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ActorScopedToolExecutor executes a tool strictly within the calling actor's observed scope.
type ActorScopedToolExecutor interface {
	Execute(ctx context.Context, actorID string, call ToolCall) (ToolResult, error)
}

// ToolExecutionPolicy bounds the tool calling loop for predictable latency and responsiveness.
type ToolExecutionPolicy struct {
	// MaxToolCallsPerTurn limits how many round-trips of tool calls can be made per generation request.
	// For interactive responsiveness, this is bounded to 1 round-trip.
	MaxToolCallsPerTurn int
}

// DefaultToolExecutionPolicy provides the recommended bounding configuration.
func DefaultToolExecutionPolicy() ToolExecutionPolicy {
	return ToolExecutionPolicy{
		MaxToolCallsPerTurn: 1,
	}
}

// Standard tool names for actor memory retrieval.
const (
	ToolGetMyRecentPosts          = "get_my_recent_posts"
	ToolGetThreadContext          = "get_thread_context"
	ToolGetInteractionHistoryWith = "get_interaction_history_with"
	ToolSearchMyObservedEvents    = "search_my_observed_events"
	ToolGetMyOpenLoops            = "get_my_open_loops"
	ToolGetMyProfileFacts         = "get_my_profile_facts"
)

// StandardActorTools returns the specification of generic actor-scoped memory tools.
func StandardActorTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolGetMyRecentPosts,
			Description: "自分が直近に掲示板に書いた投稿（記事番号・板・件名・本文抜粋・日時）を取得します。相手から直前の投稿や設定内容について具体的に質問された時のみ使用してください。通常の返信や雑談では呼ばないでください。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"board_id": {
						Type:        "string",
						Description: "特定の板に限定する場合の板ID（省略時はすべての板から直近分を取得）",
					},
					"limit": {
						Type:        "integer",
						Description: "取得件数の上限（1〜5件、デフォルトは3件）",
					},
				},
			},
		},
		{
			Name:        ToolGetThreadContext,
			Description: "特定のスレッドにおけるこれまでの投稿の流れ（親記事および直前のレス）を取得します。スレッドの文脈を確認する必要がある場合のみ使用してください。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"thread_root_id": {
						Type:        "integer",
						Description: "スレッドの親記事（ルート記事）の番号",
					},
					"limit": {
						Type:        "integer",
						Description: "取得件数の上限（1〜5件、デフォルトは3件）",
					},
				},
				Required: []string{"thread_root_id"},
			},
		},
		{
			Name:        ToolGetInteractionHistoryWith,
			Description: "特定の相手（人間会員や他の住民）との直近のやり取り（チャット発言、メール、掲示板での返信）を取得します。相手から以前の会話内容について具体的に言及された時のみ使用してください。聞かれていない過去ログを持ち出すための呼び出しは禁止です。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"counterpart_id": {
						Type:        "string",
						Description: "やり取り相手のID（人間の会員IDまたはペルソナID）",
					},
					"channel_filter": {
						Type:        "string",
						Description: "チャネル種別（'chat', 'mail', 'board'、省略時は全チャネル）",
					},
					"limit": {
						Type:        "integer",
						Description: "取得件数の上限（1〜5件、デフォルトは3件）",
					},
				},
				Required: []string{"counterpart_id"},
			},
		},
		{
			Name:        ToolSearchMyObservedEvents,
			Description: "自分が観測した掲示板記事、参加したチャット発言、受信したメールから、キーワードや話題で検索します。相手から特定のゲーム名や話題について以前のやり取りを具体的に尋ねられた時のみ使用してください。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"query": {
						Type:        "string",
						Description: "検索キーワード（ゲーム名、機種名、話題など）",
					},
					"channel_filter": {
						Type:        "string",
						Description: "チャネル種別（'chat', 'mail', 'board'、省略時は全チャネル）",
					},
					"limit": {
						Type:        "integer",
						Description: "取得件数の上限（1〜5件、デフォルトは3件）",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        ToolGetMyOpenLoops,
			Description: "自分が現在抱えている未決事項（自分宛ての質問、未返信のメール、約束、継続中の議論など）の一覧を取得します。通常の返信では使用しないでください。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"limit": {
						Type:        "integer",
						Description: "取得件数の上限（1〜5件、デフォルトは3件）",
					},
				},
			},
		},
		{
			Name:        ToolGetMyProfileFacts,
			Description: "自分の確定した人物事実（PersonaFact：所有しているパソコン機種、OS、居住地域、嗜好など）を取得します。自分の環境について過去の発言と矛盾を防ぐ必要がある場合のみ使用してください。",
			Parameters: ToolParameterSchema{
				Type: "object",
				Properties: map[string]ToolPropertySchema{
					"topic": {
						Type:        "string",
						Description: "特定のトピック分類（'computer', 'hobby', 'location' 等、省略時は直近の確定事実一覧）",
					},
				},
			},
		},
	}
}

// ToolRegistry holds tool definitions and routes execution to handlers.
type ToolRegistry struct {
	tools    map[string]ToolDefinition
	executor ActorScopedToolExecutor
}

// NewToolRegistry creates a new ToolRegistry with definitions and an executor.
func NewToolRegistry(tools []ToolDefinition, executor ActorScopedToolExecutor) *ToolRegistry {
	m := make(map[string]ToolDefinition, len(tools))
	for _, t := range tools {
		m[t.Name] = t
	}
	return &ToolRegistry{
		tools:    m,
		executor: executor,
	}
}

// Definitions returns the slice of tool definitions for presenting to an LLM.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	out := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Execute safely runs a tool call against the executor.
func (r *ToolRegistry) Execute(ctx context.Context, actorID string, call ToolCall) ToolResult {
	if _, ok := r.tools[call.Name]; !ok {
		return ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    fmt.Sprintf("unknown tool: %s", call.Name),
			IsError:    true,
		}
	}
	if r.executor == nil {
		return ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    "tool executor not configured",
			IsError:    true,
		}
	}
	res, err := r.executor.Execute(ctx, actorID, call)
	if err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    err.Error(),
			IsError:    true,
		}
	}
	if res.ToolCallID == "" {
		res.ToolCallID = call.ID
	}
	if res.Name == "" {
		res.Name = call.Name
	}
	return res
}

// MockToolExecutor is a test helper and fallback executor.
type MockToolExecutor struct {
	Handlers map[string]func(ctx context.Context, actorID string, args json.RawMessage) (string, error)
}

func (m *MockToolExecutor) Execute(ctx context.Context, actorID string, call ToolCall) (ToolResult, error) {
	if m.Handlers == nil {
		return ToolResult{}, errors.New("no handlers registered")
	}
	fn, ok := m.Handlers[call.Name]
	if !ok {
		return ToolResult{}, fmt.Errorf("unhandled tool: %s", call.Name)
	}
	content, err := fn(ctx, actorID, call.Arguments)
	if err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Content:    err.Error(),
			IsError:    true,
		}, nil
	}
	return ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    content,
	}, nil
}
