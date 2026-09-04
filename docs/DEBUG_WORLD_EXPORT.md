# Read-only world debug export

The prototype exposes a development-only snapshot so an operator or coding assistant can inspect the **already materialized** BBS world without driving the terminal UI or triggering new LLM generation.

## Backend endpoint

`GET /api/debug/export?phone=<phone>`

Optional query parameters:

- `board=<board id>` — return posts for one board while still reporting host-wide counts.
- `full=1` — include post bodies. By default bodies are omitted to keep snapshots compact; subjects and semantic intents are still included.

The response contains the current in-memory host profile, boards, host personas, persisted persona facts, posts, semantic post intents, and counts.

## Safety / semantics

This endpoint is deliberately read-only and reads the base `MemoryStore` directly. It must **not** call `WorldRepository.ListBoardPosts`, an observation gate, a materializer, or an LLM. Inspecting a snapshot therefore cannot create posts, persona facts, or other world history.

The endpoint exports fictional prototype world data only. It does not export session identifiers, user credentials, environment variables, API keys, reset tokens, or database connection information. Responses use `Cache-Control: no-store`.

This is a PoC diagnostic endpoint and is intentionally unauthenticated so external development tooling can inspect the running fictional world. Before the service contains real user/private data, replace this with operator authentication or remove it.

## Vercel inspection bridge

The browser deployment is on Vercel while the Go runtime currently lives on Render. `apps/web/api/debug-export.js` provides a fixed-origin read-only proxy:

`GET /api/debug-export?phone=<phone>[&board=<id>][&full=1]`

It forwards only `phone`, `board`, and `full` to the fixed production Go backend. It cannot be used as an arbitrary URL proxy.

This bridge is primarily for development inspection tools that can fetch the Vercel deployment but cannot attach to the live WebSocket terminal session directly.

## Current prototype limitation

The live BBS repository is still `MemoryStore`, so this snapshot describes the state held by the current Go server process. A backend restart can lose that runtime materialization. This endpoint is an observability aid, not a persistence mechanism; PostgreSQL remains the intended canonical world store as the prototype matures.
