package historicalkb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) EnsureKnowledgeSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS historical_facts (
  id text PRIMARY KEY,
  knowledge_key text NOT NULL,
  kind text NOT NULL,
  subject text NOT NULL,
  claim text NOT NULL,
  valid_from text NOT NULL DEFAULT '',
  valid_until text NOT NULL DEFAULT '',
  region text NOT NULL DEFAULT 'JP',
  audience jsonb NOT NULL DEFAULT '[]'::jsonb,
  confidence double precision NOT NULL DEFAULT 0,
  status text NOT NULL,
  sources jsonb NOT NULL DEFAULT '[]'::jsonb,
  research_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS historical_facts_lookup_idx ON historical_facts(knowledge_key, status, confidence DESC);
CREATE INDEX IF NOT EXISTS historical_facts_research_idx ON historical_facts(research_id);
CREATE TABLE IF NOT EXISTS historical_research_leases (
  knowledge_key text PRIMARY KEY,
  owner text NOT NULL,
  expires_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
`)
	return err
}

func (s *Store) FindFacts(ctx context.Context, key string) ([]HistoricalFact, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,knowledge_key,kind,subject,claim,valid_from,valid_until,region,audience,confidence,status,sources,research_id,created_at,updated_at FROM historical_facts WHERE knowledge_key=$1 AND status <> 'rejected' ORDER BY confidence DESC, updated_at DESC`, key)
	if err != nil { return nil, err }; defer rows.Close(); out:=make([]HistoricalFact,0)
	for rows.Next(){var f HistoricalFact;var kind,status string;var audience,sources []byte
		if err:=rows.Scan(&f.ID,&f.KnowledgeKey,&kind,&f.Subject,&f.Claim,&f.ValidFrom,&f.ValidUntil,&f.Region,&audience,&f.Confidence,&status,&sources,&f.ResearchID,&f.CreatedAt,&f.UpdatedAt);err!=nil{return nil,err}
		f.Kind=KnowledgeKind(kind);f.Status=FactStatus(status);_=json.Unmarshal(audience,&f.Audience);_=json.Unmarshal(sources,&f.Sources);out=append(out,f)}
	return out,rows.Err()
}

func (s *Store) UpsertFact(ctx context.Context, f HistoricalFact) error {
	audience,_:=json.Marshal(f.Audience);sources,_:=json.Marshal(f.Sources)
	_,err:=s.pool.Exec(ctx,`INSERT INTO historical_facts(id,knowledge_key,kind,subject,claim,valid_from,valid_until,region,audience,confidence,status,sources,research_id,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT(id) DO UPDATE SET claim=EXCLUDED.claim,valid_from=EXCLUDED.valid_from,valid_until=EXCLUDED.valid_until,region=EXCLUDED.region,audience=EXCLUDED.audience,confidence=EXCLUDED.confidence,status=EXCLUDED.status,sources=EXCLUDED.sources,research_id=EXCLUDED.research_id,updated_at=EXCLUDED.updated_at`,f.ID,f.KnowledgeKey,string(f.Kind),f.Subject,f.Claim,f.ValidFrom,f.ValidUntil,f.Region,audience,f.Confidence,string(f.Status),sources,f.ResearchID,f.CreatedAt,f.UpdatedAt)
	return err
}

func (s *Store) PromoteFactsForResearch(ctx context.Context,researchID,claim string,status FactStatus) error {
	_,err:=s.pool.Exec(ctx,`UPDATE historical_facts SET claim=CASE WHEN $2='' THEN claim ELSE $2 END,status=$3,confidence=CASE WHEN $3 IN ('operator_verified','canonical') THEN GREATEST(confidence,0.95) ELSE confidence END,updated_at=now() WHERE research_id=$1`,researchID,claim,string(status))
	return err
}

func (s *Store) TryAcquireResearchLease(ctx context.Context,key,owner string,ttl time.Duration)(bool,error){var acquired bool;expiresAt:=time.Now().UTC().Add(ttl)
	err:=s.pool.QueryRow(ctx,`INSERT INTO historical_research_leases(knowledge_key,owner,expires_at,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT(knowledge_key) DO UPDATE SET owner=EXCLUDED.owner,expires_at=EXCLUDED.expires_at,updated_at=now() WHERE historical_research_leases.expires_at < now() RETURNING true`,key,owner,expiresAt).Scan(&acquired)
	if errors.Is(err,pgx.ErrNoRows){return false,nil};if err!=nil{return false,err};return acquired,nil}
func (s *Store) ReleaseResearchLease(ctx context.Context,key,owner string){_,_=s.pool.Exec(ctx,`DELETE FROM historical_research_leases WHERE knowledge_key=$1 AND owner=$2`,key,owner)}
