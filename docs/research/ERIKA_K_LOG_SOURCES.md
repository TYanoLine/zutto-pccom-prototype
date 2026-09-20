# 絵理香K版ログ取得メモ

## 方針

絵理香K版の挙動を一次資料から再構成するための取得対象を記録する。
第三者サイトの本文は GitHub に再配布せず、`scripts/research/archive_pccom_sources.py` で研究用のローカル出力先へ取得する。
リポジトリには URL・出典分類・取得コードだけを残す。

## 自動取得対象

| source id | 局 | 年代 | 内容 | URL |
| --- | --- | ---: | --- | --- |
| `erikak-garakuta-chat-19960430` | 東京がらくた工房 | 1996-04-30 | 接続、メインメニュー、電報、WHO、CHAT、切断を含む長大な接続ログ | https://sixsamana.com/library/lib/D-00078.html |
| `erikak-garakuta-hidden-board-1995` | 東京がらくた工房 | 1995 | ボード画面。アペンド操作表示を含む | https://sixsamana.com/library/lib/A-00005.html |
| `erikak-garakuta-append-board-1993` | 東京がらくた工房 | 1993 | ボード／返信ログ。`APPEND` 操作痕跡を含む | https://sixsamana.com/library/lib/B-00032.html |
| `erikak-garakuta-board-1995` | 東京がらくた工房 | 1995 | 掲示板ログ | https://sixsamana.com/library/lib/A-00025.html |

これらは保存サイト側が第三者ログを掲載した資料であり、内容中の人物評や主張を史実として採用しない。
UI、コマンド、プロンプト、記事・アペンド構造などホスト挙動の証拠として扱う。

## 手動確認対象

- くにびきNETの1989/1998ログ議論: https://mixi.jp/view_bbs.pl?comm_id=386567&id=3644356
  - 1998年ログに `ERIKA-K VER1.93` / 絵理香K版 Ver1.93 の表示があると確認済み。
  - Mixiは取得条件が不安定なため自動収集対象にはまだ入れず、手動レビュー対象として台帳にのみ登録する。

## 取得

```bash
python scripts/research/archive_pccom_sources.py --output /path/outside/repository/pccom-archive
```

取得物には raw bytes、UTF-8正規化テキスト、SHA-256、HTTPヘッダ、manifest が生成される。
個人ハンドルやプロフィールなどを含む可能性が高いため、絵理香K版ログは `pii_risk: high` として扱う。
