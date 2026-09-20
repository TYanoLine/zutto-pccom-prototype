# 1990年代パソコン通信のID・ハンドル資料メモ

最終更新: 2026-09-20

## 目的

「ずっとパソコン通信」で生成する人物について、内部の人物キー、局内ログインID、表示ハンドルを混同しないための考証メモ。
主対象は1995〜1997年前後の日本。以下は、確認できた史料と、そこから実装へ採用する範囲を分けて記す。

## 確認できた事実

### NIFTY-Serve

1996年7月の『看護教育』掲載記事は、NIFTY-Serve加入時に会員識別用IDが割り当てられると説明し、
例として `XYZ12345` を示している。インターネットメールでは
`XYZ12345@niftyserve.or.jp` とした。

- https://webview.isho.jp/journal/detail/abs/10.11477/mf.1663901414
- Evidence: contemporary published article
- Confidence: high

現在の@nifty FAQも旧来の@nifty IDを「英字3文字＋数字5桁」「自動的に付与」と説明している。

- https://faq.support.nifty.com/4240
- Evidence: current official documentation describing legacy ID
- Confidence: high for format distinction, not a complete 1995 UI specification

### PC-VAN

1996-10-22のUsenet保存記事に、PC-VAN経由の実在例として
`PBB10208@pcvan.or.jp` が残っている。

- https://ie.u-ryukyu.ac.jp/~kono/fj/fj.beginners/70.html
- Evidence: preserved 1996 message
- Confidence: high for existence of this ID shape

### 草の根 Canvas Network

1995-09-27版の局内用語集には、局内IDとハンドルが別々に記録されている。
例: `CAN0061 / あお`, `CAN0051 / 蒼騎`, `CAN0001 / 辰巳`,
`CAN0120 / りとるれいでぃ`。SYSOPのハンドルは `SADA.Y`、
別ハンドルとして `理都☆` も記録されている。

同資料は「ハンドル」を「通信の世界における名前」とし、本名を使う人は少なく、
匿名・ペンネーム的に使われると説明している。

`なっぱ / CAN0078` について「なっぱだけに78番」「狙って取った」と明記されており、
局内の発番が常に無味乾燥な自動連番としてだけ受け止められていたわけではないことも確認できる。

- https://lavenderblue.jp/chair/candic/candic07.html
- Evidence: 1995 station-internal document, later republished on the web
- Confidence: high for this station

### mmm

TSG部報第189号の「いぬ。BBS」紹介は、1995年前後に mmm Rev.4.1 を使用しており、
接続後のID入力画面で `new` と入力すると自分のIDを発行してもらえる、と説明している。

- https://old.tsg.ne.jp/buho/189/tsg189
- Evidence: contemporary club bulletin
- Confidence: high for this station / mmm deployment

### BIG-Model

1999年の秀Term情報交換ログには、BIG-Model ver.5のタイトル表示例として
`NAT27811 暁の疾風` が残っており、送信者欄にIDとハンドルを併記する表示が確認できる。
目標年代より少し後なので、1995〜96の全バージョンへそのまま一般化しない。

- https://log.maruo.co.jp/hidesoft/hidesoft_3/x9901621.html
- Evidence: preserved operational log, 1999
- Confidence: high for BIG-Model 5-era display, medium for earlier versions

### ミンキームーンネットワーク

公開された保存会員リストでは `MMN00001` 〜の局固有IDと多様なハンドルが併存し、
`MMN00002`〜`MMN00005` はシステム用予備IDだったとの説明がある。
これは2025年に旧資料を再公開したものなので、一次資料そのものより一段低く扱う。

- https://minkymoon.jp/2025/10/09/minkymoon-userlist/
- Evidence: later republication of preserved member list
- Confidence: medium

## 実装に採用する解釈

1. **内部人物キーと局内IDを分ける。** `P00001` のような値は世界DBの内部キーであり、
   1990年代の利用者がログイン時や記事一覧で見るIDではない。
2. **局内IDはmembership側の属性とする。** 同じ人物が複数局へ加入できる設計なので、
   productionではPersonaではなくHostMembershipに属させる。
3. **草の根局には局固有prefix + 数字の発番を許す。** 3文字prefix + 4〜5桁は実例が複数ある。
   ただし全ホストプログラム共通仕様とはしない。
4. **欠番・予約IDを許す。** 保存会員一覧にはSYSOP/システム予約IDがあり、
   実運用の名簿が表示順どおりの連続番号になるとは限らない。
5. **ハンドルはIDと別物として自由度を高くする。** ローマ字短名、かな、漢字、
   英単語風、ピリオド入り、記号入り、機種・通信系語を含むものなどを混在させる。
6. **ハンドル衝突時の解決を単純な `-2`, `-3` に固定しない。**
   ピリオド＋イニシャル、ハイフン＋イニシャル、機種/通信語prefix、記号、
   2桁数字、別ハンドルへの取り直しを候補にする。
   これらは「当時の実例にあるハンドル形」を材料にしたサービス側生成規則であり、
   特定BBSが実際にこの順序で重複解消したという史実主張ではない。

## Persona Labでの暫定仕様

Persona Labにはまだ具体的なHost identityが無いため、局内ID用の3文字prefixはseedから作る
**架空の局コード**とする。実サービスへ統合する際は、その局のホストプログラム・局設定・
加入時期に応じた発番器へ差し替える。

当面のJSONでは:

- `id`: world-internal persona key
- `account_id`: Persona Lab上のhost-local membership ID
- `handle`: 表示ハンドル

とする。
