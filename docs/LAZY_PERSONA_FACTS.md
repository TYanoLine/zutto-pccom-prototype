# Lazy persona facts

## Purpose

A persona must not be eagerly expanded into every possible life detail when the account is first materialized. Core persona state fixes durable behavioral structure such as age range, occupation, interests, activity tendencies and writing style. Concrete details are materialized only when a world action actually needs them.

This is the same lazy-world principle used for hosts and unobserved history: **unknown is not the same as nonexistent**.

## No fixed content catalogs

BBS content generation must not be driven by a hand-written catalog of topic candidates, subject variants, information slots, canned response acts, follow-up questions, or predefined PersonaFact sentences.

Those structures caused the development PoC to produce mechanically plausible but semantically wrong conversations—for example, a broad question about a resident's computer environment could select an unrelated `log retention` slot merely because it existed in the same hard-coded topic bucket.

The development path therefore treats semantic content as open-ended. Fixed code may still represent things that are genuinely product/world mechanics (board IDs, host runtime behavior, activity probabilities, access rules, protocol/UI strings, era constraints), but it must not contain a menu of what residents are allowed or expected to talk about.

## Development PoC flow

```text
sparse persona skeleton
 -> WorldEngine selects actor / time / board / root-or-reply topology
 -> retrieve recent canonical BBS state
 -> semantic planner proposes this concrete event's free-form semantics
      - subject (for a root)
      - short free-form topic summary
      - motivation / stance / goal
      - zero to a few durable fictional personal facts genuinely needed now
 -> world layer validates the proposal
      - actor/time/topology cannot change
      - existing PersonaFact values win on key conflicts
      - bounded shape and era/meta constraints
 -> accepted Persona facts + PostIntent are persisted as world truth
 -> when body becomes visible, rebuild context from canonical BBS data
 -> render prose from Persona + committed intent + BBS context
```

The semantic planner is a **proposal mechanism**, not a source of truth. Provider chat/session state is never canonical world state. The database becomes authoritative only after the world layer accepts and persists a bounded proposal.

In a later production architecture, more of the proposal/validation policy can move into dedicated WorldEngine components. The important invariant is that a prose renderer must not silently invent or overwrite durable world facts.

## Open PersonaFact keys

`PersonaFact.Key` is an open semantic key, not an enum or slot catalog. A planner may propose a key such as:

```text
computer.communication_usage
household.shared_computer
hobby.favorite_genre
```

only when that concrete event actually needs the fact. There is no required universal schema saying every resident must eventually acquire all such keys.

If a key is already materialized, its stored value wins. A later proposal with a contradictory value does not rewrite the resident's history; an actual change would need to be represented as a world event/versioned fact instead.

## Conversation context is reconstructed, not stored in the LLM

The BBS database remains canonical history. Provider-side chat/session history is never world state.

For lazy body rendering the development PoC reconstructs a bounded chat-like context from BBS records. Earlier materialized bodies are supplied as body text; still-lazy messages contribute their committed free-form semantic envelopes. A small related-post retrieval is also supplied as a duplicate-topic hint.

Retrieved related posts are not automatically treated as the actor's personal memory. Production should additionally constrain context by the persona's read/unread/observation state where appropriate.

## Causal body materialization

Lazy body generation must not make textual history depend on the human user's reading order.

If a user opens a later reply before earlier messages in the same thread have bodies, the development path first renders the missing predecessors chronologically, commits them, and only then renders the requested reply from those canonical earlier bodies. Thus opening `1004` first still resolves the textual sequence as `1002 -> 1003 -> 1004`.

This preserves lazy generation—unrelated threads remain untouched—while preventing observation order from rewriting the conversation.

## Historical boundary

Persona facts describe fictional residents and may include fictional household/use details where they do not assert external historical specifications. A fictional resident may, for example, own or share a computer as part of world state.

Claims about real hardware limits, exact product behavior, release dates, prices, protocols or other external historical facts remain subject to `HISTORICAL_ACCURACY.md` and the Historical Knowledge boundary. The semantic planner must not turn a need for conversational specificity into unsupported external historical assertions.

## Development reset

The materialization demo host provides a `RESET` command. It deletes:

- posts for the development host;
- lazily materialized Persona facts for the development host cast;
- body-render and semantic-planning token diagnostics.

It preserves the host profile, board catalog, memberships and sparse core persona skeletons. This exists only to compare PoC behavior and is not a production-world operation.
