-- Generation version 2 introduces persisted host skeleton facts.
-- Existing worlds remain valid and can be explicitly regenerated with the
-- protected debug world-reset endpoint during the prototype phase.
ALTER TABLE worlds
    ALTER COLUMN generation_version SET DEFAULT 2;
