# worldcatalog

This package owns persistence and deterministic generation of the world-level center catalog and host skeletons. PostgreSQL is canonical. LLM output is currently limited to center names; host facts are selected by the world engine and persisted.

Generated hosts convert to the shared `hostcatalog.HostDescriptor` through `Center.Descriptor()`. Fixed stations are not generated here; they are YAML presets in `internal/hostcatalog`, and generated phone numbers never collide with them (tested). Generated hosts are listed by `/api/world/bootstrap` but are not yet dialable. See `docs/HOST_DEFINITION.md`.
