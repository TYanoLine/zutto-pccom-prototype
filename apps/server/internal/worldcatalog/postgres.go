package worldcatalog

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const GenerationVersion = 2

var worldKeyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type Center struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Phone          string  `json:"phone"`
	DialMode       string  `json:"dialMode"`
	MaxBaud        int     `json:"maxBaud"`
	SoftwareFamily string  `json:"softwareFamily"`
	LineCount      int     `json:"lineCount"`
	FoundedOn      string  `json:"foundedOn"`
	Popularity     float64 `json:"popularity"`
	MemberCount    int     `json:"memberCount"`
}
type Catalog struct {
	WorldID string
	Seed    int64
	Centers []Center
	Created bool
}
type HostInspection struct {
	Center
	DirectoryOrder int    `json:"directoryOrder"`
	Generation     int    `json:"generation"`
	SkeletonBasis  string `json:"skeletonBasis"`
}
type WorldInspection struct {
	WorldID           string           `json:"worldId"`
	Seed              int64            `json:"seed"`
	GenerationVersion int              `json:"generationVersion"`
	CreatedAt         time.Time        `json:"createdAt"`
	HostCount         int              `json:"hostCount"`
	Hosts             []HostInspection `json:"hosts"`
}
type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}
func (s *Store) EnsureSchema(ctx context.Context) error {
	statements := []string{`CREATE EXTENSION IF NOT EXISTS pgcrypto`, `CREATE TABLE IF NOT EXISTS worlds (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), world_key text NOT NULL UNIQUE, seed bigint NOT NULL, generation_version integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now())`, `CREATE TABLE IF NOT EXISTS hosts (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), phone_number text NOT NULL, name text NOT NULL, region_id uuid, founded_on date, software_family text NOT NULL DEFAULT 'unassigned', line_count integer NOT NULL DEFAULT 1 CHECK (line_count > 0), popularity double precision NOT NULL DEFAULT 0.5 CHECK (popularity BETWEEN 0 AND 1), max_baud integer NOT NULL DEFAULT 14400, member_count integer NOT NULL DEFAULT 0, visual_profile jsonb NOT NULL DEFAULT '{}'::jsonb, software_customization jsonb NOT NULL DEFAULT '{}'::jsonb, facts jsonb NOT NULL DEFAULT '{}'::jsonb, generated_at timestamptz NOT NULL DEFAULT now(), last_simulated_world_at timestamp, world_id uuid REFERENCES worlds(id) ON DELETE CASCADE, directory_order integer NOT NULL DEFAULT 0, generation integer NOT NULL DEFAULT 0)`, `ALTER TABLE hosts ADD COLUMN IF NOT EXISTS world_id uuid REFERENCES worlds(id) ON DELETE CASCADE`, `ALTER TABLE hosts ADD COLUMN IF NOT EXISTS directory_order integer NOT NULL DEFAULT 0`, `ALTER TABLE hosts ADD COLUMN IF NOT EXISTS generation integer NOT NULL DEFAULT 0`, `ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_phone_number_key`, `CREATE UNIQUE INDEX IF NOT EXISTS idx_hosts_world_phone ON hosts(world_id, phone_number)`, `CREATE INDEX IF NOT EXISTS idx_hosts_world_directory_order ON hosts(world_id, directory_order)`}
	for _, q := range statements {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("ensure world schema: %w", err)
		}
	}
	return nil
}
func ValidWorldKey(key string) bool { return worldKeyPattern.MatchString(key) }

func (s *Store) GetOrCreate(ctx context.Context, worldKey string, count int, generateNames func(context.Context, int) ([]string, error)) (Catalog, error) {
	if !ValidWorldKey(worldKey) {
		return Catalog{}, errors.New("invalid world key")
	}
	if count <= 0 {
		return Catalog{}, errors.New("center count must be positive")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return Catalog{}, fmt.Errorf("acquire postgres connection: %w", err)
	}
	defer conn.Release()
	if err := lockWorld(ctx, conn, worldKey); err != nil {
		return Catalog{}, err
	}
	defer unlockWorld(conn, worldKey)
	if catalog, found, err := loadCatalog(ctx, conn, worldKey); err != nil {
		return Catalog{}, err
	} else if found {
		return catalog, nil
	}

	names, err := generateNames(ctx, count)
	if err != nil {
		return Catalog{}, err
	}
	if len(names) != count {
		return Catalog{}, fmt.Errorf("expected %d generated names, got %d", count, len(names))
	}
	seed, err := randomSeed()
	if err != nil {
		return Catalog{}, err
	}
	centers := makeCenters(names, seed)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Catalog{}, fmt.Errorf("begin world transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var worldID string
	if err := tx.QueryRow(ctx, `INSERT INTO worlds (world_key, seed, generation_version) VALUES ($1,$2,$3) RETURNING id::text`, worldKey, seed, GenerationVersion).Scan(&worldID); err != nil {
		return Catalog{}, fmt.Errorf("insert world: %w", err)
	}
	for i, center := range centers {
		if err := insertCenter(ctx, tx, worldID, i, 0, center); err != nil {
			return Catalog{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Catalog{}, fmt.Errorf("commit world: %w", err)
	}
	return Catalog{WorldID: worldID, Seed: seed, Centers: centers, Created: true}, nil
}

func (s *Store) InspectWorld(ctx context.Context, worldKey string) (WorldInspection, error) {
	if !ValidWorldKey(worldKey) {
		return WorldInspection{}, errors.New("invalid world key")
	}
	var out WorldInspection
	err := s.pool.QueryRow(ctx, `SELECT id::text,seed,generation_version,created_at FROM worlds WHERE world_key=$1`, worldKey).Scan(&out.WorldID, &out.Seed, &out.GenerationVersion, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorldInspection{}, errors.New("world not found")
	}
	if err != nil {
		return WorldInspection{}, fmt.Errorf("inspect world: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT COALESCE(facts->>'directory_id',id::text),name,phone_number,COALESCE(facts->>'dial_mode','tone'),max_baud,software_family,line_count,COALESCE(founded_on::text,''),popularity,member_count,directory_order,generation,COALESCE(facts->>'skeleton_basis','') FROM hosts WHERE world_id=$1::uuid ORDER BY directory_order,phone_number`, out.WorldID)
	if err != nil {
		return WorldInspection{}, fmt.Errorf("inspect hosts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var host HostInspection
		if err := rows.Scan(&host.ID, &host.Name, &host.Phone, &host.DialMode, &host.MaxBaud, &host.SoftwareFamily, &host.LineCount, &host.FoundedOn, &host.Popularity, &host.MemberCount, &host.DirectoryOrder, &host.Generation, &host.SkeletonBasis); err != nil {
			return WorldInspection{}, fmt.Errorf("scan inspected host: %w", err)
		}
		out.Hosts = append(out.Hosts, host)
	}
	if err := rows.Err(); err != nil {
		return WorldInspection{}, fmt.Errorf("iterate inspected hosts: %w", err)
	}
	out.HostCount = len(out.Hosts)
	return out, nil
}

// ResetWorld removes the canonical world row. ON DELETE CASCADE deliberately
// removes all descendants so the next bootstrap creates a genuinely new world.
func (s *Store) ResetWorld(ctx context.Context, worldKey string) error {
	if !ValidWorldKey(worldKey) {
		return errors.New("invalid world key")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if err := lockWorld(ctx, conn, worldKey); err != nil {
		return err
	}
	defer unlockWorld(conn, worldKey)
	_, err = conn.Exec(ctx, `DELETE FROM worlds WHERE world_key=$1`, worldKey)
	if err != nil {
		return fmt.Errorf("reset world: %w", err)
	}
	return nil
}

// ResetHost regenerates one directory entry while preserving the world and its
// directory position. The generation counter gives the skeleton fresh entropy.
func (s *Store) ResetHost(ctx context.Context, worldKey, directoryID string, generateName func(context.Context) (string, error)) (Center, error) {
	if !ValidWorldKey(worldKey) {
		return Center{}, errors.New("invalid world key")
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return Center{}, err
	}
	defer conn.Release()
	if err := lockWorld(ctx, conn, worldKey); err != nil {
		return Center{}, err
	}
	defer unlockWorld(conn, worldKey)
	var worldID string
	var seed int64
	var order, generation int
	err = conn.QueryRow(ctx, `SELECT w.id::text,w.seed,h.directory_order,h.generation FROM worlds w JOIN hosts h ON h.world_id=w.id WHERE w.world_key=$1 AND COALESCE(h.facts->>'directory_id',h.id::text)=$2`, worldKey, directoryID).Scan(&worldID, &seed, &order, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return Center{}, errors.New("host not found")
	}
	if err != nil {
		return Center{}, fmt.Errorf("find host: %w", err)
	}
	name, err := generateName(ctx)
	if err != nil {
		return Center{}, err
	}
	center := makeCenter(name, seed, order, generation+1)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Center{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `DELETE FROM hosts WHERE world_id=$1::uuid AND directory_order=$2`, worldID, order); err != nil {
		return Center{}, fmt.Errorf("delete host: %w", err)
	}
	if err := insertCenter(ctx, tx, worldID, order, generation+1, center); err != nil {
		return Center{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Center{}, err
	}
	return center, nil
}

func lockWorld(ctx context.Context, conn *pgxpool.Conn, key string) error {
	_, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, key)
	if err != nil {
		return fmt.Errorf("lock world: %w", err)
	}
	return nil
}
func unlockWorld(conn *pgxpool.Conn, key string) {
	_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, key)
}

func insertCenter(ctx context.Context, tx pgx.Tx, worldID string, order, generation int, c Center) error {
	facts := fmt.Sprintf(`{"dial_mode":%q,"directory_id":%q,"skeleton_basis":"station-specific fictional distribution"}`, c.DialMode, c.ID)
	_, err := tx.Exec(ctx, `INSERT INTO hosts (world_id,directory_order,phone_number,name,software_family,line_count,max_baud,founded_on,popularity,member_count,facts,generation) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9,$10,$11::jsonb,$12)`, worldID, order, c.Phone, c.Name, c.SoftwareFamily, c.LineCount, c.MaxBaud, c.FoundedOn, c.Popularity, c.MemberCount, facts, generation)
	if err != nil {
		return fmt.Errorf("insert host %d: %w", order, err)
	}
	return nil
}

func loadCatalog(ctx context.Context, conn *pgxpool.Conn, worldKey string) (Catalog, bool, error) {
	var catalog Catalog
	if err := conn.QueryRow(ctx, `SELECT id::text,seed FROM worlds WHERE world_key=$1`, worldKey).Scan(&catalog.WorldID, &catalog.Seed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Catalog{}, false, nil
		}
		return Catalog{}, false, fmt.Errorf("load world: %w", err)
	}
	rows, err := conn.Query(ctx, `SELECT COALESCE(facts->>'directory_id',id::text),name,phone_number,COALESCE(facts->>'dial_mode','tone'),max_baud,software_family,line_count,COALESCE(founded_on::text,''),popularity,member_count FROM hosts WHERE world_id=$1::uuid ORDER BY directory_order,phone_number`, catalog.WorldID)
	if err != nil {
		return Catalog{}, false, fmt.Errorf("load world hosts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c Center
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.DialMode, &c.MaxBaud, &c.SoftwareFamily, &c.LineCount, &c.FoundedOn, &c.Popularity, &c.MemberCount); err != nil {
			return Catalog{}, false, err
		}
		catalog.Centers = append(catalog.Centers, c)
	}
	if err := rows.Err(); err != nil {
		return Catalog{}, false, err
	}
	return catalog, true, nil
}

func randomSeed() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7fffffffffffffff), nil
}
func mix(seed int64, index, generation int) uint64 {
	z := uint64(seed) ^ uint64(index+1)*0x9e3779b97f4a7c15 ^ uint64(generation)*0xd1b54a32d192ed03
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
func makeCenters(names []string, seed int64) []Center {
	out := make([]Center, len(names))
	for i, n := range names {
		out[i] = makeCenter(n, seed, i, 0)
	}
	return out
}
func makeCenter(name string, seed int64, index, generation int) Center {
	z := mix(seed, index, generation)
	identityZ := mix(seed, index, 0)
	bauds := []int{2400, 9600, 14400, 28800}
	families := []string{"ktbbs", "big-model", "erika-k", "mmm", "rt-bbs", "vs", "other"}
	area := 3 + int(identityZ%7)
	foundedYear := 1987 + int((z>>12)%9)
	foundedMonth := 1 + int((z>>20)%12)
	foundedDay := 1 + int((z>>28)%28)
	pop := math.Round((0.15+float64((z>>36)%76)/100)*100) / 100
	members := 20 + int((z>>44)%1981)
	lines := 1
	if (z>>9)%100 < 22 {
		lines = 2
	}
	if (z>>9)%100 < 5 {
		lines = 3
	}
	return Center{ID: fmt.Sprintf("world-%03d", index+1), Name: name, Phone: fmt.Sprintf("0%d%08d", area, 10000000+index), DialMode: "tone", MaxBaud: bauds[int((z>>8)%uint64(len(bauds)))], SoftwareFamily: families[int((z>>5)%uint64(len(families)))], LineCount: lines, FoundedOn: time.Date(foundedYear, time.Month(foundedMonth), foundedDay, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), Popularity: pop, MemberCount: members}
}
