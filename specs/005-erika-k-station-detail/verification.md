# Verification: Erika-K station detail

## 実装時の記録（PR #316、Copilot）

PR #316 の `verification.md` は、実行したコマンドの一覧だけで、結果（成功・失敗・未実施）を含んでいなかった。
PR #316 は、`go-test` が実行されないまま、作成の 17 秒後にマージされた。
したがって、実装時の `go build`・`go vet`・`go test ./...` の結果は、**記録が無く、確認できていない**。

確認できること（コミットの履歴から）:

| 項目 | 結果 | 根拠 |
|---|---|---|
| 金型が、変更前のコードで生成され、実装より前にコミットされた | 確認済み | `357b9a9`（`T003: add pre-refactor Erika-K golden fixtures`、10:51）が、`07569fa`（`T011-T017`、10:58）より前 |
| 金型ファイルが、実装のコミットで変更されていない | 確認済み | 実装のコミット `07569fa` が変更したファイルの一覧に、`erikak/testdata/*` が含まれない（`golden_test.go` のみ、`goldenBoardTable` を新しいカタログから作る形に直している） |

## 追補のレビューと修正（PR: test/erika-k-detail-coverage）

PR #316 のレビューで見つかった不足を、別の PR で補った。

| 不足 | 対応 |
|---|---|
| 新しい検証のテストが無い | `hostcatalog/program_detail_test.go`、`erikak/detail_test.go`、`world/host_detail_test.go`、`worldrepo/host_detail_test.go` を追加 |
| `TestLoginBannerUsesExactDisplayCells` が、バナーが無くても通る | HAKATA の定義から `Runtime` を作り、バナーの 6 行が揃っていることも検査するように修正 |
| `{handle}` を、検証は全部品で許可するが、実行時の置換は 2 つの部品だけ | 検証を、置換される `login_banner` と `login_greeting` だけで許可する形に修正（他の部品に書くとエラー） |
| `verification.md` に結果が無い | この記録 |

## 実行結果

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server build ./...` | 未確認 | 追補の PR の `go-test`（`go test ./...` を実行する）の結果で確認する |
| `go -C apps/server test ./...` | 未確認 | 同上。PR の CI は、`main` と合成した状態で実行されるので、PR #316 の内容も含まれる |
| 金型テスト（`TestScreensAndBoardsMatchGolden`） | 未確認 | 同上 |

この表は、追補の PR の CI の結果が出たあと、結果で更新する。**CI の結果が無い間は、成功と書かない。**
