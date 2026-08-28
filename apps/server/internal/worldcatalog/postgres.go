package worldcatalog

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const GenerationVersion = 1

var worldKeyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Center struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	DialMode string `json:"dialMode"`
	MaxBaud  int    `json:"maxBaud"`
}

type Catalog struct {
	WorldID string
	Seed    int64
	Centers []Center
	Created bool
}

type Store struct { pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" { return nil, errors.New("DATABASE_URL is not set") }
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil { return nil, fmt.Errorf("open postgres: %w", err) }
	if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, fmt.Errorf("ping postgres: %w", err) }
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { if s != nil && s.pool != nil { s.pool.Close() } }

func (s *Store) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE IF NOT EXISTS worlds (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			world_key text NOT NULL UNIQUE,
			seed bigint NOT NULL,
			generation_version integer NOT NULL DEFAULT 1,
			created_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS hosts (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			phone_number text NOT NULL,
			name text NOT NULL,
			region_id uuid,
			founded_on date,
			software_family text NOT NULL DEFAULT 'unassigned',
			line_count integer NOT NULL DEFAULT 1 CHECK (line_count > 0),
			popularity double precision NOT NULL DEFAULT 0.5 CHECK (popularity BETWEEN 0 AND 1),
			max_baud integer NOT NULL DEFAULT 14400,
			member_count integer NOT NULL DEFAULT 0,
			visual_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
			software_customization jsonb NOT NULL DEFAULT '{}'::jsonb,
			facts jsonb NOT NULL DEFAULT '{}'::jsonb,
			generated_at timestamptz NOT NULL DEFAULT now(),
			last_simulated_world_at timestamp,
			world_id uuid REFERENCES worlds(id) ON DELETE CASCADE,
			directory_order integer NOT NULL DEFAULT 0
		)`,
		`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS world_id uuid REFERENCES worlds(id) ON DELETE CASCADE`,
		`ALTER TABLE hosts ADD COLUMN IF NOT EXISTS directory_order integer NOT NULL DEFAULT 0`,
		`ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_phone_number_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_hosts_world_phone ON hosts(world_id, phone_number)`,
		`CREATE INDEX IF NOT EXISTS idx_hosts_world_directory_order ON hosts(world_id, directory_order)`,
	}
	for _, statement := range statements {
		if _, err := s.pool.Exec(ctx, statement); err != nil { return fmt.Errorf("ensure world schema: %w", err) }
	}
	return nil
}

func ValidWorldKey(key string) bool { return worldKeyPattern.MatchString(key) }

func (s *Store) GetOrCreate(ctx context.Context, worldKey string, count int, generateNames func(context.Context, int) ([]string, error)) (Catalog, error) {
	if !ValidWorldKey(worldKey) { return Catalog{}, errors.New("invalid world key") }
	if count <= 0 { return Catalog{}, errors.New("center count must be positive") }

	conn, err := s.pool.Acquire(ctx)
	if err != nil { return Catalog{}, fmt.Errorf("acquire postgres connection: %w", err) }
	defer conn.Release()

	// Serialize creation by world key across server instances without keeping a
	// long-running SQL transaction open while the LLM generates names.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, worldKey); err != nil {
		return Catalog{}, fmt.Errorf("lock world: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, worldKey) }()

	if catalog, found, err := loadCatalog(ctx, conn, worldKey); err != nil {
		return Catalog{}, err
	} else if found {
		return catalog, nil
	}

	names, err := generateNames(ctx, count)
	if err != nil { return Catalog{}, err }
	if len(names) != count { return Catalog{}, fmt.Errorf("expected %d generated names, got %d", count, len(names)) }
	seed, err := randomSeed()
	if err != nil { return Catalog{}, err }
	centers := makeCenters(names, seed)

	tx, err := conn.Begin(ctx)
	if err != nil { return Catalog{}, fmt.Errorf("begin world transaction: %w", err) }
	defer func() { _ = tx.Rollback(context.Background()) }()

	var worldID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO worlds (world_key, seed, generation_version) VALUES ($1, $2, $3) RETURNING id::text`,
		worldKey, seed, GenerationVersion,
	).Scan(&worldID); err != nil {
		return Catalog{}, fmt.Errorf("insert world: %w", err)
	}
	for i, center := range centers {
		facts := fmt.Sprintf(`{"dial_mode":%q,"directory_id":%q}`, center.DialMode, center.ID)
		if _, err := tx.Exec(ctx, `INSERT INTO hosts
			(world_id, directory_order, phone_number, name, software_family, line_count, max_baud, facts)
			VALUES ($1, $2, $3, $4, 'unassigned', 1, $5, $6::jsonb)`,
			worldID, i, center.Phone, center.Name, center.MaxBaud, facts); err != nil {
			return Catalog{}, fmt.Errorf("insert host %d: %w", i, err)
		}
	}
	if err := tx.Commit(ctx); err != nil { return Catalog{}, fmt.Errorf("commit world: %w", err) }
	return Catalog{WorldID: worldID, Seed: seed, Centers: centers, Created: true}, nil
}

func loadCatalog(ctx context.Context, conn *pgxpool.Conn, worldKey string) (Catalog, bool, error) {
	var catalog Catalog
	if err := conn.QueryRow(ctx, `SELECT id::text, seed FROM worlds WHERE world_key = $1`, worldKey).Scan(&catalog.WorldID, &catalog.Seed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return Catalog{}, false, nil }
		return Catalog{}, false, fmt.Errorf("load world: %w", err)
	}
	rows, err := conn.Query(ctx, `SELECT id::text, name, phone_number,
		COALESCE(facts->>'dial_mode', 'tone'), max_baud
		FROM hosts WHERE world_id = $1::uuid ORDER BY directory_order, phone_number`, catalog.WorldID)
	if err != nil { return Catalog{}, false, fmt.Errorf("load world hosts: %w", err) }
	defer rows.Close()
	for rows.Next() {
		var center Center
		if err := rows.Scan(&center.ID, &center.Name, &center.Phone, &center.DialMode, &center.MaxBaud); err != nil {
			return Catalog{}, false, fmt.Errorf("scan world host: %w", err)
		}
		catalog.Centers = append(catalog.Centers, center)
	}
	if err := rows.Err(); err != nil { return Catalog{}, false, err }
	return catalog, true, nil
}

func randomSeed() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil { return 0, fmt.Errorf("generate world seed: %w", err) }
	return int64(binary.BigEndian.Uint64(buf[:]) & 0x7fffffffffffffff), nil
}

func makeCenters(names []string, seed int64) []Center {
	bauds := []int{2400, 9600, 14400, 28800}
	centers := make([]Center, 0, len(names))
	state := uint64(seed)
	for i, name := range names {
		state += 0x9e3779b97f4a7c15
		z := state
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z ^= z >> 31
		area := 3 + int(z%7)
		centers = append(centers, Center{
			ID: fmt.Sprintf("world-%03d", i+1),
			Name: name,
			Phone: fmt.Sprintf("0%d%08d", area, 10000000+i),
			DialMode: "tone",
			MaxBaud: bauds[int((z>>8)%uint64(len(bauds)))],
		})
	}
	return centers
}
