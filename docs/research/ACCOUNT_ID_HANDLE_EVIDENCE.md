# 1990年代パソコン通信のログインID・ハンドル調査

最終更新: 2026-09-21

## 目的

「ずっとパソコン通信」で生成する会員について、次の3種類の識別子を混同しないための考証・設計資料。

1. world persona key — 世界DB内部で人物そのものを識別するキー。
2. account/login ID — あるBBS局の会員としてログイン・個人識別に使用する局内ID。
3. handle — 局内で表示される名前。史実上は変更可能な例があり、IDとは別概念。

主対象は1995〜1997年前後の日本のパソコン通信。以下では、確認できた史料、合理的な解釈、
プロジェクト独自の共通仕様を明確に分ける。

---

## プロジェクトで採用する共通仕様

以下は史実上すべてのホストプログラムに共通だったという主張ではない。
複数の歴史的ホストを同一世界で安全に扱うための「ずっとパソコン通信」側の共通制約である。

### Handle policy

- 同一ホスト内でハンドル重複不可
- 半角ASCIIの英字・数字・記号のみ
- 日本語、全角英数、全角記号は生成しない
- 大文字小文字を重複判定で区別するかは未決定
- 許可記号の最終集合は未決定
- 最小長・最大長は未決定
- ホスト固有史料からさらに厳しい制約が確認できた場合は、そのホスト側で追加制約を持てる設計にする

この共通仕様は、KTBBS/BIG-Model等で日本語ハンドルの実在例があることと矛盾する。
これは意図的なサービス仕様であり、「当時日本語ハンドルが使えなかった」という史実にはしない。

### 現時点での文字種判断

| 文字種 | プロジェクト共通仕様への扱い | 根拠 / 状態 |
| --- | --- | --- |
| A-Z | 許可予定 | 半角ASCII英字 |
| a-z | 許可予定 | 半角ASCII英字。case比較規則は未決定 |
| 0-9 | 許可予定 | 半角ASCII数字 |
| . | 許可候補として強い | KTBBS系Canvasで SADA.Y の実使用例あり |
| - | 保留 | 当時らしい形としては自然だが、対象ホストの純正ハンドル入力仕様を未確認 |
| _ | 保留 | 同上 |
| ; | 共通許可しない方向 | KTBBSでは半角セミコロン対応入力関数を提供する別拡張 GIS & GIG が存在 |
| 空白、: / \ @ , " ' * ? | < > = + & % 等 | 保留 / 原則禁止候補 | 対象ホストの入力・コマンド構文と衝突する可能性があるため、史料確認前に許可しない |

注意: GIS & GIG の存在は「KTBBSのハンドルだけがセミコロン不可だった」ことを直接証明するものではない。
KTBBS標準入力系で半角セミコロンが特別扱いだったことを示す手掛かりとして扱う。

---

## 設計上の結論

### 1. Personaと局内会員情報を分ける

P00001 のような値はworld内部の人物キーであり、ユーザーがログイン時や記事一覧で見るIDではない。

同一人物が複数局へ加入する可能性があるため、productionでは次のように分離する。

~~~text
Persona
  persona_id        # 世界内の人物そのもの

HostMembership
  host_id
  persona_id
  account_id        # その局での固定ログインID
  current_handle    # その局で現在名乗っているハンドル
  joined_at
  status
~~~

史料上ハンドル変更が確認できるため、将来は変更履歴もmembership側に保持する。

~~~text
HostHandleHistory
  host_id
  persona_id
  handle
  valid_from
  valid_to
~~~

これにより、過去ログ表示時に「現在のハンドルへ置換」せず、投稿時点の表示名を再現できる。

### 2. account IDは局ごとの名前空間

草の根局では局略号 + 数字の実例が複数あるが、これは全ホスト共通フォーマットではない。
発番方式はHostProgram / Station configuration側の責務とする。

局ごとに、例えば以下を選択可能にする。

- 局略号 + 連番
- システム自動発行
- SYSOP承認後発行
- 予約ID / システムID
- 欠番を含む既存会員番号空間

### 3. ハンドル変更は「人物の履歴」

KTBBS系Canvasでは、IDが固定のままハンドルを何度も変更した人物が確認できる。
したがってハンドルはPersonaの不変属性として扱わない。

### 4. ハンドル重複回避は生成器の責務

本プロジェクトでは同一ホスト内のハンドル重複を許さないため、NPC生成時には候補を選び直す。

ただし、現代Webサービスのように必ず name2, name3 と機械的に付番するのではなく、
人物が最初から別のハンドルを考えたように見える複数の命名パターンを持たせる。

これはサービス独自の生成規則であって、
特定の歴史的ホストプログラムが同じ自動リネーム処理を行ったという意味ではない。

---

## 確認できた史料

### KTBBS / Canvas Network

1995-09-27版のCanvas Network局内用語集には、局内IDとハンドルが別々に記録されている。

例:

- CAN0061 / あお
- CAN0051 / 蒼騎
- CAN0001 / 辰巳
- CAN0078 / なっぱ
- SYSOP / SADA.Y

同資料はハンドルを「通信の世界における名前」と説明し、
本名より匿名・ペンネーム的に使われ、変更する人もいると記している。

1997年版の解説ではさらに、KTBBSは基本的にIDとハンドルを並記表示するため、
ハンドルを変更してもIDで個人識別できたと明記されている。
CAN0020のハンドル変遷も長期ログから列挙されており、
「固定ID + 変更可能ハンドル」というモデルは強く裏付けられる。

CAN0078 / なっぱ については「なっぱだけに78番」「狙って取った」と記される。
少なくともCanvasでは番号取得に人間側の意図が入るケースもあった。

#### 記号に関する手掛かり

SADA.Y の実例から、ピリオド . を含むハンドルの存在は確認できる。

VectorのKTBBS関連一覧には1997年公開の
GIS & GIG 1.00 —「KTBBSに半角セミコロン対応の入力関数を提供するユニット」
が存在する。したがって標準入力系では半角セミコロンが単純な通常文字ではなかった可能性が高い。

#### KTBBS純正資料の所在

Vectorには1996-06-10公開のKTBBS 6.21Aについて、少なくとも以下が現存する。

- KTBBS ソース 6.21A
- KTBBS ユーザーズマニュアル 6.21A
- KTBBS シスオペマニュアル 6.21A
- KTBBS データベース 1.62
- PC-98 / DOS/V 実行ファイル
- システムメッセージ・ユーティリティ

したがって、ハンドルの最大長、最小長、大小文字比較、禁止文字を確定する際は、
回想やログではなく6.21Aの会員レコード定義と入力/比較ルーチンを直接読むことを優先する。

Sources:

- https://lavenderblue.jp/chair/candic/candic07.html
- https://lavenderblue.jp/chair/candic/candic10r.html
- https://lavenderblue.jp/chair/candic/candicff.html
- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/ktbbs/by_date.html
- https://www.vector.co.jp/soft/dos/net/se009531.html
- https://www.vector.co.jp/soft/dos/net/se009515.html

Evidence: station-internal documents later republished + surviving official distribution catalog  
Confidence: high for Canvas behavior and KTBBS ID/handle separation; unknown for exact stock 6.21A handle validation rules

---

### mmm / MASH

1995年前後にmmm Rev.4.1を使用していた「いぬ。BBS」の当時資料では、
ログイン画面のID入力に new と入力するとIDを発行してもらう運用が説明されている。

Midnight Drivingが公開しているMASH/mmm系ユーザーマニュアルでは、
メールの宛先としてIDまたはhandleコマンドで登録されたハンドルネームを利用できる。
また、handle はハンドルネームを設定・変更する独立コマンドとして記載されている。

このため、mmm/MASHでもIDとハンドルは別概念であり、
少なくともハンドル変更可能であることは強く支持される。

MASHの操作案内には「コマンドは小文字」とあるが、
これはハンドル名の大文字小文字比較規則を示す証拠ではない。
handle case sensitivityへ一般化してはいけない。

また、ハンドルがメール宛先として利用されるため、
mmm/MASHのハンドルは単なる装飾的表示名ではなく機能上の検索キーにもなる。
本プロジェクトで同一ホスト内ハンドル重複を禁止する方針とは相性がよいが、
これは史実上の重複禁止を証明するものではない。

Sources:

- https://old.tsg.ne.jp/buho/189/tsg189
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/manual/MASHMAN/USER-utf8.txt
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/manual/MASHMAN/COMMANDS-utf8.txt

Evidence: contemporary club bulletin + surviving operator-published manuals  
Confidence: high for ID/handle separation and handle-change command; unknown for exact charset/length/case comparison

---

### BIG-Model

1999年のBIG-Model ver.5の保存ログには、送信者欄の例として

~~~text
NAT27811 暁の疾風
~~~

が残っており、さらに投稿者自身が
「一般ボード（ID＋ハンドル表示モード）」という表示モードを説明している。

したがってBIG-Model v5ではIDとハンドルが別フィールドであることが強く確認できる。

ただし1999年資料であり、プロジェクト中心年代の1995〜96年より後である。
v5の入力文字種、長さ、変更規則を以前の版へそのまま逆輸入しない。

また日本語ハンドルが実在するため、
本プロジェクトの「ASCIIのみ」は史実上のBIG-Model共通制限ではなくサービス独自制約となる。

Source:

- https://log.maruo.co.jp/hidesoft/hidesoft_3/x9901621.html

Evidence: preserved operational log / contemporary support discussion  
Confidence: high for BIG-Model v5 ID+handle display; medium/unknown for earlier versions

---

### RT-BBS / The resource版 Turbo-BBS

Vectorに以下が現存する。

- The resource版 Turbo-BBS
- The resource版 Turbo-BBS ソースキット 5.3βb

ソースキットの説明にはTurbo Pascal 6.0Aで書かれ、ソース公開されていることが明記される。

したがって、RT-BBSについてハンドルの文字数・許可文字・case比較を確定する場合は、
ソースキット内の会員データ型、登録処理、検索/比較関数を直接確認する。

現時点ではexact handle validationは未確認。

Sources:

- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/rtbbs
- https://www.vector.co.jp/soft/dos/net/se048002.html

Evidence: surviving original distribution catalog/source kit  
Confidence: high for source availability; unknown for handle validation until archive inspection

---

### VS

Vectorに高速多回線ホスト VS for PC-9801 1.31β6の配布アーカイブが現存する。
説明から、IDによるファイル検索、プロフィール、メール、チャット等の機能が確認できる。
またVS関連ツールとしてID変換・IDクリア系ユーティリティも残っている。

ただしハンドルの入力制約については現時点で未確認。

Sources:

- https://www.vector.co.jp/download/file/dos/net/fh041403.html
- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/vs

Evidence: surviving distribution archive/catalog  
Confidence: high for archive/version existence; unknown for handle validation

---

### 絵理香K版

現時点の調査では、ID/ハンドルの文字種・最大長・case比較を確定できる
純正マニュアルまたはソースへまだ到達していない。

接続ログからログインID入力があることは別資料で確認済みだが、
ハンドル制約は推測しない。

See also: docs/host-programs/erika-k.md

---

### NIFTY-Serve

1996年7月の『看護教育』掲載記事は、
NIFTY-Serve加入時に会員識別用IDが割り当てられると説明し、
例として XYZ12345 を示す。
インターネットメールでは XYZ12345@niftyserve.or.jp とした。

現在の@nifty FAQも旧来の@nifty IDを
「英字3文字＋数字5桁」「自動的に付与」と説明している。

これは草の根ホストの実装仕様ではないが、
「システム割当ID」と「人が名乗る名前」を分ける当時の一般的な文化を理解する参考になる。

Sources:

- https://webview.isho.jp/journal/detail/abs/10.11477/mf.1663901414
- https://faq.support.nifty.com/4240

Evidence: contemporary published article + current official legacy documentation  
Confidence: high for ID format distinction

---

### PC-VAN

1996-10-22のUsenet保存記事に、
PC-VAN経由の実在例として PBB10208@pcvan.or.jp が残っている。

Source:

- https://ie.u-ryukyu.ac.jp/~kono/fj/fj.beginners/70.html

Evidence: preserved 1996 message  
Confidence: high for existence of this ID shape

---

### ミンキームーンネットワーク

後年公開された保存会員リストでは MMN00001 からの局固有IDとハンドルが併存し、
MMN00002〜MMN00005 はシステム用予備IDだったとの説明がある。

後年の再公開資料であるため一次資料より一段低く扱うが、
局内ID空間に予約番号や欠番がある設計の参考になる。

Source:

- https://minkymoon.jp/2025/10/09/minkymoon-userlist/

Evidence: later republication of preserved member list  
Confidence: medium

---

## 未確定事項

### 大文字小文字

次を区別して調査する必要がある。

1. 入力時に大文字と小文字を両方保存できるか
2. 表示時に入力caseを保持するか
3. ログイン/検索時の比較がcase-sensitiveか
4. ハンドル重複判定がcase-sensitiveか

TAKA, Taka, taka を別名として許すかどうかは、まだ決めない。

### 許可記号

現時点で対象ホストの純正入力仕様から確定できた記号集合はない。

- . : 実使用例あり
- ; : KTBBS入力系では注意が必要
- - と _ : 採用候補だが史料確認待ち
- その他ASCII記号: 保留

### 最小長 / 最大長

推測値を置かない。

KTBBS、RT-BBS、VSは配布アーカイブ/ソースが現存するため、
以下を直接確認して決める。

- Pascal等の固定長文字列定義 String[n]
- 会員レコードのhandleフィールド
- オンラインサインアップ入力関数のmax length
- 空文字列/1文字を拒否するvalidation
- compare/uppercase/lowercase正規化処理

---

## 現在の実装との差分

2026-09-20にPersona Labへ導入した実装は、今回の後続調査・仕様決定より前の暫定版である。

現状は以下の点で今後の共通仕様と一致しない。

- かな/漢字ハンドルを生成する
- ☆ のような全角記号を生成する
- collision resolverに当時の形を混ぜた自動改名処理を持つ
- HandleがPersona identity寄りに保持され、membershipごとの変更履歴モデルにはなっていない

したがってこの資料をSource of Truthとして、次のidentity生成改修時に整合させる。

このドキュメント更新だけでは実装を変更しない。

---

## 次回調査の優先順位

1. KTBBS 6.21A
   - ソースアーカイブを展開
   - member/ID record定義を特定
   - handle入力関数・変更処理を特定
   - case正規化、禁止文字、max/min lengthを確定
2. RT-BBS 5.3βb
   - ソースキットを同じ観点で解析
3. VS 1.31β6
   - 配布アーカイブ内の設定・会員データ・ドキュメントを解析
4. mmm/MASH
   - handleコマンド実装またはデータ定義を追加探索
5. BIG-Model
   - 1995〜96年版資料を優先探索
6. 絵理香K版
   - 配布物、help text、raw connection logsを追加探索

最終的にはホストごとに次の表を埋める。

| Host program / version | Handle charset | Case storage | Case comparison | Min length | Max length | Symbols | Changeable | Evidence |
| --- | --- | --- | --- | ---: | ---: | --- | --- | --- |
| KTBBS 6.21A | TBD | TBD | TBD | TBD | TBD | . 実例 / ; 要注意 | Yes (strong evidence) | source/manual + Canvas |
| mmm/MASH | TBD | TBD | TBD | TBD | TBD | TBD | Yes | published manual |
| BIG-Model v5 | Japanese handle observed | TBD | TBD | TBD | TBD | TBD | TBD | 1999 operational log |
| RT-BBS 5.3βb | TBD | TBD | TBD | TBD | TBD | TBD | TBD | source kit available |
| VS 1.31β6 | TBD | TBD | TBD | TBD | TBD | TBD | TBD | archive available |
| 絵理香K版 | TBD | TBD | TBD | TBD | TBD | TBD | TBD | further research required |
