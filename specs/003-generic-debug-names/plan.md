# Plan: generic debug names

**Spec**: `specs/003-generic-debug-names/spec.md`

## 方針

- **名前だけを変える**。ロジック、既定値、HTTP の応答は変えない。
- 環境変数は新名を正式な名前にし、旧名を非推奨の別名として読み続ける（互換のため）。
- 改名は Go のコンパイラに任せる。名前を変えたあと `go build ./...` と `go vet ./...` の
  エラーを順に直せば、使用箇所の取りこぼしがない。
- 既存ファイルに整形だけの差分を作らない。`worldrepo/repository.go` など、`gofmt` 済みでない
  ファイルがある。**`gofmt -w` を既存ファイル全体にかけない**。変更した行だけを手で整える。

## 改名表

### 設定（`apps/server/internal/config/config.go`）

| 旧 | 新 |
|---|---|
| フィールド `DebugLogHAKATAGenerated` | `DebugLogGeneratedContent` |
| フィールド `DebugHakataLLMTrace` | `DebugGenerationTrace` |
| フィールド `HakataFreeformBody` | `GenerationFreeformBody` |

### `worldrepo`

| 旧 | 新 |
|---|---|
| ファイル `hakata_freeform_body.go` | `freeform_body.go`（`git mv` で移す） |
| ファイル `hakata_freeform_body_test.go` | `freeform_body_test.go`（`git mv` で移す） |
| `(*Repository).SetHAKATAFreeformBody` | `SetFreeformBody` |
| `(*Repository).useHAKATAFreeformBody` | `useFreeformBody` |
| フィールド `hakataFreeformBody` | `freeformBody` |
| `(*Repository).SetDebugLogHAKATAGenerated` | `SetDebugLogGeneratedContent` |
| `(*Repository).shouldLogHAKATAGenerated` | `shouldLogGeneratedContent` |
| フィールド `debugLogHAKATAGenerated` | `debugLogGeneratedContent` |

### `cmd/server`

| 旧 | 新 |
|---|---|
| `newHakataTraceHandler` | `newGenerationTraceHandler` |
| エラー文言 `HAKATA generation trace is disabled` | `generation trace is disabled` |
| `main.go` の呼び出し | 上の新名に合わせる（`cfg.DebugGenerationTrace` など） |

### テスト関数名

| 旧 | 新 |
|---|---|
| `TestHAKATAGeneratedContentLoggingDefaultsOn`（config） | `TestGeneratedContentLoggingDefaultsOn` |
| `TestHakataPromptTraceFlagDefaultsOnAndCanBeDisabled`（config） | `TestGenerationTraceFlagDefaultsOnAndCanBeDisabled` |
| `TestGenerationTraceRequiresOptInAndHAKATA`（worldrepo） | `TestGenerationTraceRequiresBothSwitches` |
| `TestHAKATAAuditLogsOnlyNewCommittedWorldHeaders`（worldrepo） | `TestGeneratedContentAuditLogsOnlyNewCommittedWorldHeaders` |

上に無い `HAKATA` を含むテスト関数名やテスト内のメッセージがあれば、同じ方針で中立な名前にする。
テストの fixture（`hakata-canal-net`、`HAKATA CANAL NET` などの実在する局のデータ）は変えない。

## コメントと文言を中立にする箇所

次のファイルのコメント・メッセージで、HAKATA 専用であるかのように書かれている部分を直す。
「局のフラグと、プロセス全体のスイッチで有効になる評価用の機能」という意味が伝わる書き方にする。

| ファイル | 対象 |
|---|---|
| `llm/debug_trace.go` | `HAKATA evaluation operation` |
| `llm/board_post_freeform.go` | `temporary HAKATA evaluation prompt` |
| `worldrepo/bbs_article_body.go` | `HAKATA title-led experiment`、`HAKATA's temporary model-memory body trial` |
| `worldrepo/observation.go` | `old HAKATA-only fixed 40-root evaluation batch` |
| `bbsengine/engine.go` | `HAKATA's dedicated SYSOP exists as station configuration` |
| `cmd/server/generation_trace_handler.go` | `fictional HAKATA evaluation deployment` |
| `worldrepo/bbs_content_log.go`、`freeform_body.go` | 「The HAKATA-specific name is historical」の注記（改名後は不要なので削除） |
| `world/preset_hosts.go` | `the HAKATA-specific code` |

## 残してよい HAKATA の記述（SC-003 の許可リスト）

テスト以外の Go ファイルで、次の場所は残してよい。

| ファイル | 理由 |
|---|---|
| `world/hakata_cast.go`、`world/store.go`、`world/host_role.go` | 住民生成。preset の `population:` へ外部化するまで残す |
| `cmd/server/runtime_store.go` | `EnsureHakataExperimentPopulation` の呼び出しとログ文言（同上） |
| `hostprogram/erikak/runtime.go` | 板構成と Welcome の局データ（外部化するまで残す） |
| `hostcatalog/descriptor.go`、`hostcatalog/preset.go` | key の例示（`hakata-canal-net`）をコメントに書いているだけ |

これ以外に残った場合は、直すか、直せない理由を報告する。

## 環境変数の別名の読み方

`config.go` の既存のヘルパー（`envBool`）の挙動（空文字列は未設定として既定値）に合わせる。
次の形のヘルパーを追加する。実装は `config.go` を読んでから、既存のスタイルに合わせる。

```go
// envBoolWithLegacy reads name; if it is unset (or empty) it falls back to the
// deprecated legacy name, and finally to def. Using the legacy name is logged once.
func envBoolWithLegacy(name, legacy string, def bool) bool {
	if os.Getenv(name) != "" {
		return envBool(name, def)
	}
	if os.Getenv(legacy) != "" {
		log.Printf("config: %s is deprecated; use %s", legacy, name)
		return envBool(legacy, def)
	}
	return def
}
```

- `log` は標準ライブラリを使う。
- 「1 回だけ」は、`config.Load()` がプロセス起動時に 1 回だけ呼ばれる前提で満たされる。
  `Load()` がテスト以外でも複数回呼ばれているなら、`sync.Once` などで重複を避ける。
- 値の解釈（`1`、`true`、`0` など）は既存の `envBool` に任せる。新しい解釈を作らない。

## 検証方針

- 自動: `go -C apps/server test ./...`、`go -C apps/server vet ./...`。
- 残存確認: 次のコマンドの結果が、許可リストだけになること。

  ```bash
  grep -rliE "hakata" apps/server --include=*.go | grep -v _test.go
  ```

- 環境変数の確認: 旧名・新名・両方・どちらも無しの 4 通りが、`config_test.go` のテストで確かめられていること。
- 手動確認（起動など）は、環境がなければ「未実施」と記録する。実行していないものを成功と書かない。
