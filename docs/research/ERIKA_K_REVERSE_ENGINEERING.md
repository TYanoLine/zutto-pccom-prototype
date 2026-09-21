# 絵理香K版 リバースエンジニアリング台帳

## 目的

現存する絵理香K版の公式仕様・マニュアル・配布物が不足しているため、当時の接続ログから操作系、プロンプト、状態遷移、掲示板/アペンド構造、チャット、メール/ファイル系の仕様を段階的に復元する。

この文書は「K版の確定仕様」と「特定局のカスタマイズ」を分離するための台帳である。第三者ログの投稿本文やチャット本文は再配布せず、UI/プロトコル上の非表現的事実だけを記録する。

## 証拠レベル

- **Confirmed / repeated** — 複数ログまたは複数年代で同じ挙動を確認。
- **Confirmed / single station** — 実ログで確認できるが、局固有設定の可能性がある。
- **Inferred** — 周辺文脈から有力だが、直接の操作結果がまだ不足。
- **Unknown** — 意味を確定できていない。

## 現在確認できる仕様

### ログインとセッション

**Confirmed / single station (東京がらくた工房, 1996)**

- ID入力画面があり、ゲストIDを受け付ける。
- ログイン後に前回アクセス時刻を表示する。
- WELCOME/お知らせの後にメインメニューへ入る。
- 長文表示中の制御として、`Z`系キーで中断、`S`系キーで一時停止/再開する表示がある。
- 切断時には `NO CARRIER` / `Disconnected` 等がログに残る。

**Inferred / needs member log**

- 正規会員の `PASSWORD:` フロー。
- 認証失敗時の再試行回数、ロック、ゲストとの差分。

### 局ごとのメニュー・説明文カスタマイズ

**Confirmed / repeated across multiple K-version stations**

くにびきNETの発掘ログでは `絵理香K版 Ver1.93` と明示され、メインメニューは
`[1] ボード(BM) [2] ファイル(FM) [3] メール(MAIL) [4] 電報･チャット(C) [5] ジャンク(JUNK) [6] 各種設定(MODE) ... [9] 接続終了(BYE)`
のように構成されている。

一方、東京がらくた工房のK版ログでは、同じ中核コマンド `BM`, `MAIL`, `C`, `JUNK`, `MODE`, `BYE`, `ASET` 等を使いながら、
メニュー番号・文字キー・説明文・追加項目が異なる。たとえば MODE は `[O]`、BYE は `[Q]`、ボードマップは `[K]` と表示され、
`PROF`, 最終接続日時仮設定、GUEST入会申込み、`BAT` などもトップメニューに露出している。

この差から、リバースエンジニアリングでは少なくとも次を分離して扱う:

- **core command semantics**: `BM/FM/MAIL/C/JUNK/MODE/BYE/ASET` 等の実コマンドと状態遷移
- **station menu mapping**: 数字・文字ショートカットからコマンドへの割当
- **station labels/help text**: 「各種ボードの読み書き」「ボード」「環境設定･変更メニュー」等の説明文
- **feature exposure**: その局がトップメニューに見せる機能、GUEST権限、追加機能

したがって、ある1局のメニュー文言を「絵理香K版の固定UI」として実装しない。

Sources:
- https://mixi.jp/view_bbs.pl?comm_id=386567&id=3644356
- https://sixsamana.com/library/lib/D-00078.html

### メインメニュー

**Confirmed / single station (東京がらくた工房, 1996)**

観測プロンプトは `MAIN>M:MENU ->`。同じ機能に対し、短いメニューキーとダイレクトコマンドを併用する設計が確認できる。
観測済みのダイレクトコマンドには `BM`, `MODE`, `HELP`, `MAIL`, `PROF`, `C`, `JUNK`, `BYE`, `ASET`, `BAT` がある。

東京がらくた工房の表示では、ボード、環境設定、ヘルプ、メール、プロフィール、電報/チャット、会議室、切断、自動運転、構成マップ、入会申込み、バッチダウン等が並ぶ。ただし項目の有無・キー割当は局設定の可能性が高い。

### 電報 / WHO / CALL

**Confirmed / single station (東京がらくた工房, 1996)**

CALLサーフェスでは、回線番号指定の電報、`*` 全員宛、ReturnでWHO、`.` で前メニュー、`P` プロフィール、`H` ヘルプ、`X` チャット、ベルON/OFF、電報受信状態変更を観測。電報本文には最大文字数の表示もある。観測プロンプトは `CALL> ->`。

WHO行には少なくとも「回線番号、ID/ハンドル、現在状態、接続速度または端末表示、1行プロフィール」に相当するフィールドが存在する。アイコン/記号列の意味は未確定。

### チャット

**Confirmed / single station (東京がらくた工房, 1996)**

- CALLから `X` でチャットルーム選択へ遷移。
- 複数のCHAT ROOMを番号で選択するUI。
- Returnでルーム選択を中止する表示。
- チャット終了操作として `..` または `CTRL+B` が案内される。
- WHO状態には `CHAT #n` のようにルーム番号が表示される。

ルーム数、各ルーム定員、混雑時間帯の案内文は局固有設定の可能性が高い。

### ボード階層

**Confirmed / single station (東京がらくた工房, 1995)**

観測プロンプトは `(BJ\40) BOARD>M:MENU ?:HELP ->`。`BJ`系のパス/階層表現が存在し、ボード番号を入力して下位ボードへ入る挙動が確認できる。

記事一覧は概ね `<board>-- <article> <YY/MM/DD> <HH:MM> <author> [numeric-field] <subject>` の形。作者名直後の数値はアペンド数である可能性が高いが、現段階では `numeric_field_after_author` として保持し断定しない。

### 記事操作とアペンド

**Confirmed / repeated across 1993 and 1995 preserved logs**

記事操作行では、Return/番号=読む、`U`=読まない、`A`=アペンド、`W`=書く、`.`=戻る、`?`=HELP/その他、`K`=書き込み削除、`KA`=アペンド削除、`0/00/T/+/-/N/B`=ナビゲーション系キー群を観測している。最後のキー群の意味は未確定。

1993年ログでは親記事の後に複数の返信が連続表示され、さらに `APPEND <board> <article>` という操作痕跡が残る。これは「返信を独立記事にせず親記事へアペンドする」というK版の中核的な記事モデルを強く支持する。

**Inferred**

- 一覧の作者名後の数値はアペンド件数。
- `U` は未読状態へ戻す/読まない扱いにする操作。
- `N/B/+/-/T/0/00` の正確な移動規則。

### 未取得・優先調査項目

1. 正規会員ログインとPASSWORDエラー処理。
2. `BM/BX/BXS/BR/BW/BWX/BKILL/BJ` 系の完全な引数仕様。
3. `FM/FX/FXS/FR/FW/FWX/FKILL/FJ` とファイル転送/NMODEM。
4. `MAIL/MX/MR/MW/MKILL` のメールボックスモデル。
5. `MODE`, `ASET`, `BAT`, `GUIDE`, `MEMB`, `PASS` の詳細。
6. 未読管理と最終アクセス日時仮設定の関係。
7. ボード階層の親/子/兄弟移動キー。
8. SYSOP/SIGOP権限と `K` / `KA` の表示条件。
9. バージョン差 (1993系 vs 1995系 vs `ERIKA-K Ver1.93`)。
10. 局設定ファイルがどこまでUI/キー/機能を変更できたか。

## 機械解析

研究用アーカイブ取得後、`scripts/research/analyze_erika_k_logs.py` に `--archive`, `--json`, `--markdown` を渡す。解析器は第三者の投稿本文/チャット本文を出力せず、仕様復元に必要な構文だけを抽出する。同一構文が複数ファイルに出た場合は `confirmed-repeated` に昇格させる。

## 現在の主要一次資料

- 東京がらくた工房 1996 接続/電報/WHO/チャットログ: https://sixsamana.com/library/lib/D-00078.html
- 東京がらくた工房 1995 隠しボードログ: https://sixsamana.com/library/lib/A-00005.html
- 東京がらくた工房 1993 ボード/APPENDログ: https://sixsamana.com/library/lib/B-00032.html
- 東京がらくた工房 1995 掲示板ログ: https://sixsamana.com/library/lib/A-00025.html
- くにびきNET 1989/1998発掘ログ議論: https://mixi.jp/view_bbs.pl?comm_id=386567&id=3644356
