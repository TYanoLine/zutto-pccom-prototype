# LLM behavior policy

This document is product behavior, not merely prompt advice.

## Human neutrality

- Refer to the human-controlled account as a normal member/persona in internal prompts where possible.
- Never instruct a model to "respond helpfully to the user" for in-world generation.
- Do not generate reactions from multiple residents merely because the human posted.
- Agreement, praise and curiosity require persona/world justification.
- Silence is a valid result and should usually be decided before the LLM is called.

## Prompt construction

Prefer **purpose + materials + schema/validation** over long prompt rule stacks.

For fact-producing generation, tell the model briefly what service it is operating inside, what artifact it is producing, and how that artifact will be used downstream. Put the concrete world inputs that should shape the answer into structured request material: actor, world date, board, posting purpose, canonical facts, selected referents, recent context, and other already-decided state.

Do not turn quality preferences into growing lists of prohibitions when the same behavior can be obtained by improving those inputs. Keep prompt-level rules for hard boundaries that cannot safely be inferred from materials alone, such as canonical-state ownership, historical ceiling, reply/source identity, and output-format requirements. Enforce machine-checkable invariants in schemas and validators rather than restating them repeatedly in prose.

Small wording drift is acceptable when it does not alter canonical facts. A generator may choose a natural expression that differs from an expected phrasing; validation should reject contradictions or unauthorized world changes, not harmless stylistic variation.

Board topic classification is part of the **World-side material selection**, not an LLM wording rule. Route from the board's affirmative name/topic statement, not from keywords appearing later in exclusionary scope notes. ANIME/MANGA, GAME, software and open-ended chat can therefore receive distinct activity directions; period references are selected using that resolved domain. Do not try to repair an incorrectly selected domain by adding prohibitions to the generation prompt.

GAME is temporarily **open-topic** in production: World fixes the member,
board, time, post action and discourse mode, but does not select or send a
preset activity focus, situation_kind or topic quota. Situation generation
receives the board's actual name, member context, world date, posting purpose
and observed prior subjects (without their old facet classification). It
chooses the still-unknown concrete occurrence; the validated Situation is
accepted before title generation and body prose. Both named-game discussion
and everyday play topics may emerge naturally. Avoid introducing a named
game merely to fulfill a quota, and check resulting date accuracy in the
generation inspector. Other boards retain their existing activity selection
pending separate evaluation.

For production root Situations, pass the World-selected actor, time, board, posting purpose, persona context, and previously established canonical facts as materials. Other boards currently also receive an activity focus; GAME deliberately does not. Live production **does not automatically inject curated period-referent lists or external historical evidence** into Situation, title, or article prompts. It provides the world date and a short positive period-context frame; the model may use its period knowledge naturally within the 1996 ceiling. Historical claims require appropriate verification separately when enabled. The Situation model fills still-undecided details before canonical acceptance; title and body models then express that accepted state. Keep diagnostic sample incidents and wording controls in isolated test fixtures rather than production inputs. Preserve canonical and historical invariants through the existing generation boundary and validators, without turning them into per-topic prompt restrictions.

## Persona persistence

Persist opinions/interests/relationships independently of prose. Interests describe things this person actually tends to care or talk about, not every tool/environment they happen to use. Example:

```json
{
  "interests": {
    "communications": 0.8,
    "games": 0.9,
    "music": 0.4
  },
  "opinions": {
    "windows95": -0.65,
    "internet": 0.3
  }
}
```

If a human praises Windows 95, a persona with `windows95=-0.65` should not flip position unless a separate world event explicitly changes that opinion.

### Everyday baseline is not an interest

Ordinary already-established conditions belong in baseline/world context, not automatically in `Interests`.

Examples include the person's normal computer/terminal environment, ordinary BBS membership, normal commute or neighborhood, usual communication method, routine work/school state, and other facts that contemporary residents normally leave implicit. A machine family may be useful as internal world metadata while being completely unremarkable to the person using it every day.

The current prototype exposes this distinction as `Persona.EverydayContext`. The field remains small and human-readable; production persistence may normalize it differently. Its semantic contract matters more than its storage form:

- baseline is a contradiction/interpretation constraint;
- baseline normally remains unspoken;
- baseline does not create salience;
- a baseline fact can become relevant only when an independently selected event introduces a meaningful difference, failure, comparison, change, decision, or interaction.

Do not convert `normally uses X` into `tried X`, `X still works`, `returned to X`, `rediscovered X`, or `X feels nostalgic` unless canonical world state explicitly supports that transition.

### Persona facts are not action triggers

A persisted persona fact is a contradiction guard / durable identity fact, not a queue of future topics.

For example, `offline_meeting.preference=positive` constrains future characterization but does not make an offline-meeting post more likely by itself. Likewise an ordinary computing-environment fact does not justify repeatedly starting threads about the equipment category.

Before an LLM can realize a new post, the world layer must independently select a current causal action. Background facts may be supplied to the LLM only for consistency with that already-selected cause.

## Historical ceiling and diegetic present

Every generation request receives the world date and era rules. Historical ceiling rejects anachronisms such as modern SNS terminology, smartphones or later products/events.

In addition, every in-world generation must use **diegetic present / era normality**:

- the world date is the actor's actual present, not a period being reenacted by a later author;
- later historical reputation must not determine what the actor finds old, retro, nostalgic, surprising or worth explaining;
- ordinary contemporary tools, services, media, habits, BBS participation and local life remain unmarked unless this concrete event makes them relevant;
- do not add period props or explanatory period vocabulary merely to demonstrate historical setting;
- do not invent hiatuses, rediscoveries, purchases, upgrades, compatibility surprises, new arrivals, membership growth, maintenance or other world transitions to make a broad domain interesting.

The model should sound like a person living inside the date, not like a person who knows how that date will later be remembered.

### Period-native conversational economy

For standalone roots, economy must not conceal the subject of conversation. A supplied work/product name belongs in the subject when it identifies the matter. Shared context permits ellipsis only once the relevant referent is established. General hobby questions remain valid without proper nouns.

For an already-selected action, ordinary impressions, taste and curiosity can supply the conversational purpose without an exceptional change or fault. This does not let the model create posting actions from background facts.

The inside view also changes what contemporary residents omit, name and explain. Prompting must preserve that economy rather than translating every event into a self-contained modern explanatory post.

- Shared context may remain implicit. Subjects and bodies may be elliptical, fragmentary or locally understandable without explaining every referent to an outside reader.
- If canonical state supplies a concrete name/model/place/version and that distinction matters, prefer the concrete period-native term instead of broad later umbrella terminology.
- If canonical state does **not** supply the missing specificity, do not invent a product model, game title, station, shop, neighborhood, software version or other identifier merely to make the prose feel concrete.
- Board placement is semantic context. A root must plausibly belong on the exact selected board; a broad routing interest is not permission to drift into a generic version of the topic that would fit equally well elsewhere.
- Avoid assistant/FAQ voice. Casual members need not give complete tutorials, checklists, reassuring closure, or polished summaries. Explanation depth, politeness and certainty should follow the persona and the actual exchange.
- In a thread, inspect what has already been said. Do not simply paraphrase an agreement, anecdote or explanation already contributed. If the same actor has already replied, later prose should read as a return to the thread and should only repeat itself when the selected event genuinely supplies a new reason.

Contrastive examples in prompts are interpretation tests, not content templates. They may demonstrate why `PC-98からでも入れた` is wrong when PC-98 is only ordinary baseline, why generic chat etiquette does not belong as a root on a local-information board, or why the fourth near-identical `私もそうです` adds no value. Those examples must never be treated as permission to invent the named machine, place, game or anecdote.

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
 -> causal routing/domain selection by world layer
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
baseline/category label -> LLM invents a novelty around it
```

and never:

```text
human post -> LLM invents all consequences
```

The normal production path is observation-driven. Do not run broad periodic LLM generation for every host/person merely to make the world appear alive. Unobserved detail should remain unmaterialized until an observation or a necessary shared-world dependency requires it.

When a stale scope has been unobserved for a long time, prefer a bounded catch-up request that summarizes/selects important transitions over replaying every hour or day with separate LLM calls. Persist durable selected facts first; generate individual prose only for details that become visible or otherwise necessary.

### Temporary HAKATA title-led prose evaluation

For the fictional HAKATA quality-evaluation station only, the reversible
`HAKATA_FREEFORM_BODY=1` trial keeps the production World-selected posting
slots, accepted Situation, title and canonical persistence unchanged, but
passes a compact **view** of that accepted state to the final body worker.
The displayed subject, real board name, world date, author and any essential
accepted referent remain fixed. A single adopted Situation summary is normally
the only additional root fact; replies also receive their response purpose
and a bounded relevant thread/parent context. The model can phrase the post
naturally without copying the entire typed Situation/producer fields or treating
them as a checklist. This is a wording experiment, not authorization to change
canonical world events or historically established facts.

This temporary HAKATA path **skips optional Article Detail and historical
knowledge research calls during body reads**; initial Situation and subject
generation still happen exactly as before. Existing saved facts, titles and
bodies are never cleared or rewritten by enabling this flag. The standard
detail-first path remains the default for all other hosts and for HAKATA when
`HAKATA_FREEFORM_BODY=0`. An experiment result that invents a contradictory
event is a quality defect, not a new canonical fact.

### Article detail before prose

For an already-selected article whose body is not yet materialized, the shared
article pipeline proposes zero to two article-local details before prose. It
uses only the selected event, the author's profile and time-valid facts, bounded
prior self-posts, and the current thread when replying. These details do not
become durable persona facts or posting triggers. Persist the detail result and
completion bit before rendering prose. A successful zero-detail result is
complete; a missing planner, exhausted generation/validation retry, or failed
save must leave the body empty and the detail state retryable. This rule is the
same for host reads and direct development inspection.

## Causal event shell contract

Any LLM call that realizes a BBS event should receive an event shell whose world-owned fields are already fixed. At minimum for the current prototype:

- actor;
- timestamp;
- root/reply action;
- source/parent when applicable;
- causal routing domain/anchor;
- cause kind.

`anchor_key` is internal routing metadata. It constrains the semantic area in which a selected event is realized, but it is not the actor's wording, not a subject line, and not proof that ordinary use of that category is notable. Broad keys such as `communications`, `games`, `music`, `local`, `bbs`, or `software` must not be mechanically echoed into article semantics.

The model may realize exact subject wording and human-readable semantic summaries within world-owned constraints. It must not swap the domain for a more salient background fact, manufacture an unrelated reason for posting, or create an additional world transition. For `recent_salience`, ordinary use of the domain itself is not enough; the realization must concern a concrete contemporaneously meaningful difference/problem/decision/interaction/question/observation permitted by the shell.

A continuation is especially strict: "this person discussed X recently" is insufficient. A new root continuation requires a separate world progress/change gate; the LLM must add a materially new development rather than paraphrasing the prior post.

## Generation classes

Use cheaper/faster models for routine prose and reserve stronger models for consistency repair, complex history or multi-entity event planning. Keep exact model IDs configurable through environment/configuration rather than domain code.

## Cost and capacity policy

LLM cost is an explicit operational constraint, but it must not become hidden world corruption.

For structured multi-root Situation generation, an output truncated at the model's
completion-token limit is a capacity failure, not a reason to relax the schema
or commit partial world state. Leave already accepted in-memory chunk results
intact and retry **only the pending event shells** with smaller batches. Keep
the reduced batch size for the remainder of that planning window. If even a
single event cannot be validated within its budget, fail the observation
cleanly so the board remains retryable; do not invent fallback article headers.

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

## Sourced period referents

通常配線では出典付きの限定的な名称claimを状況・意味計画・本文へ供給する。日常性は匿名化を意味しない。世界事実の提案段階では選択済みの原因・board・人物の関心に沿って未確定の参照対象を具体名で確定してよい。本文workerが既存対象を別の名称に変えることは許可しない。名称の件数ノルマ、所有・購入・経験の自動付与、未供給の仕様や作品詳細の補完は禁止。詳細と出典は [PERIOD_REFERENTS](research/PERIOD_REFERENTS.md)。
