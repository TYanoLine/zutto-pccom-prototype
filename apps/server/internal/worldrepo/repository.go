package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
	Persona                    *world.Persona
	Intent                     world.PostIntent
	CanonicalSubject           string
	Kind                       worldengine.PostKind
	ParentPost                 *world.Post
	QuoteText                  string
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
	debugLogHAKATAGenerated                    bool
	hakataFreeformBody                        bool
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

func (r *Repository) AddPost(hostID string, p world.Post) world.Post {
	return r.Base.AddPost(hostID, p)
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
		r.Base.AddPost(host.ID, p)
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
