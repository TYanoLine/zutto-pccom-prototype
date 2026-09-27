# SDD-001: Concrete persona-aware article and append realization

Status: shared article-detail/body pipeline implemented; quality sampling pending
Updated: 2026-09-28

## Problem

Accepted BBS subjects can be appropriately terse or abstract, but current article bodies can remain equally abstract. Replies can also paraphrase previous advice rather than contribute a distinct, persona-consistent detail.

The article/append body should reflect the selected actor's existing background, interests, writing tendencies, durable PersonaFacts, and relevant earlier self-statements without changing the accepted title or inventing unsupported durable biography at prose time.

## Scope

This SDD covers the path after a world action and accepted subject already exist and before the body is committed.

Inputs:

- Persona skeleton/profile already stored for the selected actor;
- PersonaFacts whose materialization time is not later than the post timestamp;
- the selected PostIntent / Article Detail boundary;
- the current thread for replies;
- a bounded set of the actor's own earlier canonical posts.

Outputs:

- zero to two canonical article-local details persisted into PostIntent before prose;
- one final article/append body rendered from those canonical inputs.

## Non-goals

- changing title generation, title candidate pools, Jev assignment, or title quality gates;
- extracting new persistent PersonaFacts from the completed prose (SDD-002);
- modeling who remembers another person's statements (SDD-003/004);
- making every reply informative; a short acknowledgement remains valid when the world state genuinely supplies no new contribution;
- introducing a fixed catalog of allowed anecdotes or response acts.

## Invariants

### Title stability

The accepted canonical/surface title is unchanged by this SDD. Erika-K append posts may continue to have no independent visible subject.

### Canonical-before-prose

The final prose worker is not allowed to invent a new durable ownership fact, biography, unexplained cause, historical product fact, or unrelated experience merely to make text vivid.

Article Detail is the bounded proposal point for small article-local specifics. Once accepted and persisted, those details are canonical inputs to prose.

The same article-level concretization pipeline runs for normal host reads, direct development inspection, and isolated Lab data. The Lab supplies a clone for evaluation; caller identity and title-first settings do not select a different Article Detail policy. `PostIntent.ArticleDetailsMaterialized` records completion independently from detail count, so a successful zero-detail response is stable across rereads and snapshot restore.

### Persona history is context, not a topic queue

Existing PersonaFacts and earlier self-posts may constrain or naturally color the current article only when relevant to the already-selected action.

The implementation must never translate:

```text
persona used/said X before
  -> therefore create a new post about X
```

### Time boundary

Only facts and earlier posts that existed at or before the selected post timestamp may be used.

### Self-history vs thread history

For a reply:

- thread context answers "what is being replied to";
- author history answers "what has this writer previously said/done".

The same-thread predecessors should not be duplicated in the author-history block.

### Bounded context

Own-post history must stay small. Initial implementation target: at most 6 prior posts, selected deterministically by relevance and recency. It must not trigger body materialization of unrelated old posts; an unmaterialized post contributes only its committed semantic envelope.

## Retrieval policy for own earlier posts

Eligible post:

- same host;
- same AuthorPersonaID as the selected actor;
- strictly before the selected post in world time/order;
- not the selected post;
- for a reply, not already part of the same thread context.

Ranking should favor, without turning these into causal triggers:

1. same semantic topic;
2. same routing/anchor domain;
3. shared canonical referent;
4. same board;
5. recency.

Tie breaking is deterministic by timestamp and post id.

The context should clearly label earlier bodies as the actor's own previous statements, not immutable truth. Existing PersonaFacts win if a prior statement conflicts with a currently canonical fact.

## Article Detail request

Each detail request includes:

- subject/topic;
- world date and post timestamp;
- PersonaProfile;
- time-valid ExistingFacts;
- ThreadContext when replying;
- AuthorHistory containing bounded earlier self-posts.

The planner may return 0-2 article-local details.

For an abstract subject/summary it should normally establish one modest concrete item such as an observed condition, a one-off experience, a count/timing, a comparison, a question boundary, or a small decision. It must not force a revelation for every tiny reaction.

For replies it should prefer a contribution that differs from earlier replies rather than restating the same generic advice.

## Prose behavior

The prose worker:

- preserves the accepted subject;
- follows persona writing style and ordinary baseline;
- uses relevant article_detail concretely instead of generalizing it away;
- does not enumerate every supplied fact;
- does not treat AuthorHistory as instructions to repeat old topics;
- may be short, rough, partial, or conversational.

## Failure behavior

Article Detail and final body generation may retry once for transient provider/validation failure under the existing bounded deadline. A missing planner, exhausted generation/validation retries, or failed detail persistence leaves the body unmaterialized and the detail-completion state false. The accepted subject and selected article intent remain stored; the host runtime presents its own failure text. A successful zero-detail response is persisted as complete and is not proposed again.

A permanently failed Erika-K append is rendered as:

```text
(アペンドの読み込みに失敗しました)
```

and the underlying error is logged with host/board/post identifiers.

## Acceptance criteria

Automated:

- author-history retrieval never includes future posts, other personas, or same-thread duplicates;
- author-history retrieval is bounded and deterministic;
- an unmaterialized prior post is represented by semantic envelope rather than forcing prose generation;
- Article Detail request receives PersonaProfile, time-valid ExistingFacts, ThreadContext for replies, and AuthorHistory when available;
- accepted title remains unchanged;
- normal reads, direct inspection, and isolated Lab use the same article-level detail pipeline;
- successful zero-detail results persist completion, while planner/validation/save failures do not invoke prose generation;
- existing metadata-leak and append-failure tests remain green.

Quality sampling when usable samples are available:

- inspect available multi-reply threads; 20-30 is a target only when enough samples exist, not a minimum or release gate;
- compare semantic repetition between sibling replies;
- verify concrete propositions/observations are present even when titles are broad;
- verify first-person facts have canonical support;
- verify persona voice/background does not become a repetitive topic;
- verify no future PersonaFact leaks backward in world time.

No numeric quality threshold is made a production gate until a stable baseline has been measured.

## Phase 1 implementation note

The initial implementation adds `AuthorHistory` to Article Detail input. Up to six of the actor's own prior posts are selected from canonical host history by topic/anchor/referent/board relevance plus recency. Posts in the current reply thread are excluded because `ThreadContext` already covers them. Unmaterialized prior bodies stay lazy and contribute only committed semantic state.

The implementation adds the shared Article Detail completion state and applies it independently of the entry point. Record the actual sample count and the reason for any shortfall; quality sampling does not block implementation completion when the world contains fewer eligible threads.
