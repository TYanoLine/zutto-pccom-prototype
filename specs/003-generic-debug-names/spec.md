# Spec: 共通コードから HAKATA を含む名前を無くす（環境変数・識別子・文言）

**Spec ID**: 003-generic-debug-names
**Status**: Draft
**Depends on**: PR #294（局ごとのフラグ）、spec 002（Web 側の capabilities）。いずれもマージ済み

## 背景

HAKATA 固有だった判定は、PR #294 と spec 002 で、局定義（preset YAML）のフラグ
（`debug.*`、`generation.*`）に置き換わった。挙動は局のフラグで決まるようになったが、
共通コードには「HAKATA」を含む名前が残っている。

- 環境変数: `DEBUG_LOG_HAKATA_GENERATED`、`DEBUG_HAKATA_LLM_TRACE`、`HAKATA_FREEFORM_BODY`
- Go の識別子: `DebugLogHAKATAGenerated`、`DebugHakataLLMTrace`、`HakataFreeformBody`、
  `SetHAKATAFreeformBody`、`useHAKATAFreeformBody`、`shouldLogHAKATAGenerated`、`newHakataTraceHandler` など
- ファイル名: `worldrepo/hakata_freeform_body.go`（とテスト）
- エラー文言、ログ文言、コメント

名前が実態（局のフラグ + プロセス全体のスイッチ）と合っておらず、読む人が「HAKATA 専用機能」と誤解する。
**挙動を変えずに**、名前だけを中立にする。

## 要件

- **FR-001（環境変数の新名）**: 次の新しい名前を正式な名前にする。

  | 旧（非推奨） | 新 | 対応する局フラグ |
  |---|---|---|
  | `DEBUG_LOG_HAKATA_GENERATED` | `DEBUG_LOG_GENERATED_CONTENT` | `debug.content_log` |
  | `DEBUG_HAKATA_LLM_TRACE` | `DEBUG_GENERATION_TRACE` | `debug.generation_trace` |
  | `HAKATA_FREEFORM_BODY` | `GENERATION_FREEFORM_BODY` | `generation.freeform_body` |

- **FR-002（旧名の互換）**: 旧名も引き続き読む。**本番（Render）には旧名が設定されている可能性が高く、
  旧名を読まなくなると挙動が変わるため、この要件は必須。**
  - 新名が空でない値で設定されていれば、新名を使う（旧名は無視する）。
  - 新名が未設定（または空）で、旧名が空でない値で設定されていれば、旧名を使う。
  - 両方とも未設定（または空）なら、既定値を使う。
  - 空文字列は「未設定」として扱う（現在の `envBool` の挙動に合わせる）。
- **FR-003（非推奨の通知）**: 旧名が実際に使われたとき、プロセス起動時に 1 回だけ
  `config: DEBUG_HAKATA_LLM_TRACE is deprecated; use DEBUG_GENERATION_TRACE` の形式でログに出す。
  新名が設定されているときは出さない。
- **FR-004（既定値は変えない）**: 3 つとも既定値は現状どおり `true`。
- **FR-005（Go の識別子とファイル名）**: `plan.md` の改名表のとおりに改名する。
- **FR-006（文言）**: 共通コードのエラー文言・ログ文言から「HAKATA」を除く。
  HTTP の応答コード、エンドポイントのパス（`/api/debug/bbs/generation-trace`）、JSON の形は変えない。
- **FR-007（コメント）**: 共通コードのコメントのうち、HAKATA 専用であるかのように書かれているものを、
  中立な表現にする（局のフラグ・実験用であることが伝わる書き方にする）。
- **FR-008（挙動不変）**: 判定のロジック、既定値、ゲートの条件（プロセス全体のスイッチ AND 局のフラグ）を変えない。
- **FR-009（ドキュメント）**: `.env.example`、`docs/DEBUG_INSPECTION.md`、
  `apps/server/internal/hostcatalog/README.md`、そのほか旧名を書いているファイルを新名に更新する。
  旧名は「非推奨の別名」として 1 か所に記載する。

## 成功基準

- **SC-001**: `go -C apps/server test ./...` が通る。
- **SC-002**: 旧名のみを設定した場合と新名のみを設定した場合で、`config.Load()` の結果が同じになるテストがある。
  新名が旧名に優先するテストがある。
- **SC-003**: `plan.md` の「残してよい HAKATA の記述」以外で、`apps/server` の Go ファイル（テストを除く）に
  `hakata`（大文字小文字を区別しない）が残っていない。
- **SC-004**: 旧名の環境変数でも、新名の環境変数でも、サーバが従来どおり起動する。

## スコープ外（変更しない）

- 住民生成（`world/hakata_cast.go`、`EnsureHakataExperimentPopulation`、
  `ensureHakataExperimentPopulationLocked`、関連する ID `hakata-member-NNN`、ログ「HAKATA membership population restored」）。
  preset の `population:` への外部化で扱う。
- Erika-K の板構成と Welcome 文言（`hostprogram/erikak/runtime.go`）。
- preset ファイル名、局の key、テストの fixture（`hakata-canal-net`、`HAKATA CANAL NET`）。実在する局のデータであり、改名しない。
- Web 側（`apps/web`）。spec 002 で対応済み。
- 環境変数の既定値の変更、旧名の削除（互換期間の終了は別途判断する）。
- 局のフラグ名（`debug.*`、`generation.*`）。
