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

The fresh `topic-first` experiment selects researched targets before Situation proposal and preserves them into the subject/body. It is an opt-in comparison, not a silent migration of saved posts. See `MATERIALIZATION_LAB.md` for the exact invocation and diagnostics.

Root subjects are generated as the text that the selected actor would actually type into the historical BBS subject field, rather than as a modern headline or article-summary task.

For the shared production batch path, standalone roots use the same
**candidate-first** principle as the title-first Lab. The World Engine fixes the
actor, board, timestamp and root/reply topology but does not pre-commit a detailed
topic merely to manufacture a title.

The title model receives the board, world date, recent board state, recent/avoid
subjects and bounded historical referents and normally proposes one large pool
of 100 uncommitted subjects. The pool has two independent diversity pressures:
it must keep enough claim-free candidates to fill the world slots safely, while
also reserving a bounded supply of claim-bearing named real-world candidates
even when the station has no hand-authored per-board referent quota. This latter
rule is only a candidate-supply floor, not a requirement that every board adopt
brands, works or place names.

Concrete root quality is not left to the generator prompt. During Jev fit
evaluation, every candidate also receives a **root specificity** probability.
World code rejects a candidate below the specificity floor before it can compete
for any actor/event slot, including on the final ranking-only recovery pass.
The classifier asks a board-name-independent question: can a reader understand
the concrete thing, situation, symptom, action, place, work/product or question
from the root title plus genuinely shared board/recent context, without inventing
an unnamed hidden referent? Thus broad-board subjects such as `台詞の間が好き`,
`お気に入りの見開き`, `次号の展開を予想` or `クリア時間を比べたい`
are rejected when no work/game is identified, while terse wording remains valid
on a single-work board or when recent canonical context really makes the target
unambiguous. Proper nouns are not required: a concrete world-local incident can
satisfy the same gate.

If a large pool still leaves world-selected roots unresolved, one fresh large
pool is allowed as bounded recovery. Canned subjects such as
`ＰＣ－９８について` are not permitted. Exhausting the bounded generated pools
fails the batch atomically so it can be retried later rather than committing
generic filler.

Jev evaluates candidate specificity and each candidate × already-selected world
slot fit. Code performs deterministic one-title/one-slot matching. Candidates
with named real-world references are historically researched only after
tentative selection unless a debug experiment explicitly bypasses that research;
the semantic specificity gate remains active independently of that debug switch.
A candidate becomes canonical only after it survives these gates. This preserves
the creative breadth of the generation pass without giving the wording model
authority to rewrite actor/time/topology or to defer missing topic identity to
the later Article Detail prose stage.

The same header pass commits semantic state but not the article body. The body
worker receives the canonical subject, concrete matter, claims and other intent
fields later, when the article is actually read.

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
source_post_id      (when applicable)
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
