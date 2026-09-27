# SDD-004: Sparse observation, relationship, and memory retrieval

Status: specified, not implemented
Updated: 2026-09-27

## Goal

Residents should remember only what they plausibly observed. The implementation must remain sparse and avoid an all-person-pairs simulation.

## Core principle

The database may know every post. A resident does not.

Actor-specific generation receives only a bounded projection of world history supported by observation/interactions.

## Strong observations

Initially persist only strong, cheap evidence such as:

- actor replied to a post;
- actor quoted a post;
- actor participated in the same thread after the post;
- explicit host action guarantees the post was displayed to that actor.

A direct reply is sufficient evidence that the actor knew the source post at that time.

## Weak/read-state observations

Later phases may add board/thread read cursors rather than one row per member per message.

Example:

```text
member/persona + board -> last_read_position
```

This can establish plausible reading without generating an O(member × post) table.

## Sparse relationship edges

Do not pre-create every pair.

Relationship state exists only for pairs with meaningful interaction, for example:

```text
A <-> B:
  direct_reply_count
  recent_interaction_at
  optional bounded affinity/trust state
```

A 300-member BBS therefore remains a sparse graph rather than a 90,000-edge full matrix.

## Retrieval at reply time

Query by:

- current actor;
- current target/speaker;
- current semantic topic/claim key;
- direct interaction history;
- recency.

Return a small bounded set, initially roughly 3-8 items.

Do not feed all posts read by the actor to an LLM.

## Example

If A previously replied to B saying "納豆が好き", and later B says "納豆が嫌い", A's reply planner may receive the earlier claim.

A resident who never observed the earlier post must not say "前は好きって言ってたよね" solely because the service database knows it.

## Cost constraints

No generation-time operation may iterate all persona pairs.

Normal work should be proportional to:

- participants in the current thread;
- prior direct interactions for the current actor/target;
- a small indexed claim/history result set.

## Forgetting/salience

Explicit human-like forgetting is optional. Initial implementation may treat strong direct interactions as durable but retrieve only a few relevant/recent items. Later salience decay may reduce retrieval probability without deleting canonical observation history.

## Acceptance criteria

- no all-pairs precomputation;
- direct reply establishes an observation edge;
- actor-specific retrieval never includes an unobserved claim;
- result set is bounded;
- board read state, if added, uses cursor/range compression rather than per-message fanout where practical;
- absence of a remembered relevant item remains a valid outcome.