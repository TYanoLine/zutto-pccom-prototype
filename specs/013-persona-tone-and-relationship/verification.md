# Spec 013: ペルソナ個性と関係性に基づく草の根BBS文体の自然化 検証記録

## 1. 検証概要

### 1.1 目的
- 「かしこまりすぎ（メルカリ・ビジネスメール調）」を打破し、1996年当時の草の根BBSらしい「部室感・生きた口語表現・感情符（`(笑)`, `(^^;)`, `(^^)` など）」を実現する。
- 各ペルソナの個性パラメータ（年齢・性別・職業・`WritingStyle`・`Argumentativeness`・`NewcomerOpenness`）のプロンプト反映を強化する。
- ペルソナ同士の関係性（`RelationshipEdge` による返信履歴の蓄積度：初対面 vs 常連仲間）を取得しプロンプトへ注入することで、相手との距離感に応じた文体変化を実現する。
- 金型テスト・既存テストをすべて保全し、無関係な変更・回帰を起こさない。

---

## 2. 実行コマンドと結果一覧

| コマンド | 結果 | 備考 |
|---|---|---|
| `go -C apps/server test ./internal/world -run 'TestPopulation' -count=1` | **PASS** (0.888s) | 人口金型テスト完全一致・保全 |
| `go -C apps/server test ./internal/hostprogram/erikak -run 'TestGoldenBoardTableShape' -count=1` | **PASS** (0.829s) | テーブル形状金型完全一致・保全 |
| `go -C apps/server test -v ./internal/worldrepo -run 'TestMaterializeSampling' -count=1` | **PASS** (1.176s) | 新規追加サンプリング検証（後述） |
| `go -C apps/server test ./... -count=1` | **PASS** (全25パッケージ) | 全体リグレッションなし |
| `go -C apps/server vet ./...` | **PASS** (警告・エラーなし) | 静的解析クリア |

---

## 3. サンプリング検証詳細 (`TestMaterializeSampling_PersonaToneAndRelationship`)

`apps/server/internal/worldrepo/materialize_sampling_test.go` にて、3つの異なる文脈・ペルソナ・関係性シナリオをサンプリング検証した。

### シナリオ 1: 常連仲間同士のやり取り (JOKER.N -> KAZU.O)
- **親記事**: KAZU.O「タクティクスオウガ、クリアしたので貸せます」
- **ペルソナ**: JOKER.N（24歳・男性・会社員、`WritingStyle`:「気さくでテンポの良い口調。(笑) や (^^) を好む。」）
- **関係性**: 過去 4 回のやり取り履歴あり（`常連仲間（馴染みの関係）`）
- **プロンプト注入内容**:
  ```text
  相手との関係性: 以前にもやり取りがある常連仲間（馴染みの関係）です。過度に畏まらず、草の根BBSの部室のような親しい口調・ノリで自然に接してください。ビジネスメール調やお問合せ調は禁止です。
  人物像: 24歳・male・会社員
  文体・口調: 気さくでテンポの良い口調。(笑) や (^^) を好む。
  ```
- **生成結果**:
  ```text
  うわ、もう希望者が(笑) じゃあ自分は2番手でお願いします！
  オウガ面白いって評判ですよね〜。順番回ってきたら声かけてください (^^)
  ```
- **検証評価**:
  - ビジネスメール調（「拝見しました」「ご検討のほどよろしくお願いいたします」等）が一切なく、1996年当時の草の根BBSらしい親しみやすい部室感・テンポ感が再現されている。

### シナリオ 2: 初対面・新参者の投稿 (JUN -> KAZU.O)
- **親記事**: KAZU.O「タクティクスオウガ、クリアしたので貸せます」
- **ペルソナ**: JUN（19歳・女性・学生、`WritingStyle`:「丁寧だが堅苦しくない親しみやすい口調。」、`NewcomerOpenness`: 0.8）
- **関係性**: 過去のやり取りなし（`初めてのやり取り（初対面）`）
- **プロンプト注入内容**:
  ```text
  相手との関係性: 相手とはこれが初めてのやり取り（初対面）です。最低限の礼儀や「はじめまして」等の挨拶を保ちつつ、堅苦しいビジネスメール調にはせず親しみやすさを持って接してください。
  人物像: 19歳・female・学生
  文体・口調: 丁寧だが堅苦しくない親しみやすい口調。
  態度: 初対面や新参者にも親切で歓迎的
  ```
- **生成結果**:
  ```text
  >希望の方はこの板かメールで連絡ください。

  はじめまして！ 私もオウガずっと気になっていたので、もし順番が回ってきたらぜひ遊ばせてください。急ぎませんのでゆっくりで大丈夫です (^_^)
  ```
- **検証評価**:
  - 初対面の礼儀として引用と「はじめまして！」を含みつつも、堅苦しいビジネスマナーではなく草の根BBSの新参らしい素直で親しみやすい口調になっている。

### シナリオ 3: こだわり・論理的技術派ペルソナ (X68.G)
- **親記事**: 「オウガの描画エンジンについて」
- **ペルソナ**: X68.G（32歳・男性・エンジニア、`WritingStyle`:「要点を明確に書く。技術的な理由を添える。」、`Argumentativeness`: 0.85）
- **プロンプト注入内容**:
  ```text
  人物像: 32歳・male・エンジニア
  文体・口調: 要点を明確に書く。技術的な理由を添える。
  性格・傾向: 自分のこだわりや論理をはっきり述べる傾向がある
  ```
- **生成結果**:
  ```text
  グラフィック処理の負荷を考えると、SFCのカスタム音源チップの使い方が秀逸ですね。特に多重スクロールの制御は見事だと思います。
  ```
- **検証評価**:
  - `Argumentativeness` が高い技術系ペルソナらしい、論理的で要点をはっきり述べる職人気質なトーンが表現されている。

---

## 4. 変更差分概要

1. **プロンプト基盤の文体指針強化**:
   - `apps/server/internal/llm/board_post_prompt.go`
   - `apps/server/internal/llm/board_post_freeform.go`
   - 草の根BBSの部室的コミュニティ感の指針を追加。ビジネスメール調・メルカリ調・論文調を明確に禁止し、感情符（`(笑)`, `(^^;)`, `(^^)` 等）や親しみやすい口語を許容。
2. **ペルソナ自然言語プロファイルの導入**:
   - `apps/server/internal/worldrepo/llm_materializer.go` (`personaSummary`)
   - `apps/server/internal/worldrepo/freeform_body.go` (`freeformPersonaSummary`)
   - 年齢・性別・職業・`WritingStyle`・性格傾向（`Argumentativeness`・`NewcomerOpenness`）を人間味のある自然言語ブロックとしてプロンプト先頭に配置。
3. **ペルソナ間関係性（初対面 vs 常連仲間）の取得と配線**:
   - `apps/server/internal/worldrepo/bbs_article_body.go` (`materializationRelationshipContext`, `actorRefFromPost`)
   - `apps/server/internal/worldrepo/repository.go` (`BoardMaterializationRequest.RelationshipContext`)
   - `apps/server/internal/worldrepo/llm_materializer.go` (`BoardPostRequest.RelationshipContext`)
   - 返信先の著者と本人の過去やり取り回数を `RelationshipEdge` から取得し、距離感に応じた文体コンテキストを自動注入。
4. **サンプリング検証ハーネスの拡充**:
   - `apps/server/internal/worldrepo/materialize_sampling_test.go`
   - `TestMaterializeSampling_PersonaToneAndRelationship` を新設し、各種シナリオでのプロンプト注入と出力を恒久的に検証。

---

## 5. 結論
全ての要件が達成され、金型および全テストが完全通過したことを確認。
草の根BBSの自然な生きたコミュニケーションがモデルプロンプトおよびワールドマテリアライザにおいて確立された。
