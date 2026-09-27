# SDD-002: Persona history feedback from committed posts

Status: specified, not implemented
Updated: 2026-09-27

## Goal

A resident's detailed past should grow gradually from canonical events and committed posts. Later writing should be constrained by that accumulated history without eagerly generating a full biography.

## Dependency

Requires SDD-001's canonical-before-prose boundary.

## Core flow

```text
existing Persona + PersonaFacts
 -> selected world action
 -> canonical article-local details
 -> committed post body
 -> history candidates
 -> deterministic validation/provenance checks
 -> persisted versioned persona history
 -> later planning/rendering
```

## Sources

NPC-generated post:

- prefer already-canonical PostIntent / article_detail / producer facts as the source of persistent-history candidates;
- use prose extraction only when the body contains a supported self-statement not already represented by canonical semantic state.

Human-authored post:

- the text itself is canonical as an utterance;
- a bounded structured extractor may propose self-fact/preference/experience/temporary-state candidates;
- do not infer private inner truth beyond what the human actually wrote.

## Time model

Persistent history needs to distinguish:

- `occurred_at`: when the represented experience/state happened, if known;
- `materialized_at`: when the world first concretized this fact;
- `disclosed_at`: when the actor publicly stated/revealed it, if applicable.

A fact materialized in August may describe an experience from the previous year. It may constrain posts after materialization, but must never leak into an already-existing earlier post merely because its `occurred_at` is old.

## Fact classes

Initial semantic classes:

- durable fact / possession / ordinary baseline;
- preference/opinion;
- experience;
- habit/routine;
- temporary state;
- relationship-local state where appropriate.

These are semantic behavior classes, not a required universal slot catalog. Keys remain open.

Temporary state requires an expiry or end condition when one is known; it must not become a permanent personality trait by accident.

## Conflict handling

Never silently overwrite history.

If a new accepted event changes a prior durable state, persist a new version/change event. If two statements conflict but no world event establishes which is true, preserve the uncertainty for SDD-003 rather than rewriting one away.

## Provenance

Every persisted history item should be traceable to canonical evidence such as:

- source post id;
- source PostIntent/detail id or extraction evidence span;
- source kind;
- materialization timestamp.

## Idempotency

Post-history materialization must be idempotent by post/version. Re-reading the same article must not duplicate facts.

Concurrent attempts require a DB/lease/CAS boundary before production rollout.

## Non-goals

- deciding whether another resident knows the fact (SDD-003/004);
- making accumulated history a reason to post;
- filling missing biography for color.

## Acceptance criteria

- same post cannot create duplicate history on retry;
- future materialized facts do not affect earlier posts;
- conflicts are versioned/rejected, never silently overwritten;
- NPC history candidates are grounded in pre-prose canonical state or exact body evidence;
- human history candidates require exact textual support;
- temporary state can expire without deleting historical evidence.