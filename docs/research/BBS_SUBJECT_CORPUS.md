# Historical BBS subject-line corpus observations

## Purpose

This note records research used to calibrate generated BBS subject lines. The goal is **not** to reproduce one station's exact title distribution or to build a canned subject bank. It is to correct a modern-LLM bias toward explanatory, polite Q&A-style headlines by documenting how broad the surviving Japanese PC-communication subject-field evidence actually is.

Raw third-party log bodies and the full extracted subject corpora are kept in the private research archive, not in Git. Repository code and docs contain only derived observations, acquisition logic, and small non-redistributive summaries.

## Corpus snapshot

The following counts are from the private derived corpus produced from the repository's bounded research acquisition tooling. Rates are descriptive only; they are **not generation targets**.

| Source | Period / evidence scope | Records | Median subject length | `?` / `？` rate | Reply-prefix rate | Evidence note |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| ぺけろく教 `.bbs` corpus | 1988-1990 | 14,795 | 6 | 5.5% | 0.4% | surviving original-style `.bbs` logs; much earlier than the service's 1996 setting |
| Bickle Net preserved logs | preserved station pages | 518 | 9 | 3.3% | 10.4% | later-preservation / station-specific edited material |
| 横浜戸塚BBS preserved page | historical preserved page | 119 | 15 | 6.7% | 6.7% | later-preservation; older technical-community material |
| PC-VAN 経営情報SIG | 1995-1996 | 37,335 | 14 | 11.5% | 27.9% | later web preservation of PC-VAN SIG title lists; directly near the target era |
| PC-VAN SIG るーみっくわーるど | 1995-1996 subset | 1,672 | 18 | 4.6% | 15.5% | later web preservation of PC-VAN SIG index pages; directly near the target era |

A 1995 Minkymoon Network screen capture is also retained as direct visual evidence for the appearance and use of subject fields, but its small visible sample is not treated as a statistical corpus.

## What the evidence supports

Across very different stations and eras, surviving subject fields include much more than descriptive questions. Depending on local culture and host context, subjects can function as:

- very short reactions or fragments;
- noun/topic labels;
- personal updates or status notes;
- direct address to another member;
- continuation shorthand whose meaning depends on surrounding articles;
- announcements and operational notices;
- jokes or expressive remarks;
- complete declarative sentences;
- questions when the writer is actually asking something.

Most importantly, a subject is not reliably an article summary. It can be ambiguous, private-context-dependent, terse, repetitive, or only partly informative to an outsider. The 1995-1996 PC-VAN corpora are especially useful here because they show that this breadth persists close to the project's target year rather than being only an artifact of late-1980s grass-roots systems.

## What the evidence does *not* support

Do **not** turn the table above into quotas such as “5% questions” or “30% noun phrases.” The samples differ by host, software, board purpose, preservation method, and era. Their percentages are station/corpus observations, not a universal law of Japanese PC communication.

Likewise, do not copy historical subjects into generated fictional stations, rotate through a fixed list of title categories, or add a persistent numeric `subject_style` vector merely to force variety. Those approaches would replace one kind of uniformity with another.

## Generator policy derived from this research

For root posts, the semantic planner should interpret `subject` as **the exact string the actor would type into the BBS subject field**, not as a modern headline-writing task. The subject need not summarize the body, explain itself to strangers, or be a grammatical sentence.

The planner should use the persistent persona, the concrete event motivation/goal, and recent visible board history. Questions should appear when asking is genuinely the event's purpose, not as a default device for making a post interactive. When a planning batch accidentally converges on the same rhetorical construction or ending, the model may reconsider subjects that have equally natural alternatives, but it must not enforce artificial diversity or a fixed distribution.

The production structured planner embeds this calibration in `apps/server/internal/llm/bbs_subject_calibration.go`.

## Source entry points

Repository-safe source entry points are tracked in `scripts/research/sources.yaml` and `scripts/research/archive_pccom_sources.py`.

- ぺけろく教: <https://www.asahi-net.or.jp/~uv2s-oob/x68/>
- Bickle Net: <https://bm98.yaneu.com/bickle/>
- 横浜戸塚BBS preservation: <https://www.jh1ifz.com/aboutComputer/MemoryOfYTBBS2.html>
- Minkymoon Network retrospective / captures: <https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/>
- PC-VAN 経営情報SIG preservation: <https://www.zenko3.com/keieisig/>
- PC-VAN SIG るーみっくわーるど preservation: <https://www.rumic.org/forum/>

## Evidence classification

- The corpus observations are **Confirmed for the preserved source material**: the strings and counts were extracted from the acquired artifacts.
- They are **Station-specific** as descriptions of each individual community.
- Generalizing them into a broad rule such as “subject fields need not be modern explanatory headlines” is a **Likely / inferred cultural reconstruction** supported by several independent corpora.
- Generated station subjects remain **Fictional reconstruction**; the research calibrates plausibility rather than copying historical people or posts.
