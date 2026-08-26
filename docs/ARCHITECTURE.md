# Architecture

## Core rule

Keep these components separable:

1. Terminal
2. Virtual modem
3. Transport
4. Virtual telephone network
5. BBS runtime
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
BBSRuntime
      |
      +---- WorldEngine ---- PostgreSQL
      |          |
      |          +--------- OpenAIProvider
      |
      +---- output byte/text stream
```

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

LLM output must be validated. Prefer Structured Outputs for fact-producing calls.

## Context assembly

Recommended order for prompt-cache friendliness:

1. stable 1996 rules
2. stable BBS software-family rules
3. host facts
4. persona facts / persistent opinions
5. relationship summary
6. relevant recent messages/events
7. immediate selected action

Do not send all historical logs. Retrieve only relevant facts and summarize old history.

## Prototype shortcuts

Current code intentionally shortcuts:

- JSON WebSocket instead of binary CP932 stream
- memory store instead of PostgreSQL repository
- one functional host plus testing hosts
- stylized modem audio, not protocol-accurate negotiation
- simplified ANSI parser
- accelerated pseudo toll meter
- no background behavior scheduler yet
- OpenAI provider is included but not wired into posting behavior

These shortcuts are adapter boundaries and should not leak into domain rules.
