# Host definitions

How a BBS host is defined, listed, dialed and recognized. The code lives in
`apps/server/internal/hostcatalog` (definitions and the YAML loader),
`apps/server/internal/world` (the runtime `Host` and the store) and
`apps/server/internal/worldcatalog` (generated hosts). The preset file format and
its validation rules are documented in
`apps/server/internal/hostcatalog/README.md`.

## Three layers

| Layer | What | When it is fixed | Mutable? |
|---|---|---|---|
| Skeleton | `HostDescriptor`: name, phone, program, region, lines, speed, founding date, popularity, members, traits, `listed`, `role` | when the host is created (a preset file or the generated directory) | no (a host reset bumps `generation`) |
| Detail | welcome text, menus, boards, program-specific data | first access to the host | no, once materialized |
| State | posts, members, board activity | continuously | yes |

The skeleton holds no derived values. The busy rate is computed from popularity,
lines and time of day; time-varying membership belongs to the board-activity model.

## Where host definitions come from

- **Preset hosts** are YAML files embedded in the binary. They may fix any skeleton
  field. Unspecified fields are meant to be filled deterministically from the world
  seed; that is not implemented yet, so today a preset must be complete.
- **Generated hosts** are 100 per world, created by `worldcatalog` from the world seed
  (names by an LLM, everything else deterministic) and stored in PostgreSQL.
  `Center.Descriptor()` converts them to the same `HostDescriptor`.

Wiring today (the main gap): only preset hosts are dialable. `world.NewMemoryStore`
builds them from the presets, while the dial path (`telephone.Network` ->
`HostByPhone`) has no world scope, so a generated host's number is not found
(NO ANSWER). Making generated hosts dialable needs a world-scoped `Directory`
(see the roadmap).

## Listed vs. dialable

`listed` only controls whether a host appears in the host directory. It never
decides whether a number can be dialed: an unlisted host answers `ATDT` normally.

- Presets must state `listed` explicitly (there is no default), so a debug host
  cannot be published by omission.
- Generated hosts are listed by default today. The intent is a generation setting
  whose value is stored on each host when it is created, so changing the default
  never affects existing worlds.
- Hosts with role `debug` or `test` must be unlisted (validated).

## Roles

`role` is an optional operational tag copied into `world.Host.Role`.

| role | meaning |
|---|---|
| (empty) | ordinary host |
| `experiment` | the generator-evaluation station (currently HAKATA CANAL NET) |
| `test`, `debug` | development fixtures; must be unlisted |
| `event` | reserved |

Behavior that only the evaluation station gets follows the role through
`Host.IsExperiment()`, never a phone number or ID:

- the debug auto-reset on CONNECT;
- `/api/debug/bbs/reset` and `/api/debug/bbs/sample` (an unknown number and a
  non-experiment host get the same HTTP 400, so the endpoints do not reveal which
  numbers exist; `sample` defaults to the experiment host when `phone` is omitted);
- the durable snapshot and the startup baseline clear;
- the generation trace and the generated-content log;
- the title-led prose experiment;
- the resident population;
- the web client's generation-trace link (the server reports `role` in
  `dial_result.host`).

At most one preset may have the role, because the population generator uses fixed
persona IDs; loading panics otherwise. With no experiment host these features are
simply off. The environment variables that still mention HAKATA
(`DEBUG_LOG_HAKATA_GENERATED`, `DEBUG_HAKATA_LLM_TRACE`, `HAKATA_FREEFORM_BODY`)
apply to the experiment host.

## Identity rules

- A published preset's `key` and `phone` never change. Raise `revision` when its
  content changes.
- Runtime `world.Host.ID` currently equals the preset key, because existing posts
  and snapshots are keyed by it. A bare key is not unique across worlds, so
  per-world hosts need an explicit ID scheme before they are wired in.
- Code must not branch on a specific phone number or host ID.

## Test hosts

- **HAKATA CANAL NET** (`0920000196`): the real preset and the experiment host.
- **`busy-test`** (`0459999999`): a hidden preset (`role: test`, unlisted). Dials are
  numbered from 1 and the first four are BUSY, so with auto-redial it connects on
  the fifth. That behavior still lives in `telephone`, not in the preset.
- **Local test station** (`0312345678`, `LOCAL_TEST_NUMBER`): lives in the web client
  only; no server is involved.
- Everything else a test needs, the test defines itself (helpers exist in `world`,
  `worldrepo`, `worldpersist`, `bbsengine` and `hostprogram/turbobbs`). A test must
  not depend on a production station.

## Special numbers (design only)

Numbers such as 110, 117 or 119 that answer with a scripted voice although no BBS
exists are **not implemented**. The intended shape:

- resolve in a chain before any host lookup: special numbers, then hosts (listed or
  not), then unassigned;
- the result is a destination type, not necessarily a host;
- reserved numbers are enforced through `hostcatalog.Options.ReservedPhones`, so no
  host can take one;
- number normalization (digits only, 184/186 prefixes, `ATDT` vs `ATDP`) lives in
  one function;
- scripts are authored text, never LLM output at call time.

## Status and open work

Done: `HostDescriptor`; YAML presets and loader; `listed` and `role`; role-driven
experiment behavior in the server and the web client; removal of the hard-coded
stations.

Open:

1. A world-scoped `Directory` in the dial path (needed for generated hosts and for
   special numbers).
2. A `ProgramSpec` registry replacing `hostcatalog.knownPrograms` and
   `RuntimeSoftwareID`.
3. Database migration: `origin`, `listed`, `host_key`, `preset_revision`, region and
   `host_details` (with a status), replacing the ad-hoc `EnsureSchema`.
4. Region and trait generation; phone numbers derived from region (today
   `0<area><8 digits>` is unrelated to region).
5. Lazy per-host detail (welcome, menus, boards) persisted with program and schema
   version. The Erika-K welcome text, board tree and SYSOP are still HAKATA's, in code.
6. Moving the `telephone` fixtures to `dial.behavior`.
7. Web: the host list from the server (the browser still seeds `CenterDirectory.ts`
   with HAKATA).
8. Renaming HAKATA-specific identifiers (`hakata_cast.go`, the environment
   variables) once the design is stable.
