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

// Store keeps the existing MemoryStore behavior while durably snapshotting the
// development materialization host. The intentionally narrow scope avoids
// pretending this is the final normalized production world database.
type Store struct {
	*world.MemoryStore

	phone   string
	hostID  string
	backend snapshotBackend

	persistMu sync.Mutex
	statusMu  sync.RWMutex
	status    Status
}

func Open(ctx context.Context, databaseURL string, base *world.MemoryStore, phone string) (*Store, error) {
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
	store, err := newStore(ctx, base, phone, backend)
	if err != nil {
		backend.Close()
		return nil, err
	}
	return store, nil
}

func newStore(ctx context.Context, base *world.MemoryStore, phone string, backend snapshotBackend) (*Store, error) {
	host, err := base.HostByPhone(phone)
	if err != nil {
		return nil, fmt.Errorf("find development host: %w", err)
	}
	store := &Store{
		MemoryStore: base,
		phone:       phone,
		hostID:      host.ID,
		backend:     backend,
		status:      Status{Enabled: true, Backend: "postgres-jsonb"},
	}
	data, found, err := backend.Load(ctx, host.ID)
	if err != nil {
		return nil, fmt.Errorf("load development snapshot: %w", err)
	}
	if !found {
		return store, nil
	}
	var snapshot world.DevelopmentHostSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("decode development snapshot: %w", err)
	}
	if snapshot.Host.ID != host.ID || snapshot.Host.Phone != phone {
		return nil, errors.New("development snapshot identity does not match configured host")
	}
	if err := base.RestoreDevelopmentSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("restore development snapshot: %w", err)
	}
	store.status.Loaded = true
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

func (s *Store) SaveHost(host world.Host) {
	s.MemoryStore.SaveHost(host)
	if host.ID == s.hostID {
		s.persist()
	}
}

func (s *Store) AddPost(hostID string, post world.Post) world.Post {
	post = s.MemoryStore.AddPost(hostID, post)
	if hostID == s.hostID {
		s.persist()
	}
	return post
}

func (s *Store) UpdatePost(hostID string, post world.Post) (world.Post, bool) {
	updated, ok := s.MemoryStore.UpdatePost(hostID, post)
	if ok && hostID == s.hostID {
		s.persist()
	}
	return updated, ok
}

func (s *Store) ClearHostPosts(hostID string) int {
	count := s.MemoryStore.ClearHostPosts(hostID)
	if hostID == s.hostID {
		s.persist()
	}
	return count
}

func (s *Store) SaveBoards(hostID string, boards []world.Board) {
	s.MemoryStore.SaveBoards(hostID, boards)
	if hostID == s.hostID {
		s.persist()
	}
}

func (s *Store) SavePersona(persona world.Persona) {
	// Membership is the canonical link into this host snapshot. Persisting here
	// would write an identical snapshot before AddMembership makes it observable.
	s.MemoryStore.SavePersona(persona)
}

func (s *Store) AddMembership(hostID, personaID string) {
	s.MemoryStore.AddMembership(hostID, personaID)
	if hostID == s.hostID {
		s.persist()
	}
}

func (s *Store) SavePersonaFact(fact world.PersonaFact) {
	s.MemoryStore.SavePersonaFact(fact)
	if s.affectsDevelopmentHost([]string{fact.PersonaID}) {
		s.persist()
	}
}

func (s *Store) ClearPersonaFacts(personaIDs []string) int {
	count := s.MemoryStore.ClearPersonaFacts(personaIDs)
	if s.affectsDevelopmentHost(personaIDs) {
		s.persist()
	}
	return count
}

func (s *Store) affectsDevelopmentHost(personaIDs []string) bool {
	if len(personaIDs) == 0 {
		return false
	}
	snapshot, ok := s.MemoryStore.DevelopmentSnapshot(s.phone)
	if !ok {
		return false
	}
	members := make(map[string]struct{}, len(snapshot.Memberships))
	for _, personaID := range snapshot.Memberships {
		members[personaID] = struct{}{}
	}
	for _, personaID := range personaIDs {
		if _, ok := members[personaID]; ok {
			return true
		}
	}
	return false
}

func (s *Store) persist() {
	if s == nil || s.backend == nil {
		return
	}
	s.persistMu.Lock()
	defer s.persistMu.Unlock()

	snapshot, ok := s.MemoryStore.DevelopmentSnapshot(s.phone)
	if !ok {
		return
	}
	data, err := json.Marshal(snapshot)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		err = s.backend.Save(ctx, s.hostID, s.phone, snapshot.SchemaVersion, data)
		cancel()
	}

	s.statusMu.Lock()
	if err != nil {
		s.status.LastError = err.Error()
		log.Printf("development world persistence failed: %v", err)
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
