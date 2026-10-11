# Plan: 013-persona-tone-and-relationship

## 1. 全体アーキテクチャ方針

本改修は、既存のインフラ層（`world.Store.GetRelationship` や `actormemory`）を活用し、BBS 本文生成パイプラインにおいて**ペルソナ個性と人間関係の文脈をプロンプトへ自然に流し込む**ものである。

```
[BBS Article Materialization]
       │
       ├─ (1) Author Persona 取得
       │        └─ personaSummary() で自然な人物像・文体指示テキストに変換
       │
       ├─ (2) ParentPost がある場合:
       │        └─ Store.GetRelationship(from, to) を参照
       │             └─ BoardReplyCount に応じて初対面/常連仲間の文脈を構築
       │
       └─ (3) BuildBoardPostPrompt
                ├─ 投稿者プロフィール (年齢・職業・性別・文体特徴・議論性)
                ├─ 相手との関係性 (初対面 / 常連仲間)
                └─ 1996年当時の草の根BBS文体指針 (ビジネスメール調の脱却、部室感のある生きた口語)
```

---

## 2. 変更箇所の設計詳細

### 2.1 `apps/server/internal/llm/`
- **`BoardPostRequest`**:
  - `RelationshipContext string` フィールドを追加。
- **`board_post_prompt.go` / `board_post_freeform.go`**:
  - `RelationshipContext` がある場合にプロンプトの文脈セクションに「相手との関係: ...」として追加。
  - プロンプト文末の指針に以下を追加：
    - 「1996年当時の草の根パソコン通信（趣味サークルや部室のような自然な親しみやすさ）の空気感で書いてください。」
    - 「ビジネスメールやフリマのような過度に堅苦しい定型敬語（『おめでとうございます』『ご相談させてください』の連打）は避け、人物の個性に沿った生きた口語表現を使ってください。」
    - 「文体特徴や感情に合わせて、適度に口語の崩し（『〜ですねー』『〜かも』『おおっ』等）や当時の感情表現（`(笑)`、`(^^;`、`(爆)` 等）を自然に交えてください。」

### 2.2 `apps/server/internal/worldrepo/`
- **`BoardMaterializationRequest`**:
  - `RelationshipContext string` フィールドを追加。
- **`bbs_article_body.go`**:
  - 返信記事（`ParentPost` が存在し、かつ著者と相手の双方が特定可能な場合）において、`r.Base.GetRelationship(host.ID, authorRef, targetRef)` を取得。
  - `BoardReplyCount` に応じた関係性テキストを組み立てて `req.RelationshipContext` に設定。
- **`llm_materializer.go`**:
  - `personaSummary(p world.Persona)` を改修：
    - 生のキーバリュー文字列ではなく、`年齢・性別・職業`、`文体特徴 (WritingStyle)`、`議論性・論理的傾向 (Argumentativeness)`、`興味分野` を自然な日本語のプロフィール指示文に整形。
  - `generateBoardPostReq` において `req.RelationshipContext` を `boardPostReq.RelationshipContext` に渡す。

### 2.3 `materialize_sampling_test.go`
- 既存のテストに加え、以下のシナリオをサンプリング検証：
  1. **常連仲間同士の貸し借り・ゲーム雑談**: 初対面のようなかしこまった挨拶をせず、サークル感覚のフランクなノリでやり取りされること。
  2. **初対面の会員からの返信**: 丁寧だが壁を作りすぎず、趣味の仲間として親しみやすいトーンであること。
  3. **ペルソナごとの文体差**: 「勢いのある短文」「落ち着いた文体」「気さくな口調」などの個性が反映されること。

---

## 3. リスクと対策

1. **ルールスタック再導入テスト (`TestBuildBoardPostPromptUsesPurposeAndMaterialsInsteadOfRuleStack`) の制約**:
   - `board_post_prompt_test.go` では、「ルール:」や「水増ししない」などの特定の旧NGワードが存在するとテストが失敗する。
   - 対策: 機械的な禁止命令ではなく、「文体の指針:」として自然な材料・方針の形で提示する。
2. **金型テスト保全**:
   - 人物生成ロジックそのものは変更せず、記事本文生成時のプロンプト整形のみを変更するため、`TestPopulationMatchesGolden` および `TestGoldenBoardTableShape` に一切影響を与えない。
