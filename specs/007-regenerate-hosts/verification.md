# Verification: 007 regenerate hosts

## Part 1（サーバ）

### 自動確認（実装時の記録）

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 | exit code 0 |
| `go -C apps/server vet ./...` | 成功 | exit code 0 |
| `go -C apps/server test ./internal/worldcatalog/... -count=1` | 成功 | `TestMakeCenterMaintainsPhoneAcrossGenerations` を含む全テスト通過 |
| `go -C apps/server test ./... -count=1` | 成功 | 全パッケージ通過 |

### 手動確認 / 本番確認

| 項目 | 結果 |
|---|---|
| Render へのデプロイ後の `POST /api/world/reset-hosts` の動作 | **未確認**（マージ・デプロイ後に確認予定） |
| クライアント切断時の非同期完走 | **未確認**（本番で確認予定） |

---

## Part 2（Web）

（Part 1 マージ・デプロイ後に着手予定）
