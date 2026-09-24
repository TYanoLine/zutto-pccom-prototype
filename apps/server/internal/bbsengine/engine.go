package bbsengine

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// ActionWorldCatchup marks canonical posts materialized by the shared BBS world
// catch-up engine. Host programs only render these posts; they do not own the
// policy that decides when residents write or what a board's recent flow is.
const ActionWorldCatchup = "bbs-world-catchup"

// legacyActionWorldCatchup was used by the temporary Erika-K-only catch-up.
// Treat it as shared-engine generated history for cursor/reset migration only.
const legacyActionWorldCatchup = "world-catchup"

const (
	DefaultCadence     = 6 * time.Hour
	DefaultRecentLimit = 48
	MaxBatchSize       = 7
)

type Slot struct {
	Index             int
	Author            string
	AuthorPersonaID   string
	CreatedAt         time.Time
	ReplyToPostID     int64
	ReplyToSubject    string
	ReplyToAuthor     string
}

type BatchRequest struct {
	Host        world.Host
	Board       world.Board
	WorldNow    time.Time
	Since       time.Time
	RecentPosts []world.Post
	Slots       []Slot
}

type PlannedPost struct {
	SlotIndex        int
	Subject          string
	Topic            string
	Motivation       string
	Stance           string
	Goal             string
	SituationSummary string
	Claims           []string
}

type BatchPlanner interface {
	PlanBBSBatch(context.Context, BatchRequest) ([]PlannedPost, error)
}

type postReplacer interface {
	ReplaceHostPosts(hostID string, posts []world.Post) int
}

type personaSource interface {
	ListHostPersonas(hostID string) []world.Persona
}

type Engine struct {
	Store       world.Store
	Planner     BatchPlanner
	Now         func() time.Time
	Cadence     time.Duration
	RecentLimit int
}

func New(store world.Store, planner BatchPlanner, now func() time.Time) *Engine {
	return &Engine{
		Store:       store,
		Planner:     planner,
		Now:         now,
		Cadence:     DefaultCadence,
		RecentLimit: DefaultRecentLimit,
	}
}

func (e *Engine) currentTime() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Engine) cadence() time.Duration {
	if e.Cadence > 0 {
		return e.Cadence
	}
	return DefaultCadence
}

func (e *Engine) recentLimit() int {
	if e.RecentLimit > 0 {
		return e.RecentLimit
	}
	return DefaultRecentLimit
}

func (e *Engine) NeedsCatchUp(host world.Host, board world.Board) bool {
	if e == nil || e.Store == nil || strings.TrimSpace(host.ID) == "" || strings.TrimSpace(board.ID) == "" {
		return false
	}
	now := e.currentTime()
	cursor := latestCursor(filterBoard(e.Store.ListPosts(host.ID), board.ID))
	if cursor.IsZero() {
		return true
	}
	if !cursor.Before(now) {
		return false
	}
	return now.Sub(cursor) >= e.cadence()
}

// CatchUp materializes one bounded batch of board activity. The world decides
// actor/time/reply topology first; the planner then realizes those already-fixed
// slots together so it can see recent board flow and avoid title-by-title drift.
func (e *Engine) CatchUp(ctx context.Context, host world.Host, board world.Board) error {
	return e.catchUp(ctx, host, board, false, 0)
}

// CatchUpInitial materializes one batch immediately when this board has no
// shared-engine generated history yet. This is used only by explicit development
// reset flows so testers can inspect the current generator without waiting for the
// normal world-time cadence. Once a batch exists, ordinary cadence rules apply.
func (e *Engine) CatchUpInitial(ctx context.Context, host world.Host, board world.Board) error {
	return e.catchUpInitial(ctx, host, board, 0)
}

// CatchUpInitialCount is the explicit-development variant of CatchUpInitial.
// It lets generator-evaluation fixtures request a larger first materialization
// without changing normal world cadence or MaxBatchSize.
func (e *Engine) CatchUpInitialCount(ctx context.Context, host world.Host, board world.Board, count int) error {
	return e.catchUpInitial(ctx, host, board, count)
}

func (e *Engine) catchUpInitial(ctx context.Context, host world.Host, board world.Board, count int) error {
	if e == nil || e.Store == nil {
		return nil
	}
	boardPosts := filterBoard(e.Store.ListPosts(host.ID), board.ID)
	for _, post := range boardPosts {
		if post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup {
			return e.catchUp(ctx, host, board, false, 0)
		}
	}
	return e.catchUp(ctx, host, board, true, count)
}

func (e *Engine) catchUp(ctx context.Context, host world.Host, board world.Board, ignoreCadence bool, initialCount int) error {
	if e == nil || e.Store == nil || e.Planner == nil {
		return nil
	}
	now := e.currentTime()
	boardPosts := filterBoard(e.Store.ListPosts(host.ID), board.ID)
	cursor := latestCursor(boardPosts)
	if !ignoreCadence && !cursor.IsZero() {
		if !cursor.Before(now) || now.Sub(cursor) < e.cadence() {
			return nil
		}
	}

	recent := recentPosts(boardPosts, e.recentLimit())
	count := batchSize(host, cursor, now, e.cadence())
	if ignoreCadence && initialCount > 0 {
		count = initialCount
	}
	slots := e.planSlots(host, board, recent, cursor, now, count)
	if len(slots) == 0 {
		return nil
	}

	planned, err := e.Planner.PlanBBSBatch(ctx, BatchRequest{
		Host:        host,
		Board:       board,
		WorldNow:    now,
		Since:       cursor,
		RecentPosts: recent,
		Slots:       slots,
	})
	if err != nil {
		return err
	}
	if len(planned) != len(slots) {
		return fmt.Errorf("bbs article batch returned %d posts for %d slots", len(planned), len(slots))
	}

	bySlot := make(map[int]PlannedPost, len(planned))
	for _, post := range planned {
		if post.SlotIndex < 1 || post.SlotIndex > len(slots) {
			return fmt.Errorf("bbs article batch returned invalid slot %d", post.SlotIndex)
		}
		if _, exists := bySlot[post.SlotIndex]; exists {
			return fmt.Errorf("bbs article batch returned duplicate slot %d", post.SlotIndex)
		}
		bySlot[post.SlotIndex] = post
	}

	for _, slot := range slots {
		draft, ok := bySlot[slot.Index]
		if !ok {
			return fmt.Errorf("bbs article batch omitted slot %d", slot.Index)
		}
		subject := strings.TrimSpace(draft.Subject)
		if slot.ReplyToPostID != 0 {
			subject = "Re: " + strings.TrimSpace(slot.ReplyToSubject)
		}
		if subject == "" || strings.TrimSpace(draft.SituationSummary) == "" {
			return fmt.Errorf("bbs article batch slot %d is incomplete", slot.Index)
		}
		e.Store.AddPost(host.ID, world.Post{
			BoardID:         board.ID,
			ParentID:        slot.ReplyToPostID,
			Author:          slot.Author,
			AuthorPersonaID: slot.AuthorPersonaID,
			Subject:         subject,
			Body:            "",
			Intent: world.PostIntent{
				Action:           ActionWorldCatchup,
				CauseKind:        "board_activity_window",
				DiscourseMode:    func() string { if slot.ReplyToPostID != 0 { return "reply" }; return "thread_start" }(),
				SourcePostID:     slot.ReplyToPostID,
				SituationKind:    "title_first",
				SituationSummary: draft.SituationSummary,
				SituationFacts: []string{
					"title_first_subject=" + subject,
					"world_adoption=title_candidate",
					"world_adopted_summary=" + draft.SituationSummary,
					"subject_contract=Keep the adopted title verbatim. Do not replace it with a different topic.",
				},
				Topic:            draft.Topic,
				Motivation:       draft.Motivation,
				Stance:           draft.Stance,
				Goal:             draft.Goal,
				Claims:           append([]string(nil), draft.Claims...),
				RespondsToPostID: slot.ReplyToPostID,
			},
			CreatedAt: slot.CreatedAt,
		})
	}
	return nil
}

// ResetGenerated removes only history produced by this engine (plus descendants
// of removed roots). Seed history, user posts and host-program configuration stay.
func (e *Engine) ResetGenerated(hostID string) (removed int, kept int, ok bool) {
	if e == nil || e.Store == nil {
		return 0, 0, false
	}
	replacer, supported := e.Store.(postReplacer)
	if !supported {
		return 0, 0, false
	}
	all := e.Store.ListPosts(hostID)
	removeIDs := map[int64]bool{}
	for _, post := range all {
		if post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup {
			removeIDs[post.ID] = true
		}
	}
	if len(removeIDs) == 0 {
		return 0, len(all), true
	}

	changed := true
	for changed {
		changed = false
		for _, post := range all {
			if post.ParentID != 0 && removeIDs[post.ParentID] && !removeIDs[post.ID] {
				removeIDs[post.ID] = true
				changed = true
			}
		}
	}
	remaining := make([]world.Post, 0, len(all)-len(removeIDs))
	for _, post := range all {
		if !removeIDs[post.ID] {
			remaining = append(remaining, post)
		}
	}
	replacer.ReplaceHostPosts(hostID, remaining)
	return len(removeIDs), len(remaining), true
}

func latestCursor(posts []world.Post) time.Time {
	var latestGenerated time.Time
	var latestBaseline time.Time
	for _, post := range posts {
		if post.CreatedAt.IsZero() {
			continue
		}
		if post.CreatedAt.After(latestBaseline) {
			latestBaseline = post.CreatedAt
		}
		if (post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup) && post.CreatedAt.After(latestGenerated) {
			latestGenerated = post.CreatedAt
		}
	}
	if !latestGenerated.IsZero() {
		return latestGenerated
	}
	return latestBaseline
}

func filterBoard(posts []world.Post, boardID string) []world.Post {
	out := make([]world.Post, 0)
	for _, post := range posts {
		if post.BoardID == boardID {
			out = append(out, post)
		}
	}
	return out
}

func recentPosts(posts []world.Post, limit int) []world.Post {
	out := append([]world.Post(nil), posts...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func batchSize(host world.Host, cursor, now time.Time, cadence time.Duration) int {
	base := 3 + int(math.Round(host.Popularity*3))
	if base < 3 {
		base = 3
	}
	if base > 6 {
		base = 6
	}
	if !cursor.IsZero() && cadence > 0 {
		windows := int(now.Sub(cursor) / cadence)
		if windows >= 3 {
			base++
		}
	}
	if base > MaxBatchSize {
		base = MaxBatchSize
	}
	return base
}

type actor struct {
	handle    string
	personaID string
}

func (e *Engine) actorRoster(host world.Host, board world.Board, recent []world.Post, now time.Time) []actor {
	type rankedActor struct {
		actor
		score float64
	}
	recentAuthor := map[string]bool{}
	for _, post := range recent {
		if h := strings.ToUpper(strings.TrimSpace(post.Author)); h != "" {
			recentAuthor[h] = true
		}
	}

	ranked := make([]rankedActor, 0, host.Members)
	seen := map[string]bool{}
	if source, ok := e.Store.(personaSource); ok {
		for _, persona := range source.ListHostPersonas(host.ID) {
			handle := strings.TrimSpace(persona.Handle)
			key := strings.ToUpper(handle)
			if handle == "" || seen[key] {
				continue
			}
			seen[key] = true
			activity := personaActivityWeight(persona.ActivityPattern, persona.LurkerTendency)
			window := fmt.Sprintf("%s|%s|%s|%02d", host.ID, board.ID, now.Format("2006-01-02"), now.Hour()/6)
			jitter := float64(stableHash(window+"|"+persona.ID)%10000) / 10000.0
			score := .78*activity + .22*jitter
			if recentAuthor[key] {
				score += .10
			}
			ranked = append(ranked, rankedActor{
				actor: actor{handle: handle, personaID: persona.ID},
				score: score,
			})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].handle < ranked[j].handle
		}
		return ranked[i].score > ranked[j].score
	})
	target := activeActorTarget(host.Members, len(ranked))
	out := make([]actor, 0, target+8)
	for i := 0; i < target && i < len(ranked); i++ {
		out = append(out, ranked[i].actor)
	}

	// A recently visible author should not vanish from an ongoing conversation
	// merely because they missed this window's activity cutoff.
	outSeen := map[string]bool{}
	for _, a := range out {
		outSeen[strings.ToUpper(a.handle)] = true
	}
	for i := len(recent) - 1; i >= 0; i-- {
		handle := strings.TrimSpace(recent[i].Author)
		key := strings.ToUpper(handle)
		if handle == "" || key == "GUEST" || key == "USER" || outSeen[key] {
			continue
		}
		outSeen[key] = true
		out = append(out, actor{handle: handle, personaID: recent[i].AuthorPersonaID})
	}
	if len(out) == 0 {
		out = append(out, actor{handle: "SYSOP"})
	}
	return out
}

func activeActorTarget(members, available int) int {
	if available <= 0 {
		return 0
	}
	if members <= 0 || members > available {
		members = available
	}
	// This is a simulation heuristic, not a claimed historical statistic. It
	// represents the pool plausibly active in the current board/time window;
	// only a much smaller subset will actually write in a materialized batch.
	target := int(math.Round(float64(members) * .18))
	if target < 12 {
		target = 12
	}
	if target > 72 {
		target = 72
	}
	if target > available {
		target = available
	}
	return target
}

func personaActivityWeight(pattern string, lurker float64) float64 {
	var base float64
	switch strings.ToLower(strings.TrimSpace(pattern)) {
	case "regular":
		base = 1.00
	case "active":
		base = .82
	case "occasional":
		base = .56
	case "lurker":
		base = .30
	case "dormant":
		base = .10
	default:
		base = .48
	}
	if lurker > 0 {
		base *= 1 - .35*math.Min(1, lurker)
	}
	return base
}

func (e *Engine) planSlots(host world.Host, board world.Board, recent []world.Post, cursor, now time.Time, count int) []Slot {
	if count <= 0 {
		return nil
	}
	actors := e.actorRoster(host, board, recent, now)
	roots := make([]world.Post, 0)
	for _, post := range recent {
		if post.ParentID == 0 {
			roots = append(roots, post)
		}
	}

	start := cursor
	if start.IsZero() || !start.Before(now) {
		start = now.Add(-e.cadence())
	}
	span := now.Sub(start)
	if span <= 0 {
		span = e.cadence()
		start = now.Add(-span)
	}
	step := span / time.Duration(count+1)
	if step <= 0 {
		step = time.Minute
	}

	seed := stableHash(host.ID + "|" + board.ID + "|" + start.UTC().Format(time.RFC3339))
	slots := make([]Slot, 0, count)
	for i := 0; i < count; i++ {
		actorIndex := int((seed + uint64(i*3+1)) % uint64(len(actors)))
		a := actors[actorIndex]
		slot := Slot{
			Index:           i + 1,
			Author:          a.handle,
			AuthorPersonaID: a.personaID,
			CreatedAt:       start.Add(step * time.Duration(i+1)),
		}
		// Keep most events as new roots, but let the shared world engine create
		// ordinary resident-to-resident replies as canonical topology too.
		if len(roots) > 0 && (i+1)%4 == 0 {
			target := roots[int((seed+uint64(i))%uint64(len(roots)))]
			slot.ReplyToPostID = target.ID
			slot.ReplyToSubject = target.Subject
			slot.ReplyToAuthor = target.Author
			if strings.EqualFold(slot.Author, target.Author) && len(actors) > 1 {
				a = actors[(actorIndex+1)%len(actors)]
				slot.Author = a.handle
				slot.AuthorPersonaID = a.personaID
			}
		}
		slots = append(slots, slot)
	}
	return slots
}

func stableHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
