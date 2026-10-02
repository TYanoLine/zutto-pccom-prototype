# CLAUDE.md

Guidance for Claude Code when working in this repository.

This file is a Claude-oriented entry point derived from the existing agent harness. **`AGENTS.md` is the source of truth** for agent rules and `CODEX.md` holds the milestone roadmap. If this file disagrees with them, follow `AGENTS.md` / `CODEX.md` and update this file.

## Project

**ずっとパソコン通信 (Zutto PC Communication)** simulates a persistent 1996 Japanese dial-up BBS world (PC-98-style client). It is **not** a chatbot with a retro skin. Preserve that distinction in architecture, content, and UX.

Stack: Go modular monolith (`apps/server`), Vite web client (`apps/web`), PostgreSQL scaffold (`database/`), shared protocol (`packages/protocol`).

## Read before changing things

1. `docs/PRODUCT_SPEC.md`
2. `docs/ARCHITECTURE.md`
3. `docs/HISTORICAL_ACCURACY.md`
4. World/NPC work: `docs/WORLD_SIMULATION.md`, `docs/LLM_POLICY.md`
5. Terminal/modem work: `docs/TERMINAL_AND_MODEM.md` and relevant research docs
6. Host-program work: the matching file under `docs/host-programs/` and `apps/server/internal/hostprogram/`
7. `CODEX.md` for milestones (M0–M6) and `packages/protocol/README.md` for protocol details

Inspect current `main` and the relevant code immediately before editing; the human developer may be changing the repository concurrently. If docs and code disagree, do not silently guess: decide whether the code is an intentional newer implementation or the doc is stale, and update the stale side.

## Commands

Run from the repository root.

```bash
npm --prefix apps/web install   # one-time web dependency install
npm run dev                     # Go server + Vite client together (Ctrl+C stops both)
npm run dev:server              # Go server only
npm run dev:web                 # Vite client only
npm test                        # server + web tests
npm run test:server             # go -C apps/server test ./...
npm run test:web                # web tests
npm run build                   # server tests + web build
docker compose up -d postgres   # PostgreSQL scaffold (server still uses in-memory store)
```

Requirements: Go 1.23+, Node.js 22+, npm. Docker only for the PostgreSQL scaffold.

Try it by opening the Vite URL and typing `ATDT0451234567`. Fixture numbers: `0450000001` (almost always connects), `0459999999` (busy for the first few attempts, exercises redial).

## Non-negotiable design rules

- PostgreSQL/world storage is canonical world truth. Never use an LLM conversation as world state.
- The human participant is one member of the world, not its protagonist.
- The world engine decides whether actors read, ignore, reply, post, or do nothing. An LLM may verbalize a selected action; it must not decide world facts merely because the human spoke. "No NPC response" is a normal result.
- Unobserved facts may be generated lazily. Once observed/materialized, persistent facts must not silently mutate.
- 1996 Japan is the cultural/knowledge ceiling. Do not leak modern products, events, slang, SNS conventions, or internet culture into in-world content.
- Region, software, and host traits are probability distributions, not stereotypes.
- Real-money billing does not exist; tolls are atmosphere only.
- Keep transport/BBS boundaries compatible with a future RS-232C bridge.

## Historical host programs

TurboBBS, KTBBS, BIG-Model, 絵理香K版, mmm, RT-BBS, VS, etc. are **different programs**, not skins of one runtime.

Share lower layers (world data, persistence, sessions, transports, access-control primitives, utilities). Keep state machines, commands, menus, board/article semantics, unread behavior, mail/chat/files, prompts, login/logout flows, and escape-sequence behavior in each host program's own implementation. Do not create a historically nonexistent common UI just to reduce duplication.

Rule of thumb: **操作系は別実装、世界データとインフラだけ共有。**

## Historical evidence policy

Never invent exact historical behavior and present it as fact. Label claims as one of:

- **Confirmed**: directly supported by a source
- **Likely / inferred**: reasonable reconstruction, not directly confirmed
- **Station-specific**: customization of a particular BBS, not a software default
- **Fictional reconstruction**: intentionally invented for this service

Record research and source URLs in `docs/host-programs/` or `docs/research/`, not only in chat history.

## Coding workflow

- Use a feature branch and pull request. **Do not write directly to `main` unless explicitly requested.**
- Keep the modular monolith; do not split into microservices without concrete scaling evidence.
- Avoid premature abstractions. Duplication beats an abstraction that destroys host-program semantics.
- Add or update tests for important state transitions and invariants.
- **Never claim tests or CI passed unless you actually observed them.**
- Keep AI integration optional: the service must work deterministically without an LLM.
- Verify generation changes with focused Go tests and normal BBS observation on the deployed revision. Distinguish deterministic test results from observed model output. Never reset or rewrite canonical world facts for a quality comparison.
- Never commit secrets. Use `.env.example` as the template; Azure OpenAI settings (`AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_MODEL`, optional `AZURE_OPENAI_IMAGE_MODEL`) come from the environment.

## In-world writing

Use period-appropriate Japanese communication culture in fixture content: `(笑)`, `(爆)`, `(^^)`, `(^^;`, `(^_^;)`, `m(_ _)m`, `>` quoting. Avoid `草`, modern `w`, `ググる`, and SNS terminology.

Era-appropriate topics: Windows 95, PC-98, DOS/V, Macintosh, modems/ISDN, NIFTY-Serve/PC-VAN, the emerging WWW, Saturn, PlayStation, Pokémon, Evangelion, MIDI/FM sound, Akihabara, offline meetups, subject to the exact mapped world date.

## Architecture maxim

**Same world/backend primitives, different historical host-program runtimes. Database truth, simulated agency, LLM prose.**
