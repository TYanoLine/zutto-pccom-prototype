package bbsengine

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
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
	Host              world.Host
	Board             world.Board
	WorldNow          time.Time
	Since             time.Time
	RecentPosts       []world.Post
	Slots             []Slot
	BaseReplyRate     float64
	RecentReplyCounts []int
	ActiveBySlot      map[int]int
}

type PlannedPost struct {
	SlotIndex                  int
	Subject                    string
	Topic                      string
	Topics                     []string
	Motivation                 string
	Stance                     string
	Goal                       string
	AnchorKey                  string
	DiscourseMode              string
	SituationKind              string
	SituationSummary           string
	SituationFacts             []string
	ArticleDetailsMaterialized bool
	Claims                     []string
	ReplyAppeal                int
	ReplyAppealReason          string
	HasReplyAppeal             bool
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
	RNG            world.Float64Source
	Now              func() time.Time
	Cadence          time.Duration
	RecentLimit      int

	rngMu sync.Mutex
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

type lockedSource struct {
	mu  *sync.Mutex
	src world.Float64Source
}

func (l *lockedSource) Float64() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.src.Float64()
}

func (e *Engine) rng() world.Float64Source {
	if e != nil && e.RNG != nil {
		return &lockedSource{mu: &e.rngMu, src: e.RNG}
	}
	return rand.New(rand.NewSource(time.Now().UnixNano()))
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
	return e.catchUp(ctx, host, board, false, 0, 0, 0, nil, 0)
}

// CatchUpInitial materializes one batch immediately when this board has no
// shared-engine generated history yet. This is used only by explicit development
// reset flows so testers can inspect the current generator without waiting for the
// normal world-time cadence. Once a batch exists, ordinary cadence rules apply.
func (e *Engine) CatchUpInitial(ctx context.Context, host world.Host, board world.Board) error {
	return e.catchUpInitial(ctx, host, board, 0, 0, 0, nil, 0)
}

// CatchUpInitialCount is the explicit-development variant of CatchUpInitial.
// It lets generator-evaluation fixtures request a larger first materialization
// without changing normal world cadence or MaxBatchSize.
func (e *Engine) CatchUpInitialCount(ctx context.Context, host world.Host, board world.Board, count int) error {
	if count <= 0 {
		return nil
	}
	if count == 1 {
		return e.catchUpInitial(ctx, host, board, 1, 0, 0, []int{0}, 0)
	}
	quotas := quotasForTotalSlotTarget(host, board, count)
	return e.catchUpInitial(ctx, host, board, len(quotas), 0, 0, quotas, 0)
}

func quotasForTotalSlotTarget(host world.Host, board world.Board, totalSlots int) []int {
	if totalSlots <= 0 {
		return nil
	}
	replyRate := effectiveBoardReplyRate(board)
	maxRepliesPerRoot := (totalSlots - 1) / 2
	if maxRepliesPerRoot < 1 {
		maxRepliesPerRoot = 1
	}
	var quotas []int
	used := 0
	totalReplies := 0
	for used < totalSlots {
		ordinal := len(quotas) + 1
		used++ // 1 root slot
		remaining := totalSlots - used
		q := 0
		if remaining > 0 && replyRate > 0 {
			q = world.ThreadReplyCount(host.ID, board.ID, ordinal, replyRate)
			if q > maxRepliesPerRoot {
				q = maxRepliesPerRoot
			}
			if q > remaining {
				q = remaining
			}
		}
		quotas = append(quotas, q)
		used += q
		totalReplies += q
	}
	if totalReplies == 0 && len(quotas) >= 2 && replyRate > 0 {
		quotas = quotas[:len(quotas)-1]
		quotas[0] = 1
	}
	return quotas
}

func effectiveBoardReplyRate(board world.Board) float64 {
	if board.RootAuthorPolicy == "sysop_only" && board.ReplyRate <= 0 {
		return 0
	}
	if board.ReplyRate > 0 {
		return board.ReplyRate
	}
	return 1.10
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
	replyRate := effectiveBoardReplyRate(board)
	quotas := make([]int, rootCount)
	for i := range quotas {
		quotas[i] = world.ThreadReplyCount(host.ID, board.ID, i+1, replyRate)
	}
	return e.catchUpInitial(ctx, host, board, rootCount, lookback, 0, quotas, 0)
}

// InteractiveInitialRootLimit bounds the title work required by a first
// board-index observation. The existing canonical BoardActivityState still
// describes the complete retained history; unseen articles are not fabricated.
const InteractiveInitialRootLimit = 10

// CatchUpInitialBoardActivity materializes only enough headers for the current
// 10-line board index: at most InteractiveInitialRootLimit of the newest
// retained roots, each with the replies its own thread drew according to
// qualitative reply appeal. Many threads are unanswered and a few run long, so
// the board looks like a conversation rather than an even trickle. The coarse
// World-selected activity state remains unchanged.
func (e *Engine) CatchUpInitialBoardActivity(ctx context.Context, host world.Host, board world.Board, state world.BoardActivityState) error {
	if state.RetainedRoots <= 0 {
		return nil
	}
	roots := state.RetainedRoots
	if roots > InteractiveInitialRootLimit {
		roots = InteractiveInitialRootLimit
	}
	totalRoots := state.TotalRoots
	if totalRoots < state.RetainedRoots {
		totalRoots = state.RetainedRoots
	}
	lookback := time.Duration(0)
	now := e.currentTime()
	if !state.RetainedSince.IsZero() && state.RetainedSince.Before(now) {
		lookback = now.Sub(state.RetainedSince)
	}
	return e.catchUpInitial(ctx, host, board, roots, lookback, 0, nil, totalRoots-roots+1)
}

func (e *Engine) catchUpInitial(ctx context.Context, host world.Host, board world.Board, count int, initialLookback time.Duration, desiredReplies int, replyQuotas []int, startOrdinal int) error {
	if e == nil || e.Store == nil {
		return nil
	}
	boardPosts := filterBoard(e.Store.ListPosts(host.ID), board.ID)
	for _, post := range boardPosts {
		if (post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup) && post.Intent.CauseKind != "persona_reaction" {
			return e.catchUp(ctx, host, board, false, 0, 0, 0, nil, 0)
		}
	}
	return e.catchUp(ctx, host, board, true, count, initialLookback, desiredReplies, replyQuotas, startOrdinal)
}

// MaterializeDueReactions creates header-only reply posts for any PlannedReaction
// on board whose DueAt <= worldNow and MaterializedPostID == 0. It is idempotent
// and never calls the LLM BatchPlanner.
func (e *Engine) MaterializeDueReactions(host world.Host, board world.Board, now time.Time) int {
	if e == nil || e.Store == nil || strings.TrimSpace(host.ID) == "" || strings.TrimSpace(board.ID) == "" {
		return 0
	}
	planStore, ok := e.Store.(world.ReactionPlanStore)
	if !ok {
		return 0
	}
	plans := planStore.ListReactionPlans(host.ID, board.ID)
	if len(plans) == 0 {
		return 0
	}
	allPosts := e.Store.ListPosts(host.ID)
	postsByID := make(map[int64]world.Post, len(allPosts))
	byEventID := make(map[string]world.Post)
	for _, p := range allPosts {
		postsByID[p.ID] = p
		if p.Intent.ProducerEventID != "" {
			byEventID[p.Intent.ProducerEventID] = p
		}
	}
	personaByID := map[string]world.Persona{}
	for _, p := range e.personasForReactionPlanning(host, board, allPosts, now) {
		personaByID[p.ID] = p
	}

	created := 0
	for _, plan := range plans {
		targetPost, exists := postsByID[plan.TargetPostID]
		if !exists {
			continue
		}
		changed := false
		for i := range plan.Replies {
			rep := &plan.Replies[i]
			if rep.MaterializedPostID != 0 {
				continue
			}
			if rep.DueAt.After(now) {
				continue
			}
			eventID := fmt.Sprintf("react:%d:%d", plan.TargetPostID, rep.Index)
			if existing, dup := byEventID[eventID]; dup {
				rep.MaterializedPostID = existing.ID
				changed = true
				continue
			}

			persona, hasP := personaByID[rep.PersonaID]
			handle := strings.TrimSpace(persona.Handle)
			if !hasP || handle == "" {
				handle = rep.PersonaID
			}

			parentID := targetPost.ID
			subject := ""
			if e.ReplyProjector != nil {
				if projected, err := e.ReplyProjector.ProjectReply(host, targetPost, ""); err == nil {
					parentID = projected.ParentID
					subject = strings.TrimSpace(projected.Subject)
				}
			}

			topics := world.ExtractPostTopics(targetPost, world.DefaultLexicon())
			topicStr := targetPost.Intent.Topic
			if topicStr == "" && len(topics) > 0 {
				topicStr = topics[0]
			}
			if topicStr == "" {
				topicStr = strings.TrimSpace(targetPost.Subject)
			}
			summary := fmt.Sprintf("%sの投稿「%s」に対する%sからの返信", targetPost.Author, targetPost.Subject, handle)

			saved := e.Store.AddPost(host.ID, world.Post{
				BoardID:         board.ID,
				ParentID:        parentID,
				Author:          handle,
				AuthorPersonaID: rep.PersonaID,
				Subject:         subject,
				Body:            "",
				CreatedAt:       rep.DueAt,
				Intent: world.PostIntent{
					Action:                     ActionWorldCatchup,
					AnchorKey:                  targetPost.Intent.AnchorKey,
					CauseKind:                  "persona_reaction",
					DiscourseMode:              "reply",
					SourcePostID:               targetPost.ID,
					RespondsToPostID:           targetPost.ID,
					ProducerEventID:            eventID,
					Topic:                      topicStr,
					Topics:                     topics,
					SituationKind:              "reply",
					SituationSummary:           summary,
					ArticleDetailsMaterialized: true,
					SituationFacts: []string{
						fmt.Sprintf("responds_to_post_id=%d", targetPost.ID),
						"world_adopted_summary=" + summary,
					},
				},
			})
			rep.MaterializedPostID = saved.ID
			postsByID[saved.ID] = saved
			byEventID[eventID] = saved
			changed = true
			created++
		}
		if changed {
			planStore.SaveReactionPlan(host.ID, plan)
		}
	}
	return created
}

func (e *Engine) personasForReactionPlanning(host world.Host, board world.Board, recent []world.Post, at time.Time) []world.Persona {
	if source, ok := e.Store.(personaSource); ok {
		if personas := source.ListHostPersonas(host.ID); len(personas) > 0 {
			return personas
		}
	}
	// Fallback: only used when PersonaStore has no resident personas for this host
	// (e.g., synthetic unit-test hosts that only seed raw posts).
	actors := e.actorRoster(host, board, recent, at)
	out := make([]world.Persona, 0, len(actors))
	for i, a := range actors {
		id := a.personaID
		if id == "" {
			id = fmt.Sprintf("%s:actor-%02d", host.ID, i+1)
		}
		out = append(out, world.Persona{
			ID:              id,
			Handle:          a.handle,
			ActivityPattern: "active",
			ReplyTendency:   0.65,
		})
	}
	return out
}

type appealDecision struct {
	appeal    int
	reason    string
	hasAppeal bool
}

func (e *Engine) recentReplyCounts(hostID, boardID string, limit int) []int {
	planStore, ok := e.Store.(world.ReactionPlanStore)
	if !ok {
		return nil
	}
	var counts []int
	for _, p := range filterBoard(e.Store.ListPosts(hostID), boardID) {
		if world.IsSemanticRoot(p) {
			if plan, exists := planStore.ReactionPlan(hostID, p.ID); exists {
				counts = append(counts, plan.Count)
			}
		}
	}
	if limit > 0 && len(counts) > limit {
		counts = counts[len(counts)-limit:]
	}
	return counts
}

func (e *Engine) planReactionsForRoot(host world.Host, board world.Board, rootPost world.Post, ordinal int, personas []world.Persona, factsByPersona map[string][]world.PersonaFact) {
	e.planReactionsForRoots(context.Background(), host, board, []world.Post{rootPost}, ordinal, personas, factsByPersona, false, nil)
}

func (e *Engine) planReactionsForRoots(ctx context.Context, host world.Host, board world.Board, roots []world.Post, startOrdinal int, personas []world.Persona, factsByPersona map[string][]world.PersonaFact, isInitial bool, appealByPostID map[int64]appealDecision) {
	if e == nil || e.Store == nil || len(roots) == 0 {
		return
	}
	planStore, ok := e.Store.(world.ReactionPlanStore)
	if !ok {
		return
	}

	var unjudgedRoots []world.Post
	for _, r := range roots {
		if !world.IsSemanticRoot(r) {
			continue
		}
		if existing, exists := planStore.ReactionPlan(host.ID, r.ID); exists && existing.Judged {
			continue
		}
		unjudgedRoots = append(unjudgedRoots, r)
	}
	if len(unjudgedRoots) == 0 {
		return
	}

	if len(personas) == 0 {
		personas = e.personasForReactionPlanning(host, board, nil, e.currentTime())
	}
	if factsByPersona == nil {
		factsByPersona = map[string][]world.PersonaFact{}
		if fs, ok := e.Store.(world.PersonaFactStore); ok {
			for _, p := range personas {
				if facts := fs.ListPersonaFacts(p.ID); len(facts) > 0 {
					factsByPersona[p.ID] = facts
				}
			}
		}
	}

	replyRate := effectiveBoardReplyRate(board)
	appealMap := make(map[int64]int, len(unjudgedRoots))
	reasonMap := make(map[int64]string, len(unjudgedRoots))

	activeByRoot := make(map[int64]int, len(unjudgedRoots))
	standardByRoot := make(map[int64]int, len(unjudgedRoots))
	for _, r := range unjudgedRoots {
		act, std := world.ActivePopulationAt(host, board, personas, factsByPersona, r.CreatedAt)
		activeByRoot[r.ID] = act
		standardByRoot[r.ID] = std
	}

	for _, r := range unjudgedRoots {
		if dec, ok := appealByPostID[r.ID]; ok && dec.hasAppeal {
			appealMap[r.ID] = dec.appeal
			reasonMap[r.ID] = dec.reason
		} else {
			appealMap[r.ID] = 2
			reason := "fallback:no_appeal"
			if dec, ok := appealByPostID[r.ID]; ok && strings.TrimSpace(dec.reason) != "" {
				reason = dec.reason
			}
			reasonMap[r.ID] = reason
		}
	}

	rng := e.rng()
	now := e.currentTime()
	for i, rootPost := range unjudgedRoots {
		ordinal := startOrdinal + i
		if ordinal <= 0 {
			ordinal = int(rootPost.ID)
		}
		appeal := appealMap[rootPost.ID]
		reason := reasonMap[rootPost.ID]
		activeCount := activeByRoot[rootPost.ID]
		standardActive := standardByRoot[rootPost.ID]
		count := world.ReplyCountForWithStandard(rng, replyRate, appeal, activeCount, standardActive, world.MaxThreadReplies)

		plan := world.PlanReactions(world.PlanInput{
			Host:               host,
			Board:              board,
			TargetPost:         rootPost,
			Ordinal:            ordinal,
			ReplyRate:          replyRate,
			ReplyCountOverride: &count,
			Appeal:             appeal,
			Judged:             true,
			Reason:             reason,
			PlannedAt:          rootPost.CreatedAt,
			Personas:           personas,
			FactsByPersona:     factsByPersona,
			BoardPrior:         board.Topics,
			Thread: world.ThreadState{
				StarterPersonaID: rootPost.AuthorPersonaID,
				ParentPersonaID:  rootPost.AuthorPersonaID,
				LastPersonaID:    rootPost.AuthorPersonaID,
			},
		})

		if isInitial {
			at := rootPost.CreatedAt
			for k := range plan.Replies {
				rep := &plan.Replies[k]
				if !rep.DueAt.Before(now) {
					rep.DueAt = now.Add(-time.Duration(len(plan.Replies)-k) * time.Minute)
					if !rep.DueAt.After(at) {
						rep.DueAt = at.Add(time.Minute)
					}
					if !rep.DueAt.Before(now) {
						rep.DueAt = now.Add(-time.Second)
					}
				}
				at = rep.DueAt
			}
		}

		planStore.SaveReactionPlan(host.ID, plan)
		log.Printf("BBS reply appeal: host=%s board=%s root=%d subject=%q appeal=%d count=%d reason=%q",
			host.ID, board.ID, rootPost.ID, rootPost.Subject, appeal, count, reason)
	}
}

func (e *Engine) catchUp(ctx context.Context, host world.Host, board world.Board, ignoreCadence bool, initialCount int, initialLookback time.Duration, desiredReplies int, replyQuotas []int, initialStartOrdinal int) error {
	if e == nil || e.Store == nil {
		return nil
	}
	now := e.currentTime()
	e.MaterializeDueReactions(host, board, now)
	if e.Planner == nil {
		return nil
	}
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
	if len(replyQuotas) > 0 {
		slots = e.addThreadReplies(host, board, slots, replyQuotas, now)
	}
	if len(slots) == 0 {
		return nil
	}
	since := cursor
	if since.IsZero() && !initialStart.IsZero() {
		since = initialStart
	}

	personas := e.personasForReactionPlanning(host, board, recent, now)
	factsByPersona := map[string][]world.PersonaFact{}
	if fs, ok := e.Store.(world.PersonaFactStore); ok {
		for _, p := range personas {
			if facts := fs.ListPersonaFacts(p.ID); len(facts) > 0 {
				factsByPersona[p.ID] = facts
			}
		}
	}
	activeBySlot := make(map[int]int, len(slots))
	for _, s := range slots {
		if s.ReplyToSlotIndex == 0 && s.ReplyToPostID == 0 {
			act, _ := world.ActivePopulationAt(host, board, personas, factsByPersona, s.CreatedAt)
			activeBySlot[s.Index] = act
		}
	}
	recentCounts := e.recentReplyCounts(host.ID, board.ID, 10)
	baseReplyRate := effectiveBoardReplyRate(board)

	planned, err := e.Planner.PlanBBSBatch(ctx, BatchRequest{
		Host:              host,
		Board:             board,
		WorldNow:          now,
		Since:             since,
		RecentPosts:       recent,
		Slots:             slots,
		BaseReplyRate:     baseReplyRate,
		RecentReplyCounts: recentCounts,
		ActiveBySlot:      activeBySlot,
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

	var newRoots []world.Post
	persistedBySlot := make(map[int]world.Post, len(slots))
	appealByPostID := make(map[int64]appealDecision, len(slots))
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

		situationFacts := append([]string(nil), draft.SituationFacts...)
		situationKind := strings.TrimSpace(draft.SituationKind)
		discourseMode := strings.TrimSpace(draft.DiscourseMode)
		if len(situationFacts) == 0 {
			// Legacy title-first compatibility.
			situationFacts = []string{
				"world_adoption=title_candidate",
				"world_adopted_summary=" + draft.SituationSummary,
			}
			if subject != "" {
				situationFacts = append(situationFacts,
					"title_first_subject="+subject,
					"subject_contract=Keep the adopted title verbatim. Do not replace it with a different topic.",
				)
			}
			if situationKind == "" {
				situationKind = "title_first"
			}
		}
		if isReply {
			situationFacts = append(situationFacts, fmt.Sprintf("responds_to_post_id=%d", responseToID))
			if discourseMode == "" {
				discourseMode = "reply"
			}
		}
		if discourseMode == "" {
			discourseMode = "thread_start"
		}

		draftTopics := draft.Topics
		if len(draftTopics) > 5 {
			draftTopics = draftTopics[:5]
		}
		candidatePost := world.Post{
			BoardID:         board.ID,
			ParentID:        parentID,
			Author:          slot.Author,
			AuthorPersonaID: slot.AuthorPersonaID,
			Subject:         subject,
			Body:            "",
			Intent: world.PostIntent{
				Action:                     ActionWorldCatchup,
				AnchorKey:                  strings.TrimSpace(draft.AnchorKey),
				CauseKind:                  "board_activity_window",
				DiscourseMode:              discourseMode,
				SourcePostID:               responseToID,
				SituationKind:              situationKind,
				SituationSummary:           draft.SituationSummary,
				SituationFacts:             situationFacts,
				ArticleDetailsMaterialized: draft.ArticleDetailsMaterialized,
				Topic:                      draft.Topic,
				Topics:                     append([]string(nil), draftTopics...),
				Motivation:                 draft.Motivation,
				Stance:                     draft.Stance,
				Goal:                       draft.Goal,
				Claims:                     append([]string(nil), draft.Claims...),
				RespondsToPostID:           responseToID,
			},
			CreatedAt: slot.CreatedAt,
		}
		candidatePost.Intent.Topics = world.ExtractPostTopics(candidatePost, world.DefaultLexicon())
		saved := e.Store.AddPost(host.ID, candidatePost)
		persistedBySlot[slot.Index] = saved
		if !isReply {
			newRoots = append(newRoots, saved)
			appealByPostID[saved.ID] = appealDecision{
				appeal:    draft.ReplyAppeal,
				reason:    draft.ReplyAppealReason,
				hasAppeal: draft.HasReplyAppeal,
			}
		}
	}

	if len(replyQuotas) == 0 && (len(newRoots) > 0 || len(boardPosts) > 0) {
		existingRoots := 0
		var unjudgedRoots []world.Post
		if planStore, ok := e.Store.(world.ReactionPlanStore); ok {
			for _, post := range boardPosts {
				if world.IsSemanticRoot(post) {
					existingRoots++
					if _, exists := planStore.ReactionPlan(host.ID, post.ID); !exists {
						unjudgedRoots = append(unjudgedRoots, post)
					}
				}
			}
		}
		unjudgedRoots = append(unjudgedRoots, newRoots...)
		isInitial := ignoreCadence || cursor.IsZero()
		startOrdinal := existingRoots + 1
		if initialStartOrdinal > 0 && existingRoots == 0 {
			startOrdinal = initialStartOrdinal
		}
		e.planReactionsForRoots(ctx, host, board, unjudgedRoots, startOrdinal, personas, factsByPersona, isInitial, appealByPostID)
		e.MaterializeDueReactions(host, board, now)
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
		if (post.Intent.Action == ActionWorldCatchup || post.Intent.Action == legacyActionWorldCatchup) &&
			post.Intent.CauseKind != "persona_reaction" &&
			post.CreatedAt.After(latestGenerated) {
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
		// Cadence batches and board-history windows schedule replies via
		// PlanReactions / addThreadReplies rather than a fixed 1-in-4 rule;
		// desiredReplies > 0 is retained only when an explicit reply count is requested.
		shouldReply := false
		if len(targets) > 0 && desiredReplies > 0 {
			if desiredReplies > count-1 {
				desiredReplies = count - 1
			}
			repliesSoFar := 0
			for _, prior := range slots {
				if prior.ReplyToPostID != 0 || prior.ReplyToSlotIndex != 0 {
					repliesSoFar++
				}
			}
			if i > 0 {
				expectedByNow := (i * desiredReplies) / (count - 1)
				shouldReply = expectedByNow > repliesSoFar
			}
		}
		// Staff-origin boards are a World-level actor constraint, not
		// something the language model may simulate by changing a handle.
		// A dedicated SYSOP can exist as station configuration; reuse a
		// stored SYSOP persona if one has been materialized.
		if !shouldReply && board.RootAuthorPolicy == "sysop_only" {
			slot.Author = "SYSOP"
			slot.AuthorPersonaID = host.ID + ":sysop"
			for _, candidate := range actors {
				if strings.EqualFold(candidate.handle, "SYSOP") {
					slot.Author = candidate.handle
					slot.AuthorPersonaID = candidate.personaID
					break
				}
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

// addThreadReplies places each root's replies after it in simulated time.
// roots must be the root-only slots of planSlots in chronological order and
// quotas[i] is the number of replies thread i drew. When the store provides
// resident personas (PersonaStore), reactor selection and reply timing use the
// unified persona-reaction model (world.ReactionWeight, world.PickReactor,
// world.ReactionDueAt). Fallback to actorRoster is permitted only for minimal
// test fixtures that do not populate host personas.
func (e *Engine) addThreadReplies(host world.Host, board world.Board, roots []Slot, quotas []int, now time.Time) []Slot {
	var personas []world.Persona
	if source, ok := e.Store.(personaSource); ok {
		personas = source.ListHostPersonas(host.ID)
	}
	if len(personas) > 0 {
		return e.addThreadRepliesWithPersonas(host, board, personas, roots, quotas, now)
	}

	// Fallback: only used when PersonaStore has no resident personas for this host
	// (e.g., synthetic unit-test hosts that only seed raw posts).
	actors := e.actorRoster(host, board, nil, now)
	out := append([]Slot(nil), roots...)
	for i, root := range roots {
		if i >= len(quotas) || quotas[i] <= 0 {
			continue
		}
		key := func(k int, salt string) float64 {
			return float64(stableHash(fmt.Sprintf("%s|%s|%d|%d|%s", host.ID, board.ID, root.Index, k, salt))%1000000+1) / 1000001.0
		}
		at, last := root.CreatedAt, root.Author
		for k := 0; k < quotas[i]; k++ {
			// Pareto-like gap: median about half an hour, a tail reaching into the next day.
			gap := time.Duration(15*math.Pow(key(k, "gap"), -.9)) * time.Minute
			if gap < time.Minute {
				gap = time.Minute
			}
			at = at.Add(gap)
			if !at.Before(now) {
				// A thread cannot outrun the present: squeeze the tail in.
				at = now.Add(-time.Duration(quotas[i]-k) * time.Minute)
				if !at.After(root.CreatedAt) {
					at = root.CreatedAt.Add(time.Minute)
				}
			}
			a := actors[int(stableHash(fmt.Sprintf("%s|%s|%d|%d|author", host.ID, board.ID, root.Index, k))%uint64(len(actors)))]
			if k > 0 && key(k, "starter") < .3 {
				a = actor{handle: root.Author, personaID: root.AuthorPersonaID}
			}
			if strings.EqualFold(a.handle, last) && len(actors) > 1 {
				for _, c := range actors {
					if !strings.EqualFold(c.handle, last) {
						a = c
						break
					}
				}
			}
			last = a.handle
			out = append(out, Slot{
				Author:           a.handle,
				AuthorPersonaID:  a.personaID,
				CreatedAt:        at,
				ReplyToSlotIndex: root.Index,
				ReplyToAuthor:    root.Author,
			})
		}
	}
	return reindexChronologicalSlots(roots, out)
}

func (e *Engine) addThreadRepliesWithPersonas(host world.Host, board world.Board, personas []world.Persona, roots []Slot, quotas []int, now time.Time) []Slot {
	personaByID := make(map[string]world.Persona, len(personas))
	for _, p := range personas {
		personaByID[p.ID] = p
	}
	factsByPersona := map[string][]world.PersonaFact{}
	if fs, ok := e.Store.(world.PersonaFactStore); ok {
		for _, p := range personas {
			if facts := fs.ListPersonaFacts(p.ID); len(facts) > 0 {
				factsByPersona[p.ID] = facts
			}
		}
	}

	out := append([]Slot(nil), roots...)
	for i, root := range roots {
		if i >= len(quotas) || quotas[i] <= 0 {
			continue
		}
		quota := quotas[i]
		var starterTopics []string
		if starter, ok := personaByID[root.AuthorPersonaID]; ok {
			for k, v := range starter.Interests {
				if v >= 0.5 {
					starterTopics = append(starterTopics, k)
				}
			}
			sort.Strings(starterTopics)
		}
		syntheticRoot := world.Post{
			ID:              int64(root.Index),
			BoardID:         board.ID,
			Author:          root.Author,
			AuthorPersonaID: root.AuthorPersonaID,
			CreatedAt:       root.CreatedAt,
		}
		plan := world.PlanReactions(world.PlanInput{
			Host:               host,
			Board:              board,
			TargetPost:         syntheticRoot,
			Ordinal:            root.Index,
			ReplyRate:          board.ReplyRate,
			ReplyCountOverride: &quota,
			PlannedAt:          root.CreatedAt,
			Personas:           personas,
			FactsByPersona:     factsByPersona,
			Topics:             starterTopics,
			Thread: world.ThreadState{
				StarterPersonaID: root.AuthorPersonaID,
				ParentPersonaID:  root.AuthorPersonaID,
				LastPersonaID:    root.AuthorPersonaID,
			},
		})
		at := root.CreatedAt
		for k, rep := range plan.Replies {
			chosen := personaByID[rep.PersonaID]
			handle := strings.TrimSpace(chosen.Handle)
			if handle == "" {
				handle = rep.PersonaID
			}
			dueAt := rep.DueAt
			if !dueAt.Before(now) {
				dueAt = now.Add(-time.Duration(len(plan.Replies)-k) * time.Minute)
				if !dueAt.After(at) {
					dueAt = at.Add(time.Minute)
				}
				if !dueAt.Before(now) {
					dueAt = now.Add(-time.Second)
				}
			}
			at = dueAt
			out = append(out, Slot{
				Author:           handle,
				AuthorPersonaID:  rep.PersonaID,
				CreatedAt:        dueAt,
				ReplyToSlotIndex: root.Index,
				ReplyToAuthor:    root.Author,
			})
		}
	}
	return reindexChronologicalSlots(roots, out)
}

func reindexChronologicalSlots(roots []Slot, out []Slot) []Slot {
	// Keep root slots ahead of their replies when timestamps tie, then re-index.
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	newIndex := make(map[int]int, len(roots))
	for i := range out {
		if out[i].ReplyToSlotIndex == 0 {
			newIndex[out[i].Index] = i + 1
		}
	}
	for i := range out {
		if out[i].ReplyToSlotIndex != 0 {
			out[i].ReplyToSlotIndex = newIndex[out[i].ReplyToSlotIndex]
		}
		out[i].Index = i + 1
	}
	return out
}

func stableHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}