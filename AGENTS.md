# AI / Coding Agent Guide

This repository is the source of truth for **ずっとパソコン通信 (Zutto PC Communication)**.

The project simulates a persistent 1996 Japanese dial-up BBS world. It is not a chatbot with a retro skin. Preserve that distinction in architecture, content, and UX.

## Before making changes

1. Read `docs/PRODUCT_SPEC.md`.
2. Read `docs/ARCHITECTURE.md`.
3. Read `docs/HISTORICAL_ACCURACY.md`.
4. For world/NPC work, read `docs/WORLD_SIMULATION.md` and `docs/LLM_POLICY.md`.
5. For terminal/modem work, read `docs/TERMINAL_AND_MODEM.md` and relevant research documents.
6. For historical host-program work, read the corresponding file under `docs/host-programs/` and inspect the current implementation under `apps/server/internal/hostprogram/`.
7. For host definitions, the host directory, presets, roles or the experiment station, read `docs/HOST_DEFINITION.md` and `apps/server/internal/hostcatalog/README.md`.
8. Inspect current `main` and the relevant code immediately before editing. The human developer may be changing the repository concurrently.

If a document and current code disagree, do not silently guess. Determine whether the code is an intentional newer implementation or the documentation is stale, and update the stale side when appropriate.

## Non-negotiable design rules

- PostgreSQL/world storage is canonical world truth. Never use an LLM conversation as world state.
- The human participant is one member of the world, not its protagonist.
- The world engine decides whether actors read, ignore, reply, post independently, or do nothing. An LLM may verbalize a selected action; it must not decide the world's facts merely because the human spoke.
- Unobserved facts may be generated lazily. Once observed/materialized, persistent facts must not silently mutate.
- 1996 Japan is the cultural/knowledge ceiling unless a more specific era document explicitly says otherwise.
- Do not leak modern products, events, slang, SNS conventions, or internet culture into in-world 1996 content.
- Region changes probability distributions and local context; do not turn regional differences into stereotypes.

## Historical host programs

TurboBBS, KTBBS, BIG-Model, 絵理香K版, mmm, RT-BBS, VS, and other historical packages are **different programs**, not skins for one generic runtime.

Share lower layers such as world data, persistence, sessions, transports, access-control primitives, and reusable utilities. Keep software-specific state machines, commands, menus, board/article semantics, unread behavior, mail/chat/files, prompts, login/logout flows, and escape-sequence behavior in their own host-program implementations.

**Rule of thumb: 操作系は別実装、世界データとインフラだけ共有。**

Do not create a historically nonexistent common UI merely to reduce code duplication.

## Host definitions

Details and open work: `docs/HOST_DEFINITION.md`.

- Hosts are data: YAML presets (`apps/server/internal/hostcatalog/presets/`) and the generated per-world catalog. Do not add a station by writing a host literal in code.
- Never identify a station by phone number or host ID in logic. Use attributes. The evaluation station is the host with `role: experiment` (`world.Host.IsExperiment()`), and at most one host may have that role.
- `listed` controls directory visibility only, never dialability. Presets state it explicitly; `debug` and `test` hosts are unlisted.
- A published preset's `key` and `phone` never change. Raise `revision` when its content changes.
- Tests define the hosts they need. Do not make a test depend on a production station.
- Special numbers (110 and the like) are resolved before host lookup. They are a design only; see the document.

## Historical evidence policy

Never invent exact historical behavior and present it as fact.

For historical reconstruction, prefer surviving manuals, source code, distribution archives, contemporary connection logs, magazine articles, screenshots, and other period material. Keep these categories explicit:

- **Confirmed** — directly supported by a source.
- **Likely / inferred** — reasonable reconstruction, but not directly confirmed.
- **Station-specific** — customization known for a particular BBS, not necessarily software default behavior.
- **Fictional reconstruction** — intentionally invented content/behavior for this service.

A fictional sample station may combine historically grounded software behavior with fictional station names, users, banners, boards, and local customizations. Never call that a byte-for-byte or vendor-default reproduction unless evidence supports it.

Record important research and source URLs in `docs/host-programs/` or `docs/research/` rather than leaving the evidence only in chat history.

## Coding workflow

- Prefer a feature branch and pull request for changes. Do not write directly to `main` unless explicitly requested.
- Preserve the modular-monolith direction unless there is a concrete reason to change it.
- Add or update tests for important state transitions and invariants.
- Do not claim tests or CI passed unless you actually observed them.
- Keep AI integration optional. The service must remain capable of deterministic/non-AI operation while host runtimes and world mechanics are developed.
- Avoid premature abstractions. If two historical programs only look superficially similar, duplication is preferable to an abstraction that destroys their semantics.

## Working agreements

These come from how this repository is actually developed. They apply to every agent.

**Language.** Talk with the maintainer in Japanese. Write code, comments, commit messages and repository documentation in English. In-world content is Japanese (see "In-world writing").

**Verify before you push.**

- Build and test the whole module you changed, not only the packages you think are affected: `go vet ./... && go test ./...` in `apps/server`; `npm ci && npx tsc -b && npm test` in `apps/web`. Removing or changing a shared fixture breaks tests in packages that look unrelated.
- Show that a guard or regression test fails when the behavior it guards is reverted (a quick mutation check) before relying on it.
- Say exactly what you ran and what you did not run. Never write "tests pass" or "CI passes" for something you did not observe.

**Before deleting or renaming anything,** search every reference: server code and tests, web code and tests, docs and READMEs, `.env.example`, workflows. Tests often depend on a fixture indirectly.

**Writing to the repository.**

- Do not invent a blob SHA or any other identifier; fetch it.
- After pushing, verify each file against the version you tested (for example by blob SHA). Trailing newlines and whitespace count. Do not reformat code you were not asked to touch.
- If you cannot clone the repository or run something in your environment, say so and reduce the claim accordingly.

**Branches and pull requests.**

- Never write to `main`. One concern per branch. Prefix: `feat/`, `fix/`, `chore/`, `refactor/` or `docs/`.
- `main` and open pull requests move. Re-check the base immediately before editing. When a change depends on an unmerged branch, say which branch it is stacked on and the merge order.

**Scope.**

- Do what was asked. Report adjacent problems instead of fixing them silently. If your own change causes a regression, fix it right away and tell the maintainer plainly.
- Keep documentation in the same change set as the behavior it describes, and fix stale documents you notice.
- Mark temporary evaluation behavior as temporary, in code comments and in the docs.

**Communication.** State assumptions instead of asking. Ask only when the decision is genuinely the maintainer's, one question at a time. Lead with the result, then say what was verified, what was not, and what remains.

## In-world writing

Use period-appropriate Japanese communication culture when generating fixture content. Examples include `(笑)`, `(爆)`, `(^^)`, `(^^;`, `(^_^;)`, `m(_ _)m`, and `>` quoting where appropriate. Avoid modern conventions such as `草`, modern `w` usage, `ググる`, and SNS terminology.

Era-appropriate topics may include Windows 95, PC-98, DOS/V, Macintosh, modems/ISDN, NIFTY-Serve/PC-VAN, the emerging WWW/Internet, Saturn, PlayStation, Pokémon, Evangelion, MIDI/FM sound, Akihabara, and local/offline events — subject to the exact mapped world date.

## Architecture maxim

**Same world/backend primitives, different historical host-program runtimes. Database truth, simulated agency, LLM prose.**
