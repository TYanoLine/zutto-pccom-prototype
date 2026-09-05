# LLM behavior policy

This document is product behavior, not merely prompt advice.

## Human neutrality

- Refer to the human-controlled account as a normal member/persona in internal prompts where possible.
- Never instruct a model to "respond helpfully to the user" for in-world generation.
- Do not generate reactions from multiple residents merely because the human posted.
- Agreement, praise and curiosity require persona/world justification.
- Silence is a valid result and should usually be decided before the LLM is called.

## Persona persistence

Persist opinions/interests/relationships independently of prose. Example:

```json
{
  "pc98": 0.8,
  "windows95": -0.65,
  "internet": 0.3,
  "games": 0.9
}
```

If a human praises Windows 95, a persona with `windows95=-0.65` should not flip position unless a separate world event explicitly changes that opinion.

### Persona facts are not action triggers

A persisted persona fact is a contradiction guard / durable identity fact, not a queue of future topics.

For example, `offline_meeting.preference=positive` constrains future characterization but does not make an offline-meeting post more likely by itself. Likewise `computer.communication_usage=PC-98` does not justify repeatedly starting PC-98 threads.

Before an LLM can realize a new post, the world layer must independently select a current causal anchor/action. Background facts may be supplied to the LLM only for consistency with that already-selected cause.

## Historical ceiling

Every generation request receives the world date and a compact era rule set. Reject/regenerate obvious anachronisms such as modern SNS terminology, smartphones or later products/events.

## Activity pipeline

Preferred sequence:

```text
observation / scheduled eligibility
 -> determine stale scopes that actually matter
 -> online/activity sampling
 -> board/read sampling
 -> write/no-write decision
      -> ROM/no-op is a normal terminal result
 -> action selection
      -> root/reply + source thread/event
 -> causal anchor/topic selection by world layer
 -> if semantic/text realization is necessary: LLM
 -> validation
 -> persistence
```

Never:

```text
activity slot -> LLM invents why this person must post
```

and never:

```text
human post -> LLM invents all consequences
```

The normal production path is observation-driven. Do not run broad periodic LLM generation for every host/person merely to make the world appear alive. Unobserved detail should remain unmaterialized until an observation or a necessary shared-world dependency requires it.

When a stale scope has been unobserved for a long time, prefer a bounded catch-up request that summarizes/selects important transitions over replaying every hour or day with separate LLM calls. Persist durable selected facts first; generate individual prose only for details that become visible or otherwise necessary.

## Causal event shell contract

Any LLM call that realizes a BBS event should receive an event shell whose world-owned fields are already fixed. At minimum for the current prototype:

- actor;
- timestamp;
- root/reply action;
- source/parent when applicable;
- causal anchor;
- cause kind.

The model may realize exact subject wording and human-readable semantic summaries within those constraints. It must not swap the anchor for a more salient background fact, manufacture an unrelated reason for posting, or create an additional event.

A continuation is especially strict: "this person discussed X recently" is insufficient. A new root continuation requires a separate world progress/change gate; the LLM must add a materially new development rather than paraphrasing the prior post.

## Generation classes

Use cheaper/faster models for routine prose and reserve stronger models for consistency repair, complex history or multi-entity event planning. Keep exact model IDs configurable through environment/configuration rather than domain code.

## Cost and capacity policy

LLM cost is an explicit operational constraint, but it must not become hidden world corruption.

Generation code may consider:

- estimated input/output tokens;
- prompt-cache opportunities;
- per-request generation limits;
- rolling per-service or per-scope cost/token budgets;
- provider concurrency/rate limits;
- queue depth and latency.

When capacity is constrained, prefer in this order where semantically valid:

1. do not call an LLM for an action that can resolve to silence/no-op deterministically;
2. narrow context to relevant persisted facts;
3. compress long catch-up periods into fewer structured transitions;
4. materialize only the current observation's required detail;
5. use an approved cheaper/faster generation class;
6. defer nonessential detail;
7. let the telephone/host admission layer apply explicit backpressure such as `BUSY` when the required scope cannot safely be advanced yet.

Do not produce a cheaper contradictory history just because a budget threshold was reached. If the required generation cannot satisfy world invariants within current capacity, delay/admission-control the observation instead of committing bad facts.

## Concurrency and idempotence

LLM calls that may create persistent facts must be associated with an expected simulation version/generation lease or equivalent idempotency boundary. If another request advances the same scope first, discard or revalidate the stale result rather than committing an alternate future.

LLM conversation/session state is never the lock, version, or canonical history. PostgreSQL/world persistence remains authoritative.
