# Historical Texture PoC — 1996-08 curated set

## Purpose

The fresh materialization Lab can compare generic historical-reference suppression against a small **historical texture** set. The goal is not to turn every post into a list of 1990s props. It is to test whether supplying a bounded set of real contemporary referents gives otherwise-natural BBS conversation more of the period's lived texture while preserving the project's world-truth rules.

The experiment is enabled only by:

```text
historical_texture=1996-08-curated
```

Ordinary runtime remains unchanged. The service-wide `HISTORICAL_REFERENCES_ENABLED` configuration is not switched on globally.

## Semantics

Historical texture is **permission/background**, not an event catalog.

- The world layer still chooses actor, time, board, root/reply topology, routing domain, cause kind, and discourse mode first.
- The batch Situation proposer may use a supplied real referent only when it naturally sharpens an already-plausible situation.
- A supplied name does not imply ownership, purchase, upgrade, popularity, exact release date, price, specification, plot detail, service fee, or other unsupplied fact.
- Many posts should use none of the supplied referents.
- The article worker receives the same texture as factual support, so a real name accepted into canonical Situation state can survive prose rendering without being replaced by a generic umbrella term.
- Model memory is still not permission to add other real names or details.

## Initial curated set

The initial set is intentionally conservative and limited to period anchors already approved by the repository's in-world guidance in `AGENTS.md` for the 1996 setting:

- Windows 95
- PC-98
- DOS/V
- Macintosh
- NIFTY-Serve
- PC-VAN
- ISDN
- PlayStation
- Sega Saturn
- Pocket Monsters / ポケモン
- 新世紀エヴァンゲリオン / Evangelion
- MIDI / FM sound
- 秋葉原 / Akihabara

`AGENTS.md` explicitly lists these categories/names as era-appropriate candidates subject to the mapped world date. The repository also contains near-target-era preserved PC-VAN material in `docs/research/BBS_SUBJECT_CORPUS.md` and archived commercial-service source collection machinery under `scripts/research/archive_pccom_sources.py`.

This PoC deliberately does **not** add exact release dates, prices, episode status, game/platform relationships, service fees, technical specifications, named stores, or current-event outcomes. Those require source-specific verification before becoming production historical evidence.

## Evidence status

For this experiment, treat the list as **project-approved period anchors for fixture evaluation**, not as a complete research record for every referent. Before promoting any referent/detail into production historical knowledge, follow `docs/HISTORICAL_ACCURACY.md`: prefer manuals, official material, contemporary logs, magazines/books, and preserved period sources, and record source URLs/evidence class.

This distinction matters: the experiment asks whether *bounded real-world texture* improves perceived realism. It does not relax the historical evidence policy.

## Evaluation

Compare two archived fresh jobs with the same Lab shape:

```text
situation_mode=batch
board_count=6
shell_limit=8
historical_texture=off
```

versus:

```text
situation_mode=batch
board_count=6
shell_limit=8
historical_texture=1996-08-curated
```

Use `/poc/materialization-lab-viewer` and evaluate:

1. Does the textured run feel more like people living in 1996 rather than modern people avoiding modern words?
2. Are real names used only where they matter, or do they become conspicuous period cosplay?
3. Does the proposer invent unsupported real-world details around allowed names?
4. Do generic everyday posts remain generic when no concrete referent is needed?
5. Does the same-person/thread continuity remain natural?
6. Does topical diversity degrade because the model clusters around the supplied names?

The experiment should be considered a failure if adding texture merely replaces generic repetition with repeated Windows 95 / NIFTY-Serve / PlayStation references.

## Model-memory experiment

`historical_texture=model-memory` is an isolated fresh-Lab comparison mode. It injects no proper-noun/referent dictionary. The Situation proposer and article worker may use their own historical knowledge only when confident that a real referent existed and was knowable in Japan by the world date. Uncertain timing/details must be omitted rather than guessed. Real-world existence never establishes persona ownership/experience, and BBS-internal history still requires canonical evidence. This mode is experimental and is not ordinary-runtime policy.


## Model-memory concrete-name preference experiment

`historical_texture=model-memory-concrete` is a fresh-Lab-only A/B mode built on `model-memory`. It still injects **no proper-noun dictionary, PeriodReferents, or curated historical texture**. The only difference is a prompt-level concretization preference: if an already-selected canonical situation naturally maps to a real name the model confidently knows existed and was knowable in Japan by the world date, prefer that concrete name over a generic label.

This is deliberately **not** a name quota or topic-selection mechanism. The model must not redirect a generic event toward a remembered product/work/service merely to insert period texture, must not repeat one favorite name across unrelated roots, and must remain generic whenever timing or identity is uncertain. Persona ownership/experience and BBS-internal history remain canonical-world facts, not model memory.

The comparison target is `model-memory` under the same fresh-world topology. If concrete names increase without collapsing topic diversity or introducing future/unsupported facts, this suggests that a lightweight preference can recover period texture without a curated referent list becoming a hidden topic menu.
