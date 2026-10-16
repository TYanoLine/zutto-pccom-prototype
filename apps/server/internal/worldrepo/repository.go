package worldrepo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zutto-pccom/apps/server/internal/actormemory"
	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/hostprogram"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type EvidenceResolver interface {
	ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error)
}

type EvidenceLookupResolver interface {
	LookupEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error)
}

type Materializer interface {
	GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error)
}

type BoardMaterializationRequest struct {
	Host                       world.Host
	BoardID                    string
	BoardTopic                 string
	FreeformFromSubject        bool
	WorldDate                  string
	AuthorHandle               string
	Persona                    *world.Persona
	Intent                     world.PostIntent
	CanonicalSubject           string
	Kind                       worldengine.PostKind
	ParentPost                 *world.Post
	QuoteText                  string
	RelationshipContext        string
	BodyMinChars, BodyMaxChars int
}

type Repository struct {
	Base                 world.Store
	Engine               EvidenceResolver
	Materializer         Materializer
	ArticleDetailPlanner llm.BBSTitleArticleDetailPlanner
	WorldDate            string
	worldNow             func() time.Time
	bbsArticles          *bbsengine.Engine
	debugLogBBSArticleDetails                  bool
	debugLogGeneratedContent                    bool
	titleVariants                               bool
	freeformBody                        bool
	generationTrace                           *generationTraceStore

	mu                     sync.Mutex
	materialized           map[string]bool
	hosts                  map[string]world.Host
	populationMaterialized map[string]bool
	debugImmediateBBS      map[string]bool

	observationMu        sync.Mutex
	observationBoardJobs map[string]*observationJob
	observationBodyJobs  map[string]*observationJob

	prefetchMu      sync.Mutex
	prefetchQueues  map[string][]world.Board
	prefetchRunning map[string]bool

	// indexer is optional: when set, every saved post is handed to it after the
	// canonical write. It must not block (see PostIndexer).
	indexerMu sync.RWMutex
	indexer   PostIndexer

	// actorMemoryIndex is optional: when set, it provides the embedding index for
	// actor-scoped memory tool execution during prose materialization.
	actorMemoryMu    sync.RWMutex
	actorMemoryIndex *actormemory.Index

	// residentChat lets residents enter chat rooms (spec 016 FR-004). It is
	// off unless the server can also give them words.
	residentChat atomic.Bool
	// presenceWindows caches presence on the window grid for chat plans.
	presenceWindows presenceWindowCache

	// chatTalk is who watches each chat room and what residents' lines are
	// being written (spec 017).
	chatTalk chatTalk
	// lineTalk is the residents' line messages to human callers (spec 018).
	lineTalk lineTalk
}

func New(base world.Store, engine EvidenceResolver, materializer Materializer, worldDate string) *Repository {
	r := &Repository{
		Base:                   base,
		Engine:                 engine,
		Materializer:           materializer,
		ArticleDetailPlanner:   articleDetailPlannerFromMaterializer(materializer),
		WorldDate:              worldDate,
		worldNow:               func() time.Time { return worldTime(worldDate) },
		materialized:           map[string]bool{},
		hosts:                  map[string]world.Host{},
		populationMaterialized: map[string]bool{},
		debugImmediateBBS:      map[string]bool{},
		observationBoardJobs:   map[string]*observationJob{},
		observationBodyJobs:    map[string]*observationJob{},
		prefetchQueues:         map[string][]world.Board{},
		prefetchRunning:        map[string]bool{},
	}
	r.bbsArticles = bbsengine.New(base, repositoryBBSBatchPlanner{repo: r}, r.currentWorldTime)
	r.bbsArticles.ReplyProjector = bbsengine.ReplyProjectorFunc(func(host world.Host, source world.Post, proposedSubject string) (bbsengine.ReplyRepresentation, error) {
		projected, err := hostprogram.ProjectReply(host, source, proposedSubject)
		if err != nil {
			return bbsengine.ReplyRepresentation{}, err
		}
		return bbsengine.ReplyRepresentation{ParentID: projected.ParentID, Subject: projected.Subject}, nil
	})
	return r
}

func articleDetailPlannerFromMaterializer(materializer Materializer) llm.BBSTitleArticleDetailPlanner {
	switch value := materializer.(type) {
	case LLMMaterializer:
		planner, _ := value.Renderer.(llm.BBSTitleArticleDetailPlanner)
		return planner
	case *LLMMaterializer:
		if value != nil {
			planner, _ := value.Renderer.(llm.BBSTitleArticleDetailPlanner)
			return planner
		}
	}
	return nil
}

// SetArticleDetailPlanner explicitly wires the shared article detail capability.
func (r *Repository) SetArticleDetailPlanner(planner llm.BBSTitleArticleDetailPlanner) {
	if r != nil {
		r.ArticleDetailPlanner = planner
	}
}

// SetDebugLogBBSArticleDetails enables diagnostic logging of the final validated
// Article Detail payload before it is persisted. It must remain opt-in because
// the payload is world content rather than ordinary operational telemetry.
func (r *Repository) SetDebugLogBBSArticleDetails(enabled bool) {
	if r != nil {
		r.debugLogBBSArticleDetails = enabled
	}
}

func (r *Repository) debugBBSArticleDetailLoggingEnabled() bool {
	return r != nil && r.debugLogBBSArticleDetails
}

// SetWorldNow supplies the mapped 1996 world clock used by background
// simulation/catch-up. Tests and non-runtime tools keep the deterministic
// WorldDate fallback installed by New.
func (r *Repository) SetWorldNow(now func() time.Time) {
	if now != nil {
		r.worldNow = now
		if r.bbsArticles != nil {
			r.bbsArticles.Now = r.currentWorldTime
		}
	}
}

// WorldNow implements world.WorldTimeSource.
func (r *Repository) WorldNow() time.Time {
	return r.currentWorldTime()
}

func (r *Repository) currentWorldTime() time.Time {
	if r.worldNow != nil {
		return r.worldNow()
	}
	return worldTime(r.WorldDate)
}

// HostByPhone returns the host definition as stored. The definition is
// immutable: it is complete when it is loaded (presets are validated at startup)
// and is never filled in or rewritten here.
func (r *Repository) HostByPhone(phone string) (world.Host, error) {
	h, err := r.Base.HostByPhone(phone)
	if err != nil {
		return h, err
	}
	r.mu.Lock()
	r.hosts[h.ID] = h
	r.mu.Unlock()

	return h, nil
}

func (r *Repository) PopulationWasMaterialized(hostID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.populationMaterialized[hostID]
}

func (r *Repository) ListPosts(hostID string) []world.Post {
	// Reads must not create world history. A successful CONNECT explicitly starts
	// host observation; host-program index/body reads use the observation barriers
	// when they need data that has not finished materializing yet.
	return r.Base.ListPosts(hostID)
}

// PostIndexer receives each saved post so it can be made searchable. Implementations
// must return quickly and must never fail the save: embedding work belongs in the
// background, not here.
type PostIndexer interface {
	IndexPost(hostID string, p world.Post)
}

// SetPostIndexer enables (or, with nil, disables) post indexing.
func (r *Repository) SetPostIndexer(ix PostIndexer) {
	r.indexerMu.Lock()
	r.indexer = ix
	r.indexerMu.Unlock()
}

// SetActorMemoryIndex wires the optional actor memory index for tool-capable materialization.
func (r *Repository) SetActorMemoryIndex(idx *actormemory.Index) {
	r.actorMemoryMu.Lock()
	r.actorMemoryIndex = idx
	r.actorMemoryMu.Unlock()
}

// ActorMemoryIndex returns the configured actor memory index, if any.
func (r *Repository) ActorMemoryIndex() *actormemory.Index {
	r.actorMemoryMu.RLock()
	defer r.actorMemoryMu.RUnlock()
	return r.actorMemoryIndex
}

func (r *Repository) AddPost(hostID string, p world.Post) world.Post {
	saved := r.Base.AddPost(hostID, p)
	r.recordPostInteraction(hostID, saved)
	r.planHumanPostReactions(hostID, saved)
	r.indexerMu.RLock()
	ix := r.indexer
	r.indexerMu.RUnlock()
	if ix != nil {
		ix.IndexPost(hostID, saved)
	}
	return saved
}

func (r *Repository) recordPostInteraction(hostID string, p world.Post) {
	actor := world.PostActor(p)
	var actorLogin string
	if actor.Kind == world.ActorKindMember {
		actorLogin = world.PostLogin(p)
	}

	sourceKind := "human_authored"
	if actor.Kind == "persona" {
		sourceKind = "npc_world_selected"
	}

	parentID := p.ParentID
	if parentID == 0 && p.Intent.RespondsToPostID != 0 {
		parentID = p.Intent.RespondsToPostID
	}

	channel := "board_root"
	var addressedTo []world.ActorRef
	var addressedLogins []string
	var respondsToEventID string
	var knownFrom, knownTo world.ActorRef

	if parentID != 0 {
		channel = "board_reply"
		respondsToEventID = fmt.Sprintf("post:%s:%d", hostID, parentID)

		for _, post := range r.Base.ListPosts(hostID) {
			if post.ID == parentID {
				if parent := world.PostActor(post); !parent.IsZero() {
					addressedTo = []world.ActorRef{parent}
					if parent.Kind == world.ActorKindMember {
						addressedLogins = []string{world.PostLogin(post)}
					} else {
						addressedLogins = []string{""}
					}
				}
				knownTo = world.PostKnownAs(post)
				break
			}
		}
		knownFrom = world.PostKnownAs(p)
	}

	eventID := fmt.Sprintf("post:%s:%d", hostID, p.ID)
	scope := fmt.Sprintf("public_board:%s:%s", hostID, p.BoardID)
	event := world.InteractionEvent{
		EventID:           eventID,
		HostID:            hostID,
		Channel:           channel,
		Actor:             actor,
		AudienceScope:     scope,
		AddressedTo:       addressedTo,
		RespondsToEventID: respondsToEventID,
		ContentRef:        strconv.FormatInt(p.ID, 10),
		OccurredAt:        p.CreatedAt,
		SourceKind:        sourceKind,
		ActorLogin:        actorLogin,
		AddressedLogins:   addressedLogins,
	}

	if ledgerStore, ok := r.Base.(world.InteractionLedgerStore); ok {
		_ = ledgerStore.AppendInteractionEvent(event)
	}

	// Residents know people by login ID, so relationships are kept between the
	// people they know; a guest is nobody they know (spec 019).
	if channel == "board_reply" && len(addressedTo) > 0 {
		actor, toActor := knownFrom, knownTo
		if !actor.IsZero() && !toActor.IsZero() {
			if relStore, ok := r.Base.(world.RelationshipStore); ok {
				edge, exists := relStore.GetRelationship(hostID, actor, toActor)
				if edge.LastEventID != eventID {
					count := 1
					if exists {
						count = edge.BoardReplyCount + 1
					}
					newEdge := world.RelationshipEdge{
						HostID:            hostID,
						FromActor:         actor,
						ToActor:           toActor,
						BoardReplyCount:   count,
						LastInteractionAt: p.CreatedAt,
						LastEventID:       eventID,
					}
					_ = relStore.UpdateRelationship(newEdge)
				}
			}
		}
	}
}

func (r *Repository) ListBoardPosts(host world.Host, boardID, boardTopic string) []world.Post {
	if existing := filterBoard(r.Base.ListPosts(host.ID), boardID); len(existing) > 0 {
		return existing
	}
	_ = r.ensureBoard(host, boardID, boardTopic)
	return filterBoard(r.Base.ListPosts(host.ID), boardID)
}

func (r *Repository) ensureBoard(host world.Host, boardID, boardTopic string) error {
	if r.Engine == nil || r.Materializer == nil {
		return nil
	}
	key := host.ID + "|" + boardID
	r.mu.Lock()
	if r.materialized[key] {
		r.mu.Unlock()
		return nil
	}
	r.materialized[key] = true
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     boardTopic,
		WorldDate:   r.WorldDate,
		Region:      host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボードに自然な投稿を生成するために必要な時代背景", host.Name, boardTopic),
		Persistence: true,
		Importance:  .25,
		Specificity: .25,
	})
	if err != nil {
		r.mu.Lock()
		delete(r.materialized, key)
		r.mu.Unlock()
		return err
	}
	req := BoardMaterializationRequest{Host: host, BoardID: boardID, BoardTopic: boardTopic, WorldDate: r.WorldDate, Kind: worldengine.PostKindNewPost}
	req = r.prepareBoardComposition(req, world.Post{BoardID: boardID, Subject: boardTopic})
	posts, err := r.Materializer.GenerateBoardPosts(ctx, req, decision)
	if err != nil {
		r.mu.Lock()
		delete(r.materialized, key)
		r.mu.Unlock()
		return err
	}
	for _, p := range posts {
		p.BoardID = boardID
		r.AddPost(host.ID, p)
	}
	return nil
}

func personaByHandle(all []world.Persona, handle string) (world.Persona, bool) {
	for _, p := range all {
		if strings.EqualFold(p.Handle, handle) {
			return p, true
		}
	}
	return world.Persona{}, false
}

func filterBoard(all []world.Post, boardID string) []world.Post {
	out := make([]world.Post, 0)
	for _, p := range all {
		if p.BoardID == boardID {
			out = append(out, p)
		}
	}
	return out
}

func worldTime(v string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return time.Now()
	}
	return t.Add(22*time.Hour + 15*time.Minute)
}

func (r *Repository) sysopPersona(host world.Host) world.Persona {
	founded := host.FoundedOn
	if strings.TrimSpace(founded) == "" {
		founded = "1994年頃"
	}
	lines := host.Lines
	if lines <= 0 {
		lines = 1
	}
	maxBaud := host.MaxBaud
	if maxBaud <= 0 {
		maxBaud = 14400
	}
	sw := host.Software
	if strings.TrimSpace(sw) == "" {
		sw = "草の根BBS"
	}

	everyday := []string{
		fmt.Sprintf("草の根BBS「%s」（%s）のシステムオペレータ（開局日: %s）。", host.Name, host.Region, founded),
		fmt.Sprintf("自前の機器（%s、%d回線、最高%d bps）でホストを24時間運用している。", sw, lines, maxBaud),
		"局の健全な運営と、利用者同士の気持ち良い交流・回線の譲り合いを大切にしている。",
	}

	return world.Persona{
		ID:                  host.ID + ":sysop",
		Handle:              "SYSOP",
		Age:                 38,
		Gender:              "男性",
		Occupation:          "局長・システムオペレータ",
		ActivityPattern:     "daily",
		ReplyTendency:       0.8,
		ThreadStartTendency: 0.5,
		LurkerTendency:      0.0,
		WritingStyle:        "丁寧で落ち着いた敬語。利用者を気遣い、ホストの責任者として案内やルールを分かりやすく伝える",
		EverydayContext:     everyday,
	}
}

func (r *Repository) resolvePersona(host world.Host, personaID, author string) *world.Persona {
	if strings.EqualFold(strings.TrimSpace(author), "SYSOP") || personaID == host.ID+":sysop" {
		p := r.sysopPersona(host)
		return &p
	}
	if ps, ok := r.Base.(world.PersonaStore); ok && personaID != "" {
		if p, found := ps.PersonaByID(personaID); found {
			copy := p
			return &copy
		}
	}
	return nil
}
