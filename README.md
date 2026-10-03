# ずっとパソコン通信 — prototype starter

A runnable vertical-slice starter for a 1996 Japanese PC-98-style persistent AI BBS world.

## What already works

- PC-98-ish 640×400 / 80×25 Canvas terminal
- small ANSI subset (clear/home/SGR colors)
- Hayes-ish `AT`, `ATI`, `ATDT`, `A/`, `ATH`
- stylized dialing / busy / modem-handshake audio via Web Audio
- time/popularity/line-count influenced `BUSY`
- auto-redial
- automatic modem-server reconnection and lifecycle cleanup
- `CONNECT 2400/9600/14400/28800`
- 1996-mapped Japan world clock and data-driven pseudo tariffs
- one sample BBS with read/write/users/logout commands
- posts survive reconnects for the lifetime of the Go process
- PostgreSQL schema scaffold
- optional Azure OpenAI Responses API provider scaffold
- product/architecture/Codex handoff docs

## Prerequisites

- Go 1.23+
- Node.js 22+
- npm
- Docker only if you want the PostgreSQL scaffold now

## Run

Install the web dependencies once:

```bash
npm --prefix apps/web install
```

Then start both the Go server and Vite client from the repository root:

```bash
npm run dev
```

`Ctrl+C` stops both processes. Individual processes remain available through
`npm run dev:server` and `npm run dev:web`.

Open the Vite URL and type:

```text
ATDT0450000001
```

That is the generic-runtime test host (see the fixture numbers below). Once
connected:

```text
H  help
B  read board
W  write
U  users
G  goodbye
```

`A/` repeats the previous dial. AUTO REDIAL is enabled by default.

Prototype hosts are defined in `apps/server/internal/hostcatalog/presets/`
(see its README):

- `0920000196` — HAKATA CANAL NET, the sample station, running the Erika-K style runtime (its commands differ from the list above)
- `0450000001` — hidden test host on the generic runtime, nearly guaranteed connection for testing
- `0459999999` — hidden test host, deliberately busy for the first few attempts to exercise redial

## PostgreSQL scaffold

```bash
docker compose up -d postgres
```

The running server still uses its in-memory store. Connecting pgx is an intentional next milestone; see `CODEX.md`.

## Azure OpenAI

The prototype runs without an API key. `internal/llm/openai.go` shows the provider boundary and Responses API call shape. Configure later with:

```bash
export AZURE_OPENAI_ENDPOINT=https://YOUR-RESOURCE-NAME.openai.azure.com
export AZURE_OPENAI_API_KEY=...
# Use the Azure deployment name (not necessarily the underlying model ID).
export AZURE_OPENAI_MODEL=zutto-pccom-gpt-6-luna
# Optional, if the image PoC is used:
# Requires a separate image model deployment on the Azure resource.
export AZURE_OPENAI_IMAGE_MODEL=...
```

Do not make Azure OpenAI conversation state the world database. See `docs/LLM_POLICY.md`.

## Important prototype compromises

This starter is meant to establish architecture and feel, not historical fidelity yet. Current terminal traffic is JSON/text rather than a CP932 binary serial stream, modem audio is stylized, and the data-driven phone tariff is atmospheric rather than a claim of historical NTT accuracy.

## Test

From the repository root:

```bash
npm test
```

Read `CODEX.md` before extending the project.
