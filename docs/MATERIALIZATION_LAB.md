# タイトル候補先行の実験（title-first）

```text
POST /api/debug/materialization-lab-fresh?action=start&situation_mode=title-first&board_count=4&shell_limit=8
GET /api/debug/materialization-lab-fresh?action=status&id=<id>
```

Vercelでは `/api/materialization-lab-fresh`。完了後は `/poc/materialization-lab-viewer?job=<id>`。
選択した板の「タイトル候補の比較」で原文20候補・採用後・発言者・採否理由を比較する。

- 最初の生成には「<世界日付>のパソコン通信botを再現します。以下条件の掲示板における記事タイトル候補を20個作ってください。掲示板名『<板名>』具体的な固有名詞を含めても良いです。」のみを使用する。人物・投稿理由・件名の文体指示を混ぜない。1995年固定ではなく実際の世界日付を使う。
- 独立rootの投稿枠がある板ごとに、人物・投稿理由・canonical situationを見せずに20候補を自由生成する。タイトル候補はまだ世界事実ではなく、具体的な固有名詞や個人経験を含む「世界エンジンへの提案」として扱う。
- Era Validatorで時代上の成立可否を先に振り分ける。`research` は候補のまま保持し、人物枠へ仮採用された候補だけWeb史料確認する。`ng` / `unverified` は世界へ採用しない。
- 後段でモデルが既存の人物・日時・発言目的との整合を提案し、世界側がID・重複・必須項目・Era結果を検査して採用する。既存PersonaFactsと明確に矛盾する候補は採用しないが、既存Factsにないという理由だけで個人経験を一律拒否しない。
- 採用が確定した時点で、タイトルとreview summaryがその投稿の `title_first` canonical world eventになる。summaryはタイトルから直接読み取れる最小限の出来事だけを正本化し、タイトルにない機種・場所・原因・購入経路・進捗等は追加しない。本文workerはこの採用済みeventと既存Persona/BBS factsの範囲だけを文章化する。
- title/persona/Era採用後にだけ専用Article Detail Materializerを実行し、採用記事ごとに2〜4件の `article_detail=<kind>:<fact>` を正本化する。detailはタイトル/summaryの言い換えや『読者に尋ねる』等の編集指示を禁止し、locator/timing/sequence/comparison/observation/question_scope/decision/reaction_contextのうち2種類以上で、本文を具体化する記事ローカル情報を固定する。20候補すべてにdetailを作らない。
- replyではrootのdetailを `source_article_detail` 等の `source_` namespaceへ移し、source authorの事実として扱う。返信者自身の購入・利用・開始・訪問・発見等へ一人称で継承してはならない。
- 無矛盾の候補は原文保持。具体的矛盾・長さの補正のみ理由付きで認める。採用済みタイトルは本文workerで再生成しない。返信は従来の親記事に基づく件名処理。通常世界への書き戻しなし。
- `title_candidates` に原文を含めてアーカイブする。理由空欄の判定はその候補だけ不採用。板のreview失敗はunreviewedとして残し、他の板を続行する。生成の再試行で候補を勝手に作り直さない。既存の開始制限・排他を共用。

# Materialization Lab — 生成の反復検証IF

## 対象を先に確定する件名検証（topic-first）

次のPOSTで、記事の対象を先に史料で確認・選択し、その後にSituationと件名・本文を生成する。通常世界は変更せず、既存の開始制限・排他・アーカイブを使用する。

```text
POST /api/debug/materialization-lab-fresh?action=start&situation_mode=topic-first&historical_texture=search-grounded&board_count=6&shell_limit=8
GET /api/debug/materialization-lab-fresh?action=status&id=<開始応答のid>
```

Vercel経由では `/api/materialization-lab-fresh`。完了後は `/poc/materialization-lab-viewer?job=<id>` で閲覧する。「内部Situationを表示」で選定対象・件名保持・用件・検索根拠を確認できる。

- `topic-first` は `historical_texture=search-grounded` のみ対応。他の組合せは開始枠を消費する前に400。
- 投稿の存在・人物・日時・board・root/reply・discourse modeは従来どおり世界側で決定する。
- games/music/softwareの独立rootについて、板・領域ごとに最大6プール、同時3検索。各プールの最も早い記事日付を検索上限とする。候補の順位、既存の重複抑制と安定ハッシュで世界側が名前を選び、それをSituation入力へ渡す。
- 検索済み名称は今回の架空の関与・感想・相談の対象であって、所有や購入の証明ではない。史料が未供給のゲーム仕様・攻略・価格・発売予定は補完しない。
- 一般雑談は無理に命名しない。検索失敗・候補なし・予算超過は `topic_target_status` に明示され、匿名のままの結果を具体化成功とは扱わない。
- 各記事の `topic_target`、`topic_target_status`、`subject_target_present` を追加。用件と根拠は `situation_summary` / `situation_facts` に保存。旧アーカイブの欠損フィールドは未計測。
- 選定対象がSituationから落ちた場合は既存の一回の修復対象。本文workerが対象を件名から消した場合は生成失敗として未確定のまま残す。名前を後付けして成功に見せない。
- `completed` だけで合格とはしない。件数、空本文、失敗数と対象保持を確認した上で、自然さ・同一作品の別用件・年代と版の整合を目視する。

このモードは出典検索の外部実行時間を含むため、規模の大きい実験は既存の15分上限が適用される。モデルAPI使用量の表示には検索基盤側の全コストが含まれるとは限らない。

開発者・AIエージェントがHTTP経由で実際の生成処理を起動し、結果を比較するための開発専用IF。端末操作を人に繰り返してもらわず、生成 → 結果照合 → 修正 → 再生成を行う。

各labは既存の開発ホストを独立したMemoryStoreへ複製し、その中で生成する。実験の記事・人物事実を保存済みデモ世界へ書き戻さない。OpenAIの認証情報はサーバー側に保持され、実際のproviderを使うため実行にはLLM利用が発生し得る。世界の正本や通常の世界進行スケジューラとして使わない。

## IFの使い分け

パスの共通prefixは `/api/debug/`。

| パス | 検証対象 | 主な開始パラメータ |
| --- | --- | --- |
| `materialization-lab` | 既存Producer記事の本文を消し、時系列順にArticle Workerを再実行 | `suite=worker-replay`、`runs`（1–5、既定1）、`post_ids`（カンマ区切り、最大15件）、`timeout_ms` |
| `materialization-lab-random` | 既存記事をランダム順に読み、依存記事の生成や順序の影響を検証 | `runs`（1–8、既定3）、`seed`（既定19660826）、`timeout_ms` |
| `materialization-lab-allbody` | 既存Producer記事を使い、端末と同じmaterializationdemo RuntimeのALLBODY処理を検証 | `runs`（1–5、既定3） |
| `materialization-lab-fresh` | **現在は会話ビューPoC**。RESET相当 → world-selected shell保存 → DBから会話文脈を再構成 → ALLBODYを一連で検証 | `phone`, `situation_mode=facets|facetless|batch`, `historical_texture=sourced|off|1996-08-curated`, `board_count=3..6`, `shell_limit=1..10`。1ジョブ1回で、`runs`指定には対応しない |

すべて `phone` を省略するとサーバーの `developmentMaterializationPhone` を使う。
worker/randomの `timeout_ms` は既定35000、範囲5000–120000。
パラメータはPOSTでもURLクエリで渡す。fresh以外には `action=list` もある。

freshの `situation_mode` は既定 `facets`。`facetless` はA/B実験専用で、手書きのsituation facet/occurrenceを事前選択せず、routing domain + discourse mode + 会話文脈だけからArticle Workerに小さな出来事を具体化させる。通常runtimeには影響しない。

freshはホスト・人物・ボードを維持し、複製上の記事と遅延人物事実を消してから生成する。ホストや人物の初回生成自体の試験ではない。**現在のfresh専用Repositoryでは `EnableDevelopmentConversationViewPoC()` を有効化し、host-wide semantic Producerを迂回する。** 世界層が決めた投稿者・日時・board・root/reply・source・routing domain・cause kind・discourse modeをshellとしてDBへ保存し、本文生成直前にthread本文、explicit source、同一人物の最近のcanonical投稿、related retrievalをDBから一時的な会話ビューとして再構成する。通常runtimeでも開発ホスト `0450000196` は Conversation View + title-first を有効化する。Web端末から `ATDT0450000196` で接続し、`B` で未生成Envelopeを計画、記事番号を開いてArticle Detail込み本文を遅延生成できる。傾向確認用の通常runtimeだけは、板を6種（フリートーク／パソコン通信・モデム／地域の話題／ゲーム／音楽／ソフトウェア）に広げ、過去28日の活動から1板最大12 Envelopeを選ぶ。fresh Labの `board_count` / `shell_limit` と14日活動窓は比較条件として従来どおり維持する。`ALLBODY` では全本文を一括生成でき、`RESET` 後はtitle-first候補・割当も新しいplanning passへ再初期化する。ほかのホストプログラムにはこの開発専用経路を適用しない。

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


### fresh batch Situation / scale experiment

`situation_mode=batch` removes the hand-written situation facet/occurrence selection without moving concrete world truth into article prose. All independent roots in the bounded fresh window are proposed together, validated by the world layer, and accepted Situation fields are persisted before ALLBODY prose rendering. A validator rejection triggers at most one batched repair call for only the rejected roots.

`board_count` and `shell_limit` are fresh-isolated load-test controls. Defaults remain 3 boards and 5 shells per board. `board_count>3` adds development-only `ゲーム`, `音楽`, `ソフトウェア` boards to the copied MemoryStore snapshot. Nothing from these scaled runs is written back to the saved development world or ordinary runtime.

## Read-only generated BBS viewer

Completed `materialization-lab-fresh` jobs are archived separately from the canonical BBS world when `DATABASE_URL` is configured. The archive stores the completed job JSON (including generated article bodies) in `development_materialization_fresh_archives`; it is **development experiment evidence**, not world state, and is never restored into the normal runtime.

Read-only API:

- `GET /api/debug/materialization-lab-fresh-view` — newest archived completed fresh job
- `GET /api/debug/materialization-lab-fresh-view?id=<job-id>` — one archived job
- `GET /api/debug/materialization-lab-fresh-view?list=1&limit=20` — summaries only
- non-GET methods return `405`; this handler never starts generation, RESETs state, observes boards, or invokes an LLM.

Vercel exposes a strict GET-only proxy at `/api/materialization-lab-viewer` and an evaluator UI at `/poc/materialization-lab-viewer`. The UI defaults to normal BBS reading (board → thread → article). World/Situation diagnostics are hidden unless the evaluator explicitly enables them.



### Historical Texture A/B

`historical_texture=1996-08-curated` is a fresh-Lab-only experiment. It does not change ordinary runtime or the service-wide `HISTORICAL_REFERENCES_ENABLED` setting. The isolated Lab clones the LLM materializer, supplies a small curated set of contemporary real referents, and allows the batch Situation proposer/article worker to use those names only when they naturally sharpen an already-selected event. The texture is permission/background, not a topic quota. `off` preserves the previous generic-name behavior. Completed jobs archive the texture label so the read-only viewer can compare runs. See `docs/research/HISTORICAL_TEXTURE_POC.md`.

### 出典付き名称の既定モード

freshのhistorical_texture省略時は `sourced`。通常配線と共通の出典・日付付き名称claimを使用する。`off` は複製materializerの出典付き供給・追加texture・広い歴史参照を明示的に無効化する。旧 `1996-08-curated` はfixture専用として残す。situation_modeの既定は引き続きfacetsなので、状況の具体化を比較する際はbatchを明示する。Web proxyも4つの実験パラメータを転送する。詳細は [固有名詞不足の分析](research/PERIOD_REFERENTS.md)。

