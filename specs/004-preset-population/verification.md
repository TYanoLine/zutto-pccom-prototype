# Verification: preset population

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 成功 | |
| `go -C apps/server vet ./...` | 成功 | |
| `go -C apps/server test ./...` | 失敗 | 既存の `worldrepo/TestGeneratedContentAuditLogsOnlyNewCommittedWorldHeaders` が他ホストへの監査ログ漏れで失敗 |
| 金型テスト（3 回連続） | 成功 | `TestPopulationMatchesGolden` と `TestPopulationShape`、4 経路すべて |

## 金型ファイルが変わっていないこと

`git diff 3d84fdf29a618d7087bb1f06f5132c2c13cb3f69..HEAD -- apps/server/internal/world/testdata`
の結果は空。T002 で生成して以降、変更なし。

## 残った hakata を含むファイル（world のテスト以外）

`grep -rniE "hakata" apps/server/internal/world --include='*.go' | grep -v _test.go`
の結果は空。

## 手動確認

- サーバを起動して HAKATA に接続し、板の一覧が従来どおり表示される: 未実施
