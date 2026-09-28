# BBS title quality gates

Status: canonical quality specification for shared BBS root-title generation.

This document defines what must be true before a generated root title is
considered good enough to become canonical world state. The thresholds in the
"batch naturalness" section are prototype tuning values, not historical claims.
They may be recalibrated against real 1990s logs, but changes must be explicit.

The title generator is allowed to over-generate candidates. Quality gates apply
to both the candidate pool and, more importantly, to the titles actually adopted
into world-selected root slots.

## Gate classes

### Q0: structural integrity — blocking

A title cannot be adopted unless all of the following hold:

- the title is non-empty after trimming;
- the visible subject is at most 36 Unicode characters;
- it contains no CR/LF;
- it does not begin with a synthetic universal `Re:`;
- the adopted subject is one of the generated candidates and is not silently
  rewritten into another topic;
- adopted root subjects are unique after the same normalization used by the
  shared planner;
- no adopted pair reaches the planner's near-duplicate threshold
  (`titleSimilarity >= 0.78`);
- large production pools contain exactly the requested 100 candidates;
- each large-pool candidate owns its own nested `historical_claims[]`;
- every historical claim has a supported kind, non-empty subject and non-empty
  verification need.

A Q0 failure is a programming/provider-contract failure, not a reason to publish
a weaker title.

### Q1: world correctness — blocking

A title may be grammatically natural and still be invalid world state. Before
adoption:

- it must fit the board's semantic scope;
- it must fit the already-selected author, root slot and discourse role without
  changing those world facts;
- it must not contradict established persona facts;
- it must not contain knowledge, products, services, works, events or terminology
  that are later than the assigned article's `CreatedAt`;
- a real named referent whose existence/availability at that date is not already
  supplied as verified fact must carry a historical claim;
- a selected claim-bearing candidate must pass Historical KB/research before
  adoption;
- a claim-free title must not smuggle in an unmarked real-world fact that would
  have required historical verification;
- time-sensitive wording must fit the **assigned slot date**, not merely the
  earliest date of the whole catch-up batch. This includes seasons, holidays,
  "today", "this weekend", current events, operating-state wording and similar
  temporal cues.

The earliest-date rule is still useful for preventing future products from
leaking backward across a catch-up window, but it is not sufficient for
slot-specific seasonal/temporal coherence.

### Q2: batch naturalness and diversity — blocking batch gate

Individual titles can all be plausible while the board still reads obviously
machine-generated. For an adopted batch with at least 20 root titles, use these
prototype thresholds:

| Metric | Gate |
| --- | --- |
| Exact / near-duplicate adopted roots | 0 |
| Generic board-name paraphrases / topicless filler | <= 10% |
| Question-form titles on a non-Q&A board | <= 60% |
| Titles beginning with the same broad board-scope place/token family | < 80% |
| Titles sharing one obvious surface frame, e.g. `<place>で…` | < 60% |
| Clear slot-date/season contradictions | 0 |

For boards whose purpose naturally depends on real local/product/cultural
texture, a batch of 20+ roots should also contain a non-trivial amount of
verified specificity. As a prototype floor, at least 10% of adopted roots should
mention a verified specific referent more precise than broad scope labels such as
a city/region name, unless that board explicitly opts out. Zero or near-zero
specific referents is treated as a quality failure even when every individual
title is otherwise safe.

The applicability of this floor must not be hard-coded from a board display name.
When a board does not carry an explicit world/station tuning override, the shared
planner classifies the board's **SemanticScope** as `none`, `light`, or
`regular`: `none` has no named-referent floor, `light` uses approximately
5%, and `regular` uses approximately 10%. This classification is editorial
generation control, not a world fact, and BoardScope is the primary signal.

The debug switch that bypasses Historical KB/Web verification does **not** disable
this composition gate. In that mode a visible claim-bearing referent may count as
specific-but-unverified for diagnostic generation, and telemetry must distinguish
that count from genuinely verified specificity. This lets debug runs measure
whether the candidate/selection pipeline is still collapsing into generic titles
without pretending that skipped historical verification succeeded.

This gate deliberately measures **adopted output**, not only the raw 100-title
pool. A diverse candidate pool is not useful if fit/assignment consistently picks
one repetitive subset.

### Q3: corpus authenticity — audit gate

The following cannot yet be reduced to a reliable numeric rule and should be
sample-audited against period material:

- titles should look like BBS subjects, not modern SEO/search-result headlines;
- short fragments, personal remarks, calls to a specific audience, questions,
  reactions, continuation-like wording and plain noun phrases should coexist;
- modern internet/app/social-media vocabulary must not leak backward;
- punctuation, politeness and sentence completeness should not become
  suspiciously uniform;
- a board should feel like several different people used it, rather than one
  copywriter producing variations of the same template;
- historical specificity should be supported by period sources, while fictional
  station-local events remain clearly world fiction rather than fabricated real
  history.

Q3 is initially a release/audit gate. Repeated failure patterns should be
promoted into Q0-Q2 rules or generator prompts once they can be detected
reliably.

## Audit procedure

For each generator change:

1. generate at least one clean zero-state board with 40+ roots;
2. record total planner time and number of candidate pools;
3. audit all adopted root titles against Q0-Q2;
4. inspect at least 20 titles manually for Q3;
5. separately inspect the 100-candidate pool when available, including
   claim-free/claim-bearing balance and claim/title association;
6. repeat on a second board with a different semantic domain before claiming a
   general quality improvement.

Passing one board is evidence for that board/run, not proof that the generator is
globally solved.

## Current production audit: HAKATA board 6

Audit target: HAKATA CANAL NET, Erika-K board `6` ("街角情報スポット"),
production commit `d496bfb`, zero-state generation observed on 2026-09-27 JST.

Observed adopted output:

- 48 root titles, all 48 distinct;
- average title length 12.94 characters; maximum 19;
- 0 multiline subjects;
- 0 synthetic `Re:` subjects;
- 3/48 question-form titles;
- 47/48 titles (97.9%) begin with one of the broad scope labels
  `福岡`, `博多`, `天神`;
- 31/48 titles (64.6%) use the obvious `<place>で…` opening frame;
- no Historical KB lookup occurred for the adopted set in this run;
- the 48 root slots span 386 days, from 1995-07-25 to 1996-08-14.

### Result by gate

**Q0: PASS for the observable adopted set.**

Length, non-empty, single-line, `Re:`, and exact-uniqueness checks all pass.
The planner also already applies its 0.78 near-duplicate filter before adoption.

**Q1: FAIL.**

At least one clear slot-date contradiction exists:

- `福岡で夏物の上着を買いたい` was assigned to 1995-12-07.

There is also at least one claim-routing miss worth treating as a correctness
defect: `博多駅近くで時間をつぶすなら` refers to a specific real station, the
evidence phase supplied zero historical facts, and the run performed zero title
Historical-KB lookups. Under the current large-pool contract, that named station
should have been marked as a historical claim unless already supplied as
verified evidence.

`福岡で夏の贈答品を選ぶ` on 1996-04-03 is not as unambiguous as the December
"夏物" case, but it should be included in temporal-coherence audit examples.

**Q2: FAIL.**

The adopted board is too syntactically concentrated:

- broad place-name lead: 97.9%, over the <80% gate;
- `<place>で…` frame: 64.6%, over the <60% gate.

The individual subjects are mostly concrete and the question ratio is healthy,
but the aggregate strongly exposes the generator pattern. The board also has
very little adopted specificity beyond the broad `福岡/博多/天神` labels; the
selection path chose only claim-free material in this run, so the historical
texture present in the raw candidate design did not survive into the final
board.

**Q3: MIXED / needs improvement.**

Several subjects are period-plausible and pleasantly ordinary, for example
`天神で写真を焼き増ししたい`, `福岡で傘をなくしました`, and
`福岡のバス、乗り換えが苦手`. However, read as a 48-title corpus the repeated
place-first construction makes the board feel generated rather than like many
independent users. A few subjects also read as awkward task-list paraphrases,
for example `福岡の郵便、土曜に出す用事`.

## Current conclusion

The 100-title approach is fast enough to continue with and its per-title
specificity is generally usable, but the current adopted output does **not** yet
pass the title quality gate as a batch.

The next fixes should target selection/output quality rather than increasing
candidate count:

- make fit/assignment penalize already-overrepresented surface frames and broad
  place-prefix repetition;
- add a slot-date temporal-coherence check after assignment;
- strengthen detection of unmarked named real referents before treating a
  candidate as claim-free;
- ensure some verified specific historical/local texture survives candidate
  selection instead of systematically preferring only claim-free generic-local
  titles.

These corrections should be measured against this document before changing the
thresholds.
