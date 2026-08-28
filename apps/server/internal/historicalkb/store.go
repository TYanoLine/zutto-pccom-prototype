package historicalkb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil { return nil, err }
	if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, err }
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { if s != nil && s.pool != nil { s.pool.Close() } }

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS historical_research_cases (
  id text PRIMARY KEY,
  topic text NOT NULL,
  question text NOT NULL,
  world_date text NOT NULL,
  status text NOT NULL,
  summary text NOT NULL DEFAULT '',
  provisional_answer text NOT NULL DEFAULT '',
  missing_info jsonb NOT NULL DEFAULT '[]'::jsonb,
  confidence double precision NOT NULL DEFAULT 0,
  sources jsonb NOT NULL DEFAULT '[]'::jsonb,
  messages jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS historical_research_cases_status_updated_idx
  ON historical_research_cases(status, updated_at DESC);
`)
	return err
}

func (s *Store) Upsert(ctx context.Context, c ResearchCase) error {
	missing, _ := json.Marshal(c.MissingInfo)
	sources, _ := json.Marshal(c.Sources)
	messages, _ := json.Marshal(c.Messages)
	_, err := s.pool.Exec(ctx, `
INSERT INTO historical_research_cases
(id, topic, question, world_date, status, summary, provisional_answer, missing_info, confidence, sources, messages, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (id) DO UPDATE SET
 topic=EXCLUDED.topic,
 question=EXCLUDED.question,
 world_date=EXCLUDED.world_date,
 status=EXCLUDED.status,
 summary=EXCLUDED.summary,
 provisional_answer=EXCLUDED.provisional_answer,
 missing_info=EXCLUDED.missing_info,
 confidence=EXCLUDED.confidence,
 sources=EXCLUDED.sources,
 messages=EXCLUDED.messages,
 updated_at=EXCLUDED.updated_at`,
		c.ID, c.Topic, c.Question, c.WorldDate, string(c.Status), c.Summary, c.ProvisionalAnswer,
		missing, c.Confidence, sources, messages, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (ResearchCase, error) {
	var c ResearchCase
	var status string
	var missing, sources, messages []byte
	err := s.pool.QueryRow(ctx, `SELECT id,topic,question,world_date,status,summary,provisional_answer,missing_info,confidence,sources,messages,created_at,updated_at FROM historical_research_cases WHERE id=$1`, id).
		Scan(&c.ID,&c.Topic,&c.Question,&c.WorldDate,&status,&c.Summary,&c.ProvisionalAnswer,&missing,&c.Confidence,&sources,&messages,&c.CreatedAt,&c.UpdatedAt)
	if err != nil { return ResearchCase{}, err }
	c.Status = Status(status)
	_ = json.Unmarshal(missing, &c.MissingInfo)
	_ = json.Unmarshal(sources, &c.Sources)
	_ = json.Unmarshal(messages, &c.Messages)
	return c, nil
}

func (s *Store) List(ctx context.Context, limit int) ([]ResearchCase, error) {
	if limit <= 0 || limit > 200 { limit = 100 }
	rows, err := s.pool.Query(ctx, `SELECT id,topic,question,world_date,status,summary,provisional_answer,missing_info,confidence,sources,messages,created_at,updated_at FROM historical_research_cases ORDER BY updated_at DESC LIMIT $1`, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make([]ResearchCase,0)
	for rows.Next() {
		var c ResearchCase
		var status string
		var missing, sources, messages []byte
		if err := rows.Scan(&c.ID,&c.Topic,&c.Question,&c.WorldDate,&status,&c.Summary,&c.ProvisionalAnswer,&missing,&c.Confidence,&sources,&messages,&c.CreatedAt,&c.UpdatedAt); err != nil { return nil, err }
		c.Status = Status(status)
		_ = json.Unmarshal(missing,&c.MissingInfo)
		_ = json.Unmarshal(sources,&c.Sources)
		_ = json.Unmarshal(messages,&c.Messages)
		out = append(out,c)
	}
	return out, rows.Err()
}

func (s *Store) SetStatus(ctx context.Context, id string, status Status) (ResearchCase, error) {
	if !validStatus(status) { return ResearchCase{}, fmt.Errorf("invalid status %q", status) }
	tag, err := s.pool.Exec(ctx, `UPDATE historical_research_cases SET status=$2, updated_at=now() WHERE id=$1`, id, string(status))
	if err != nil { return ResearchCase{}, err }
	if tag.RowsAffected() == 0 { return ResearchCase{}, pgx.ErrNoRows }
	return s.Get(ctx,id)
}

func validStatus(s Status) bool {
	switch s {
	case StatusProvisional, StatusNeedsReview, StatusOperatorVerified, StatusCanonical, StatusRejected:
		return true
	default:
		return false
	}
}

func NewCaseID(now time.Time) string { return fmt.Sprintf("research-%d", now.UTC().UnixNano()) }

var ErrNotConfigured = errors.New("historical knowledge store is not configured")
