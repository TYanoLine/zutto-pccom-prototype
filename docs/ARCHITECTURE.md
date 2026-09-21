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
      |                                      +--------- OpenAIProvider
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
      -> optional OpenAIProvider prose/enrichment
      -> validate + COMMIT
      -> release lease
 -> HostProgram renders committed state
```

A successful CONNECT is the host observation boundary. Directory listing, host metadata lookup/creation, BUSY, NO CARRIER, and NO ANSWER do not trigger article generation. CONNECT starts independent board-header catch-up jobs in the background for that host.

Host-program reads are synchronization barriers over the narrowest required scope. A board/index request waits only for that board's header job; a thread/article read waits for the selected thread body job. If the needed data finished while the caller was navigating login/menu screens, the read returns immediately. A slow board must not hold unrelated boards behind a host-wide barrier, and the terminal never needs a modern "AI generation progress" workflow.

For long elapsed intervals, catch-up should be time-compressed: select durable important transitions first, then materialize only the detailed posts/events required by the current observation.

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

## Current prototype shortcuts

Current code intentionally still has shortcuts, including:

- JSON WebSocket rather than a binary CP932 stream
- memory store rather than PostgreSQL as the live repository
- incomplete historical host-program coverage
- incomplete real line-occupancy/NPC scheduler
- simplified terminal/ANSI behavior
- atmospheric rather than fully historical telephone tariffs
- OpenAI provider boundary present but AI not yet part of normal host posting behavior

These shortcuts are adapter/prototype boundaries and must not become domain rules.
