# Structured BBS semantic realization

The development BBS semantic layer uses the Responses API Structured Outputs path (`text.format.type = json_schema`, `strict = true`) rather than relying on prompt-only JSON formatting.

The important boundary is **causal selection before semantic realization**, together with **diegetic present / era normality**. Structured output is an operational transport contract, not the mechanism that decides what the world does or what contemporary residents should find historically noteworthy.

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
 -> world routing-domain selection
      -> a persisted persona interest relevant to this board
      -> or a gated continuation of the actor's own recent root
 -> bounded structured LLM realization
      -> concrete contemporaneously meaningful matter inside that domain
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
- `anchor_key` selected by the world layer as internal routing metadata;
- `cause_kind`;
- a causal summary explaining the allowed reason/shape of this event.

`anchor_key` is not the actor's vocabulary, headline, or proof that the category itself is notable. It only constrains the semantic domain. A broad key such as `communications`, `games`, `music`, `local`, `bbs`, or `software` must not mechanically become "I used X" or "X still works".

The currently used cause kinds are intentionally small operational semantics rather than content categories:

- `recent_salience` — inside a board-relevant routing domain, a concrete present-tense difference/problem/decision/interaction/question/observation is worth mentioning; ordinary participation in the domain itself is not the event;
- `observed_thread` — the actor read a selected thread and chose to reply;
- `continuation_progress` — a deterministic progress gate allows the actor to revisit one of their own recent roots because something materially changed.

These do not encode subjects, article templates, or fixed conversational moves.

## Activity is not posting

A major invariant is:

```text
board visit != post
```

Activity sampling only creates plausible visits. A separate write probability uses lurker tendency, reply/thread-start tendency, and board affinity. Even a write-capable visit can still end as ROM if there is no plausible board-relevant root domain or reply target.

This matters both for realism and scale: silence/no-op is decided before any LLM call.

## Everyday baseline is not salience

The actor model distinguishes interests from ordinary already-established context. The current PoC stores the latter in `Persona.EverydayContext`.

Examples of baseline context include a person's normal computer/communication environment, ordinary membership in the BBS, usual local life, and other conditions that contemporaries would normally leave implicit. They are supplied for interpretation and contradiction avoidance, not as article ideas.

The realization layer follows these invariants:

```text
normally uses X       != reason to post about X
belongs to this BBS   != reason to announce BBS participation
likes games/music     != proof of a recent hiatus or rediscovery
lives locally         != reason to narrate the locality as period flavor
```

A baseline detail may become visible when a concrete event makes the distinction relevant. For example, a model/setup name may matter to a technical comparison or malfunction. If the distinction does not matter, the ordinary environment stays unspoken.

This is deliberately general rather than PC-specific. The same rule applies to operating systems, communication tools, games, music, local life, school/work, BBS usage and other everyday culture.

## Persona facts are not topic suggestions

`PersonaFact` exists to prevent contradictions and preserve durable fictional identity. It is **not** a topic queue.

For example:

```text
offline_meeting.preference = positive
```

means later text must not contradict that preference without a world event changing it. It does **not** mean that the persona should repeatedly start threads about offline meetings.

Existing facts are therefore labeled as background-only in the structured realization request. A fact may be reused only when the already-selected causal event genuinely requires it.

The development planner may propose at most one new durable persona fact for an event, and most ordinary posts should propose none.

## Board relevance without a global prose topic catalog

The development fixture has three known boards, so it has fixture-level routing weights connecting existing persona interest keys to those boards. The keys still come from each persona's persisted `Interests` map; there is no global prose topic bank.

For example, the technical board can route through existing domains such as `communications`, `modem`, `software`, or `bbs`. A persona with only `games`/`music` interests may still visit that board or socially reply to a thread, but they are not forced to manufacture an unrelated game root merely because they were online.

Ordinary equipment families are intentionally not used as root domains simply because they are common in the actor's environment. Such environment belongs in baseline state and appears only when a concrete event makes a specific distinction relevant.

Production host/program implementations should replace fixture-specific routing with host/board/world data while preserving the same causal and diegetic boundary.

## Diegetic present / era normality

The world date is the actor's literal present. Semantic realization must not use a later historian's or retro-computing enthusiast's interpretation of ordinary contemporary life.

The planner/body renderer therefore must not invent a hiatus, nostalgia, rediscovery, `still usable` framing, compatibility surprise, purchase, upgrade, new arrival, membership growth, maintenance or other world transition merely to make a broad routing domain interesting. Such a transition needs canonical support.

Likewise, internal classification labels are not automatically words a resident would choose. If a broad machine family or cultural category is ordinary background, it normally remains unnamed. A specific product/model/setup can be mentioned only when the exact distinction matters and the claim is supported by canonical state or allowed historical evidence.

This supplements, rather than replaces, the historical ceiling: avoiding future knowledge and avoiding retrospective meaning are separate requirements.

## Short-lived open loops

The development fixture uses a cheap approximation of an open loop: one of the actor's own recent root posts can become a `continuation_progress` source only when a low-probability deterministic progress gate fires and enough time has passed.

This intentionally differs from "recently discussed topic => discuss it again". A continuation must add a genuinely new development.

A future persistent world store may promote compact open-loop state to an explicit canonical model when needed across larger time spans. It should remain bounded and sparse rather than becoming a minute-by-minute life simulator.

## Subject-line calibration

### Concrete target and matter (2026-09)

Standalone root subjects identify what the member is talking about even when terse. Shared board membership is not evidence that everyone knows an unnamed game. Preserve a supplied short work/product name in its root subject; ordinary hobby-wide questions need no proper noun. Replies may rely on the established thread.

An already-selected action can express a modest impression, preference or curiosity. It does not require a malfunction or exceptional change. The action/write gate remains world-owned; interests alone still do not cause posting.

Duplicate detection distinguishes a public work/product identity from a private occurrence. Situation validation permits the same object with different matters but still rejects duplicate novelty keys and near-identical occurrences. Producer referent isolation permits only world-supplied public identities to recur across independent roots; private incidents remain isolated. The sourced ordinary producer supplies date-filtered public names, never a model-declared exemption.

The former isolated `topic-first` experiment has been retired. Production continues to preserve World-selected Situation facts into subjects and article bodies.

Root subjects are generated as the text that the selected actor would actually type into the historical BBS subject field, rather than as a modern headline or article-summary task.

For the shared production batch path, standalone roots now use a
**Situation-first** pipeline. The wording model is not asked to invent a topic
while simultaneously satisfying subject-line, persona, historical and diversity
constraints.

The normal root path is:

```text
world-selected actor / time / board / root topology
 -> routing domain + discourse_mode
 -> mode-compatible Situation kind
 -> structured Situation realization
      -> one small concrete occurrence
      -> typed facts matching the discourse mode
 -> accept Situation as canonical world state
 -> subject wording from that fixed Situation
 -> commit PostIntent + subject
 -> body wording remains lazy until article observation
```

The typed Situation shape is part of the semantic contract:

- `share_observation` carries an observation;
- `share_experience` carries experience + result;
- `state_opinion` carries stance + basis;
- `share_tip` carries attempted actions + result + a small practical point;
- `ask_peers` carries attempted actions + an unresolved question.

Only `ask_peers` has a question field. This deliberately moves conversational
intent out of prose micromanagement and into world state: the body renderer does
not need a growing list of instructions such as “do not end every post with a
question”.

Situation kinds are selected before prose and are diversity-weighted against the
retained board window. GAME currently has a richer mode-aware vocabulary covering
ordinary play, retry/progress, manuals and notes, passwords/saves, lending and
storage, local multiplayer, household timing/volume and other mundane game-life
situations. The vocabulary is world/event scaffolding, not a title or body
template bank. Other routing domains use their own sparse Situation facets and can
be expanded independently without changing the rendering contract.

Historical proper nouns are also world input rather than free wording-model
decoration. Curated period referents are filtered against **each event's own
timestamp** before they can be supplied to that event. This prevents a long
catch-up batch from leaking later products backward while still allowing later
events in the same history to know things that had become available by then.
Additional Historical KB evidence remains a separate evidence path. The Situation
and subject renderers must not manufacture unsupported specifications, dates,
prices, story facts or ownership/use history from an existence claim.

Once the Situation is canonical, the subject pass has one responsibility:
write the short root subject that this actor would naturally type for that
Situation. It may be terse or fragmentary and need not resemble a modern search
headline. It must not invent a different event or repair missing world state by
adding a new target. Recent subjects are soft repetition context, not a ban on
natural duplicate subjects.

New Situation-first roots set `ArticleDetailsMaterialized=true` when committed,
because the article-local facts already exist before the subject. They therefore
skip the legacy “infer Article Detail back from the adopted title” pass. That
detail pass remains only for older title-first state and reply/compatibility cases
where a small article-local fact or externally grounded referent is genuinely
missing.

The previous large title-candidate/Jev pipeline and its experimental
endpoints have been deleted. The shared BBS engine uses Situation proposal
and Situation-title wording as its only live planning path.

The production structured planner includes a compact calibration derived from preserved Japanese PC-communication subject-line corpora. The evidence shows that subject fields can be terse, fragmentary, person-directed, context-dependent, declarative, announcement-like, playful, or interrogative. Questions are therefore not the default form, and subjects do not need to summarize the body or make sense to an outsider without board context.

This calibration is deliberately **not** a subject template bank or percentage distribution. The planner does not rotate through title categories, copy historical strings, or assign a fixed numeric subject-style vector to every persona. It uses the already-selected causal event, the actor, prior board history, and existing persona behavior, while checking a batch for accidental convergence on the same rhetorical construction.

Research basis and corpus caveats are recorded in `docs/research/BBS_SUBJECT_CORPUS.md`. Raw third-party corpus data remains outside Git.

## Persistence and atomicity

The structured semantic calls are still batched chronologically for latency and token efficiency. Earlier batch semantics may be supplied as transient consistency context, but no post envelope or proposed persona fact is committed until the complete plan succeeds.

Once committed, `PostIntent` keeps the causal provenance alongside the human-readable semantics:

```text
action
anchor_key          (internal routing domain)
cause_kind
discourse_mode
source_post_id      (when applicable)
situation_kind
situation_summary
situation_facts     (typed canonical article-local facts)
article_details_materialized
topic
motivation
stance
goal
claims
responds_to_post_id
```

The article body remains lazily materialized. Body rendering receives these canonical causal fields and the persona's baseline context and is not allowed to change them or reinterpret them from a later historical viewpoint.

Body formatting is also calibrated separately from subject generation. Explicit
newlines should represent author-intentional structure such as paragraphs,
quotes, dialogue, short reactions or signature-like layout; the prose worker
must not default to modern smartphone-style 10-20-character line breaks or one
sentence per line. Ordinary long logical lines may instead wrap on the emulated
80-column-class terminal surface. Historical evidence, caveats and the
intentional-newline/display-wrap distinction are recorded in
`docs/research/BBS_BODY_CORPUS.md`.


### Generated subject surface normalization

A structured title pool is data, not display prose. If a generated pool
pathologically encloses most titles in decorative Japanese corner quotes
(`「...」`), the generator adapter removes that dominant outer wrapper before
candidate matching. An occasional meaningful quoted phrase is preserved. The
prompt also explicitly states that topic identity does not require a complete
sentence and that whole-title decorative quotes are not a desired style. This is
a transport/surface normalization only; it does not invent or rewrite the topic.
