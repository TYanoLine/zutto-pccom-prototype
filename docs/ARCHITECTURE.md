# Architecture

## Core rule

Keep these components separable:

1. Terminal
2. Virtual modem
3. Transport
4. Virtual telephone network
5. Host-program runtime
6. World engine
7. Persistence
8. LLM provider

The browser is only one terminal/transport implementation.

## Proposed flow

```text
Browser UI
  React shell
  PC-98 Canvas terminal
  VirtualModem / AT parser
      |
      | WebSocket
      v
Go gateway
  VirtualTelephoneNetwork
      |
      v
HostProgram registry
  +-- TurboBBS runtime
  +-- KTBBS runtime
  +-- BIG-Model runtime
  +-- Erika K runtime
  +-- mmm runtime
  +-- RT-BBS runtime
  +-- VS runtime
      |
      +---- shared BBS/world services ---- WorldEngine ---- PostgreSQL
      |                                      |
      |                                      +--------- Azure OpenAI provider
      |
      +---- output byte/text stream
```

## Host-program boundary

Historical BBS packages are separate programs, not profiles for one giant configurable state machine.

A deliberately thin interface is appropriate, conceptually:

```go
type HostProgram interface {
    Welcome() string
    HandleLine(string) Result
}
```

Each implementation may have completely different internal state and concepts. Share lower-level services such as sessions, world/store access, members, posts where semantics genuinely match, line presence, and transport helpers. Do not share menu/state-machine abstractions merely to remove duplication.

**操作系は別実装、世界データとインフラだけ共有。**

## Call session vs transport

A WebSocket is an attachment to a logical call session, not the call itself. Brief transport loss must not immediately destroy the BBS runtime or release the logical line. Reconnect may resume the server-side call within a grace period; explicit hangup/logout or expiry terminates it.

This separation is also required for the future RS-232C bridge: WebSocket and Serial should attach to the same logical host/session model.

## Observation gate and world catch-up

Normal production world advancement is demand-driven. Before a host runtime exposes facts that may have become stale, an observation gate determines which world scopes must be current enough for that action.

Conceptually:

```text
terminal/host action
 -> ObservationGate
 -> resolve required scopes
 -> GenerationCoordinator
      -> read last_simulated_at + simulation_version
      -> acquire narrow per-scope lease if stale
      -> WorldEngine catch-up
      -> optional Azure OpenAI provider prose/enrichment
      -> validate + COMMIT
      -> release lease
 -> HostProgram renders committed state
```

A successful CONNECT is the host observation boundary. Directory listing, host metadata lookup/creation, BUSY, NO CARRIER, and NO ANSWER do not trigger article generation. CONNECT starts independent board-header catch-up jobs in the background for that host.

Host-program reads are synchronization barriers over the narrowest required scope. A board/index request waits only for that board's header job; a thread/article read waits for the selected thread body job. If the needed data finished while the caller was navigating login/menu screens, the read returns immediately. A slow board must not hold unrelated boards behind a host-wide barrier, and the terminal never needs a modern "AI generation progress" workflow.

For long elapsed intervals, catch-up should be time-compressed: select durable important transitions first, then materialize only the detailed posts/events required by the current observation.

### Preplanned board activity

A board may have world history before any article wording has been observed.
For hosts with a known founding date, membership count, world time, and
station-specific board activity metadata, World computes a prose-free
`BoardActivityState` first. It includes cumulative/retained root and reply
counts plus the retained-history window.

A HostProgram may render those counts in its own native board menu before any
subjects/bodies exist. Opening the board then materializes the already-decided
retained root/reply slots; reading an article still materializes only its body.

This prevents an observer from causing a previously empty board to acquire
history merely by entering it. The current numeric model is experimental
fictional reconstruction rather than measured 1996 traffic statistics. See
`docs/BOARD_ACTIVITY_PLANNING.md`.

## Shared BBS article engine

BBS article generation is a world service, not a host-program feature. Historical
host runtimes own menus, commands, board topology, threading/append presentation,
limits, and access rules; they do not each implement a separate "AI posting"
algorithm.

The canonical direction is:

```text
World/observation clock
 -> shared BBS article engine
      -> decide bounded catch-up window
      -> fix actor/time/root-vs-reply slots before prose
      -> load recent canonical board history
      -> generate a board-level title pool in one batch
      -> realize multiple selected slots together
      -> validate diversity / era / causal consistency
      -> commit canonical posts
 -> host program renders those committed posts in its own grammar
```

The shared planner keeps actor/time/root-vs-reply topology world-owned, then
uses a **title-first candidate pool** for root wording. The normal contextual
Azure OpenAI path generates **100 uncommitted subjects in one structured call** using
the world date, recent board history, avoid-list and bounded historical
referents. Each candidate carries its own nested `historical_claims[]` so a
claim cannot drift onto a different title by array index. The prompt asks for a
large concrete claim-free reserve while still preserving board/period texture
through claim-bearing real names where appropriate.

Title-era routing is claim-driven rather than Jev-driven. Claim-free candidates
do not go to Historical KB merely because a classifier is uncertain. If a
tentative winner carries one or more historical claims, those claims must be
verified by Historical KB/research before adoption. The generator's claim marker
is a routing hint, never historical proof.

Jev is currently retained only for candidate × already-selected world-slot /
persona fit plus a candidate-level topic-identity score. To keep that fit request
bounded, the adaptive title pool (currently 60..100 candidates) is evaluated in
chunks of at most 20 titles × 20 remaining world events; Jev's era questions are
suppressed for this path. The topic-identity hard floor is deliberately low so it
blocks clear topicless/hidden-referent roots without preferring complete sentences;
higher specificity is a ranking signal. If Jev fit is unavailable, the existing
Azure OpenAI title reviewer remains the fallback. Code performs the one-title/one-slot
assignment.

Only after fit, duplicate checks and any required historical verification does
the selected subject and its minimal summary become canonical world state. The
system does **not** first invent a detailed canonical topic and then force the
subject to echo it; specificity is proposed broadly and filtered before adoption.

Title generation and adoption are additionally governed by
`docs/BBS_TITLE_QUALITY_GATES.md`. That document distinguishes blocking
structural/world-correctness gates from provisional batch-naturalness thresholds
and corpus-authenticity audit criteria.

After a selected post is read, normal host navigation, direct development
inspection, and isolated Lab runs use the same article-detail/body pipeline.
`PostIntent.ArticleDetailsMaterialized` is canonical article state independent of
detail count: a persisted zero-detail result is complete, while planner,
validation, or persistence failure stops body rendering and remains retryable.
Host programs continue to own their append/title presentation and failure text.

The planner uses up to 48 recent board posts as title/flow context and may include
short excerpts from recent bodies when available. A contextual generation
normally needs one 100-title pool; one additional fresh 100-title pool is allowed
only as recovery if fit/duplicate/research attrition still leaves world-selected
roots unresolved. Failed/duplicate/too-similar candidates are discarded.

Header planning does not pre-render article bodies. It commits only the canonical
header/topology/semantic brief required to show the index. Body prose remains
lazy and is materialized when the article is read.

A host software identifier must not select a different article-generation
algorithm. Host-program-specific code may supply topology or representation
constraints only. The isolated materialization-demo host remains a diagnostic
harness and is not a production host-program exception to this rule.


### Semantic response vs host-native reply representation

The shared world layer must not equate "responds to another post" with any one
host's visible reply syntax.

Canonical response causality lives in `PostIntent.RespondsToPostID`
(and legacy `SourcePostID` where applicable). Host-native article representation
is a separate projection:

- `ParentID` is non-zero only when that host software exposes/stores native
  parent/child or append topology;
- `Subject` is the host-native subject for that article and may be empty for a
  response that has no independent subject;
- a flat-message host may therefore have `ParentID == 0` while
  `RespondsToPostID != 0`;
- an append-style host may have `ParentID != 0` and an empty response subject.

The shared engine must never synthesize `Re:` as a universal convention.
Each concrete HostProgram projects the already-selected semantic response into
its own article model. Unknown historical host programs should fail closed until
their reply representation is researched or explicitly marked provisional,
rather than inheriting another program's syntax by default.

Debug resets must likewise operate at the shared engine boundary. They may remove
engine-generated history for an experiment host while retaining seed history,
human/user posts, boards, personas, and host-program configuration.

## Generation coordination and concurrency

The runtime/store boundary exposes an optional observation capability rather than embedding AI calls into each historical host program. A host runtime supplies its own board catalog; the shared coordinator owns jobs and waiting. This preserves separate KTBBS/Erika/etc. state machines while sharing world synchronization.

Conceptually:

```go
BeginHostObservation(host, hostProgramBoards) // non-blocking
WaitForBoardHeaders(ctx, host, board)         // joins only this board job
WaitForArticleBody(ctx, host, board, postID)  // joins thread job
```

The generation coordinator serializes persistent fact creation at a narrow world scope. It is responsible for concepts such as:

- `last_simulated_at`;
- `simulation_version`;
- generation/update lease ownership and expiry;
- idempotency/retry handling;
- stale-result rejection;
- queue depth and bounded generation capacity.

If two callers observe the same stale scope concurrently, only one may commit the next version. The other caller waits, receives admission backpressure, or re-reads the winner's committed state. It must not commit an alternate future.

Avoid a single global world lock. Prefer host/board/thread/persona or another domain-appropriate scope.

## Backpressure boundary

Runtime capacity and generation budgets may participate in call admission. The world/coordination layer exposes machine-readable pressure such as scope lock contention, queue saturation, provider pressure, or configured token/cost budget state. The telephone/host layer decides how that becomes period-appropriate UX.

Examples include:

```text
scope unavailable / no admission capacity
 -> VirtualTelephoneNetwork
 -> BUSY

connected call + slow materialization
 -> host wait state or output pacing

new call admitted at a plausible lower tier
 -> CONNECT <lower supported rate>
```

Do not leak raw provider errors, HTTP status codes, or modern cloud terminology into the in-world terminal experience.

The backend should still complete work as efficiently as possible. Simulated bps pacing belongs at the terminal/transport presentation boundary; it is not a reason to keep expensive backend work artificially slow.

## Persistence rule

Generation is two-phase:

```text
stable seed + deterministic distributions
        -> core facts
        -> DB COMMIT
        -> optional LLM enrichment
        -> validate against facts/schema
        -> DB COMMIT
```

Never regenerate an already committed identity/history merely because a prompt is rerun.

Observation-driven catch-up follows the same invariant: once an observation materializes previously unknown history, it becomes shared persisted history rather than a viewer-specific alternate past.

## LLM division of responsibility

World engine owns:

- activity timing
- read probability
- reply probability
- topic selection
- line occupancy
- demographic sampling
- relationship/opinion values
- whether an event occurs
- which stale scopes require catch-up
- coarse-vs-detailed materialization policy

LLM owns:

- wording a selected post/message
- enriching a newly generated persona within fixed constraints
- summarizing relationship/history into compact context
- generating prose for already-selected world events

LLM output must be validated. Prefer structured outputs for fact-producing calls.

## Context assembly

Recommended order for prompt-cache friendliness:

1. stable 1996 rules
2. stable rules for the specific host program/version
3. host facts
4. persona facts / persistent opinions
5. relationship summary
6. relevant recent messages/events
7. immediate selected action

Do not send all historical logs. Retrieve only relevant facts and summarize old history.

## Temporary HAKATA generator evaluation mode

While the shared BBS article generator is being evaluated, the fixed experiment
station `0920000196` has **no article seed at all**. The former hand-authored
sample posts and the generated 40-root-per-board baseline have been removed.
The station keeps a sparse membership population matching the code-defined
`Host.Members` count (currently 326). These records are cheap identity/activity
skeletons rather than article/content templates; expensive biography and life
facts remain lazy.

On process startup, any older persisted HAKATA article snapshot is cleared. On
every successful CONNECT the server clears the station's entire article state
again, clears completed observation leases, and enables immediate
first-observation generation. During this temporary mode, user-written test posts
also do not survive the next call.

CONNECT itself must not fan out generation over every empty board. After login,
the runtime starts one deliberately small speculative board-header job. Entering
a forum may prefetch only its first child board. When the user actually requests
a leaf-board index, that command joins/starts exactly that board's shared job and
**waits until its headers are committed**; ordinary UI must never show an empty
placeholder that later requires BX/refresh to reveal completed generation.

The initial board batch materializes canonical headers + semantic state only.
For the HAKATA generator-evaluation station, the first observed leaf board is
materialized as a **28-day accumulated history with 40 visible root articles**.
Reply events are added on top of those roots and may target roots selected earlier
in the same simulated window, so the index can show APE activity instead of
forty unrelated threads compressed into one six-hour cadence. This is an
evaluation fixture, not a historical claim about a universal posting rate, and
does not change the ordinary 3..7-event catch-up batch size used by other hosts.
Title vocabulary and historical verification for a multi-date catch-up window
are conservatively gated by its earliest event date so a later release cannot
leak backward into an older article. Article bodies remain empty until BR/read
observation, where the existing
thread-body barrier materializes only the requested thread. This is the intended
minimum-scope execution pattern even while HAKATA's reset-on-call behavior itself
remains a temporary generator-quality evaluation override.

The former hidden bare `99` reset command has been removed; `BJ 99` continues
to mean the station-specific hidden board.

## Current prototype shortcuts

Current code intentionally still has shortcuts, including:

- JSON WebSocket rather than a binary CP932 stream
- memory store rather than PostgreSQL as the live repository
- incomplete historical host-program coverage
- incomplete real line-occupancy/NPC scheduler
- simplified terminal/ANSI behavior
- atmospheric rather than fully historical telephone tariffs
- Azure OpenAI provider boundary present but AI not yet part of normal host posting behavior

These shortcuts are adapter/prototype boundaries and must not become domain rules.
