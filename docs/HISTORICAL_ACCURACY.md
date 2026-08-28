# Historical accuracy policy

## Goal

「ずっとパソコン通信」は1990年代の見た目だけを借りた現代BBSではなく、当時の日本のパソコン通信を、資料で確認できる範囲では実際の文化・技術・操作感に沿って再構成する。

ただしサービス内の全局を実在局のコピーにすることは目的ではない。**歴史的に根拠のあるソフトウェア挙動 + 架空の局・会員・地域コミュニティ**を基本形とする。

## Evidence classes

実装・文書では可能な限り次を区別する。

### Confirmed
一次資料または信頼できる当時資料で直接確認できる事実。例: マニュアルに記載されたコマンド、通信ログに残るプロンプト、ソースコード上の状態遷移。

### Likely / inferred
複数資料や周辺仕様から合理的に推定できるが、直接確認できていないもの。推定を確定仕様のように表現しない。

### Station-specific
特定局の設定・改造・メニュー・ボード構成・WELCOME等。ホストソフト標準仕様と混同しない。

### Fictional reconstruction
サービス用に意図的に創作した局名、SYSOP、会員、記事、地域イベント、WELCOME、隠しボード等。史料準拠部分と矛盾しない範囲で自由に作る。

## Source preference

歴史的な挙動を決める際の優先順位の目安:

1. 当時のソースコード / 公式配布物
2. 公式マニュアル / README / ヘルプ
3. 当時の実通信ログ
4. 当時の雑誌・書籍・紹介記事
5. 当時の利用者による記録
6. 後年の回顧・保存サイト
7. 合理的推定

下位資料しかないこと自体は問題ではない。証拠強度を取り違えないことが重要。

## Host-program reconstruction

KTBBS、BIG-Model、絵理香K版、mmm、RT-BBS、VS等は独立したソフトウェアとして扱う。共通Runtimeの表示テーマとして再現しない。

特に以下は資料に基づき個別に確認する:

- 接続直後 / ID / password / guest flow
- main menu / command mode
- board / conference / forum hierarchy
- article numbering and reply/append semantics
- unread semantics
- mail / chat / telegram features
- file areas and transfer protocols
- SYSOP / member / guest permissions
- pagination and prompts
- logout sequence
- ANSI / ESC / cursor-control behavior
- local customization mechanisms

完全な資料がない場合は、確認済みの構造を維持しながら不足部分を局固有の架空カスタマイズとして補う。

## Era boundary

基本世界は1996年日本。NPC・記事・広告・局内ニュース等は、その世界日時点で知り得ない未来を知らない。

避けるもの:

- 後年発売の製品・サービス
- 後年の事件を既知の事実として語ること
- SNS前提の語彙や文化
- 現代ネットスラング
- 後知恵でしか成立しない評価

将来の予想として当時あり得た発言は可能。ただし「未来を知っているNPC」にならないこと。

## Period writing

局・人物ごとの差を優先するが、1990年代パソコン通信らしい表現として `(笑)`, `(爆)`, `(^^)`, `(^^;`, `(^_^;)`, `m(_ _)m`, `>` 引用などを使用できる。

現代的な `草`、SNS用語、現代的な `w` の連打、`ググる` 等を1996年の通常表現として使わない。

## Research records

重要な調査結果は会話履歴だけに残さず、`docs/host-programs/` または `docs/research/` に記録する。可能ならURL、資料名、確認したバージョン/年代、Confirmed/Likely/Station-specific/Fictional の区分を残す。
