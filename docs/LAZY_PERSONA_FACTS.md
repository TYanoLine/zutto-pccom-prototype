# Lazy persona facts

## Purpose

A persona must not be eagerly expanded into every possible life detail when the account is first materialized. Core persona state fixes durable behavioral structure such as age range, occupation, interests, activity tendencies and writing style. Concrete details are materialized only when a world action actually needs them.

This is the same lazy-world principle used for hosts and unobserved history: **unknown is not the same as nonexistent**.

## Development PoC flow

```text
sparse persona skeleton
 -> WorldEngine selects actor / board / thread / post-or-reply action
 -> choose a small actor-driven conversational tendency
    (share experience / brief reaction / different view / occasional question ...)
 -> lazily materialize only the persona facts needed by that post
 -> persist those facts + PostIntent claims as world truth
 -> when body becomes visible, rebuild context from canonical BBS data
      - recent thread messages
      - semantic envelopes for still-lazy messages
      - a few related earlier board posts (retrieval hint)
 -> render prose from Persona + committed facts + retrieved BBS context
```

Once a persona fact has been materialized, later posts reuse it. A prose renderer must not silently improvise a contradictory durable fact.

The important boundary is that conversation may **cause a previously unknown part of a persona to become concrete**, but the LLM writing the visible prose does not get to decide that durable fact on its own.

## Conversation context is reconstructed, not stored in the LLM

The BBS database remains canonical history. Provider-side chat/session history is never world state.

For lazy body rendering the development PoC reconstructs a bounded chat-like context from BBS records. Up to a small number of earlier posts in the same thread are supplied in chronological order. If an earlier body has already been materialized, its bounded body text is included. If its body is still lazy, its committed semantic envelope (claims / response act / semantic target) is included instead.

This means causal conversation does not require eagerly generating every old body, while a thread that has already been read/materialized can use the actual prose that participants saw.

A small related-post retrieval also searches earlier posts on the same board using committed semantic topic + recency. This is intentionally only a PoC for the future RAG boundary; it is not yet an embedding/vector search. Retrieved related posts are hints for duplicate-topic awareness and are explicitly **not** proof that the actor personally read them.

The eventual production design should additionally constrain thread/retrieval context by persona read/unread/observation state where appropriate.

## Information slots are metadata, not a conversation engine

The `pc98_environment` PoC still has a tiny set of development-only fact slots (usage pattern, modem style, startup configuration habits, log retention, shared-machine use, other use and pain points). These slots exist only to select/materialize bounded Persona facts and to measure semantic repetition.

They must **not** be treated as a checklist such as:

```text
configuration -> logs -> pain point -> shared machine -> ...
```

and they must not automatically create a new follow-up question after every reply.

The current PoC instead scores available details from the selected persona, their interests, the thread context and a stable seed. Novel information gets a modest preference, but repetition is allowed. A reply can be short, merely compare experiences, disagree, pick up a recent point, or occasionally ask a question. The world is not required to keep every thread productive.

`PostIntent.ResponseAct` records this actor-driven tendency. `InformationSlots` records which durable Persona facts happened to be materialized for that post. `RespondsToPostID` / `RespondsToClaims` may identify a semantic target, but they do not force a questionnaire progression.

## Historical boundary

Persona facts describe fictional residents and may include fictional household/use details where they do not assert external historical specifications. For example, `TAKA uses the same PC-98 for communication and games` is fictional world state.

Claims about real hardware limits, exact product behavior, release dates, prices, protocols or other external historical facts remain subject to `HISTORICAL_ACCURACY.md` and the Historical Knowledge boundary.

## Current PoC scope

The first concrete schema is `pc98_environment`, because the development materialization host exposed thin replies around the topic `通信に使ってる98の構成`.

The fixed candidate set is development scaffolding, not the intended production representation of every possible fact a person can have. The production direction is to let the world engine generate/materialize a bounded fact only when conversation or another world action requires it, subject to existing persona state, already-observed facts, historical constraints and deterministic/shared-world rules.

`PostIntent.Claims` records durable/persistent meaning the actor will express. The LLM receives the actual recent BBS flow as rendering context so it can avoid producing a sequence of mutually unaware standalone messages.

## Development reset

The materialization demo host provides a `RESET` command. It deletes:

- posts for the development host;
- topic-triggered persona facts for the development host cast;
- development token-usage totals.

It preserves the host profile, board catalog, memberships and sparse core persona skeletons. This exists only to compare PoC behavior and is not a production-world operation.
