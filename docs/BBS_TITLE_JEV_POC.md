# 100-title generation / Jev title-era PoC

Date: 2026-09-27 (project development time)

This document records the development experiment that led to the current production title-planning policy. The measured findings remain historical experiment data; the implementation decision is recorded below.

## Question

At the start of this experiment, the title-first BBS planner generated title candidates in 20-title pools and used Jev both for:

1. title-era routing (`safe_without_research` / `research` / `logically_impossible`), and
2. title-to-world-event/persona fit.

The experiment asked whether a single large Azure OpenAI title pool can emit both concrete candidate titles and their historical-research hints accurately enough that the first Jev role is no longer necessary.

The second role (persona/event fit) is a separate question and was **not** measured by this PoC, because the PoC sent no world events to Jev.

World date: `1996-08-26`.

## PoC shape

The debug endpoint `/api/debug/bbs-title-jev-poc` generates 100 candidate titles in one structured Azure OpenAI call, then asks Jev to perform title-era routing over the same candidates.

This endpoint remains a diagnostic harness. After the experiment, the shared production planner was changed to request `CandidateCount == 100` for contextual title generation; the 20-candidate shape remains only as a compatibility default for other callers.

### Important schema finding

The first 100-title versions returned:

- one `titles[]` array, and
- a separate `historical_claims[]` array whose entries referred to titles by 1-based candidate index.

After prompt refinement, only 1 of 11 sampled claim references matched the title at the referenced position. Both arrays were schema-valid; the semantic cross-array alignment was not reliable enough at this size.

For large pools the schema was therefore changed to:

```text
candidates[]
  title
  historical_claims[]
```

The provider then converts that nested result back into the existing internal `BBSTitleCandidates` type. This eliminated the observed claim/title index drift in the following runs.

## Runs

| Run | Board | Generation | Jev | Claim-free | With claims | Jev safe | Jev research | Jev impossible | Claim-free routed to research |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Baseline | 6 / 街角情報スポット | 24.128 s | 0.399 s | 52 | 48 | 0 | 100 | 0 | 52 |
| Prompt refinement, old indexed schema | 6 / 街角情報スポット | 19.832 s | 0.385 s | 89 | 11 | 5 | 95 | 0 | 84 |
| Nested claims + local texture | 6 / 街角情報スポット | 24.619 s | 0.341 s | 49 | 51 | 16 | 84 | 0 | 33 |
| Cross-domain rerun | 6 / 街角情報スポット | 24.074 s | 0.560 s | 50 | 50 | 8 | 92 | 0 | 42 |
| Cross-domain rerun | 20/1 / GAME | 25.509 s | 0.340 s | 51 | 49 | 17 | 83 | 0 | 34 |

The startup autorun used to collect these samples was removed after the experiment. The explicit debug endpoint remains available for later manual measurement.

## Manual audit

### Claim-free candidates

In the nested-schema runs, claim-free examples included ordinary titles such as:

- `傘を忘れた日の帰り道`
- `昼休みの定食屋探し`
- `引っ越しの段ボール集め`
- `町内会の回覧板が来ました`
- `名前入力で毎回悩む`
- `ボス戦でつい連打`
- `ゲームを借りる約束`

These do not assert an externally verifiable named product/facility/service that needs historical lookup. They may establish an ordinary fictional world event if adopted, which is different from making a real-world historical claim.

Nevertheless, Jev routed 33/49, 42/50, and 34/51 claim-free candidates to `research` in the three nested-schema samples. Many had `safe_without_research` values around 0.72-0.79, immediately below the current 0.80 threshold, while semantically similar ordinary titles landed at 0.80-0.83.

For the title-era role, these are false-positive research routes under the project distinction between fictional world events and external historical facts.

### Claim-bearing candidates

Once claims were nested with their titles, the generator reliably attached claims to concrete named referents in the sampled output. Examples included local facilities and transport names, and game/product names.

Jev routed every sampled claim-bearing title to `research`; none were marked `logically_impossible`.

This is conservative and safe, but it does not add routing information beyond the generator's own claim marker.

The local sample also contained real names known to be later than the 1996-08-26 world date, such as 福岡三越, 博多リバレイン, and 博多座. The GAME sample contained later products such as サクラ大戦, マリオカート64, グランツーリスモ, and 電車でGO!. Jev still routed these to `research`, not `logically_impossible`.

That behavior is consistent with the current Jev prompt: unsupported real-world claims are intentionally sent to research rather than decided from model memory. Historical KB / research is therefore the authoritative mechanism that can actually verify and reject a future named referent.

## Prompt refinement that helped

For large pools, the generator is now told to:

- make its own `historical_claims` classification without assuming Jev will repair it;
- provide at least `RemainingNeeded` claim-free reserve candidates;
- mark unsupplied real facilities, products, stations, routes, services, works, events, etc. as historical claims;
- avoid unsupported transient real-world operating-state assertions;
- keep claim-free candidates concrete rather than generic;
- after the reserve is satisfied, still include enough named local/product-specific candidates to preserve period and board texture.

The nested schema was more important than prompt wording alone for reliable claim/title association.

## Current conclusion

### Title-era Jev

The measured evidence does **not** justify mandatory Jev title-era routing.

A simpler candidate path is supported by the PoC:

```text
World Engine fixes author/time/root slots
  -> one large structured title-candidate generation
       title + nested historical_claims
  -> local syntax/length/duplicate checks
  -> claim-free candidates: no Historical KB lookup
  -> claim-bearing candidates: verify only the shortlist that may actually be adopted
  -> persona/event fit
  -> adopt canonical title
```

Historical KB/research remains the authority for real-world historical validity. The generator's claim marker is a routing hint, not historical proof.

This removes the main observed false-positive behavior: treating ordinary claim-free titles as if they needed Web historical research.

### Jev overall

Do **not** conclude from this experiment that Jev can be removed from the entire title pipeline.

The production Jev integration also scores title × world-event/persona fit and performs deterministic assignment. This PoC deliberately did not supply world events, so that role remains unmeasured.

Before removing Jev entirely, compare its fit decisions with either:

- the existing Azure OpenAI `ReviewBBSTitleCandidates` path, or
- a large-pool structured generation/review design that includes the already-fixed world-event slots without allowing the model to rewrite them.

The production planner now follows that scoped conclusion: the **era-routing role has been removed from Jev**, while the **fit/assignment role remains** and is evaluated separately.

## Implementation decision

As of the production change following this PoC:

- contextual title generation requests 100 candidates in one structured call;
- each large-pool candidate nests its own `historical_claims[]`;
- claim-free candidates bypass title-era Historical KB lookup;
- claim-bearing tentative winners require Historical KB/research verification;
- Jev is invoked in fit-only mode and does not ask title-era questions;
- Jev/Azure OpenAI fit review is bounded to at most 20 titles × 20 remaining world events per fit batch;
- a second 100-title generation is recovery only, not the normal path.

This is intentionally a title-planning decision, not a conclusion that Jev is unnecessary elsewhere.

## Performance implication

A 100-title structured generation took roughly 20-26 seconds in these runs. Jev era evaluation itself took only about 0.3-0.6 seconds.

The direct Jev call is therefore not the main latency cost. The larger performance problem is indirect: conservative Jev `research` routing can trigger Historical KB/Web work, reject otherwise usable candidates, and force additional title pools.

The former 20-title design could require up to 9 semantic pools for a 48-root initial history. The production planner now uses a 100-title primary pool, keeps claim-bearing historical checks on the tentative shortlist only, and bounds retained Jev fit work to chunks of at most 20 titles × 20 world events. One second 100-title pool is permitted only as recovery. End-to-end materialization behavior should continue to be measured independently from this PoC.
