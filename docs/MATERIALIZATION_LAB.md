# Materialization Lab — 生成の反復検証IF

開発者・AIエージェントがHTTP経由で実際の生成処理を起動し、結果を比較するための開発専用IF。端末操作を人に繰り返してもらわず、生成 → 結果照合 → 修正 → 再生成を行う。

各labは既存の開発ホストを独立したMemoryStoreへ複製し、その中で生成する。実験の記事・人物事実を保存済みデモ世界へ書き戻さない。OpenAIの認証情報はサーバー側に保持され、実際のproviderを使うため実行にはLLM利用が発生し得る。世界の正本や通常の世界進行スケジューラとして使わない。

## IFの使い分け

パスの共通prefixは `/api/debug/`。

| パス | 検証対象 | 主な開始パラメータ |
| --- | --- | --- |
| `materialization-lab` | 既存Producer記事の本文を消し、時系列順にArticle Workerを再実行 | `suite=worker-replay`、`runs`（1–5、既定1）、`post_ids`（カンマ区切り、最大15件）、`timeout_ms` |
| `materialization-lab-random` | 既存記事をランダム順に読み、依存記事の生成や順序の影響を検証 | `runs`（1–8、既定3）、`seed`（既定19660826）、`timeout_ms` |
| `materialization-lab-allbody` | 既存Producer記事を使い、端末と同じmaterializationdemo RuntimeのALLBODY処理を検証 | `runs`（1–5、既定3） |
| `materialization-lab-fresh` | RESET相当 → World Window Producer → ALLBODYの一連の生成を検証 | `phone`。1ジョブ1回で、`runs`指定には対応しない |

すべて `phone` を省略するとサーバーの `developmentMaterializationPhone` を使う。
worker/randomの `timeout_ms` は既定35000、範囲5000–120000。
パラメータはPOSTでもURLクエリで渡す。fresh以外には `action=list` もある。

freshはホスト・人物・ボードを維持し、複製上の記事と遅延人物事実を消してから生成する。ホストや人物の初回生成自体の試験ではない。本文品質とProducer指示の整合性を見る場合はfreshを使う。

## 接続条件

- サーバー環境変数 `MATERIALIZATION_LAB_TOKEN` が設定されていること。未設定または認証不一致なら403。
- リクエストに `X-Zutto-Lab-Token` ヘッダーを付ける。`token` クエリも実装上は受け付けるが、共有URLやログに値を残さないためヘッダーを使う。
- `GET /health` の `materialization_lab: true` はトークン設定の有無を示すだけで、認証・生成成功の確認にはならない。
- 複製元ホストには `Intent.ProducerEventID` を持つ記事が最低1件必要。**freshも共通snapshot関数を使うためこの条件がある。** 空の複製元では `no matching producer posts in current canonical snapshot` で失敗する。

## freshの実行例

以下は利用者側のshellに `ZUTTO_SERVER_URL`（末尾スラッシュなし）と `ZUTTO_LAB_TOKEN` を設定済みとする。実際の秘密値はドキュメント・PR・検証記録に含めない。

```bash
curl --fail-with-body -sS -X POST \
  -H "X-Zutto-Lab-Token: ${ZUTTO_LAB_TOKEN}" \
  "${ZUTTO_SERVER_URL}/api/debug/materialization-lab-fresh?action=start"
```

応答の `id` を `ZUTTO_LAB_JOB_ID` に保持し、同じIFで状態を取得する。

```bash
curl --fail-with-body -sS \
  -H "X-Zutto-Lab-Token: ${ZUTTO_LAB_TOKEN}" \
  "${ZUTTO_SERVER_URL}/api/debug/materialization-lab-fresh?action=status&id=${ZUTTO_LAB_JOB_ID}"
```

開始は非同期で、通常 `queued` → `running` → `completed` または `failed`。
数秒間隔を目安にstatusを読み、終了後に次の実験を開始する。
statusの `id` 省略時は実行中または最新ジョブ、履歴がなければ `idle`。
不明なidは404。開始時の競合が検出された場合は409。

## 結果の読み方

**ジョブの `status=completed` は検証処理の終了であり、生成成功の保証ではない。**

freshでは次を照合する。

- `runtime_state` が `COMPLETED` か。
- `failures`、`empty_post_ids`、`post_count` と `body_count` に欠落がないか。0件同士の一致だけで品質検証成功としない。
- `planning_diagnostic` と `status_text` に異常がないか。
- `duration_ms` と `usage` による実行時間・利用量。
- `articles` に含まれる件名・本文・投稿者・日時・board/parentと、`source_post_id` / `responds_to_post_id` による返信・因果関係。
- 各記事の `producer_episode`、`producer_referents`、`producer_actor_knowledge`、`producer_audience_context`、`producer_contribution`、`producer_must_not` と本文の整合性。

本文が埋まっていても、他記事の言い換え反復、住人の知識範囲逸脱、ボード違い、未来知識、根拠のない固有名詞や出来事の追加があれば品質上の失敗として記録する。
判断基準は [LLM_POLICY.md](LLM_POLICY.md)、[HISTORICAL_ACCURACY.md](HISTORICAL_ACCURACY.md)、[WORLD_WINDOW_PRODUCER.md](WORLD_WINDOW_PRODUCER.md) を参照。

worker/randomは `results` と `summary` から成功率、失敗分類、時間分布を読む。
randomは `run_orders` と依存生成の情報も返す。同じseedはアクセス順の比較に使えるが、LLM出力の一致を保証しない。
allbodyは各runの `complete`、`runtime_state`、`failures`、`missing_post_ids` を確認する。

## エージェントの反復検証手順

1. 最新main・関連仕様・対象実装を読む。接続先の稼働ビルドが検証対象のコミットを含むかを確認する。
2. 目的に合うlabを選び、最小限の回数で基準結果を取得する。
3. ジョブid、対象コミット/稼働ビルド、世界日付、モデル、phone、パラメータ、結果JSONと問題記事を記録する。取得できない情報は未確認と明記する。
4. Producerの指示とWorkerの本文を照合し、どの段階で矛盾や重複が生じたかを切り分ける。
5. ブランチで修正し、重要な挙動のテストを行う。変更を実行環境へ反映したことを確認してから同条件で再検証する。
6. 実測結果と未確認事項をPRへ残す。コード確認・ユニットテスト・稼働環境のlab検証は区別して報告する。

## 現在の制約

- ジョブと結果はプロセスメモリ内にあり、再起動・再デプロイで失われる。必要な結果は終了後に保存する。
- lab種別間の排他確認は対称ではなく、全IFを横断する完全な同時実行防止を保証しない。比較実験はクライアント側でも直列に行う。
- freshは約10分、allbodyは各run約8分の待機上限を持つ。この上限は処理のキャンセル保証ではない。非終端runtime状態でlabが終了した場合は成功扱いせず、別ジョブを重ねる前に稼働状態を確認する。
- このIFは生成パイプラインを検証する。Canvas表示、WebSocket、モデム、実際の端末入力のE2E検証は別途必要。

## 実装への入口

- [ルート登録](../apps/server/cmd/server/main.go)
- [認証・worker replay・snapshot](../apps/server/cmd/server/materialization_lab.go)
- [random replay](../apps/server/cmd/server/materialization_lab_random.go)
- [ALLBODY runtime replay](../apps/server/cmd/server/materialization_lab_allbody.go)
- [fresh生成・記事とProducer指示の取得](../apps/server/cmd/server/materialization_lab_fresh.go)
