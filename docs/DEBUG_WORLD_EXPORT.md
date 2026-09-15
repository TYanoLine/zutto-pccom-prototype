# Read-only world debug export

The prototype exposes a development-only snapshot so an operator or coding assistant can inspect the **already materialized** BBS world without driving the terminal UI or triggering new LLM generation.

## Backend endpoint

`GET /api/debug/export?phone=<phone>`

Optional query parameters:

- `board=<board id>` — return posts for one board while still reporting host-wide counts.
- `full=1` — include post bodies. By default bodies are omitted to keep snapshots compact; subjects and semantic intents are still included.

The response contains the current runtime host profile, boards, host personas, persisted persona facts, posts, semantic post intents, counts, build identity, and (when enabled) development persistence status.

## Safety / semantics

This endpoint is deliberately read-only and reads the base store directly. It must **not** call `WorldRepository.ListBoardPosts`, an observation gate, a materializer, or an LLM. Inspecting a snapshot therefore cannot create posts, persona facts, or other world history.

The endpoint exports fictional prototype world data only. It does not export session identifiers, user credentials, environment variables, API keys, reset tokens, or database connection information. Responses use `Cache-Control: no-store`.

This is a PoC diagnostic endpoint and is intentionally unauthenticated so external development tooling can inspect the running fictional world. Before the service contains real user/private data, replace this with operator authentication or remove it.

## Vercel inspection bridge

The browser deployment is on Vercel while the Go runtime currently lives on Render. `apps/web/api/debug-export.js` provides a fixed-origin read-only proxy:

`GET /api/debug-export?phone=<phone>[&board=<id>][&full=1]`

It forwards only `phone`, `board`, and `full` to the fixed production Go backend. It cannot be used as an arbitrary URL proxy.

This bridge is primarily for development inspection tools that can fetch the Vercel deployment but cannot attach to the live WebSocket terminal session directly.

## Development materialization persistence

The general prototype repository is still backed by `MemoryStore`, but the expensive development host at `0450000196` is wrapped by a narrow PostgreSQL JSONB snapshot layer when `DATABASE_URL` is configured. The snapshot contains the host profile, boards, core personas and memberships, persona facts, post envelopes, rendered bodies, and the next post id.

Every canonical mutation relevant to that host is synchronously snapshotted. In particular, each added envelope and each completed lazy body update is committed before generation moves on. On server startup the saved snapshot is restored into a fresh `MemoryStore` before the host is observed. This means a Render restart can interrupt an `ALLBODY` job, but already committed envelopes and bodies survive and can be reused after reconnecting.

`RESET` remains meaningful: clearing the development host posts and lazily materialized persona facts is also persisted, while the host, boards, and core persona skeletons remain.

This snapshot table is deliberately a development stepping stone, not the final world schema. The project direction remains a normalized PostgreSQL-backed canonical world store with process memory acting as cache rather than authority.
