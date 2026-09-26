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
	ReplyToSlotIndex  int
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

// ReplyRepresentation is the host-program projection of a semantic response.
// RespondsToPostID remains canonical world causality; ParentID/Subject describe
// only how that host software stores or exposes the response as an article.
type ReplyRepresentation struct {
	ParentID int64
	Subject  string
}

// ReplyProjector keeps host-specific article/reply semantics out of the shared
// world engine. Implementations may model append-without-subject, Re:-style
// threaded replies, flat response messages with their own subject, etc.
type ReplyProjector interface {
	ProjectReply(host world.Host, source world.Post, proposedSubject string) (ReplyRepresentation, error)
}

type ReplyProjectorFunc func(host world.Host, source world.Post, proposedSubject string) (ReplyRepresentation, error)

func (f ReplyProjectorFunc) ProjectReply(host world.Host, source world.Post, proposedSubject string) (ReplyRepresentation, error) {
	return f(host, source, proposedSubject)
}

type postReplacer interface {
	ReplaceHostPosts(hostID string, posts []world.Post) int
}

type personaSource interface {
	ListHostPersonas(hostID string) []world.Persona
}

type Engine struct {
	Store          world.Store
	Planner        BatchPlanner
	ReplyProjector ReplyProjector
	Now            func() time.Time
	Cadence        time.Duration
	RecentLimit    int
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
	return e.catchUp(ctx, host, board, false, 0, 0, -1)
}

// CatchUpInitial materializes one batch immediately when this board has no
// shared-engine generated history yet. This is used only by explicit development
// reset flows so testers can inspect the current generator without waiting for the
// normal world-time cadence. Once a batch exists, ordinary cadence rules apply.
func (e *Engine) CatchUpInitial(ctx context.Context, host world.Host, board world.Board) error {
	return e.catchUpInitial(ctx, host, board, 0, 0, -1)
}

// CatchUpInitialCount is the explicit-development variant of CatchUpInitial.
// It lets generator-evaluation fixtures request a larger first materialization
// without changing normal world cadence or MaxBatchSize.
func (e *Engine) CatchUpInitialCount(ctx context.Context, host world.Host, board world.Board, count int) error {
	return e.catchUpInitial(ctx, host, board, count, 0, -1)
}

// CatchUpInitialRootHistoryCount is the explicit-development history variant.
// rootCount is the number of board-index roots to materialize; reply events are
// added on top of that target. lookback spreads the simulated activity through
// the recent past instead of compressing a large evaluation sample into one
// six-hour cadence window.
func (e *Engine) CatchUpInitialRootHistoryCount(ctx context.Context, host world.Host, board world.Board, rootCount int, lookback time.Duration) error {
	if rootCount <= 0 {
		return nil
	}
	return e.catchUpInitial(ctx, host, board, slotCountForRootTarget(rootCount), lookback, -1)
}

// CatchUpInitialBoardActivity realizes the already-decided retained activity
// window for a board. The world plan fixes root/reply counts before wording; the
// planner only materializes titles and later bodies for those slots.
func (e *Engine) CatchUpInitialBoardActivity(ctx context.Context, host world.Host, board world.Board, state world.BoardActivityState) error {
	if state.RetainedRoots <= 0 {
		return nil
	}
	replyCount := state.RetainedReplies
	if replyCount < 0 {
		replyCount = 0
	}
	count := state.RetainedRoots + replyCount
	if count <= 0 {
		return nil
	}
	lookback := time.Duration(0)
	now := e.currentTime()
	if !state.RetainedSince.IsZero() && state.RetainedSince.Before(now) {
		lookback = now.Sub(state.RetainedSince)
	}
	return e.catchUpInitial(ctx, host, board, count, lookback, replyCount)
}

func slotCountForRootTarget(rootCount int) int {
	if rootCount <= 0 {
		return 0
	}
	slots, roots := 0, 0
	for roots < rootCount {
		slots++
		// planSlots uses every fourth event as a reply once an earlier root exists.
		if slots%4 != 0 {
			roots++
		}
	}
	return slots
}

func (e *Engine) catchUpInitial(ctx context.Context, host world.Host, board world.Board, count int, initialLookback time.Duration, desiredReplies int) error {
	if e == nil || e.Store == nil {
		return nil
	}
	boardPosts := filterBoard(e.Store.ListPosts(host.ID), board.ID)
	for _, post := range boardPosts {
		if post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup {
			return e.catchUp(ctx, host, board, false, 0, 0, -1)
		}
	}
	return e.catchUp(ctx, host, board, true, count, initialLookback, desiredReplies)
}

func (e *Engine) catchUp(ctx context.Context, host world.Host, board world.Board, ignoreCadence bool, initialCount int, initialLookback time.Duration, desiredReplies int) error {
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
	var initialStart time.Time
	if ignoreCadence && cursor.IsZero() && initialLookback > 0 {
		initialStart = now.Add(-initialLookback)
	}
	slots := e.planSlots(host, board, recent, cursor, now, count, initialStart, desiredReplies)
	if len(slots) == 0 {
		return nil
	}
	since := cursor
	if since.IsZero() && !initialStart.IsZero() {
		since = initialStart
	}

	planned, err := e.Planner.PlanBBSBatch(ctx, BatchRequest{
		Host:        host,
		Board:       board,
		WorldNow:    now,
		Since:       since,
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

	slotDefs := make(map[int]Slot, len(slots))
	for _, slot := range slots {
		slotDefs[slot.Index] = slot
	}
	for _, slot := range slots {
		if slot.ReplyToSlotIndex == 0 {
			continue
		}
		target, ok := slotDefs[slot.ReplyToSlotIndex]
		if !ok || target.Index >= slot.Index || target.ReplyToPostID != 0 || target.ReplyToSlotIndex != 0 {
			return fmt.Errorf("bbs article batch slot %d has invalid in-batch reply target %d", slot.Index, slot.ReplyToSlotIndex)
		}
	}

	if batchStore, ok := e.Store.(world.PostBatchStore); ok {
		batchStore.BeginPostBatch(host.ID)
		defer batchStore.EndPostBatch(host.ID)
	}

	persistedBySlot := make(map[int]world.Post, len(slots))
	for _, slot := range slots {
		draft, ok := bySlot[slot.Index]
		if !ok {
			return fmt.Errorf("bbs article batch omitted slot %d", slot.Index)
		}
		responseToID := slot.ReplyToPostID
		responseSource := world.Post{
			ID:      slot.ReplyToPostID,
			BoardID: board.ID,
			Author:  slot.ReplyToAuthor,
			Subject: slot.ReplyToSubject,
		}
		if slot.ReplyToSlotIndex != 0 {
			target, ok := persistedBySlot[slot.ReplyToSlotIndex]
			if !ok {
				return fmt.Errorf("bbs article batch slot %d target %d was not persisted first", slot.Index, slot.ReplyToSlotIndex)
			}
			responseToID = target.ID
			responseSource = target
		} else if responseToID != 0 {
			for _, existing := range e.Store.ListPosts(host.ID) {
				if existing.ID == responseToID {
					responseSource = existing
					break
				}
			}
		}

		subject := strings.TrimSpace(draft.Subject)
		parentID := int64(0)
		isReply := responseToID != 0
		if isReply {
			if e.ReplyProjector != nil {
				projected, err := e.ReplyProjector.ProjectReply(host, responseSource, subject)
				if err != nil {
					return fmt.Errorf("bbs article batch slot %d reply projection: %w", slot.Index, err)
				}
				parentID = projected.ParentID
				subject = strings.TrimSpace(projected.Subject)
			} else {
				// Low-level/test compatibility: preserve the semantic relationship
				// without inventing a textual Re: convention. Production repositories
				// install a host-program projector.
				parentID = responseToID
			}
		}
		if (!isReply && subject == "") || strings.TrimSpace(draft.SituationSummary) == "" {
			return fmt.Errorf("bbs article batch slot %d is incomplete", slot.Index)
		}

		situationFacts := []string{
			"world_adoption=title_candidate",
			"world_adopted_summary=" + draft.SituationSummary,
		}
		if subject != "" {
			situationFacts = append(situationFacts,
				"title_first_subject="+subject,
				"subject_contract=Keep the adopted title verbatim. Do not replace it with a different topic.",
			)
		}
		if isReply {
			situationFacts = append(situationFacts, fmt.Sprintf("responds_to_post_id=%d", responseToID))
		}

		saved := e.Store.AddPost(host.ID, world.Post{
			BoardID:         board.ID,
			ParentID:        parentID,
			Author:          slot.Author,
			AuthorPersonaID: slot.AuthorPersonaID,
			Subject:         subject,
			Body:            "",
			Intent: world.PostIntent{
				Action:           ActionWorldCatchup,
				CauseKind:        "board_activity_window",
				DiscourseMode:    func() string { if isReply { return "reply" }; return "thread_start" }(),
				SourcePostID:     responseToID,
				SituationKind:    "title_first",
				SituationSummary: draft.SituationSummary,
				SituationFacts:   situationFacts,
				Topic:            draft.Topic,
				Motivation:       draft.Motivation,
				Stance:           draft.Stance,
				Goal:             draft.Goal,
				Claims:           append([]string(nil), draft.Claims...),
				RespondsToPostID: responseToID,
			},
			CreatedAt: slot.CreatedAt,
		})
		persistedBySlot[slot.Index] = saved
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

func (e *Engine) planSlots(host world.Host, board world.Board, recent []world.Post, cursor, now time.Time, count int, initialStart time.Time, desiredReplies int) []Slot {
	if count <= 0 {
		return nil
	}
	actors := e.actorRoster(host, board, recent, now)
	type replyTarget struct {
		postID    int64
		slotIndex int
		subject   string
		author    string
	}
	targets := make([]replyTarget, 0)
	for _, post := range recent {
		if world.IsSemanticRoot(post) {
			targets = append(targets, replyTarget{postID: post.ID, subject: post.Subject, author: post.Author})
		}
	}

	start := cursor
	if cursor.IsZero() && !initialStart.IsZero() {
		start = initialStart
	}
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
		nominal := start.Add(step * time.Duration(i+1))
		// An even spacing is useful for allocating a bounded window but looks
		// artificial when exposed as historical timestamps. Apply deterministic
		// bounded jitter inside each interval. +/-30% still leaves at least 40%
		// of one step between adjacent slots, so chronological ordering is stable.
		maxJitter := step * 3 / 10
		jitterUnit := int64(stableHash(fmt.Sprintf("%s|%s|%d|slot-time-v1", host.ID, board.ID, i+1))%2001) - 1000
		createdAt := nominal.Add(time.Duration(int64(maxJitter) * jitterUnit / 1000))
		slot := Slot{
			Index:           i + 1,
			Author:          a.handle,
			AuthorPersonaID: a.personaID,
			CreatedAt:       createdAt,
		}
		// Earlier roots in this same simulated window are valid reply targets even
		// though their database IDs are assigned only during chronological commit.
		// A preplanned board-history window may request an exact reply count;
		// ordinary cadence batches retain the older one-in-four heuristic.
		shouldReply := false
		if len(targets) > 0 {
			if desiredReplies >= 0 {
				if desiredReplies > count-1 {
					desiredReplies = count - 1
				}
				repliesSoFar := 0
				for _, prior := range slots {
					if prior.ReplyToPostID != 0 || prior.ReplyToSlotIndex != 0 {
						repliesSoFar++
					}
				}
				if i > 0 && desiredReplies > 0 {
					expectedByNow := (i * desiredReplies) / (count - 1)
					shouldReply = expectedByNow > repliesSoFar
				}
			} else {
				shouldReply = (i+1)%4 == 0
			}
		}
		if shouldReply {
			target := targets[int((seed+uint64(i))%uint64(len(targets)))]
			slot.ReplyToPostID = target.postID
			slot.ReplyToSlotIndex = target.slotIndex
			slot.ReplyToSubject = target.subject
			slot.ReplyToAuthor = target.author
			if strings.EqualFold(slot.Author, target.author) && len(actors) > 1 {
				a = actors[(actorIndex+1)%len(actors)]
				slot.Author = a.handle
				slot.AuthorPersonaID = a.personaID
			}
		}
		slots = append(slots, slot)
		if slot.ReplyToPostID == 0 && slot.ReplyToSlotIndex == 0 {
			targets = append(targets, replyTarget{slotIndex: slot.Index, author: slot.Author})
		}
	}
	return slots
}

func stableHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
