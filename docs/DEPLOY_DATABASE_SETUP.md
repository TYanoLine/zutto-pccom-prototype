# デプロイ時のDB接続セットアップとトラブルシュート

本番(Render)でメインメニュー「1 センターの呼び出し」が表示されなかった問題の原因と、
自前の PostgreSQL に Render から接続できるようにした手順の記録。

> 注意: パスワード・実ホスト名・IPアドレスはこのドキュメントに書かない。

## 症状と原因の切り分け

| 症状 | 原因 |
|---|---|
| 「1」で「センター情報を読み込むことができませんでした」 | 起動時の `GET /api/world/bootstrap` が失敗(クライアントは `error` 状態) |
| `{"error":"persistent world database is not configured"}` | サーバに `DATABASE_URL` が渡っておらず、DBなしで起動していた |
| `/health` が `persistent_worlds: false` のまま | 新しいデプロイが起動失敗し、古いインスタンスが応答していた |

`/health` の見方:

- `persistent_worlds: true` / `historical_research: true` / `historical_knowledge: true` → DB接続成功
- `azure_openai_configured: true` → センター名生成に必要な Azure OpenAI 設定済み

`DATABASE_URL` が設定されていてDB接続に失敗すると、サーバは `log.Fatalf` / 起動時エラーで終了する
(`apps/server/cmd/server/main.go`)。したがって `/health` が応答していて `persistent_worlds: false`
の場合は「`DATABASE_URL` が空」か「新デプロイが失敗して旧版が動いている」のどちらか。

## 手順(PostgreSQL をWindows PCで自前運用する場合)

### 1. 待ち受けを外部向けにする

`postgresql.conf`:

```
listen_addresses = '*'
ssl = on
ssl_cert_file = 'server.crt'
ssl_key_file = 'server.key'
```

自己署名証明書の例(データフォルダで実行):

```powershell
openssl req -new -x509 -days 825 -nodes -text -out server.crt -keyout server.key -subj "/CN=<ホスト名>"
```

確認:

```powershell
Get-NetTCPConnection -LocalPort 5432 -State Listen   # 0.0.0.0 / :: になっていること
```

### 2. 接続許可(`pg_hba.conf`)

```
hostssl  zutto  zutto  <許可する接続元CIDR>  scram-sha-256
```

反映: `SELECT pg_reload_conf();` またはサービス再起動。

- 動作確認時は `0.0.0.0/0` でも可だが、確認後は Render のアウトバウンドIP範囲に絞る
  (Render ダッシュボード → 対象サービス → Connect → Outbound に CIDR 表記で表示される。
  範囲内のどのIPでも使われる可能性があるため、単一IPだけの許可は避ける)。

### 3. ファイアウォールとポート転送

```powershell
New-NetFirewallRule -DisplayName "PostgreSQL 5432" -Direction Inbound -Protocol TCP -LocalPort 5432 -Action Allow -Profile Any
```

自宅回線の場合はルーターで TCP 5432 → 該当PC のポート転送が必要。

### 4. ユーザーとDBの作成

```sql
CREATE USER zutto WITH PASSWORD '<英数字のみの強いパスワード>';
CREATE DATABASE zutto OWNER zutto;
-- 既存の場合
ALTER USER zutto WITH PASSWORD '<新しいパスワード>';
```

記号を含むパスワードは `DATABASE_URL` でURLエンコードが必要になるため、切り分け時は英数字のみにする。

### 5. Render の環境変数

```
DATABASE_URL=postgres://zutto:<パスワード>@<ホスト名>:5432/zutto?sslmode=require
AZURE_OPENAI_ENDPOINT=https://<リソース名>.openai.azure.com
AZURE_OPENAI_API_KEY=<キー>
AZURE_OPENAI_MODEL=<デプロイ名>
```

保存後に再デプロイ(Manual Deploy)する。起動時にスキーマが自動作成される。

### 6. 動作確認

1. `https://<サービス>.onrender.com/health` が `persistent_worlds: true`
2. アプリのメインメニューで `1` を押す。初回はセンター100局の生成(Azure OpenAI)で
   最大1〜2分「読み込み中」画面になり、完了すると自動でリストが開く
3. 2回目以降はDBからの読み込みのみ(ブラウザごとの `worldKey` で世界が分かれる)

## 起動時エラーと対処(Render ログ)

| ログ | 原因 / 対処 |
|---|---|
| `DATABASE_URL is not set; persistent generated worlds ...` | 環境変数が空。名前は `DATABASE_URL` と完全一致が必要。再デプロイ漏れも確認 |
| `no pg_hba.conf entry for host "<IP>", user "zutto", database "zutto", SSL encryption` | `pg_hba.conf` に接続元の許可行がない。`hostssl` 行を追加して reload |
| `failed SASL auth: ... password authentication failed for user "zutto"` | ユーザー未作成、またはパスワード不一致(URLエンコード漏れ・空白混入も確認) |
| `database "zutto" does not exist` / `role "zutto" does not exist` | DB/ユーザー未作成 |
| `SSL is not enabled on the server` | サーバ側SSL未設定。`ssl = on` と証明書を設定 |
| `connection refused` / タイムアウト | PostgreSQL停止、ポート転送/ファイアウォール未設定 |

## 外部からの疎通確認

- `check-host.net` の「TCP port」で `<ホスト名>:5432` を確認する(複数拠点から接続を試せる)。
- 開発用サンドボックスなど、外向き通信が制限された環境では 5432 がタイムアウトすることがある。
  その結果だけで「届いていない」と判断せず、外部のポートチェッカーや Render 側の結果で確認する。

## ローカルPCでの注意

- 同一PCに複数の PostgreSQL(サービス版・zip版・Docker)が共存すると、5432 の取り合いや設定ファイルの
  取り違えが起きる。`Get-NetTCPConnection -LocalPort 5432 -State Listen | Select OwningProcess` と
  `SHOW data_directory;` でどれが動いているか確認する。
- 手動起動した zip版は `pg_ctl -D <data> stop -m fast` で停止する(強制終了しない)。

## 今後の改善案

- センター取得失敗時に、内蔵センター(`DEFAULT_CENTERS`)へフォールバックし、再試行できるようにする
  (現状は取得失敗が `error` 状態のまま固定され、リロードするまで復旧しない)。
- `pg_hba.conf` を `0.0.0.0/0` から Render のアウトバウンドIP範囲へ絞る。
- 自己署名証明書ではなく正式な証明書を使い、`sslmode=verify-full` にする。
- 公開DBの運用が重い場合は、Render Postgres 等のマネージドDBも検討する。
