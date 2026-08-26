CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS regions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  prefecture text NOT NULL,
  area_code text NOT NULL,
  name text NOT NULL,
  generation_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(area_code, name)
);

CREATE TABLE IF NOT EXISTS hosts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone_number text NOT NULL UNIQUE,
  name text NOT NULL,
  region_id uuid REFERENCES regions(id),
  founded_on date,
  software_family text NOT NULL,
  line_count integer NOT NULL CHECK (line_count > 0),
  popularity double precision NOT NULL DEFAULT 0.5 CHECK (popularity BETWEEN 0 AND 1),
  max_baud integer NOT NULL DEFAULT 14400,
  member_count integer NOT NULL DEFAULT 0,
  visual_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  software_customization jsonb NOT NULL DEFAULT '{}'::jsonb,
  facts jsonb NOT NULL DEFAULT '{}'::jsonb,
  generated_at timestamptz NOT NULL DEFAULT now(),
  last_simulated_world_at timestamp
);

CREATE TABLE IF NOT EXISTS personas (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  home_region_id uuid REFERENCES regions(id),
  handle text NOT NULL,
  demographic_profile jsonb NOT NULL,
  behavior_profile jsonb NOT NULL,
  interest_profile jsonb NOT NULL,
  opinion_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  private_facts jsonb NOT NULL DEFAULT '{}'::jsonb,
  public_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS memberships (
  host_id uuid NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  persona_id uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
  member_level integer NOT NULL DEFAULT 1,
  joined_world_at timestamp NOT NULL,
  reputation double precision NOT NULL DEFAULT 0,
  PRIMARY KEY(host_id, persona_id)
);

CREATE TABLE IF NOT EXISTS boards (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  code text NOT NULL,
  title text NOT NULL,
  access_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(host_id, code)
);

CREATE TABLE IF NOT EXISTS threads (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  board_id uuid NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  subject text NOT NULL,
  topic_tags text[] NOT NULL DEFAULT '{}',
  momentum double precision NOT NULL DEFAULT 0,
  created_world_at timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
  id bigserial PRIMARY KEY,
  thread_id uuid NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
  author_persona_id uuid REFERENCES personas(id),
  human_controlled boolean NOT NULL DEFAULT false,
  body_utf8 text NOT NULL,
  created_world_at timestamp NOT NULL,
  generation_metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS relationships (
  from_persona_id uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
  to_persona_id uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
  familiarity double precision NOT NULL DEFAULT 0,
  trust double precision NOT NULL DEFAULT 0,
  conflict double precision NOT NULL DEFAULT 0,
  summary text NOT NULL DEFAULT '',
  PRIMARY KEY(from_persona_id, to_persona_id)
);

CREATE TABLE IF NOT EXISTS world_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid REFERENCES hosts(id) ON DELETE CASCADE,
  event_type text NOT NULL,
  world_at timestamp NOT NULL,
  facts jsonb NOT NULL,
  prose text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS generation_facts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL,
  entity_id uuid,
  schema_version integer NOT NULL DEFAULT 1,
  facts jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hosts_region ON hosts(region_id);
CREATE INDEX IF NOT EXISTS idx_hosts_facts_gin ON hosts USING gin(facts);
CREATE INDEX IF NOT EXISTS idx_personas_interests_gin ON personas USING gin(interest_profile);
CREATE INDEX IF NOT EXISTS idx_events_host_world_at ON world_events(host_id, world_at);
