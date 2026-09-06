# World-window producer / article-worker architecture

The development materialization host uses a producer/worker split for BBS content generation.

The purpose is to keep a persistent independent world coherent without asking each article-generation call to rediscover the world from scratch.

## Authority boundary

The world engine remains authoritative for **whether an action exists**:

```text
activity / visit sampling
 -> ROM vs write
 -> board
 -> actor
 -> timestamp
 -> root vs reply
 -> parent/source topology
 -> routing domain / cause kind
```

These facts are selected before any producer LLM call and cannot be changed by the producer.

The **world-window producer** then sees the complete bounded host window across all development boards/personas at once. It coordinates semantic specificity and issues one canonical article brief per already-selected event.

The producer decides semantic realization only within the fixed world action shell:

- the small concrete episode represented by the selected cause;
- stable referents shared by related events;
- what the actor is entitled to know;
- what the BBS audience/thread has actually established and may safely leave implicit;
- what information/reaction/question this post must contribute;
- event-specific prohibitions that prevent contradictions, fact theft and unsupported specificity;
- subject, topic, motivation, stance and goal;
- at most one genuinely necessary durable fictional persona fact.

The producer does **not** create extra posts, move an event to another board, change the actor/time, retarget replies, or invent a world transition merely to make a post more interesting.

## Canonical article brief

Accepted producer output is persisted in `PostIntent`:

```text
ProducerEventID
ProducerEpisode
ProducerReferents[]
ProducerActorKnowledge[]
ProducerAudienceContext[]
ProducerContribution[]
ProducerMustNot[]
```

This is canonical semantic/editorial state. It survives server restarts through the development PostgreSQL snapshot together with the post envelope.

A short contextual subject may be natural only when `ProducerAudienceContext` actually establishes the referent. For example, 「あの面どうした？」 is acceptable only if the BBS audience has a shared game/scene referent. The worker must not manufacture shared context after the fact.

## Article worker

Article prose is still lazy. Opening an article (or `ALLBODY`) invokes a per-article worker through the existing board-post renderer.

The worker receives:

- canonical actor/header;
- canonical `PostIntent`;
- the complete producer brief above;
- thread/retrieved BBS context;
- persona style/background;
- allowed historical evidence.

The worker may choose surface expression such as wording, line breaks, period-native ellipsis, quoting and persona-appropriate emoticons. It may **not** replace the producer episode, invent a new referent/backstory, give the actor knowledge they do not have, substitute a different contribution, or violate `ProducerMustNot`.

This makes each article renderer analogous to a sub-agent receiving a detailed instruction sheet from one producer.

## Whole-window coordination

For the current PoC the first empty-board observation selects causal shells for all three development boards and submits the full two-week set to one producer request.

```text
all board visits
 -> per-board world selection (BoardScope + ParticipationState)
 -> merge selected shells chronologically
 -> ONE host-wide producer pass
 -> persist article briefs/envelopes
 -> later per-article workers render bodies lazily
```

The host-wide producer can therefore keep the same person's activities/referents coherent across free talk, technical and local boards. It should not force unrelated posts into one artificial narrative; sparse independent episodes are normal.

The older six-event board-local timeline planner remains in code as a compatibility path for tests and non-producer development materializers. Production wiring uses `StructuredOpenAIProvider`, which implements the world-window producer.

## Specificity and historical evidence

The producer is not permission to hallucinate period facts. Named commercial games/products/software, real stations/shops, prices, release dates, exact specifications and historical events require supplied canonical/historical support.

When an external identity is not yet supported, the producer should make the episode concrete through actions/state/attempts/relationships while leaving the unsupported external name unspecified. A later structured Life Context / verified referent catalog can provide richer historically grounded objects without weakening this boundary.

## Future evolution

The current producer brief is deliberately compact. It can later be normalized into first-class canonical models such as:

```text
Episode
Referent/Object
KnowledgeEdge
FactAttribution
OpenLoop
```

without changing the producer/worker authority split. The DB remains the world-state source of truth; LLM conversation history never becomes canonical world state.
