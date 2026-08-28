# Migrations

SQL migrations document schema evolution. During the current prototype, server startup also calls `worldcatalog.Store.EnsureSchema` so deployed environments do not depend on a separate migration runner for required additive schema changes.
