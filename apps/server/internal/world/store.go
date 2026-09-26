package world

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Store interface {
	HostByPhone(phone string) (Host, error)
	ListPosts(hostID string) []Post
	AddPost(hostID string, p Post) Post
}

// BoardPostStore is an optional capability used by host runtimes after they have
// already established that the historical host-program board exists. Keeping it
// optional preserves simple stores while allowing a repository layer to lazily
// materialize content for known-present boards without teaching the host runtime
// anything about AI or research.
type BoardPostStore interface {
	ListBoardPosts(host Host, boardID, boardTopic string) []Post
}

// HostObservationStore separates host existence from observation. A successful
// dial may start catch-up in the background, while host-program reads wait only
// when the canonical headers/body they need are not ready yet.
//
// Implementations must single-flight concurrent callers for the same host/board
// or thread so observation never creates per-user copies of world history.
type HostObservationStore interface {
	BeginHostObservation(host Host, boards []Board)
	WaitForBoardHeaders(ctx context.Context, host Host, board Board) ([]Post, error)
	WaitForArticleBody(ctx context.Context, host Host, board Board, postID int64) (Post, bool, error)
}

// HostPrefetchStore is an optional low-priority observation queue. Prefetch work
// is serialized in queue order, while an explicit board demand may remove a
// queued board and start/join it immediately in parallel with the current
// background item.
type HostPrefetchStore interface {
	BeginHostPrefetch(host Host, boards []Board)
}

// The following writer capabilities are intentionally optional. WorldRepository
// uses them to persist materialized state without making every Store implementation
// support the development materialization demo.
type HostWriter interface{ SaveHost(Host) }
type BoardStore interface {
	ListBoards(hostID string) []Board
	SaveBoards(hostID string, boards []Board)
}

// BoardActivityStore exposes prose-free canonical board activity state. Host
// runtimes may use it to display counts before article headers are materialized.
type BoardActivityStore interface {
	BoardActivity(host Host, board Board) (BoardActivityState, bool)
}

// BoardActivityStateStore is the persistence capability used by Repository when
// it computes/refreshes one deterministic activity plan.
type BoardActivityStateStore interface {
	BoardActivityState(hostID, boardID string) (BoardActivityState, bool)
	SaveBoardActivityState(hostID string, state BoardActivityState)
	ListBoardActivityStates(hostID string) []BoardActivityState
}
// PostBatchStore is an optional persistence optimization for a canonical batch
// whose posts have already been fully planned/validated. The in-memory world
// mutations remain visible immediately; a snapshot-backed store may defer its
// durable snapshot until EndPostBatch so one world batch does not cause one
// database write per article.
type PostBatchStore interface {
	BeginPostBatch(hostID string)
	EndPostBatch(hostID string)
}

type PostUpdater interface{ UpdatePost(hostID string, p Post) (Post, bool) }

// PersonaStore keeps the global-persona / host-membership split explicit even in
// the in-memory PoC. A persona can later be attached to more than one host without
// cloning their identity or behavior profile.
type PersonaStore interface {
	ListHostPersonas(hostID string) []Persona
	SavePersona(Persona)
	PersonaByID(id string) (Persona, bool)
	AddMembership(hostID, personaID string)
}

// PersonaFactStore persists concrete persona details that are generated only
// when a topic/action needs them. Persona skeletons remain intentionally sparse.
type PersonaFactStore interface {
	ListPersonaFacts(personaID string) []PersonaFact
	SavePersonaFact(PersonaFact)
}

// DevelopmentConversationResetStore is intentionally a development-only escape
// hatch. It lets the materialization demo clear observed posts and lazily-created
// persona details so a tester can compare generation behavior without rebuilding
// the core cast or host profile.
type DevelopmentConversationResetStore interface {
	ClearHostPosts(hostID string) int
	ClearPersonaFacts(personaIDs []string) int
}

type MemoryStore struct {
	mu           sync.RWMutex
	hosts        map[string]Host
	boards       map[string][]Board
	boardActivity map[string]map[string]BoardActivityState
	posts        map[string][]Post
	personas     map[string]Persona
	personaFacts map[string][]PersonaFact
	memberships  map[string][]string
	next         int64
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		hosts:        map[string]Host{},
		boards:       map[string][]Board{},
		boardActivity: map[string]map[string]BoardActivityState{},
		posts:        map[string][]Post{},
		personas:     map[string]Persona{},
		personaFacts: map[string][]PersonaFact{},
		memberships:  map[string][]string{},
		next:         1000,
	}

	h := Host{ID: "moonlight-yokohama", Phone: "0451234567", Name: "YOKOHAMA MOONLIGHT NETWORK", Region: "神奈川県横浜市", Software: "KTBBS compatible / customized", SoftwareID: "generic", Lines: 4, Popularity: .70, MaxBaud: 14400, Members: 187, FoundedOn: "1994-06-12", ANSI: true, GuestAllowed: true, TelehoFriendly: true}
	s.hosts[h.Phone] = h
	s.posts[h.ID] = []Post{
		{ID: 1, BoardID: "main", Author: "SYSOP", Subject: "HDD増設しました", Body: "先週、HDDを340MBに増設しました。\r\nファイルボードも少し整理しています。", CreatedAt: time.Date(1996, 8, 25, 21, 14, 0, 0, time.Local)},
		{ID: 2, BoardID: "main", Author: "NEKO", Subject: "土曜のオフ", Body: "集合は18時に関内駅でいいんでしたっけ？(^^;", CreatedAt: time.Date(1996, 8, 26, 0, 42, 0, 0, time.Local)},
		{ID: 3, BoardID: "main", Author: "TAKA", Subject: "Win95どうです？", Body: "うちはまだ3.1です。98で使うには重い気もしますが…。", CreatedAt: time.Date(1996, 8, 26, 1, 7, 0, 0, time.Local)},
	}

	erika := Host{ID: "hakata-canal-net", Phone: "0920000196", Name: "HAKATA CANAL NET", Region: "福岡県福岡市", Software: "絵理香K版", SoftwareID: "erika-k", Lines: 3, Popularity: .58, MaxBaud: 14400, Members: 326, FoundedOn: "1994-11-03", ANSI: false, GuestAllowed: true, TelehoFriendly: true}
	s.hosts[erika.Phone] = erika
	s.posts[erika.ID] = nil
	ensureHakataExperimentPopulationLocked(s, erika)


	turbo := Host{ID: "silver-horizon-bbs", Phone: "0470001080", Name: "SILVER HORIZON BBS", Region: "千葉県", Software: "TurboBBS 1.08 compatible / customized", SoftwareID: "turbobbs", Lines: 1, Popularity: .18, MaxBaud: 2400, Members: 52, FoundedOn: "1989-08-20", ANSI: false, GuestAllowed: false, TelehoFriendly: true}
	s.hosts[turbo.Phone] = turbo
	s.posts[turbo.ID] = []Post{
		{ID: 601, BoardID: "1", Author: "SYSOP", Subject: "まだ動いてます", Body: "1980年代から手を入れながら使っているTurboBBSです。\r\n古い作りですが、のんびり使ってください(^^)", CreatedAt: time.Date(1996, 8, 24, 22, 18, 0, 0, time.Local)},
		{ID: 602, BoardID: "4", Author: "TARO YAMADA", Subject: "2400bpsモデム", Body: "高速局が増えましたが、このくらいの速度も落ち着きますね。\r\n巡回にはちょっと時間がかかります(^^;", CreatedAt: time.Date(1996, 8, 25, 0, 14, 0, 0, time.Local)},
		{ID: 603, BoardID: "2", Author: "MIKA", Subject: "98のDOS環境", Body: "CONFIG.SYSを整理したら空きメモリが少し増えました。\r\nまだDOSも手放せません。", CreatedAt: time.Date(1996, 8, 25, 21, 47, 0, 0, time.Local)},
		{ID: 604, BoardID: "8", Author: "KEN", Subject: "夏も終わりかな", Body: "夜は少し涼しくなってきましたね。\r\n電話代を気にしつつ、また深夜に来ます(笑)", CreatedAt: time.Date(1996, 8, 26, 1, 8, 0, 0, time.Local)},
	}

	s.hosts["0450000001"] = Host{ID: "quiet-test", Phone: "0450000001", Name: "QUIET TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, FoundedOn: "1996-05-05", ANSI: false, GuestAllowed: true}
	s.hosts["0459999999"] = Host{ID: "busy-test", Phone: "0459999999", Name: "POPULAR TEST BBS", Region: "神奈川県", Software: "BIG-Model compatible", SoftwareID: "generic", Lines: 1, Popularity: 1, MaxBaud: 14400, Members: 912, FoundedOn: "1993-09-15", ANSI: true, GuestAllowed: true}

	// Development-only seed: intentionally incomplete. WorldRepository fills the
	// missing profile when this number is first dialed, then stores the result.
	s.hosts["0450000196"] = Host{ID: "materialize-demo", Phone: "0450000196", Region: "神奈川県", SoftwareID: "materialization-demo"}
	return s
}

func (s *MemoryStore) HostByPhone(phone string) (Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hosts[phone]
	if !ok {
		return Host{}, errors.New("host not found")
	}
	return h, nil
}
func (s *MemoryStore) SaveHost(h Host) { s.mu.Lock(); defer s.mu.Unlock(); s.hosts[h.Phone] = h }
func (s *MemoryStore) ListPosts(hostID string) []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.posts[hostID]
	out := make([]Post, len(p))
	copy(out, p)
	return out
}
func (s *MemoryStore) ListBoardPosts(host Host, boardID, _ string) []Post {
	all := s.ListPosts(host.ID)
	out := make([]Post, 0)
	for _, p := range all {
		if p.BoardID == boardID {
			out = append(out, p)
		}
	}
	return out
}
func (s *MemoryStore) AddPost(hostID string, p Post) Post {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	p.ID = s.next
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.posts[hostID] = append(s.posts[hostID], p)
	return p
}
func (s *MemoryStore) UpdatePost(hostID string, p Post) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.posts[hostID] {
		if s.posts[hostID][i].ID == p.ID {
			s.posts[hostID][i] = p
			return p, true
		}
	}
	return Post{}, false
}
func (s *MemoryStore) ClearHostPosts(hostID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.posts[hostID])
	delete(s.posts, hostID)
	return count
}
func (s *MemoryStore) ReplaceHostPosts(hostID string, posts []Post) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldCount := len(s.posts[hostID])
	out := make([]Post, len(posts))
	copy(out, posts)
	s.posts[hostID] = out
	return oldCount - len(out)
}
func (s *MemoryStore) ListBoards(hostID string) []Board {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := s.boards[hostID]
	out := make([]Board, len(v))
	copy(out, v)
	return out
}
func (s *MemoryStore) SaveBoards(hostID string, boards []Board) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Board, len(boards))
	copy(out, boards)
	s.boards[hostID] = out
}


func (s *MemoryStore) BoardActivityState(hostID, boardID string) (BoardActivityState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.boardActivity[hostID][boardID]
	return state, ok
}

func (s *MemoryStore) SaveBoardActivityState(hostID string, state BoardActivityState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.boardActivity[hostID] == nil {
		s.boardActivity[hostID] = map[string]BoardActivityState{}
	}
	s.boardActivity[hostID][state.BoardID] = state
}

func (s *MemoryStore) ListBoardActivityStates(hostID string) []BoardActivityState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	states := s.boardActivity[hostID]
	out := make([]BoardActivityState, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	return out
}
func (s *MemoryStore) SavePersona(p Persona) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.personas[p.ID] = p
}
func (s *MemoryStore) PersonaByID(id string) (Persona, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.personas[id]
	return p, ok
}
func (s *MemoryStore) AddMembership(hostID, personaID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.memberships[hostID] {
		if existing == personaID {
			return
		}
	}
	s.memberships[hostID] = append(s.memberships[hostID], personaID)
}
func (s *MemoryStore) ListHostPersonas(hostID string) []Persona {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.memberships[hostID]
	out := make([]Persona, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.personas[id]; ok {
			out = append(out, p)
		}
	}
	return out
}
func (s *MemoryStore) ListPersonaFacts(personaID string) []PersonaFact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	facts := s.personaFacts[personaID]
	out := make([]PersonaFact, len(facts))
	copy(out, facts)
	return out
}
func (s *MemoryStore) SavePersonaFact(f PersonaFact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	facts := s.personaFacts[f.PersonaID]
	for i := range facts {
		if facts[i].Key == f.Key {
			facts[i] = f
			s.personaFacts[f.PersonaID] = facts
			return
		}
	}
	s.personaFacts[f.PersonaID] = append(facts, f)
}
func (s *MemoryStore) ClearPersonaFacts(personaIDs []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, id := range personaIDs {
		count += len(s.personaFacts[id])
		delete(s.personaFacts, id)
	}
	return count
}
