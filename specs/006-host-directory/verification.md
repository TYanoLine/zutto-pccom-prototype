# Verification: host directory

## Part 1（サーバ）— PR #319

### 自動確認（実装時の記録、Copilot）

| コマンド | 結果 |
|---|---|
| `go -C apps/server build ./...` | 成功 |
| `go -C apps/server vet ./...` | 成功 |
| `go -C apps/server test ./... -count=1` | 成功 |

CI（`go-test`、PR #319、2026-10-05 11:55 UTC）も成功した。

### 本番の確認

`GET https://zutto-pccom-prototype.onrender.com/api/directory`（デプロイ `9f5299c`、2026-10-05 11:58 UTC に live）を、
運用者がブラウザで開き、次の JSON が返ることを確認した。

```json
{"centers":[{"id":"hakata-canal-net","name":"HAKATA CANAL NET","software":"絵理香K版","phone":"0920000196","dialMode":"tone","maxBaud":14400}]}
```

`busy-test`（`listed: false`）は含まれていない。Render のログに、デプロイ後のエラー・警告はない。

## Part 2（Web）— PR #320

### 方針の変更（改訂 2）

実装の途中で、方針が変わった。**生成局（LLM が作った局）も、電話帳に出す**（繋がらなくてよい）。
プリセット局を先頭に、生成局を後ろに並べる。`listed: false` の局は、引き続き載せない。
詳細は `revision-2.md`。これに合わせて、Copilot の実装（生成局の取得と世界の鍵を削除したもの）を、
同じ PR の中で直した。

### 自動確認

| 対象 | 結果 | 備考 |
|---|---|---|
| Copilot の実装（`262d47d`）の `web-build`（`npm ci`、`npm test`、`npm run build`） | 成功 | 2026-10-05 21:30 UTC。Copilot は、95 テストと記録した |
| 改訂 2 の修正の `web-build` | **未確認** | 修正のコミット後の結果で更新する |

### 手動確認

| 項目 | 結果 |
|---|---|
| Vercel のプレビュー（`262d47d`）で、電話帳に HAKATA が表示される | 確認済み（運用者、2026-10-05 21:35 UTC）。1 局、`HAKATA CANAL NET [絵理香K版]`、`092-000-0196`、`14400` |
| 同、サーバが眠りから起動した直後は、読み込み中の画面のまま止まることがある | 確認済み（一度発生。再読み込みで解消）。以前からある挙動で、別の修正として扱う |
| 改訂 2 の版で、プリセット局の後ろに生成局が表示される | **未確認** |
| 改訂 2 の版で、生成局の取得が遅い・失敗しても、プリセット局が表示される | **未確認** |
| 電話帳から HAKATA に発信できる | **未確認** |
