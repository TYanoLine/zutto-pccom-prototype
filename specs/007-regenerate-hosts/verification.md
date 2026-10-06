# Verification: 007 regenerate hosts

## Part 1（サーバ）— PR #323

### 自動確認（実装時の記録）

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 | exit code 0 |
| `go -C apps/server vet ./...` | 成功 | exit code 0 |
| `go -C apps/server test ./internal/worldcatalog/... -count=1` | 成功 | `TestMakeCenterMaintainsPhoneAcrossGenerations` を含む全テスト通過 |
| `go -C apps/server test ./... -count=1` | 成功 | 全パッケージ通過 |

CI（`go-test`、PR #323）も成功した。コミット `eeb3fec` で `main` にマージ済み。

### 手動確認 / 本番確認

| 項目 | 結果 | 備考 |
|---|---|---|
| Render 本番の `GET /api/world/reset-hosts` の応答 | 確認済み | 405 Method Not Allowed (`{"error":"POST only"}`) が返る |
| Render 本番の `POST /api/world/reset-hosts?key=invalid` の応答 | 確認済み | 400 Bad Request (`{"error":"invalid world key"}`) が返る |
| クライアント切断時の非同期完走 | **未確認**（本番で確認待ち） | |

---

## Part 2（Web）— PR #324

### 自動確認（実装時の記録）

| コマンド | 結果 | 備考 |
|---|---|---|
| `npm --prefix apps/web test` | 成功 | 20 test files, 109 tests passed |
| `npm --prefix apps/web run build` | 成功 | TypeScript 型チェックおよび Vite ビルド成功 |

CI（`web-build`、PR #324）も成功した。コミット `094ebc0` で `main` にマージ済み。

### 手動確認 / プレビュー確認

| 項目 | 結果 |
|---|---|
| メインメニューで `1r` を入力するとホスト再生成画面が表示され、完了後にメインメニューへ戻る | **未確認**（運用者の確認待ち） |
| 生成局の取得失敗時に「ほかのセンターの読み込みに失敗しました（再試行中...）」が表示される | **未確認**（運用者の確認待ち） |
| 指数バックオフ（2s, 10s, 30s）および `online` イベントで生成局が自動再試行される | **未確認**（運用者の確認待ち） |
| プリセット局読み込み待ち中にエラーが発生した際、エラー画面に切り替わる | **未確認**（運用者の確認待ち） |

