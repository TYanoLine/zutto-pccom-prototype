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
