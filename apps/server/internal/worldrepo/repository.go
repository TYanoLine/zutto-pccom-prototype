package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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

	mu                     sync.Mutex
	materialized           map[string]bool
	hosts                  map[string]world.Host
	hostMaterialized       map[string]bool
	populationMaterialized map[string]bool
}

func New(base world.Store, engine EvidenceResolver, materializer Materializer, worldDate string) *Repository {
	return &Repository{
		Base:                   base,
		Engine:                 engine,
		Materializer:           materializer,
		WorldDate:              worldDate,
		materialized:           map[string]bool{},
		hosts:                  map[string]world.Host{},
		hostMaterialized:       map[string]bool{},
		populationMaterialized: map[string]bool{},
	}
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

	// The development host demonstrates the intended split: first observation
	// fixes a small core cast, while the host's full member count remains only a
	// population fact. Hundreds of dormant accounts do not need detailed personas.
	if h.SoftwareID == "materialization-demo" {
		_, _ = r.MaterializationPersonas(h)
	}
	return h, nil
}

// HostWasMaterialized is exposed for the development-only materialization host.
// Historical host runtimes do not need to render this internal state.
func (r *Repository) HostWasMaterialized(hostID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hostMaterialized[hostID]
}

// PopulationWasMaterialized reports whether the core cast was first committed
// during this repository lifetime. It exists only so the development host can
// make lazy materialization visible to a human tester.
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
		{ID: host.ID + "-sysop", Handle: "SYSOP", Age: 34, Gender: "male", Occupation: "会社員 / SYSOP", ActivityPattern: "平日22:00-01:00、週末は深夜まで", ReplyTendency: .72, ThreadStartTendency: .32, LurkerTendency: .04, NewcomerOpenness: .82, Argumentativeness: .18, WritingStyle: "簡潔で面倒見がよい。運営連絡は端的。顔文字は少なめ。", Interests: map[string]float64{"bbs": 1, "pc98": .76, "local": .62}, Opinions: map[string]float64{"newcomers": .66}},
		{ID: host.ID + "-neko", Handle: "NEKO", Age: 24, Gender: "female", Occupation: "会社員", ActivityPattern: "23:00-02:00中心", ReplyTendency: .70, ThreadStartTendency: .42, LurkerTendency: .18, NewcomerOpenness: .74, Argumentativeness: .22, WritingStyle: "短めで砕けた口調。質問を投げることが多く、顔文字は時々。", Interests: map[string]float64{"chat": .82, "local": .58, "games": .46}, Opinions: map[string]float64{"windows95": .12}},
		{ID: host.ID + "-mari", Handle: "MARI", Age: 21, Gender: "female", Occupation: "短大生", ActivityPattern: "21:00-00:30中心、週末は昼も接続", ReplyTendency: .62, ThreadStartTendency: .30, LurkerTendency: .24, NewcomerOpenness: .88, Argumentativeness: .08, WritingStyle: "柔らかい口調。初対面にも返事をしやすく、(^^) 系を時々使う。", Interests: map[string]float64{"chat": .76, "music": .61, "local": .70}, Opinions: map[string]float64{"offline_meetings": .54}},
		{ID: host.ID + "-nori", Handle: "NORI", Age: 29, Gender: "male", Occupation: "技術職", ActivityPattern: "0:00-03:00中心", ReplyTendency: .48, ThreadStartTendency: .36, LurkerTendency: .34, NewcomerOpenness: .40, Argumentativeness: .38, WritingStyle: "やや長めで具体的。技術話では引用を使い、曖昧な断定を嫌う。", Interests: map[string]float64{"pc98": .91, "modem": .88, "software": .84}, Opinions: map[string]float64{"windows95": -.18}},
		{ID: host.ID + "-yuki", Handle: "YUKI", Age: 19, Gender: "male", Occupation: "大学生", ActivityPattern: "22:30-02:30中心", ReplyTendency: .56, ThreadStartTendency: .54, LurkerTendency: .16, NewcomerOpenness: .64, Argumentativeness: .31, WritingStyle: "勢いのある短文が多い。雑談では(笑)をたまに使う。", Interests: map[string]float64{"games": .86, "music": .48, "chat": .66}, Opinions: map[string]float64{"offline_meetings": .71}},
		{ID: host.ID + "-taka", Handle: "TAKA", Age: 27, Gender: "male", Occupation: "営業職", ActivityPattern: "平日は23:30前後、接続しない日も多い", ReplyTendency: .36, ThreadStartTendency: .18, LurkerTendency: .57, NewcomerOpenness: .46, Argumentativeness: .14, WritingStyle: "普段はROM気味。書く時は2-4行程度で、顔文字はほぼ使わない。", Interests: map[string]float64{"local": .72, "pc98": .44, "games": .35}, Opinions: map[string]float64{"windows95": -.06}},
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

// ListPosts is the legacy Store boundary. When a known host has no posts at all,
// materialize a minimal default board once. More capable host runtimes can use
// BoardPostStore below to request a specific, already-known board lazily.
func (r *Repository) ListPosts(hostID string) []world.Post {
	if existing := r.Base.ListPosts(hostID); len(existing) > 0 {
		return existing
	}
	r.mu.Lock()
	h, known := r.hosts[hostID]
	r.mu.Unlock()
	if known && h.SoftwareID != "materialization-demo" {
		_ = r.ensureBoard(h, "main", "フリートーク")
	}
	return r.Base.ListPosts(hostID)
}

func (r *Repository) AddPost(hostID string, p world.Post) world.Post {
	return r.Base.AddPost(hostID, p)
}

// ListBoardPosts is invoked only after a host-program runtime has established
// that the board path is real. Empty storage therefore means an unmaterialized
// known board, not permission to invent a nonexistent board.
func (r *Repository) ListBoardPosts(host world.Host, boardID, boardTopic string) []world.Post {
	if existing := filterBoard(r.Base.ListPosts(host.ID), boardID); len(existing) > 0 {
		return existing
	}
	_ = r.ensureBoard(host, boardID, boardTopic)
	return filterBoard(r.Base.ListPosts(host.ID), boardID)
}

// MaterializationBoards creates and stores only the board catalog. No article
// prose is generated here; this mirrors the intended lazy hierarchy.
func (r *Repository) MaterializationBoards(host world.Host) ([]world.Board, bool) {
	bs, ok := r.Base.(world.BoardStore)
	if !ok {
		return nil, false
	}
	if existing := bs.ListBoards(host.ID); len(existing) > 0 {
		return existing, false
	}
	boards := []world.Board{{ID: "1", Name: "フリートーク"}, {ID: "2", Name: "パソコン通信・モデム"}, {ID: "3", Name: "地域の話題"}}
	bs.SaveBoards(host.ID, boards)
	return boards, true
}

// MaterializationArticleHeaders creates the canonical post envelope when a board
// is first entered: actor, time, subject and semantic intent are committed first.
// Prose remains empty until the article itself is read.
func (r *Repository) MaterializationArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}
	personas, _ := r.MaterializationPersonas(host)
	stamp := worldTime(r.WorldDate)

	type headerTemplate struct {
		handle  string
		subject string
		intent  world.PostIntent
	}
	templates := []headerTemplate{
		{handle: "NEKO", subject: board.Name + "、どうです？", intent: world.PostIntent{Action: "thread_start", Topic: board.Name, Motivation: "最近の話題を軽く振って常連の反応を見たい", Stance: "好奇心が強く、他の人の意見を聞きたい"}},
		{handle: "MARI", subject: "はじめまして", intent: world.PostIntent{Action: "thread_start", Topic: board.Name, Motivation: "このボードではまだあまり書いていないので挨拶したい", Stance: "控えめだが友好的"}},
		{handle: "SYSOP", subject: "このボードについて", intent: world.PostIntent{Action: "announcement", Topic: board.Name, Motivation: "ボードの使い方と雰囲気を簡単に案内したい", Stance: "運営者として穏やかに案内する"}},
	}
	out := make([]world.Post, 0, len(templates))
	for i, t := range templates {
		persona, ok := personaByHandle(personas, t.handle)
		p := world.Post{BoardID: board.ID, Author: t.handle, Subject: t.subject, Intent: t.intent, CreatedAt: stamp.Add(time.Duration(i) * 17 * time.Minute)}
		if ok {
			p.Author = persona.Handle
			p.AuthorPersonaID = persona.ID
		}
		p = r.Base.AddPost(host.ID, p)
		out = append(out, p)
	}
	return out, true
}

// MaterializationArticle completes article prose only when the article is read.
// The actor, subject and semantic intent selected earlier stay canonical; the LLM
// is only allowed to render those already-committed facts into period prose.
func (r *Repository) MaterializationArticle(host world.Host, board world.Board, postID int64) (world.Post, bool, bool) {
	var selected world.Post
	found := false
	for _, p := range r.Base.ListPosts(host.ID) {
		if p.ID == postID && p.BoardID == board.ID {
			selected = p
			found = true
			break
		}
	}
	if !found {
		return world.Post{}, false, false
	}
	if selected.Body != "" {
		return selected, true, false
	}
	if r.Engine == nil || r.Materializer == nil {
		return selected, true, false
	}

	var persona *world.Persona
	if ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {
		if p, found := ps.PersonaByID(selected.AuthorPersonaID); found {
			copy := p
			persona = &copy
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     board.Name,
		WorldDate:   r.WorldDate,
		Region:      host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボード、%sによる件名『%s』の記事本文を、確定済みの投稿意図を変えず1996年の自然なパソコン通信文体で補完する", host.Name, board.Name, selected.Author, selected.Subject),
		Persistence: true,
		Importance:  .30,
		Specificity: .30,
	})
	if err != nil {
		return selected, true, false
	}
	posts, err := r.Materializer.GenerateBoardPosts(ctx, BoardMaterializationRequest{
		Host:             host,
		BoardID:          board.ID,
		BoardTopic:       selected.Subject,
		WorldDate:        r.WorldDate,
		Persona:          persona,
		Intent:           selected.Intent,
		CanonicalSubject: selected.Subject,
	}, decision)
	if err != nil || len(posts) == 0 {
		return selected, true, false
	}
	selected.Body = posts[0].Body
	if selected.Body == "" {
		return selected, true, false
	}
	if u, ok := r.Base.(world.PostUpdater); ok {
		updated, ok := u.UpdatePost(host.ID, selected)
		if ok {
			return updated, true, true
		}
	}
	return selected, true, true
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
		return err
	}
	posts, err := r.Materializer.GenerateBoardPosts(ctx, BoardMaterializationRequest{Host: host, BoardID: boardID, BoardTopic: boardTopic, WorldDate: r.WorldDate}, decision)
	if err != nil {
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

// FallbackMaterializer is deterministic/non-AI. It keeps AI optional and gives
// the materialization pipeline a safe degradation path. A prose renderer can be
// swapped in without changing host-program runtimes or repository semantics.
type FallbackMaterializer struct{}

func (FallbackMaterializer) GenerateBoardPosts(_ context.Context, r BoardMaterializationRequest, d worldengine.EvidenceDecision) ([]world.Post, error) {
	if d.Level == historicalkb.EvidenceVerified && !d.Knowledge.CanUse {
		return nil, fmt.Errorf("verified historical knowledge unavailable")
	}
	author := "GUEST"
	if r.Persona != nil && r.Persona.Handle != "" {
		author = r.Persona.Handle
	}
	subject := r.BoardTopic + "の話"
	if r.CanonicalSubject != "" {
		subject = r.CanonicalSubject
	}
	return []world.Post{{Author: author, Subject: subject, Body: "このボード、まだ書き込み少ないですね(^^;\r\nとりあえず足あとだけ残しておきます。", CreatedAt: worldTime(r.WorldDate)}}, nil
}

func worldTime(v string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return time.Now()
	}
	return t.Add(22*time.Hour + 15*time.Minute)
}
