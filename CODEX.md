# Codex handoff

This repository is a deliberate vertical-slice starter for **ずっとパソコン通信**.

Read, in order:

1. `docs/PRODUCT_SPEC.md`
2. `docs/ARCHITECTURE.md`
3. `docs/LLM_POLICY.md`
4. `packages/protocol/README.md`
5. `docs/HOST_DEFINITION.md` (host definitions, roles, the experiment station)
6. `AGENTS.md` (design rules and working agreements)

## Preserve these invariants

- 1996 Japan is the knowledge/culture ceiling.
- PC-98 is the initial client environment.
- PostgreSQL is the eventual canonical world state.
- LLM sessions are never canonical memory.
- Human-controlled members are not protagonists.
- "No NPC response" is a normal result.
- Host/software/region traits are distributions, not hard stereotypes.
- Real-money billing does not exist; tolls are atmosphere only.
- Transport/BBS boundaries must permit a future RS-232C bridge.

## First implementation milestones

### M0 — make this starter solid

- Wire server and web dev scripts from root.
- Add cleanup/reconnect behavior to `VirtualModem`.
- Add tests around dial/busy/Telehodai clock boundaries.
- Replace prototype tariff counter with a data-driven pseudo tariff service.
- Create `WorldClock` abstraction.

### M1 — real persistence

- Add pgx PostgreSQL repository.
- Run migrations from a migration tool rather than init-only Docker scripts.
- Persist host generation by normalized phone number.
- Add transactions/unique constraints so concurrent first calls cannot generate two different hosts.

### M2 — host generation

Status: preset hosts (`HostDescriptor`, YAML presets, `listed`, `role`) are in place; generated hosts becoming dialable, region/traits and per-program detail are not. See `docs/HOST_DEFINITION.md`.

- Region profile + phone number stable seed.
- Host facts first, LLM enrichment second.
- Add software profiles: KTBBS-like, BIG-Model-like, mmm-like first.
- Persist visual/command customizations.

### M3 — non-player-centric residents

- Persona behavioral distributions.
- Event queue / `next_action_at`.
- Read-without-reply state.
- Topic momentum.
- NPC-to-NPC threads.
- Lurkers and inactive accounts.
- Only call an LLM after an action has statistically been selected.

### M4 — Azure OpenAI production path

- Use Azure OpenAI's OpenAI-compatible v1 Responses API through the provider boundary.
- Keep the Azure resource endpoint, API key, and deployment names configurable through environment settings.
- Use Structured Outputs for fact-producing calls.
- Add validation/anachronism checks.
- Keep model IDs configurable by generation class.

### M5 — terminal fidelity

- Binary WebSocket frames for terminal payload.
- CP932/Shift_JIS encoder/decoder at transport edge.
- Expand ESC parser using documented PC-98/period terminal behavior.
- Better bitmap-like rendering without shipping proprietary font files.
- Accurate DTMF/ringback/busy/modem sound profiles.
- X/Y/ZMODEM later.

### M6 — physical PC-98

- `apps/serial-bridge` in Go.
- Hayes command parser shared where sensible.
- RS-232C serial adapter.
- carrier/DTR/RTS-CTS handling.
- TLS tunnel from bridge to public service.

## Do not prematurely microservice this

Keep a modular monolith until scaling evidence says otherwise. Domain interfaces matter; deployment boundaries do not yet.

## Iterative generation verification

Verify generation changes with focused Go tests and normal BBS observation on the deployed revision. Distinguish deterministic test results from observed model output; never reset or rewrite canonical world facts for a quality comparison.
