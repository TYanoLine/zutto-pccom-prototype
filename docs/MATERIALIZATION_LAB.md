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
| `materialization-lab-fresh` | **現在は会話ビューPoC**。RESET相当 → world-selected shell保存 → DBから会話文脈を再構成 → ALLBODYを一連で検証 | `phone`, `situation_mode=facets|facetless`。1ジョブ1回で、`runs`指定には対応しない |

すべて `phone` を省略するとサーバーの `developmentMaterializationPhone` を使う。
worker/randomの `timeout_ms` は既定35000、範囲5000–120000。
パラメータはPOSTでもURLクエリで渡す。fresh以外には `action=list` もある。

freshの `situation_mode` は既定 `facets`。`facetless` はA/B実験専用で、手書きのsituation facet/occurrenceを事前選択せず、routing domain + discourse mode + 会話文脈だけからArticle Workerに小さな出来事を具体化させる。通常runtimeには影響しない。

freshはホスト・人物・ボードを維持し、複製上の記事と遅延人物事実を消してから生成する。ホストや人物の初回生成自体の試験ではない。**現在のfresh専用Repositoryでは `EnableDevelopmentConversationViewPoC()` を有効化し、host-wide semantic Producerを迂回する。** 世界層が決めた投稿者・日時・board・root/reply・source・routing domain・cause kind・discourse modeをshellとしてDBへ保存し、本文生成直前にthread本文、explicit source、同一人物の最近のcanonical投稿、related retrievalをDBから一時的な会話ビューとして再構成する。通常runtimeのmaterializationはこのPoCを自動では有効化せず、既存Producer経路を維持する。

会話ビューPoCの設計意図と正本境界は [CONVERSATION_VIEW_POC.md](CONVERSATION_VIEW_POC.md) を参照。

## 接続条件とテスト環境の公開範囲

- このテスト環境では4種類のlabをトークンなしで利用できる。既存の `MATERIALIZATION_LAB_TOKEN` は認証に使用しない。Render APIトークンも不要。
- 開始は **POSTのみ**。status/listはGETで取得できる。URLのプレビューや巡回によるGETでは生成を開始しない。
- 対象は `0450000196` のみ。別のphoneは400で拒否する。
- 全4種類で同時に1ジョブ、開始間隔60秒、UTC日付で合計20実行まで。複数 `runs` はその回数分を開始時に消費し、失敗しても返却しない。競合は409、間隔・日次上限は429と `Retry-After`。
- 制限は単一サーバープロセス内で共有する。再起動でリセットされ、複数インスタンスを横断しない。課金の厳密な上限ではない。テスト環境は単一インスタンスで運用する。
- 誰でも実験を起動でき、生成本文・架空住人の指示を閲覧できる。第三者によるLLM利用と利用枠消費のリスクを承認したテスト環境専用。実ユーザー・秘密情報を扱う環境へ持ち込まない。
- サーバー環境変数 `MATERIALIZATION_LAB_DISABLED=1` で全labを停止（503）。保存済み世界を操作するRESET APIの認証は変更していない。
- `GET /health` の `materialization_lab` は有効状態、`materialization_lab_auth: "none-test-only"` は本方式の稼働確認に使う。
- 複製元ホストには `Intent.ProducerEventID` を持つ記事が最低1件必要。freshも現在は共通snapshot関数で複製元を取得してから記事を消すため、この前提は残る。これはfresh内でProducer briefを新規生成することを意味しない。

## freshの実行例

以下は `ZUTTO_SERVER_URL`（GoサーバーのURL、末尾スラッシュなし）を設定済みとする。認証情報の設定は不要。

```bash
curl --fail-with-body -sS -X POST \
  "${ZUTTO_SERVER_URL}/api/debug/materialization-lab-fresh?action=start"
```

応答の `id` を `ZUTTO_LAB_JOB_ID` に保持し、同じIFで状態を取得する。

```bash
curl --fail-with-body -sS \
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
- **現在の会話ビューPoCでは** `producer_episode`、`producer_referents`、`producer_actor_knowledge`、`producer_audience_context`、`producer_contribution`、`producer_must_not` は空であることが正常。Producer briefの整合性ではなく、world-selected shellに反していないか、返信が実際のthread/source本文を自然に受けているか、独立rootが別rootを勝手に共有文脈として扱っていないか、同一人物の発言が継続しているかを確認する。これらのProducer fieldは旧方式との比較用にレスポンス形状へ残している。

本文が埋まっていても、他記事の言い換え反復、住人の知識範囲逸脱、ボード違い、未来知識、根拠のない固有名詞や出来事の追加があれば品質上の失敗として記録する。
判断基準は [LLM_POLICY.md](LLM_POLICY.md)、[HISTORICAL_ACCURACY.md](HISTORICAL_ACCURACY.md)、[WORLD_WINDOW_PRODUCER.md](WORLD_WINDOW_PRODUCER.md)、[CONVERSATION_VIEW_POC.md](CONVERSATION_VIEW_POC.md) を参照。

worker/randomは `results` と `summary` から成功率、失敗分類、時間分布を読む。
randomは `run_orders` と依存生成の情報も返す。同じseedはアクセス順の比較に使えるが、LLM出力の一致を保証しない。
allbodyは各runの `complete`、`runtime_state`、`failures`、`missing_post_ids` を確認する。

## エージェントの反復検証手順

1. 最新main・関連仕様・対象実装を読む。接続先の稼働ビルドが検証対象のコミットを含むかを確認する。
2. 目的に合うlabを選び、最小限の回数で基準結果を取得する。
3. ジョブid、対象コミット/稼働ビルド、世界日付、モデル、phone、パラメータ、結果JSONと問題記事を記録する。取得できない情報は未確認と明記する。
4. 検証対象の生成経路に応じて切り分ける。Producer replayではProducer指示とWorker本文を照合し、現在のfresh会話ビューPoCではworld shell・explicit source・thread本文と生成記事の整合性を照合する。
5. ブランチで修正し、重要な挙動のテストを行う。変更を実行環境へ反映したことを確認してから同条件で再検証する。
6. 実測結果と未確認事項をPRへ残す。コード確認・ユニットテスト・稼働環境のlab検証は区別して報告する。

## 現在の制約

- ジョブと結果はプロセスメモリ内にあり、再起動・再デプロイで失われる。必要な結果は終了後に保存する。
- 全HTTP labは共通の排他ゲートを使う。端末からの通常ALLBODYや複数サーバープロセスはこのゲートの対象外。
- freshは約10分、allbodyは各run約8分の待機上限を持つ。超過するとジョブをfailedにし、処理が残存する可能性があるため全labの新規開始を再起動まで503で停止する。自動で後続runを開始しない。
- このIFは生成パイプラインを検証する。Canvas表示、WebSocket、モデム、実際の端末入力のE2E検証は別途必要。

## 実装への入口

- [ルート登録](../apps/server/cmd/server/main.go)
- [認証・worker replay・snapshot](../apps/server/cmd/server/materialization_lab.go)
- [random replay](../apps/server/cmd/server/materialization_lab_random.go)
- [ALLBODY runtime replay](../apps/server/cmd/server/materialization_lab_allbody.go)
- [fresh生成・記事/因果メタデータ取得](../apps/server/cmd/server/materialization_lab_fresh.go)
- [会話ビューPoC](../apps/server/internal/worldrepo/materialization_conversation_view.go)

## Vercel経由で呼び出す場合

フロントエンドの同名IFは `/api/materialization-lab-fresh`（`debug/` なし）。他3種類も同様。Vercel側の既存プロキシはPOSTとクエリを固定のGoサーバーへ転送するため、トークンなしでそのまま利用できる。開始はPOST、結果はGETとし、statusのidは同じ種類のIFに渡す。
