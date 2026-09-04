# Structured BBS semantic realization

The development BBS semantic layer uses the Responses API Structured Outputs path (`text.format.type = json_schema`, `strict = true`) rather than relying on prompt-only JSON formatting.

The important boundary is now **causal selection before semantic realization**. Structured output is an operational transport contract, not the mechanism that decides what the world does.

## Sparse causal pipeline

For the development materialization host, the flow is:

```text
observation of a board
 -> deterministic activity / board-visit sampling
 -> write gate
      -> most visits may resolve to ROM / no-op
 -> world action selection
      -> root or reply
      -> source thread/event when applicable
 -> world causal anchor selection
      -> a persisted persona interest relevant to this board
      -> or a gated continuation of the actor's own recent root
 -> bounded structured LLM realization
      -> exact subject text
      -> human-readable topic/motivation/stance/goal
      -> at most one genuinely necessary new durable persona fact
 -> validate
 -> commit PostIntent + envelope
 -> body remains lazy until article observation
```

The LLM does **not** receive an empty posting slot and then decide what topic would be convenient. Every event passed to it already includes:

- actor;
- timestamp;
- root/reply topology;
- source event when applicable;
- `anchor_key` selected by the world layer;
- `cause_kind`;
- a causal summary explaining why this event exists now.

The currently used cause kinds are intentionally small operational semantics rather than content categories:

- `recent_salience` — a board-relevant persisted interest has a current small experience/observation/thought worth mentioning;
- `observed_thread` — the actor read a selected thread and chose to reply;
- `continuation_progress` — a deterministic progress gate allows the actor to revisit one of their own recent roots because something materially changed.

These do not encode subjects, article templates, or fixed conversational moves.

## Activity is not posting

A major invariant is:

```text
board visit != post
```

Activity sampling only creates plausible visits. A separate write probability uses lurker tendency, reply/thread-start tendency, and board affinity. Even a write-capable visit can still end as ROM if there is no plausible board-relevant root anchor or reply target.

This matters both for realism and scale: silence/no-op is decided before any LLM call.

## Persona facts are not topic suggestions

`PersonaFact` exists to prevent contradictions and preserve durable fictional identity. It is **not** a topic queue.

For example:

```text
offline_meeting.preference = positive
```

means later text must not contradict that preference without a world event changing it. It does **not** mean that the persona should repeatedly start threads about offline meetings.

Existing facts are therefore labeled as background-only in the structured realization request. A fact may be reused only when the already-selected causal anchor genuinely requires it.

The development planner may propose at most one new durable persona fact for an event, and most ordinary posts should propose none.

## Board relevance without a global topic catalog

The development fixture has three known boards, so it has fixture-level routing weights connecting existing persona interest keys to those boards. The keys still come from each persona's persisted `Interests` map; there is no global prose topic bank.

For example, the technical board can select existing interests such as `modem`, `software`, `pc98`, or `bbs`. A persona with only `games`/`music` interests may still visit that board or socially reply to a thread, but they are not forced to manufacture an unrelated game root merely because they were online.

Production host/program implementations should replace fixture-specific routing with host/board/world data while preserving the same causal boundary.

## Short-lived open loops

The development fixture uses a cheap approximation of an open loop: one of the actor's own recent root posts can become a `continuation_progress` source only when a low-probability deterministic progress gate fires and enough time has passed.

This intentionally differs from "recently discussed topic => discuss it again". A continuation must add a genuinely new development.

A future persistent world store may promote compact open-loop state to an explicit canonical model when needed across larger time spans. It should remain bounded and sparse rather than becoming a minute-by-minute life simulator.

## Subject-line calibration

Root subjects are generated as the text that the selected actor would actually type into the historical BBS subject field, rather than as a modern headline or article-summary task.

The production structured planner includes a compact calibration derived from preserved Japanese PC-communication subject-line corpora. The evidence shows that subject fields can be terse, fragmentary, person-directed, context-dependent, declarative, announcement-like, playful, or interrogative. Questions are therefore not the default form, and subjects do not need to summarize the body or make sense to an outsider without board context.

This calibration is deliberately **not** a subject template bank or percentage distribution. The planner does not rotate through title categories, copy historical strings, or assign a fixed numeric subject-style vector to every persona. It uses the already-selected causal event, the actor, prior board history, and existing persona behavior, while checking a batch for accidental convergence on the same rhetorical construction.

Research basis and corpus caveats are recorded in `docs/research/BBS_SUBJECT_CORPUS.md`. Raw third-party corpus data remains outside Git.

## Persistence and atomicity

The structured semantic calls are still batched chronologically for latency and token efficiency. Earlier batch semantics may be supplied as transient consistency context, but no post envelope or proposed persona fact is committed until the complete plan succeeds.

Once committed, `PostIntent` keeps the causal provenance alongside the human-readable semantics:

```text
action
anchor_key
cause_kind
source_post_id   (when applicable)
topic
motivation
stance
goal
claims
responds_to_post_id
```

The article body remains lazily materialized. Body rendering receives these canonical causal fields and is not allowed to change them.
