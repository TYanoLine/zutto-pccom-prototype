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
7. Inspect current `main` and the relevant code immediately before editing. The human developer may be changing the repository concurrently.

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

### Station-specific content lives in the station definition

A station's own content is data, not host-program code. Station names, banners, headings, greetings, board layouts, and resident members are defined in the station's preset (`apps/server/internal/hostcatalog/presets/*.yaml`). The host program implements behavior and prints what the definition gives it; it does not build, pad, translate, or default station wording, and it does not name any particular station. Behavior that only an evaluation station needs is switched on by that station's explicit `debug` / `generation` flags, never by a phone number, a host ID, or the role.

The host definition is immutable at runtime. Changes to a running station are recorded as separate, dated differences, not as edits to the definition. See `apps/server/internal/hostcatalog/README.md`.

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

## Implementing a spec

Larger changes are written down first in `specs/NNN-name/` (`spec.md`, `plan.md`, `tasks.md`; see `specs/README.md`). When you are asked to implement one:

- **The deliverable is code.** Go, YAML and tests, opened as a pull request. The spec files already exist on `main`: do not create or edit them. The only spec-folder file you add is the `verification.md` that the tasks name. "The spec files already exist" or "the working tree is clean" is not completion.
- **Read in order and follow the tasks in numeric order.** `spec.md`, then `plan.md`, then `tasks.md`, including the rules at the top of `tasks.md`. The last task (open a draft pull request) defines "done".
- **Evidence comes first.** When a task asks for a golden file or baseline recorded from the unchanged code, generate and commit it *before* the implementation commits, and never regenerate it afterwards to make a test pass. A failing golden test means the implementation is wrong. Evidence produced by the new code proves nothing.
- **Stay in scope.** Do not touch anything listed under "Out of scope". Do not create formatting-only diffs (do not run `gofmt -w` over existing files you are not otherwise changing). Do not edit `.github/` unless the task says so.
- **Do not guess.** If the spec and the code disagree, or an instruction cannot be followed as written, stop and say so in the pull request description instead of choosing a different design.
- **Stop after repeated failure.** If the same test failure survives two fixes, stop and report which assertion or line differs.
- **Report honestly.** The pull request description and `verification.md` list the exact commands you ran and their results. Write "not run" with the reason for anything you could not run. Workflows may not run automatically on an agent's pull request; that is not a pass.

## In-world writing

Use period-appropriate Japanese communication culture when generating fixture content. Examples include `(笑)`, `(爆)`, `(^^)`, `(^^;`, `(^_^;)`, `m(_ _)m`, and `>` quoting where appropriate. Avoid modern conventions such as `草`, modern `w` usage, `ググる`, and SNS terminology.

Era-appropriate topics may include Windows 95, PC-98, DOS/V, Macintosh, modems/ISDN, NIFTY-Serve/PC-VAN, the emerging WWW/Internet, Saturn, PlayStation, Pokémon, Evangelion, MIDI/FM sound, Akihabara, and local/offline events — subject to the exact mapped world date.

## Architecture maxim

**Same world/backend primitives, different historical host-program runtimes. Database truth, simulated agency, LLM prose.**
