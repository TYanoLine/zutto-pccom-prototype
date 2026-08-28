# 1990s Japanese BBS host naming reference

Purpose: grounding material for future AI generation of fictional Japanese dial-up BBS hosts in the `ずっとパソコン通信` world. This is a style/reference document, not a database of stations to reproduce verbatim.

## Important generation rule

Historical station names below are examples of naming *style*. Generated hosts should normally invent a new name rather than reuse a historical real-world station name. The goal is to reproduce the period's naming grammar, typography and cultural range without impersonating an actual station.

## Strong period sources

### 1996 BBS telephone book

- `インターネット/パソコン通信BBS電話帳 1996年春・夏号`
- Edited by マイコンBASICマガジン編集部 / ツールボックス, 電波新聞社, 1996.
- 180 pages, with one CD-ROM.
- NDL Search: https://ndlsearch.ndl.go.jp/books/R100000136-I1970867909807007239
- CiNii: https://ci.nii.ac.jp/ncid/BA35323644

This is probably the single best period-correct source for a larger naming corpus if a lawful scan/CD or library copy becomes available later.

### INTERNET magazine, October 1995

A period article contains a B.I.G. project participant list. The names are especially useful because they show how inconsistent real naming was even inside one technical network:

- SRD-NET
- EXE-NET
- Labyrinth-NET
- GIGA SONIC FACTOR
- Ichinomiya Mysterious Net
- NTT-BBS
- とよねっと
- Mimorist AVENUE
- ねこさんちぃむねっと
- びぎな～ずネット
- 那珂ネット
- SET-NET

Source: https://iwparchives.jp/wp-content/themes/twentytwelve/bn/pdf/im199510-168-bbsbig.pdf

### Preserved / documented historical stations

Useful additional real names from preserved systems, operator histories, contemporary recollections and archival material:

- Midnight Driving — preserved MASH(mmm) station; the name literally reflected late-night-only operation before 24-hour service.
  - https://wwwmid.kinet.ne.jp/
  - https://www.kinet.ne.jp/activity/
- Ｐｏｎ－ＮＥＴ — predecessor of Midnight Driving.
  - https://www.kinet.ne.jp/activity/
- はんぞーBBS — KTBBS station with surviving welcome/menu material.
  - https://www.hanzou.jp/hanzoubbs/index.php
- T.C.M.network / TimeCafeMatrix — X68000 NET-COCK station.
  - https://tcm.jp/
- ミンキームーンネットワーク — opened in 1992; surviving operator archive includes 1993 promotional material and RTBBS details.
  - https://minkymoon.jp/
  - https://minkymoon.jp/senden-cg/
- FFC-BBS, 魔界ねっと, 大魔界通信網 — documented in the history of 通信用語の基礎知識.
  - https://www.wdic.org/w/WDIC/%E9%80%9A%E4%BF%A1%E7%94%A8%E8%AA%9E%E3%81%AE%E5%9F%BA%E7%A4%8E%E7%9F%A5%E8%AD%98
- 東京BBS, CATCG-NET, ナツメネット, まきちゃんネット, 聖まりあんぬBBS, ALTAネット, ふぁんきぃ核爆ネット, メイプルタウンネットワーク, Sunday Net — retrospective list from a participant-oriented history of Japanese grass-roots BBS culture.
  - https://www.paradisearmy.com/doujin/pasok3m.htm
- Kamishimo Transtation — music/techno-oriented station remembered by a 1996 user.
  - https://note.com/astroid/n/n53a0ef12e939
- 泉北ニュータウンネット — techno/music-related station mentioned in an interview about mid-1990s network culture.
  - https://akaobi.wordpress.com/2015/03/11/interview-with-jea-sharpnelsound/
- Mind-NET, ZOB Station — user recollection of early-1990s BBS browsing.
  - https://aki.nekoruri.jp/profile
- ゆいねっと — cited as a well-known grass-roots BBS in a 1998 `japan.bbs` discussion.
  - https://groups.google.com/g/japan.bbs/c/O1VECiNNHyI
- ネットワーク杉並ここと, 夢の扉, トーコロBBS, 稲城ハートフルネット, ピアネット, 埼玉ふれあいネット, みんなのねがいネット — examples from a 1990s welfare/volunteer BBS network.
  - https://www.nginet.or.jp/kinbe/work/sonobepsv1997.html

## Naming grammar observed in the references

These are qualitative patterns, not statistically measured frequencies.

### 1. `X-NET`, `X Net`, `Xネット`

Very common-looking period form. `X` may be an acronym, place, fandom reference, joke, project name, or English noun.

Examples: `SRD-NET`, `EXE-NET`, `SET-NET`, `Mind-NET`, `那珂ネット`, `ナツメネット`.

### 2. `X-BBS` / `X BBS`

Plain and technical. Works particularly well for institutional, club, local, or operator-nickname identities.

Examples: `NTT-BBS`, `FFC-BBS`, `東京BBS`, `はんぞーBBS`.

### 3. `X NETWORK` / `Xネットワーク` / `X通信網`

Feels larger, more ambitious or more thematic than a simple `BBS` suffix.

Examples: `ミンキームーンネットワーク`, `メイプルタウンネットワーク`, `大魔界通信網`.

### 4. English phrase without a network suffix

A station name did not have to explain itself as a BBS at all.

Examples: `Midnight Driving`, `GIGA SONIC FACTOR`, `Mimorist AVENUE`, `ZOB Station`.

### 5. Place-name + unexpected English / theme word

A local identity can coexist with deliberately grand, mysterious, cute or technical English.

Examples: `Ichinomiya Mysterious Net`, `泉北ニュータウンネット`, `那珂ネット`, `東京BBS`.

### 6. Cute / fandom / doujin / hobby naming

Names can be highly personal and culturally specific rather than corporate-sounding.

Examples: `ミンキームーンネットワーク`, `ねこさんちぃむねっと`, `聖まりあんぬBBS`, `ふぁんきぃ核爆ネット`.

### 7. Deliberately non-uniform typography

Real names mix:

- uppercase/lowercase ASCII
- `NET`, `Net`, `network`, `BBS`
- Japanese hiragana/katakana/kanji with Latin letters
- hyphens and spaces
- full-width and half-width presentation in terminal/promotional material
- playful long vowels such as `～`

Do **not** normalize every generated host into modern brand-style title case.

## What makes a generated name feel wrong

Avoid making every host sound like a 2020s startup or web service. In particular, do not systematically use words such as `Cloud`, `Social`, `Platform`, `App`, `AI`, `Web3`, `Metaverse`, modern emoji, domain-name puns, or contemporary SaaS naming conventions.

Also avoid making every station use `Cyber`, `Digital`, `Network`, `PC98`, or `Moon`. Those are plausible individually but become obvious AI clichés when overused.

## Recommended generation strategy

Generate a hidden `naming_dna` first, then the station name.

Suggested fields:

```yaml
naming_dna:
  era_year: 1996
  region_identity: local | weak | none
  operator_taste: technical | cute | fandom | music | literary | joke | institutional | plain
  language_mix: japanese | english | mixed
  suffix_family: net | bbs | network | none | other-periodic
  typography: plain_ascii | mixed_case | japanese_mixed | playful
  grandiosity: low | medium | high
```

Then ask the model for 5-10 candidates and select one with deterministic scoring/rules. Persist the selected name permanently.

### Diversity rules for a batch of 100 preset stations

The batch generator should penalize repeated structures. For example:

- no more than a modest fraction ending in exactly `-NET`
- do not reuse the same leading noun (`BLUE`, `MOON`, `CYBER`, etc.) repeatedly
- mix Japanese-only, English-only and mixed names
- mix local/place-derived names with hobby-derived and abstract names
- allow some extremely plain names and some gloriously weird ones
- allow acronyms whose expansion is never shown to the user
- some names can feel dated even by 1996; older stations may retain 1980s naming conventions

## Prompt guidance for future AI generator

The model should be told approximately:

> Invent a fictional Japanese dial-up BBS host name that could plausibly appear in a 1990-1996 BBS telephone book. Use the historical examples only to learn naming style. Do not copy an actual historical station name. Prefer eccentric period-authentic variety over polished modern branding. The name may be Japanese, English, mixed, an acronym, a local place reference, a hobby/fandom reference, or an unexplained phrase.

The generator should receive the station's region, opening year, subject focus, operator personality, size, host software and target demographic so that the name has a reason to exist rather than being independent random decoration.

## Future research

Best next acquisition: a lawful view/copy of the 1996 `インターネット/パソコン通信BBS電話帳` or its bundled CD-ROM. Instead of committing scans, extract only factual/derived metadata needed for the project (name style, region, category, software, line count, speeds, etc.) with provenance and avoid redistributing copyrighted page content.
