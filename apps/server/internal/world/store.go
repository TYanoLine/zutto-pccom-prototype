package world

import (
	"context"
	"errors"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
)

type Store interface {
	HostByPhone(phone string) (Host, error)
	ListPosts(hostID string) []Post
	AddPost(hostID string, p Post) Post
}

type HostDetailStore interface {
	HostDetail(hostID string) (hostcatalog.PresetDetail, bool)
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

type PostUpdater interface {
	UpdatePost(hostID string, p Post) (Post, bool)
}

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
	mu            sync.RWMutex
	hosts         map[string]Host
	boards        map[string][]Board
	boardActivity map[string]map[string]BoardActivityState
	posts         map[string][]Post
	personas      map[string]Persona
	personaFacts  map[string][]PersonaFact
	memberships   map[string][]string
	populations   map[string]hostcatalog.Population
	details       map[string]hostcatalog.PresetDetail
	// directory is the dialing directory: the listed hosts, from the presets.
	directory []DirectoryEntry
	next      int64
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		hosts:         map[string]Host{},
		boards:        map[string][]Board{},
		boardActivity: map[string]map[string]BoardActivityState{},
		posts:         map[string][]Post{},
		personas:      map[string]Persona{},
		personaFacts:  map[string][]PersonaFact{},
		memberships:   map[string][]string{},
		populations:   map[string]hostcatalog.Population{},
		details:       map[string]hostcatalog.PresetDetail{},
		next:          1000,
	}

	// Host definitions and resident populations come from the embedded YAML presets.
	populations := presetPopulations()
	for key, detail := range presetDetails() {
		s.details[key] = detail
	}
	for _, h := range presetHosts() {
		s.hosts[h.Phone] = h
		if h.IsExperiment() {
			s.posts[h.ID] = nil
		}
		if spec, ok := populations[h.ID]; ok {
			s.populations[h.ID] = spec
			ensurePopulationLocked(s, h, spec)
		}
	}
	s.directory = presetDirectory()

	return s
}

func (s *MemoryStore) HostDetail(hostID string) (hostcatalog.PresetDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.details[hostID]
	return d, ok
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
