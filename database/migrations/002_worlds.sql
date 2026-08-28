CREATE TABLE IF NOT EXISTS worlds (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  world_key text NOT NULL UNIQUE,
  seed bigint NOT NULL,
  generation_version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE hosts ADD COLUMN IF NOT EXISTS world_id uuid REFERENCES worlds(id) ON DELETE CASCADE;

-- Preserve any old prototype hosts if this migration is applied to an existing DB.
INSERT INTO worlds (id, world_key, seed, generation_version)
VALUES ('00000000-0000-0000-0000-000000000001', 'legacy-prototype-world', 0, 0)
ON CONFLICT (world_key) DO NOTHING;

UPDATE hosts
SET world_id = '00000000-0000-0000-0000-000000000001'
WHERE world_id IS NULL;

ALTER TABLE hosts ALTER COLUMN world_id SET NOT NULL;
ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_phone_number_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_hosts_world_phone ON hosts(world_id, phone_number);
CREATE INDEX IF NOT EXISTS idx_hosts_world ON hosts(world_id);
