# Conversation-view materialization PoC

The isolated `fresh` materialization lab can run a development experiment that bypasses the host-wide semantic Producer.

## Goal

Test whether the naturalness of chat-style BBS roleplay can be recovered without making model chat history authoritative world state.

The database remains canonical for:

- actor identity
- timestamps
- board placement
- root/reply topology
- explicit source post
- world-selected routing domain/cause kind
- root discourse mode
- sparse world-owned micro-situation
- already-rendered BBS prose

The experimental header pass does **not** persist Producer `episode / referents / actor_knowledge / contribution / goal` briefs.

## Sparse situation layer

A broad routing domain such as `local`, `games`, `communications` or `modem` is too weak by itself: prose generation can repeatedly invent the same kind of occurrence or make an `ask_peers` post impossible to answer. The fresh-lab PoC therefore selects a small canonical situation only **after a write action has already been selected**.

`PostIntent` stores:

- `situation_kind` — a reusable everyday facet such as a local notice change, route condition, game naming choice, or post-confirmation delay
- `situation_summary` — the canonical occurrence + discourse-mode boundary
- `situation_facts` — open fact/boundary strings used by the renderer

This is deliberately sparse. The world does **not** simulate a detailed daily life for every NPC. Visits can still resolve to ROM/no-op without any situation being created. A concrete situation is materialized only for an actual selected write slot.

Situation selection uses local novelty rather than a large collection of per-person rules:

- avoid the same situation facet on the same board for roughly the next 36 hours when alternatives exist
- avoid reusing the same facet for the same actor for roughly ten days when alternatives exist
- if every facet is exhausted, allow weighted reuse rather than failing or growing a brittle exception catalog
- replies inherit the actual canonical source situation instead of inventing another topic
- continuation roots keep the source situation but require a materially new development

The facet catalog is compositional routing vocabulary, **not article templates**. It fixes what happened at a small semantic level and defines what must not be inferred; the LLM still decides natural Japanese wording and incidental expression. The design is intended to be replaceable later by persisted host/domain world data without changing the conversation-view contract.

## Render-time conversation view

Immediately before prose rendering, the repository rebuilds a transient conversation view from canonical data:

- the exact thread so far
- the explicit source post selected by the world layer
- the current sparse situation and its facts/boundaries
- a few recent canonical posts by the same actor, marked as continuity/style context only
- small related-post retrieval already used by the existing renderer
- the current shell's world-layer cause boundary

Independent roots explicitly tell the renderer not to merge event details from other roots or the actor's previous posts. `ask_peers` roots additionally require enough concrete referent/observable detail to be answerable without guessing an unnamed title, place, product, device or hidden choice.

The article worker then chooses natural subject/body wording. Root subjects are committed from the worker result; reply subjects are canonicalized to `Re: <root subject>` after the root has been rendered. Existing chronological dependency rendering guarantees that a reply sees earlier thread prose first.

## Scope

`EnableDevelopmentConversationViewPoC()` is opt-in per repository instance. The server currently enables it only for the isolated fresh RESET-equivalent -> ALLBODY lab. Ordinary runtime materialization continues to use the existing Producer path.

This remains an experiment, not a production architecture decision. The important architectural direction is **sparse canonical world state + delayed concretization + transient DB-reconstructed conversation context**, rather than an always-running full-life simulation or a larger semantic Producer.


## Facetless A/B experiment

The fresh Lab also supports `situation_mode=facetless` for a deliberately weaker A/B condition. In this mode root shells keep actor/time/board/topology/routing-domain/cause/discourse-mode, but **no predefined situation facet or occurrence is selected**. The Article Worker must invent one small concrete occurrence while rendering from the conversation view. This mode is intentionally not the proposed production architecture: concrete root occurrence details are not canonical before prose. It exists to measure what the hand-written facet layer contributes to diversity, answerability and cross-root isolation. Replies still bind to the actual canonical source post. Default fresh behavior remains `situation_mode=facets`.


## Batched world-situation proposal PoC

The fresh Lab supports `situation_mode=batch`. World-selected standalone root shells across the bounded host window are sent to one compact Situation Proposer call. The proposer does not write subjects/bodies and does not choose actor/time/board/topology/routing/discourse. It returns free-form `object_class`, `change_class`, `occurrence`, `actor_observation`, `impact`, `uncertainty`, and `novelty_key` fields. These are not selected from a hand-written facet catalog.

The world layer validates proposals before committing them as `PostIntent.Situation*`. Within one board, duplicate object classes and highly similar occurrences are rejected; duplicate novelty keys are rejected host-wide. Rejected roots are retried together at most once while the already accepted situations are supplied as an avoid set. Replies are not separately proposed; they continue to bind to the actual canonical source situation/body. Only after accepted situations are stored does Conversation View render article prose.

For scale testing only, fresh can expand the isolated development snapshot from 3 to at most 6 boards and raise the Conversation View shell limit from 5 to at most 10 per board. These extra boards and posts are never written back to the saved demo world.
