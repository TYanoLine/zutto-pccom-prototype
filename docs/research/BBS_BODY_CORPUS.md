# Historical BBS body-text and line-layout observations

## Purpose

This note records historical evidence used to calibrate generated **article bodies** for the mid-1990s Japanese PC-communication world.

The goal is not to copy historical posts, impose one station's formatting rules on every host, or create a fixed "retro" writing template. It is to prevent modern-chat / modern-Web prose habits from leaking into generated BBS articles, especially:

- short smartphone-like line breaks after every phrase;
- uniformly tidy one-sentence-per-line prose;
- identical three-paragraph structure across personas;
- modern article-summary / FAQ tone;
- arbitrary reflow that destroys period fixed-pitch layout.

Raw third-party bodies remain outside Git. This repository keeps only source entry points, small non-redistributive observations, and implementation policy.

## Evidence snapshot

| Source | Period / scope | What it supports | Evidence class / caveat |
| --- | --- | --- | --- |
| ぺけろく教 preserved `.bbs` logs | surviving logs mainly 1988-1990; station continued later | The operator says the recovered `.bbs` files are DOS-family text and warns that some articles become misaligned unless viewed with a fixed-pitch Japanese font. Large board logs survive for GAME, PC, FREE, MUSIC and other boards. | Strong direct evidence for fixed-pitch text layout, but materially earlier than the project's 1996 target. |
| Bickle Net preserved log pages | preserved station material including a page explicitly catalogued as "びっくるネット96のええとこ" | Preserved text shows both very short lines and long hard-wrapped paragraphs. On the preserved ED page, several long Japanese lines measure roughly 72-78 display columns before a stored line break, while paragraph-final lines are naturally shorter. Blank lines, indentation, aligned signatures and dialogue layouts are also common. | Station-specific later preservation. The observed width is useful evidence, not a universal wrap constant. |
| Minkymoon Network preserved 1995 material | 1995 BBS screen/log material; operator retrospective | The operator explicitly describes the recreated BBS screen as **80 columns × 24 lines**. Preserved text/CG material also demonstrates layout that assumes a monospaced character grid. | Strong evidence for this station's screen geometry; not a body-length distribution. |
| "インテリジェントなパソコン通信システムの開発" | early-1990s Japanese PC-communication system | Its message-composition ("便箋") function is described as handling text up to **80 half-width columns × 300 lines**. This supports 80-column composition as a real contemporary implementation constraint. | Contemporary/near-contemporary technical evidence for one system, not a universal etiquette rule. |
| PC Watch retrospective on BBS writing/display | retrospective article published 2004 | Describes late-1980s PC-9800 terminal software as commonly displaying **80 columns × 25 lines**, and emphasizes that BBS authors/readers often shared similar display geometry. | Useful secondary retrospective; not primary 1996 station evidence. |
| Later participant retrospectives about PC-communication wrapping | retrospective recollections | Some participants recall writing below the full 80-column width (often around the mid-70s) so that later `>` quote prefixes would still fit. | Anecdotal support only. Do not convert this into a fixed generation target. |

## Source entry points

- ぺけろく教 preserved BBS logs  
  https://www.asahi-net.or.jp/~uv2s-oob/x68/
- Bickle Net preserved logs  
  https://bm98.yaneu.com/bickle/
- Bickle Net preserved ED text used for line-layout spot checks  
  https://bm98.yaneu.com/bickle/bick16.html
- Minkymoon Network 30-year retrospective / 1995 BBS material  
  https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/
- Minkymoon Network preserved period promotional material  
  https://minkymoon.jp/senden-cg/
- Nara National College of Technology, "インテリジェントなパソコン通信システムの開発"  
  https://www.nara-k.ac.jp/nnct-library/publication/pdf/h5kiyo_all.pdf
- PC Watch, 山田祥平「Re:config.sys」 (2004 retrospective discussion of BBS display environment)  
  https://pc.watch.impress.co.jp/docs/2004/0730/config011.htm

Additional candidate corpora already tracked by the repository include NIFTY-Serve/FTRAIN preserved archives and other station-specific collections under `scripts/research/sources.yaml` and `scripts/research/archive_pccom_sources.py`.

## What the evidence supports

### 1. The physical display grid mattered

A horizontal text grid near 80 half-width columns was not merely cosmetic. It affected menus, ASCII/MAG-style layouts, article composition, and how long prose was broken across lines.

For Japanese text, a full-width character consumes roughly two half-width display columns. Therefore an approximately 74-78-column prose line often appears visually as roughly 37-39 full-width Japanese characters when it contains little ASCII.

This does **not** mean authors universally typed exactly 37 or 38 Japanese characters per line.

### 2. Historical bodies were not smartphone prose

The preserved material does not support a default where every semantic phrase is manually broken after 10-20 Japanese characters.

Long sentences often continue until close to the display/editor width and then break. Short utterances, paragraph endings, jokes, dialogue, quotations and signatures can of course be much shorter.

The useful contrast is:

```text
modern-looking accidental uniformity:
短い意味単位で、
毎回このくらいに
きれいに改行する。
```

versus a period-plausible mixture:

```text
long prose may run close to the terminal/editor width before wrapping,
while a paragraph-ending line can be short.

A one-line reaction can also simply end here.
```

The second is a **structural description**, not a phrase template.

### 3. Intentional newlines and display wrapping are different things

Historical text can contain at least two distinct phenomena:

1. **author-intentional structure** — blank lines, paragraph boundaries, quote lines, dialogue layout, signatures, ASCII-art alignment;
2. **width-driven line breaks / wrapping** — a line reaches the communication software/editor/terminal width and continues on the next display line.

The simulator should avoid conflating these.

A user's explicit blank line or quote layout is world content. A terminal's visual wrap is presentation behavior.

### 4. Variation between people and stations is historically important

Preserved material contains:

- one-line remarks;
- dense paragraphs;
- many blank lines;
- indented dialogue;
- aligned pseudo-signatures;
- quote-like responses;
- ASCII/box layouts;
- idiosyncratic punctuation and emoticons.

The implementation should therefore **not** force every persona to use the same width, paragraph count, signature style, or quote style.

## What the evidence does not support

Do not infer any of the following as universal historical laws:

- every BBS used exactly 80 columns;
- every user hard-wrapped at exactly 76 columns;
- every Japanese line should contain 37-38 full-width characters;
- every article ended with a signature;
- every reply used `>` quoting;
- every body should contain blank lines;
- long articles were more authentic than short articles.

Host software, communication software, screen mode, editor settings, station culture and individual habits all matter.

## Generator policy derived from this research

### Body semantics

Article workers should continue to vary body length and paragraph count according to the persistent persona and the actual event.

A body may be:

- one sentence;
- several uneven sentences in one paragraph;
- multiple paragraphs;
- a short direct reply;
- a longer technical explanation when the actor/event genuinely supports it.

Do not add length merely to look historical.

### Newline policy

The prose worker should use explicit newlines for **intentional structure**, not to prettify every sentence into short modern-chat lines.

In particular:

- do not default to 10-20-character Japanese lines;
- do not make every sentence its own line;
- blank lines should mean a real paragraph / pause;
- quote, dialogue and signature layout may use intentional line breaks when natural for that persona;
- ordinary prose may remain as a long logical line and be visually wrapped by the historical terminal surface.

### Presentation policy

Host output must preserve stored intentional newlines.

The terminal renderer may wrap a long logical line according to the emulated terminal width; responsive/mobile UI must not rewrite host output into a separate modern text layout.

For the current PC-98-style terminal, 80-column presentation is historically plausible and is already the project's default visual model. Host-specific evidence may override this for a particular runtime.

### Future refinement

The next corpus pass should compute body-level descriptive statistics from legally preserved research copies where format boundaries can be validated:

- display-column length of non-empty lines;
- article body character count;
- non-empty line count;
- blank-line / paragraph rate;
- quote-prefix rate;
- indentation rate;
- signature-like trailing block rate;
- emoticon rate;
- half-width / full-width mix;
- reply-vs-root differences;
- station and board differences.

These values are **observations**, not generation quotas.

A later world/persona model may represent a persistent writing habit such as "often writes compactly" or "tends to quote previous lines", but should not manufacture a numeric fixed wrap width solely for variety unless source or communication-software state supports it.

## Evidence classification

- **Confirmed for the preserved source:** the cited source actually contains the described layout / screen / implementation statement.
- **Station-specific:** exact line widths, indentation and signature habits from Bickle/Minkymoon/ぺけろく must not be promoted to host-program defaults.
- **Likely reconstruction:** 80-column-class terminals strongly shaped mid-1990s Japanese PC-communication prose presentation.
- **Fictional reconstruction:** generated HAKATA CANAL NET articles remain fictional; historical evidence calibrates form, not content or historical persons.
