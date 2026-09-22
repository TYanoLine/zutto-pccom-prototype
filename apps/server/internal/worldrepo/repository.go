package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type EvidenceResolver interface {
	ResolveEvidence(context.Context, worldengine.EvidenceRequest) (worldengine.EvidenceDecision, error)
}

type Materializer interface {
	GenerateBoardPosts(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, error)
}

type BoardMaterializationRequest struct {
	Host             world.Host
	BoardID          string
	BoardTopic       string
	WorldDate        string
	Persona          *world.Persona
	Intent           world.PostIntent
	CanonicalSubject string
}

type Repository struct {
	Base         world.Store
	Engine       EvidenceResolver
	Materializer Materializer
	WorldDate    string
	worldNow     func() time.Time
	bbsArticles  *bbsengine.Engine

	mu                     sync.Mutex
	materialized           map[string]bool
	hosts                  map[string]world.Host
	hostMaterialized       map[string]bool
	populationMaterialized map[string]bool
	debugImmediateBBS      map[string]bool

	observationMu        sync.Mutex
	observationBoardJobs map[string]*observationJob
	observationBodyJobs  map[string]*observationJob

	bbsMaterializationMu    sync.Mutex
	bbsMaterializationLocks map[string]*sync.Mutex
}

func New(base world.Store, engine EvidenceResolver, materializer Materializer, worldDate string) *Repository {
	r := &Repository{
		Base:                   base,
		Engine:                 engine,
		Materializer:           materializer,
		WorldDate:              worldDate,
		worldNow:               func() time.Time { return worldTime(worldDate) },
		materialized:           map[string]bool{},
		hosts:                  map[string]world.Host{},
		hostMaterialized:       map[string]bool{},
		populationMaterialized: map[string]bool{},
		debugImmediateBBS:      map[string]bool{},
		observationBoardJobs:     map[string]*observationJob{},
		observationBodyJobs:      map[string]*observationJob{},
		bbsMaterializationLocks:  map[string]*sync.Mutex{},
	}
	r.bbsArticles = bbsengine.New(base, repositoryBBSBatchPlanner{repo: r}, r.currentWorldTime)
	return r
}

func (r *Repository) sharedBBSHostMaterializationLock(hostID string) *sync.Mutex {
	r.bbsMaterializationMu.Lock()
	defer r.bbsMaterializationMu.Unlock()
	lock := r.bbsMaterializationLocks[hostID]
	if lock == nil {
		lock = &sync.Mutex{}
		r.bbsMaterializationLocks[hostID] = lock
	}
	return lock
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

func (r *Repository) HostByPhone(phone string) (world.Host, error) {
	h, err := r.Base.HostByPhone(phone)
	if err != nil {
		return h, err
	}
	if incompleteHost(h) {
		h = completeDevelopmentHost(h)
		if w, ok := r.Base.(world.HostWriter); ok {
			w.SaveHost(h)
		}
		r.mu.Lock()
		r.hostMaterialized[h.ID] = true
		r.mu.Unlock()
	}
	r.mu.Lock()
	r.hosts[h.ID] = h
	r.mu.Unlock()

	// Development-only sparse cast fixture. These are persistent persona skeletons,
	// not content templates: concrete life facts remain unknown until an event needs
	// them and are materialized through the generic semantic planning path.
	if h.SoftwareID == "materialization-demo" {
		_, _ = r.MaterializationPersonas(h)
	}
	return h, nil
}

func (r *Repository) HostWasMaterialized(hostID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hostMaterialized[hostID]
}

func (r *Repository) PopulationWasMaterialized(hostID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.populationMaterialized[hostID]
}

// MaterializationPersonas persists a small core cast at first host observation.
// The complete host member count is intentionally not expanded into hundreds of
// fully described personas. Persona identity is stored separately from host
// membership so the same person can later belong to neighboring hosts.
func (r *Repository) MaterializationPersonas(host world.Host) ([]world.Persona, bool) {
	ps, ok := r.Base.(world.PersonaStore)
	if !ok {
		return nil, false
	}
	if existing := ps.ListHostPersonas(host.ID); len(existing) > 0 {
		return existing, false
	}

	core := []world.Persona{
		{ID: host.ID + "-sysop", Handle: "SYSOP", Age: 34, Gender: "male", Occupation: "会社員 / SYSOP", ActivityPattern: "平日22:00-01:00、週末は深夜まで", ReplyTendency: .72, ThreadStartTendency: .32, LurkerTendency: .04, NewcomerOpenness: .82, Argumentativeness: .18, WritingStyle: "簡潔で面倒見がよい。運営連絡は端的。顔文字は少なめ。", EverydayContext: []string{"この局の運営と日常的な接続確認は普段の生活の一部で、それ自体は珍しい出来事ではない", "会員の増減・案内変更・メンテナンス等は実際に起きた時だけ話題になる"}, Interests: map[string]float64{"bbs": 1, "communications": .76, "local": .62}, Opinions: map[string]float64{"newcomers": .66}},
		{ID: host.ID + "-neko", Handle: "NEKO", Age: 24, Gender: "female", Occupation: "会社員", ActivityPattern: "23:00-02:00中心", ReplyTendency: .70, ThreadStartTendency: .42, LurkerTendency: .18, NewcomerOpenness: .74, Argumentativeness: .22, WritingStyle: "短めで砕けた口調。質問を投げることが多く、顔文字は時々。", EverydayContext: []string{"BBSやチャットは普段の交流手段で、接続したこと自体を特別な体験とは感じない", "ゲームは普段の娯楽の一つで、久しぶり・懐かしい等の意味は実際の間隔がある時だけ生じる"}, Interests: map[string]float64{"chat": .82, "local": .58, "games": .46}, Opinions: map[string]float64{"windows95": .12}},
		{ID: host.ID + "-mari", Handle: "MARI", Age: 21, Gender: "female", Occupation: "短大生", ActivityPattern: "21:00-00:30中心、週末は昼も接続", ReplyTendency: .62, ThreadStartTendency: .30, LurkerTendency: .24, NewcomerOpenness: .88, Argumentativeness: .08, WritingStyle: "柔らかい口調。初対面にも返事をしやすく、(^^) 系を時々使う。", EverydayContext: []string{"BBSやチャットは身近な交流手段で、利用していること自体を説明する必要はない", "音楽や地域の話は日常の関心事で、時代らしさを演出するための小道具ではない"}, Interests: map[string]float64{"chat": .76, "music": .61, "local": .70}, Opinions: map[string]float64{"offline_meetings": .54}},
		{ID: host.ID + "-nori", Handle: "NORI", Age: 29, Gender: "male", Occupation: "技術職", ActivityPattern: "0:00-03:00中心", ReplyTendency: .48, ThreadStartTendency: .36, LurkerTendency: .34, NewcomerOpenness: .40, Argumentativeness: .38, WritingStyle: "やや長めで具体的。技術話では引用を使い、曖昧な断定を嫌う。", EverydayContext: []string{"自宅のパソコンと通信環境は日常の道具で、使っている機械の系統そのものを珍しい話題として扱わない", "機種・通信ソフト・モデム等の名前は、不具合・設定差・比較など今回の具体的な差分に必要な時だけ出す"}, Interests: map[string]float64{"communications": .91, "modem": .88, "software": .84}, Opinions: map[string]float64{"windows95": -.18}},
		{ID: host.ID + "-yuki", Handle: "YUKI", Age: 19, Gender: "male", Occupation: "大学生", ActivityPattern: "22:30-02:30中心", ReplyTendency: .56, ThreadStartTendency: .54, LurkerTendency: .16, NewcomerOpenness: .64, Argumentativeness: .31, WritingStyle: "勢いのある短文が多い。雑談では(笑)をたまに使う。", EverydayContext: []string{"BBSやチャットは普通の遊び・交流の場で、参加したこと自体はニュースではない", "ゲームや音楽は日常の趣味で、昔を懐かしむ視点は実際の出来事がある時だけ生じる"}, Interests: map[string]float64{"games": .86, "music": .48, "chat": .66}, Opinions: map[string]float64{"offline_meetings": .71}},
		{ID: host.ID + "-taka", Handle: "TAKA", Age: 27, Gender: "male", Occupation: "営業職", ActivityPattern: "平日は23:30前後、接続しない日も多い", ReplyTendency: .36, ThreadStartTendency: .18, LurkerTendency: .57, NewcomerOpenness: .46, Argumentativeness: .14, WritingStyle: "普段はROM気味。書く時は2-4行程度で、顔文字はほぼ使わない。", EverydayContext: []string{"自宅のパソコンと通信環境は普段使いの道具で、電源を入れた・接続できたというだけでは話題にしない", "機械の名前は今回の問題や比較に関係するときだけ意識する"}, Interests: map[string]float64{"local": .72, "communications": .44, "games": .35}, Opinions: map[string]float64{"windows95": -.06}},
	}
	for _, p := range core {
		ps.SavePersona(p)
		ps.AddMembership(host.ID, p.ID)
	}
	r.mu.Lock()
	r.populationMaterialized[host.ID] = true
	r.mu.Unlock()
	return ps.ListHostPersonas(host.ID), true
}

func (r *Repository) ListPosts(hostID string) []world.Post {
	// Reads must not create world history. A successful CONNECT explicitly starts
	// host observation; host-program index/body reads use the observation barriers
	// when they need data that has not finished materializing yet.
	return r.repairDevelopmentPendingReplySubjects(hostID, r.Base.ListPosts(hostID))
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

func developmentMaterializationBoardCatalog() []world.Board {
	return []world.Board{
		{ID: "1", Name: "フリートーク"},
		{ID: "2", Name: "パソコン通信・モデム"},
		{ID: "3", Name: "地域の話題"},
		{ID: "4", Name: "ゲーム"},
		{ID: "5", Name: "音楽"},
		{ID: "6", Name: "ソフトウェア"},
	}
}

func (r *Repository) MaterializationBoards(host world.Host) ([]world.Board, bool) {
	bs, ok := r.Base.(world.BoardStore)
	if !ok {
		return nil, false
	}
	catalog := developmentMaterializationBoardCatalog()
	desired := catalog[:3]
	if developmentInteractiveTitleFirstEnabled(r) {
		desired = catalog
	}
	if existing := bs.ListBoards(host.ID); len(existing) > 0 {
		if !developmentInteractiveTitleFirstEnabled(r) {
			return existing, false
		}
		seen := make(map[string]bool, len(existing))
		for _, board := range existing {
			seen[board.ID] = true
		}
		merged := append([]world.Board(nil), existing...)
		changed := false
		for _, board := range desired {
			if seen[board.ID] {
				continue
			}
			merged = append(merged, board)
			seen[board.ID] = true
			changed = true
		}
		if changed {
			bs.SaveBoards(host.ID, merged)
			return merged, true
		}
		return existing, false
	}
	boards := append([]world.Board(nil), desired...)
	bs.SaveBoards(host.ID, boards)
	return boards, true
}

// Compatibility entry point: there is no separate fixed three-header fixture.
func (r *Repository) MaterializationArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	return r.MaterializationPersonaArticleHeaders(host, board)
}

// Compatibility entry point delegates to the same causal lazy renderer used by
// the development diagnostics, without exposing diagnostics to legacy callers.
func (r *Repository) MaterializationArticle(host world.Host, board world.Board, postID int64) (world.Post, bool, bool) {
	post, found, created, _ := r.MaterializationArticleWithDebug(host, board, postID)
	return post, found, created
}

// Generic non-development boards can still ask the renderer for one post using
// the actual board name as an open cue. No hard-coded subject/body fallback is
// used: renderer failure leaves the board empty and can be retried later.
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
	posts, err := r.Materializer.GenerateBoardPosts(ctx, BoardMaterializationRequest{Host: host, BoardID: boardID, BoardTopic: boardTopic, WorldDate: r.WorldDate}, decision)
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

func incompleteHost(h world.Host) bool {
	return h.Name == "" || h.Software == "" || h.Lines <= 0 || h.MaxBaud <= 0
}

func completeDevelopmentHost(h world.Host) world.Host {
	if h.Name == "" {
		h.Name = "LAZY MATERIALIZE BBS"
	}
	if h.Region == "" {
		h.Region = "神奈川県"
	}
	if h.Software == "" {
		h.Software = "局固有の架空ホスト (development)"
	}
	if h.Lines <= 0 {
		h.Lines = 2
	}
	if h.MaxBaud <= 0 {
		h.MaxBaud = 14400
	}
	if h.Members <= 0 {
		h.Members = 48
	}
	if h.Popularity <= 0 {
		h.Popularity = .22
	}
	h.GuestAllowed = true
	h.TelehoFriendly = true
	return h
}

func worldTime(v string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return time.Now()
	}
	return t.Add(22*time.Hour + 15*time.Minute)
}
