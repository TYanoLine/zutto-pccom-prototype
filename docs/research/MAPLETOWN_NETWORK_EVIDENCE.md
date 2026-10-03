# Mapletown Network preservation evidence

## Purpose

Mapletown Network の保存公開サイトを、「ずっとパソコン通信」の1990年代日本のBBS文化・板構成・話題分化・局運営文化を校正するための研究資料として記録する。

この資料は **Mapletown Network という特定局の保存記録** であり、特定ホストプログラムの標準仕様を示す資料としては扱わない。

## Source

- Site: <https://www.maple.town/>
- Board directory: <https://www.maple.town/boards>
- Retrieved / reviewed: 2026-09-27
- Site description: 1986年に開始したアニメーション専門の電子掲示板で、開局以来のほぼすべての記事を閲覧可能としている。
- Current access mode: read-only preservation site. The top page publishes shared read-only authentication instructions for the BBS archive.
- Rights note: project owner reported on 2026-09-27 that copyright/usage confirmation for research collection has been obtained. Raw third-party article bodies should nevertheless remain outside Git according to the repository research-archive policy unless redistribution terms are separately documented.

## Evidence classification

### Confirmed for the preserved site

The preservation site's current top page directly states that:

- Mapletown Network started in 1986;
- it is an animation-specialized electronic bulletin board;
- almost all articles since opening can be read;
- the currently published archive is read-only.

The public board directory currently exposes board numbers `#1` through `#272` and their names.

### Station-specific

All board names, numbering, topic partitioning, naming conventions, and community-purpose boards below are evidence about **Mapletown Network**. Do not treat them as generic BBS requirements or host-program defaults.

### Likely / inferred

The numerical board sequence strongly suggests an accumulating/evolving board set, and titles such as `旧 ...` plus later replacement/topic boards show explicit reorganization. However, board number alone must not be used as a precise creation-date timestamp until article/system logs provide direct dates.

## High-value observations for the 1996 world model

### 1. A large station can mix broad category boards with very narrow title-specific boards

The directory contains broad boards such as:

- `#15 ファンタジーアニメーション`
- `#16 SF・メカ・ロボットアニメ`
- `#20 その他のアニメーション`
- `#21 音楽・声優・歌手`
- `#22 フォーラム`
- `#68 computer`
- `#69 computer program`
- `#70 本・雑誌・漫画`
- `#74 ゲーム`
- `#99 プログラム技術`
- `#110 Audio & Visual ハードウェア`
- `#124 実写作品`
- `#139 アニメーション一般`
- `#159 海外のアニメーション`
- `#163 インターネット`
- `#197 ラジオ`
- `#206 模型・モデル`

At the same time, many individual works receive dedicated boards. This is strong station-specific evidence against modeling every grass-roots network as a fixed handful of broad modern-forum categories.

### 2. Boards can represent community actions, not just media topics

Examples include:

- `#26 あいさつ（はじめまして・お休みします）`
- `#27 ロボットアニメーション鑑賞会`
- `#39 コンテスト`
- `#40 同人誌即売会（コミケット）`
- `#52 その他のオフラインミーティング`
- `#58 名作劇場シリーズ鑑賞会`
- `#80 告知・宣伝`
- `#117 Mapletown ブックメーカー`
- `#167 その他の鑑賞会`

For fictional-station generation this supports modeling boards around recurring social practices, local events, greetings/absence notices, meetups, viewing parties, announcements, and station games in addition to subject taxonomy.

### 3. Technical and creative subcultures are visibly interleaved with fandom discussion

The directory separately preserves areas for computer music, CG, programs, data, Q&A, and impressions/opinions, including:

- `#29 computer music Q & A`
- `#30 CG Q & A`
- `#31 computer music 感想・意見`
- `#32 CG 感想・意見`
- `#45 computer music program`
- `#46 CG program`
- `#47 MMM・MML`
- `#60 APL データ`
- `#61 MDT データ`
- `#62 その他の computer music データ`
- `#63 NL3 データ`
- `#64 ML1 データ`
- `#65 その他の CG データ`
- `#73 声優データベース`
- `#138 アニメーション データ`
- `#142 computer program & data 感想・意見`
- `#146 その他のデータ`

This is useful evidence that a hobby network's board tree can blend discussion, software exchange/data areas, technical help, creative tooling, and databases rather than cleanly separating “community” and “technology.”

### 4. The directory contains unusually useful 1995-1996 cultural anchors

Examples around the target era include:

- `#172 ロミオの青い空`
- `#173 SLAM DUNK`
- `#179 飛べ！イサミ`
- `#181 愛天使伝説 ウェディングピーチ`
- `#182 ふしぎ遊戯`
- `#183 スレイヤーズ`
- `#185 あずきちゃん`
- `#187 KEY THE METAL IDOL`
- `#189 ナースエンジェル りりかＳＯＳ`
- `#192 怪盗セイント・テール`
- `#194 新世紀エヴァンゲリオン`
- `#198 バーチャファイター`
- `#199 神秘の世界 エルハザード`
- `#200 爆れつハンター`
- `#202 名犬ラッシー`
- `#203 るろうに剣心`
- `#204 爆走兄弟レッツ＆ゴー`
- `#207 勇者指令ダグオン`
- `#209 名探偵コナン`
- `#210 フリー'96`
- `#211 こどものおもちゃ`
- `#212 水色時代`
- `#213 地獄先生ぬ～べ～`
- `#215 天空のエスカフローネ`
- `#216 みどりのマキバオー`
- `#217 こちら葛飾区亀有公園前派出所`
- `#218 赤ちゃんと僕`
- `#219 ガンバリスト！駿`
- `#220 逮捕しちゃうぞ`
- `#221 家なき子レミ`
- `#222 ハーメルンのバイオリン弾き`
- `#223 セイバーマリオネット`
- `#224 機動戦艦ナデシコ`
- `#225 ＹＡＴ安心！宇宙旅行`

These names are useful as **period plausibility anchors**, not as a requirement to mention famous works in every generated post. They can help validate whether a generated 1996 station's available boards and contemporary conversation topics are temporally plausible.

### 5. The station visibly reorganized its information architecture

Early boards `#4` through `#12` are explicitly named `旧 ...`, followed by newer broad-category boards from `#15` onward. A later `#210 フリー'96` coexists with earlier `#130 フリー`, and `#272 サロン「メイプル２」` coexists with `#13 サロン「メイプル」`.

This supports allowing a persistent fictional BBS to accumulate renamed, superseded, year-specific, or successor boards instead of constantly presenting a clean normalized taxonomy.

## Implications for Zutto PC Communication

Use this source to calibrate **station ecology**, not to clone Mapletown:

- permit topic trees to grow over time and retain old/superseded areas where historically plausible;
- allow dedicated boards to appear for currently active works/topics when station culture and membership justify them;
- include social-practice boards (greetings, offline meetings, viewing parties, announcements) when appropriate;
- let specialist stations develop dense, idiosyncratic taxonomies rather than forcing a universal shared board schema;
- keep board creation and retirement as canonical world events so NPCs and article generation see the same station history;
- do not infer host-program command/menu semantics from this web preservation interface.

## Collection status and next research pass

Confirmed and recorded in Git in this pass:

- preservation-site identity and stated history;
- public board directory and its `#1`-`#272` board-name structure;
- derived station-ecology observations above.

The article/board content under `/bbs/...` is HTTP-authenticated. The research session's web fetch interface can read the public directory but cannot attach a Basic-Authorization header, so authenticated article bodies were **not** claimed as collected in this pass.

A later private-archive acquisition should use the site's published read-only credentials without embedding them in Git. High-value sampling should prioritize:

1. system information / system Q&A / proposal boards for station operation history;
2. `サロン「メイプル」`, `フリー`, and `フリー'96` for ordinary social writing;
3. greetings, offline-meeting, viewing-party, announcement boards for community behavior;
4. technical boards (`computer`, program, CG, computer music, Internet) for period terminology and practical discussion;
5. several 1995-1996 title-specific boards for subject/body style near the project's target date.

When article bodies are archived, preserve raw bytes and provenance outside Git, then add only derived/non-redistributive observations and corpus statistics here or in the relevant corpus notes.
