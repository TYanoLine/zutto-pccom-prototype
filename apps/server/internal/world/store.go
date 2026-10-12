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

// InteractionLedgerStore persists the append-only interaction ledger events.
type InteractionLedgerStore interface {
	AppendInteractionEvent(event InteractionEvent) error
	ListInteractionEvents(hostID string, limit int) []InteractionEvent
	GetInteractionEvent(eventID string) (InteractionEvent, bool)
}

// RelationshipStore manages directional relationship edges between actors.
type RelationshipStore interface {
	GetRelationship(hostID string, from, to ActorRef) (RelationshipEdge, bool)
	UpdateRelationship(edge RelationshipEdge) error
	ListRelationshipsFrom(hostID string, from ActorRef) []RelationshipEdge
}

// PresenceStore keeps observed connection sessions and live human calls. A
// session that was shown to a caller (WHO) or decided a caller's dial is world
// history and is never rewritten. See spec 015.
type PresenceStore interface {
	ListObservedSessions(hostID string, from, to time.Time) []PresenceSession
	// SaveObservedSessions stores sessions that are not stored yet; a session
	// already stored for the same actor and start is left unchanged.
	SaveObservedSessions(hostID string, sessions []PresenceSession)
	SaveHumanCall(call HumanCall)
	HumanCall(callID string) (HumanCall, bool)
	// EndHumanCall removes a live call and records it as an observed session
	// that ended at the given world time.
	EndHumanCall(callID string, at time.Time) (HumanCall, bool)
	ListHumanCalls(hostID string) []HumanCall
	// ListCallActivities returns the stored plans of calls that started in
	// [from, to). A zero to means no upper bound.
	ListCallActivities(hostID string, from, to time.Time) []CallActivity
	// SaveCallActivities stores plans that are not stored yet; a plan already
	// stored for the same actor and call start is left unchanged.
	SaveCallActivities(hostID string, acts []CallActivity)
}

// ChatEventKind is a resident entering or leaving a chat room, or a line said
// in it.
type ChatEventKind string

const (
	ChatJoin  ChatEventKind = "join"
	ChatLeave ChatEventKind = "leave"
	ChatSay   ChatEventKind = "say"
)

// ChatEvent is a resident entering or leaving a chat room at a world time. A
// leave is also reported when the call ends while the resident is in the room.
// A say event is a recorded line, a human's or a resident's (spec 017).
type ChatEvent struct {
	At     time.Time
	Kind   ChatEventKind
	Room   int
	Actor  ActorRef
	Handle string
	// Body and UtteranceID are set on a say event.
	Body        string
	UtteranceID string
}

// ChatHost gives host programs what residents do within their calls and in
// chat rooms (spec 016). It is shared by every host program: the names and
// arguments carry no program's screen wording. Host programs use it through a
// type assertion, like PresenceObserver.
type ChatHost interface {
	// CallActivitiesNow returns the plans of the residents on the host's lines
	// now, saving them as observed.
	CallActivitiesNow(host Host) []CallActivity
	// ChatEvents returns room's join and leave events and recorded lines in
	// (from, to], in time order, saving the plans they come from as observed.
	ChatEvents(host Host, room int, from, to time.Time) []ChatEvent
	// RecordChatUtterance saves what a caller said in a room and records it
	// in the ledger, addressed to the residents in the room at u.At.
	RecordChatUtterance(host Host, u ChatUtterance) (ChatUtterance, error)
	// SendLineMessage saves a line message and records it in the ledger,
	// addressed to its recipient. The caller sets DeliverAt.
	SendLineMessage(host Host, m LineMessage) (LineMessage, error)
	// EnterChatRoom and LeaveChatRoom tell the world a caller is in a room.
	// Residents' lines are written only while a caller is there (spec 017).
	EnterChatRoom(host Host, room int, actor ActorRef, handle string, at time.Time)
	LeaveChatRoom(host Host, room int, actor ActorRef, at time.Time)
}

// LineOccupancy decides and registers line use on behalf of the telephone and
// call-session layers. Implementations compute resident presence, so they live
// above the plain stores.
type LineOccupancy interface {
	// DialLine reports the lowest free line at world time at, or ok=false when
	// every line is in use. The sessions that decided it become observed.
	DialLine(host Host, at time.Time) (line int, ok bool)
	// BeginHumanCall re-checks the line and registers the call at the current
	// world time; ok=false means the line was taken in the meantime.
	BeginHumanCall(host Host, callID, humanID string, baud int) (line int, ok bool)
	SetHumanCallHandle(callID, handle string)
	EndHumanCall(callID string)
}

// PresenceObserver is what host runtimes use: the sessions holding a line of
// the host now, saved as observed world history.
type PresenceObserver interface {
	ObservePresenceNow(host Host) []PresenceSession
}

// WorldTimeSource is implemented by stores that know the world clock. Host
// runtimes stamp what the human writes with it, so human posts share the
// world's timeline with resident activity.
type WorldTimeSource interface {
	WorldNow() time.Time
}

// WorldNowOf returns the world time of store, or the real time for stores
// without a world clock (plain in-memory stores in tests and tools).
func WorldNowOf(store any) time.Time {
	if source, ok := store.(WorldTimeSource); ok {
		return source.WorldNow()
	}
	return time.Now()
}

// DevelopmentConversationResetStore is intentionally a development-only escape
// hatch. It lets the materialization demo clear observed posts and lazily-created
// persona details so a tester can compare generation behavior without rebuilding
// the core cast or host profile.
type DevelopmentConversationResetStore interface {
	ClearHostPosts(hostID string) int
	ClearPersonaFacts(personaIDs []string) int
}

type relationshipKey struct {
	hostID string
	from   ActorRef
	to     ActorRef
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
	eventsByID    map[string]InteractionEvent
	eventsByHost  map[string][]InteractionEvent
	relationships map[relationshipKey]RelationshipEdge
	// presence holds observed sessions per host; humanCalls the live calls.
	presence   map[string][]PresenceSession
	humanCalls map[string]HumanCall
	// callPlans holds observed resident call plans per host (spec 016).
	callPlans map[string][]CallActivity
	// chatLines and lineMessages are what callers said in chat rooms and sent
	// to lines (spec 016).
	chatLines    map[string][]ChatUtterance
	lineMessages map[string][]LineMessage
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
		eventsByID:    map[string]InteractionEvent{},
		eventsByHost:  map[string][]InteractionEvent{},
		relationships: map[relationshipKey]RelationshipEdge{},
		presence:      map[string][]PresenceSession{},
		humanCalls:    map[string]HumanCall{},
		callPlans:     map[string][]CallActivity{},
		chatLines:     map[string][]ChatUtterance{},
		lineMessages:  map[string][]LineMessage{},
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

func (s *MemoryStore) AppendInteractionEvent(event InteractionEvent) error {
	if event.EventID == "" {
		return errors.New("event id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.eventsByID[event.EventID]; ok {
		return nil
	}
	s.eventsByID[event.EventID] = event
	s.eventsByHost[event.HostID] = append(s.eventsByHost[event.HostID], event)
	return nil
}

func (s *MemoryStore) ListInteractionEvents(hostID string, limit int) []InteractionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.eventsByHost[hostID]
	n := len(events)
	if n == 0 {
		return nil
	}
	count := n
	if limit > 0 && limit < count {
		count = limit
	}
	out := make([]InteractionEvent, 0, count)
	for i := n - 1; i >= 0 && len(out) < count; i-- {
		out = append(out, events[i])
	}
	return out
}

func (s *MemoryStore) GetInteractionEvent(eventID string) (InteractionEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.eventsByID[eventID]
	return e, ok
}

func (s *MemoryStore) GetRelationship(hostID string, from, to ActorRef) (RelationshipEdge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := relationshipKey{hostID: hostID, from: from, to: to}
	edge, ok := s.relationships[key]
	return edge, ok
}

func (s *MemoryStore) UpdateRelationship(edge RelationshipEdge) error {
	if edge.HostID == "" || edge.FromActor.IsZero() || edge.ToActor.IsZero() {
		return errors.New("invalid relationship edge")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := relationshipKey{hostID: edge.HostID, from: edge.FromActor, to: edge.ToActor}
	s.relationships[key] = edge
	return nil
}

func (s *MemoryStore) ListRelationshipsFrom(hostID string, from ActorRef) []RelationshipEdge {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []RelationshipEdge
	for k, edge := range s.relationships {
		if k.hostID == hostID && k.from == from {
			out = append(out, edge)
		}
	}
	return out
}

