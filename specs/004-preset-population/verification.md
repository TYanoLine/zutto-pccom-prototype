# Verification: preset population

実装時（Copilot の実行）の記録。表の「失敗」は、実行時点の `main` にあった既存のテスト失敗によるもので、
下の「追記」のとおり、その後に別の PR で修正されている。

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

- サーバを起動して HAKATA に接続し、板の一覧が従来どおり表示される: 未実施（実装時点）

## 追記（マージ後）

- 上の `go test ./...` の失敗は、PR #294（局のフラグ化）で生成コンテンツのログの判定が
  `host.Debug.ContentLog` に変わったのに、テストが `other := host` でフラグを引き継いでいたことが原因だった。
  PR #305（マージ済み）で、テストに `other.Debug.ContentLog = false` を加えて修正した。
  実装時の実行は、この修正より前の `main` から作ったブランチで行われた。
- 修正後の `main` で `go -C apps/server test ./...` が通るかは、**この記録の時点では未確認**。
  PR #309 では `server-test` ワークフローが実行されていない（Copilot の PR は実行に承認が必要）。
  確認する場合は、`main` で `server-test` を実行するか、ローカルで次を実行する。

  ```bash
  go -C apps/server test ./... -count=1
  ```

- HAKATA の手動確認: 運用者が、マージ前に実施したと報告している。
  確認した項目の詳細は、この記録には含まれていない。
