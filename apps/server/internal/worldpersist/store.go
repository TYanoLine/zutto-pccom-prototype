package worldpersist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"zutto-pccom/apps/server/internal/world"
)

const persistTimeout = 5 * time.Second

type snapshotBackend interface {
	Load(context.Context, string) ([]byte, bool, error)
	Save(context.Context, string, string, int, []byte) error
	Close()
}

type Status struct {
	Enabled     bool      `json:"enabled"`
	Backend     string    `json:"backend"`
	Loaded      bool      `json:"loaded"`
	LastSavedAt time.Time `json:"last_saved_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// HostTarget selects a host (by phone) whose materialized world state should
// survive process restarts. Only the evolving in-world state (boards, posts,
// memberships, personas and persona facts) is restored from the snapshot. The
// host definition is immutable and always comes from the preset: the snapshot's
// copy of the host is never read back.
type HostTarget struct {
	Phone string
}

type persistedHost struct {
	phone  string
	hostID string
}

// Store keeps the existing MemoryStore behavior while durably snapshotting a
// deliberately small set of hosts that opt in with debug.snapshot. It is a
// development stopgap: the intentionally narrow scope avoids pretending this is
// the final normalized production world database.
type Store struct {
	*world.MemoryStore

	targets map[string]persistedHost
	backend snapshotBackend

	persistMu sync.Mutex

	batchMu    sync.Mutex
	batchDepth map[string]int
	batchDirty map[string]bool

	statusMu sync.RWMutex
	status   Status
}

func Open(ctx context.Context, databaseURL string, base *world.MemoryStore, targets []HostTarget) (*Store, error) {
	if base == nil {
		return nil, errors.New("memory store is nil")
	}
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	backend, err := openPostgresBackend(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	store, err := newStore(ctx, base, targets, backend)
	if err != nil {
		backend.Close()
		return nil, err
	}
	return store, nil
}

func newStore(ctx context.Context, base *world.MemoryStore, targets []HostTarget, backend snapshotBackend) (*Store, error) {
	if len(targets) == 0 {
		return nil, errors.New("no snapshot persistence targets configured")
	}
	store := &Store{
		MemoryStore: base,
		targets:     make(map[string]persistedHost, len(targets)),
		backend:     backend,
		batchDepth:  make(map[string]int),
		batchDirty:  make(map[string]bool),
		status:      Status{Enabled: true, Backend: "postgres-jsonb"},
	}
	seenPhones := make(map[string]bool, len(targets))
	for _, configured := range targets {
		if configured.Phone == "" {
			return nil, errors.New("snapshot persistence target phone is empty")
		}
		if seenPhones[configured.Phone] {
			return nil, fmt.Errorf("duplicate snapshot persistence target phone %s", configured.Phone)
		}
		seenPhones[configured.Phone] = true

		host, err := base.HostByPhone(configured.Phone)
		if err != nil {
			return nil, fmt.Errorf("find snapshot persistence host %s: %w", configured.Phone, err)
		}
		if _, exists := store.targets[host.ID]; exists {
			return nil, fmt.Errorf("duplicate snapshot persistence host id %s", host.ID)
		}
		target := persistedHost{
			phone:  configured.Phone,
			hostID: host.ID,
		}
		store.targets[host.ID] = target

		data, found, err := backend.Load(ctx, host.ID)
		if err != nil {
			return nil, fmt.Errorf("load snapshot for host %s: %w", host.ID, err)
		}
		if !found {
			continue
		}
		var snapshot world.DevelopmentHostSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("decode snapshot for host %s: %w", host.ID, err)
		}
		// The host definition is immutable and owned by the preset. Whatever
		// host the snapshot carries (older snapshots stored one) is discarded;
		// only the evolving world state is restored.
		snapshot.Host = host
		if err := base.RestoreDevelopmentSnapshot(snapshot); err != nil {
			return nil, fmt.Errorf("restore snapshot for host %s: %w", host.ID, err)
		}
		store.status.Loaded = true
	}
	return store, nil
}

func (s *Store) Close() {
	if s != nil && s.backend != nil {
		s.backend.Close()
	}
}

func (s *Store) DevelopmentPersistenceStatus() Status {
	if s == nil {
		return Status{}
	}
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

func (s *Store) AddPost(hostID string, post world.Post) world.Post {
	post = s.MemoryStore.AddPost(hostID, post)
	if target, ok := s.targets[hostID]; ok {
		s.persistTargetOrMarkBatchDirty(target)
	}
	return post
}

// BeginPostBatch / EndPostBatch coalesce snapshot persistence for one already
// planned canonical article batch. Calls are nestable per host because demanded
// board work may overlap a background prefetch on the same experiment station.
func (s *Store) BeginPostBatch(hostID string) {
	if _, ok := s.targets[hostID]; !ok {
		return
	}
	s.batchMu.Lock()
	s.batchDepth[hostID]++
	s.batchMu.Unlock()
}

func (s *Store) EndPostBatch(hostID string) {
	target, targeted := s.targets[hostID]
	if !targeted {
		return
	}

	shouldPersist := false
	s.batchMu.Lock()
	depth := s.batchDepth[hostID]
	if depth > 1 {
		s.batchDepth[hostID] = depth - 1
	} else if depth == 1 {
		delete(s.batchDepth, hostID)
		shouldPersist = s.batchDirty[hostID]
		delete(s.batchDirty, hostID)
	}
	s.batchMu.Unlock()

	if shouldPersist {
		s.persistTarget(target)
	}
}

func (s *Store) persistTargetOrMarkBatchDirty(target persistedHost) {
	s.batchMu.Lock()
	if s.batchDepth[target.hostID] > 0 {
		s.batchDirty[target.hostID] = true
		s.batchMu.Unlock()
		return
	}
	s.batchMu.Unlock()
	s.persistTarget(target)
}

func (s *Store) UpdatePost(hostID string, post world.Post) (world.Post, bool) {
	updated, ok := s.MemoryStore.UpdatePost(hostID, post)
	if ok {
		if target, targeted := s.targets[hostID]; targeted {
			s.persistTarget(target)
		}
	}
	return updated, ok
}

func (s *Store) ClearHostPosts(hostID string) int {
	count := s.MemoryStore.ClearHostPosts(hostID)
	if target, ok := s.targets[hostID]; ok {
		s.persistTarget(target)
	}
	return count
}

func (s *Store) ReplaceHostPosts(hostID string, posts []world.Post) int {
	removed := s.MemoryStore.ReplaceHostPosts(hostID, posts)
	if target, ok := s.targets[hostID]; ok {
		s.persistTarget(target)
	}
	return removed
}

func (s *Store) SaveBoards(hostID string, boards []world.Board) {
	s.MemoryStore.SaveBoards(hostID, boards)
	if target, ok := s.targets[hostID]; ok {
		s.persistTarget(target)
	}
}


func (s *Store) SaveBoardActivityState(hostID string, state world.BoardActivityState) {
	// Activity planning must stay cheap enough for board-menu rendering. The
	// deterministic plan is recomputable before any prose exists; once article
	// materialization writes a post, the normal AddPost snapshot persists the
	// activity state together with the resulting canonical history.
	s.MemoryStore.SaveBoardActivityState(hostID, state)
}

func (s *Store) SavePersona(persona world.Persona) {
	s.MemoryStore.SavePersona(persona)
	// New personas are normally followed by AddMembership, which persists the
	// first observable snapshot. Existing member persona updates also need to be
	// durable, so persist any target that already owns this persona.
	s.persistTargets(s.targetsForPersonaIDs([]string{persona.ID}))
}

func (s *Store) AddMembership(hostID, personaID string) {
	s.MemoryStore.AddMembership(hostID, personaID)
	if target, ok := s.targets[hostID]; ok {
		s.persistTarget(target)
	}
}

func (s *Store) SavePersonaFact(fact world.PersonaFact) {
	s.MemoryStore.SavePersonaFact(fact)
	s.persistTargets(s.targetsForPersonaIDs([]string{fact.PersonaID}))
}

func (s *Store) ClearPersonaFacts(personaIDs []string) int {
	count := s.MemoryStore.ClearPersonaFacts(personaIDs)
	s.persistTargets(s.targetsForPersonaIDs(personaIDs))
	return count
}

func (s *Store) targetsForPersonaIDs(personaIDs []string) []persistedHost {
	if len(personaIDs) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(personaIDs))
	for _, personaID := range personaIDs {
		wanted[personaID] = struct{}{}
	}
	out := make([]persistedHost, 0, len(s.targets))
	for _, target := range s.targets {
		snapshot, ok := s.MemoryStore.DevelopmentSnapshot(target.phone)
		if !ok {
			continue
		}
		for _, personaID := range snapshot.Memberships {
			if _, ok := wanted[personaID]; ok {
				out = append(out, target)
				break
			}
		}
	}
	return out
}

func (s *Store) persistTargets(targets []persistedHost) {
	for _, target := range targets {
		s.persistTarget(target)
	}
}

func (s *Store) persistTarget(target persistedHost) {
	if s == nil || s.backend == nil {
		return
	}
	s.persistMu.Lock()
	defer s.persistMu.Unlock()

	snapshot, ok := s.MemoryStore.DevelopmentSnapshot(target.phone)
	if !ok {
		return
	}
	data, err := json.Marshal(snapshot)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		err = s.backend.Save(ctx, target.hostID, target.phone, snapshot.SchemaVersion, data)
		cancel()
	}

	s.statusMu.Lock()
	if err != nil {
		s.status.LastError = err.Error()
		log.Printf("development world persistence failed for host %s: %v", target.hostID, err)
	} else {
		s.status.LastSavedAt = time.Now().UTC()
		s.status.LastError = ""
	}
	s.statusMu.Unlock()
}

type postgresBackend struct{ pool *pgxpool.Pool }

func openPostgresBackend(ctx context.Context, databaseURL string) (*postgresBackend, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse development persistence database config: %w", err)
	}
	// Snapshot writes are synchronous and tiny. One connection is sufficient and
	// avoids needlessly expanding connection usage on the free development DB.
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open development persistence postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping development persistence postgres: %w", err)
	}
	backend := &postgresBackend{pool: pool}
	if err := backend.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return backend, nil
}

func (b *postgresBackend) ensureSchema(ctx context.Context) error {
	_, err := b.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS development_world_snapshots (
		host_id text PRIMARY KEY,
		phone text NOT NULL,
		schema_version integer NOT NULL,
		snapshot jsonb NOT NULL,
		updated_at timestamptz NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("ensure development persistence schema: %w", err)
	}
	return nil
}

func (b *postgresBackend) Load(ctx context.Context, hostID string) ([]byte, bool, error) {
	var data []byte
	err := b.pool.QueryRow(ctx, `SELECT snapshot FROM development_world_snapshots WHERE host_id=$1`, hostID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (b *postgresBackend) Save(ctx context.Context, hostID, phone string, schemaVersion int, data []byte) error {
	_, err := b.pool.Exec(ctx, `INSERT INTO development_world_snapshots (host_id, phone, schema_version, snapshot, updated_at)
		VALUES ($1,$2,$3,$4::jsonb,now())
		ON CONFLICT (host_id) DO UPDATE SET phone=EXCLUDED.phone, schema_version=EXCLUDED.schema_version, snapshot=EXCLUDED.snapshot, updated_at=now()`,
		hostID, phone, schemaVersion, string(data))
	return err
}

func (b *postgresBackend) Close() {
	if b != nil && b.pool != nil {
		b.pool.Close()
	}
}
