package actormemory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

const (
	defaultLimit    = 3
	maxLimit        = 5
	excerptRunes    = 160
	historyScanSize = 500
	maxProfileFacts = 10

	msgNothing        = "該当する記録はありません。"
	msgNoOpenLoops    = "未決事項はありません。"
	msgSearchDisabled = "検索は現在利用できません。"
	msgNoFacts        = "確定した人物事実はありません。"
)

// Source is the read-only slice of the world store the tools need.
type Source interface {
	ListPosts(hostID string) []world.Post
	ListInteractionEvents(hostID string, limit int) []world.InteractionEvent
	ListPersonaFacts(personaID string) []world.PersonaFact
}

var _ Source = (*world.MemoryStore)(nil)

// Executor answers the standard actor memory tools for one host. Every answer is
// restricted to what the calling actor may observe, and no generation LLM is used.
type Executor struct {
	hostID string
	src    Source
	idx    *Index
}

var _ llm.ActorScopedToolExecutor = (*Executor)(nil)

// NewExecutor builds an executor for one host. idx may be nil: the search tool
// then reports that search is unavailable, and the other tools are unaffected.
func NewExecutor(hostID string, src Source, idx *Index) *Executor {
	return &Executor{hostID: hostID, src: src, idx: idx}
}

type toolArgs struct {
	BoardID       string `json:"board_id"`
	Limit         int    `json:"limit"`
	ThreadRootID  int64  `json:"thread_root_id"`
	CounterpartID string `json:"counterpart_id"`
	ChannelFilter string `json:"channel_filter"`
	Query         string `json:"query"`
	Topic         string `json:"topic"`
}

// Execute runs one tool call for actorID, which is a world.ActorRef.String().
func (e *Executor) Execute(ctx context.Context, actorID string, call llm.ToolCall) (llm.ToolResult, error) {
	actor, err := parseActor(actorID)
	if err != nil {
		return llm.ToolResult{}, err
	}
	var args toolArgs
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return llm.ToolResult{}, fmt.Errorf("invalid arguments for %s: %w", call.Name, err)
		}
	}
	limit := clampLimit(args.Limit)

	var content string
	switch call.Name {
	case llm.ToolGetMyRecentPosts:
		content = e.recentPosts(actor, args.BoardID, limit)
	case llm.ToolGetThreadContext:
		if args.ThreadRootID <= 0 {
			return llm.ToolResult{}, fmt.Errorf("%s requires thread_root_id", call.Name)
		}
		content = e.threadContext(args.ThreadRootID, limit)
	case llm.ToolGetInteractionHistoryWith:
		if strings.TrimSpace(args.CounterpartID) == "" {
			return llm.ToolResult{}, fmt.Errorf("%s requires counterpart_id", call.Name)
		}
		content = e.interactionHistory(actor, args.CounterpartID, args.ChannelFilter, limit)
	case llm.ToolSearchMyObservedEvents:
		if strings.TrimSpace(args.Query) == "" {
			return llm.ToolResult{}, fmt.Errorf("%s requires query", call.Name)
		}
		content = e.search(ctx, actor, args.Query, args.ChannelFilter, limit)
	case llm.ToolGetMyOpenLoops:
		content = msgNoOpenLoops
	case llm.ToolGetMyProfileFacts:
		content = e.profileFacts(actor, args.Topic)
	default:
		return llm.ToolResult{}, fmt.Errorf("unhandled tool: %s", call.Name)
	}
	return llm.ToolResult{ToolCallID: call.ID, Name: call.Name, Content: content}, nil
}

func (e *Executor) recentPosts(actor world.ActorRef, boardID string, limit int) string {
	posts := e.src.ListPosts(e.hostID)
	var mine []world.Post
	for _, p := range posts {
		if postActor(p) != actor {
			continue
		}
		if boardID != "" && p.BoardID != boardID {
			continue
		}
		mine = append(mine, p)
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].ID > mine[j].ID })
	if len(mine) > limit {
		mine = mine[:limit]
	}
	return formatPosts(mine)
}

func (e *Executor) threadContext(anyID int64, limit int) string {
	posts := e.src.ListPosts(e.hostID)
	byID := make(map[int64]world.Post, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	if _, ok := byID[anyID]; !ok {
		return "そのスレッドは見つかりません。"
	}
	rootID := rootOf(byID, anyID)

	var rest []world.Post
	for _, p := range posts {
		if p.ID != rootID && rootOf(byID, p.ID) == rootID {
			rest = append(rest, p)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].ID < rest[j].ID })
	if keep := limit - 1; len(rest) > keep {
		rest = rest[len(rest)-keep:]
	}
	return formatPosts(append([]world.Post{byID[rootID]}, rest...))
}

func (e *Executor) interactionHistory(actor world.ActorRef, counterpart, channelFilter string, limit int) string {
	if !boardChannelAllowed(channelFilter) {
		return msgNothing
	}
	posts := e.src.ListPosts(e.hostID)
	byID := make(map[int64]world.Post, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	var lines []string
	for _, ev := range e.src.ListInteractionEvents(e.hostID, historyScanSize) {
		if !isBoardChannel(ev.Channel) || !between(ev, actor, counterpart) {
			continue
		}
		line := "[" + ev.Channel + "] "
		if id, err := strconv.ParseInt(ev.ContentRef, 10, 64); err == nil {
			if p, ok := byID[id]; ok {
				line += formatPost(p)
			} else {
				line += "#" + ev.ContentRef
			}
		} else {
			line += ev.ContentRef
		}
		lines = append(lines, line)
		if len(lines) >= limit {
			break
		}
	}
	if len(lines) == 0 {
		return msgNothing
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) search(ctx context.Context, actor world.ActorRef, query, channelFilter string, limit int) string {
	if !boardChannelAllowed(channelFilter) {
		return msgNothing
	}
	hits, err := e.idx.Search(ctx, SearchQuery{
		HostID:  e.hostID,
		Text:    query,
		Limit:   limit,
		Visible: visibleTo(e.hostID, actor),
	})
	if err != nil {
		return msgSearchDisabled
	}
	if len(hits) == 0 {
		return msgNothing
	}
	lines := make([]string, 0, len(hits))
	for _, h := range hits {
		lines = append(lines, formatItem(h.Item))
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) profileFacts(actor world.ActorRef, topic string) string {
	if actor.Kind != "persona" {
		return msgNoFacts
	}
	facts := e.src.ListPersonaFacts(actor.ID)
	var picked []world.PersonaFact
	for _, f := range facts {
		if topic != "" && !strings.EqualFold(f.Topic, topic) {
			continue
		}
		picked = append(picked, f)
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].MaterializedAt.After(picked[j].MaterializedAt) })
	if len(picked) > maxProfileFacts {
		picked = picked[:maxProfileFacts]
	}
	if len(picked) == 0 {
		return msgNoFacts
	}
	lines := make([]string, 0, len(picked))
	for _, f := range picked {
		lines = append(lines, "- "+f.Key+": "+f.Value)
	}
	return strings.Join(lines, "\n")
}

// visibleTo returns what an actor may observe: public board scopes of its own
// host, and anything it wrote or was addressed in. Other hosts are never visible.
func visibleTo(hostID string, actor world.ActorRef) func(Item) bool {
	return func(it Item) bool {
		if it.HostID != hostID {
			return false
		}
		if strings.HasPrefix(it.Scope, "public_board:") {
			return true
		}
		if it.Author == actor {
			return true
		}
		for _, a := range it.Addressed {
			if a == actor {
				return true
			}
		}
		return false
	}
}

func parseActor(id string) (world.ActorRef, error) {
	kind, rest, ok := strings.Cut(id, ":")
	if !ok || rest == "" || (kind != "persona" && kind != "login") {
		return world.ActorRef{}, fmt.Errorf("invalid actor id %q (want persona:<id> or login:<id>)", id)
	}
	return world.ActorRef{Kind: kind, ID: rest}, nil
}

// postActor names a post's author as residents know them (spec 019): a
// resident by persona ID, a human by the login ID shown on the post. The author
// is shown to the reader whatever the login ID, guests included; the human ID
// is not a thing residents have.
func postActor(p world.Post) world.ActorRef {
	if p.AuthorPersonaID != "" {
		return world.ActorRef{Kind: world.ActorKindPersona, ID: p.AuthorPersonaID}
	}
	if login := world.PostLogin(p); login != "" {
		return world.ActorRef{Kind: world.ActorKindLogin, ID: login}
	}
	return world.ActorRef{}
}

func parentOf(p world.Post) int64 {
	if p.ParentID != 0 {
		return p.ParentID
	}
	return p.Intent.RespondsToPostID
}

// rootOf climbs reply links to the thread root, guarding against cycles.
func rootOf(byID map[int64]world.Post, id int64) int64 {
	for i := 0; i < 64; i++ {
		parent := parentOf(byID[id])
		if parent == 0 {
			return id
		}
		if _, ok := byID[parent]; !ok {
			return id
		}
		id = parent
	}
	return id
}

func matchesCounterpart(ref world.ActorRef, counterpart string) bool {
	if kind, rest, ok := strings.Cut(counterpart, ":"); ok && (kind == "persona" || kind == "login" || kind == "member") {
		// A human ID (member:<id>) is not something residents know.
		return kind != "member" && ref.Kind == kind && ref.ID == rest
	}
	return ref.ID == counterpart
}

// between reports whether ev is an actor→counterpart or counterpart→actor
// exchange, between the people residents know.
func between(ev world.InteractionEvent, actor world.ActorRef, counterpart string) bool {
	from := ev.KnownActor()
	if from == actor {
		for i := range ev.AddressedTo {
			if matchesCounterpart(ev.KnownAddressed(i), counterpart) {
				return true
			}
		}
	}
	if matchesCounterpart(from, counterpart) {
		for i := range ev.AddressedTo {
			if ev.KnownAddressed(i) == actor {
				return true
			}
		}
	}
	return false
}

func isBoardChannel(ch string) bool { return ch == "board_root" || ch == "board_reply" }

// boardChannelAllowed: only board events exist in the ledger today, so the chat
// and mail filters (and unknown ones) match nothing instead of failing.
func boardChannelAllowed(filter string) bool {
	return filter == "" || strings.EqualFold(filter, "board")
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	}
	return n
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= excerptRunes {
		return s
	}
	return string(r[:excerptRunes]) + "…"
}

func formatPost(p world.Post) string {
	return fmt.Sprintf("#%d [%s] %s (%s) %s: %s",
		p.ID, p.BoardID, p.Subject, p.CreatedAt.Format("2006-01-02 15:04"), p.Author, excerpt(p.Body))
}

func formatPosts(posts []world.Post) string {
	if len(posts) == 0 {
		return msgNothing
	}
	lines := make([]string, 0, len(posts))
	for _, p := range posts {
		lines = append(lines, formatPost(p))
	}
	return strings.Join(lines, "\n")
}

func formatItem(it Item) string {
	return fmt.Sprintf("#%d [%s] %s (%s) %s: %s",
		it.PostID, it.BoardID, it.Subject, it.OccurredAt.Format("2006-01-02 15:04"), it.Author.ID, excerpt(it.Text))
}
