# Plan: Spec 014 80桁端末環境に即した記事レイアウトと改行指針 (terminal-line-wrap-and-layout)

## 1. 段階的実装アプローチ (Phase Breakdown)

### Phase 0: ベースライン確認
- 現在の全 25 パッケージテストと金型テストがすべて PASS していることを確認する。

### Phase 1: プロンプトへの改行・レイアウト指針の導入
- `apps/server/internal/llm/board_post_prompt.go` の文体指針に、全角30〜35文字目安・句点や文節での適度な改行・ノーブレイク長文や極端な短文改行の回避ルールを追加。
- `apps/server/internal/llm/board_post_freeform.go` にも同様の改行・レイアウト指針を追加。
- 関連する prompt テストを更新・確認。

### Phase 2: サンプリング検証と行長レイアウトの確認
- `apps/server/internal/worldrepo/materialize_sampling_test.go` に、改行位置と行長の検証シナリオ（`TestMaterializeSampling_LineWrapAndLayout`）を追加。
- サンプリング出力において、文節や句点で適切に改行が入り、80桁端末（全角40文字）の画面端での不自然な単語分断が解消されていることを確認。

### Phase 3: 金型保全・全テスト・静的解析
- `TestPopulation` (金型)、`TestGoldenBoardTableShape` (金型) を実行し完全一致を確認。
- `go test ./...` (全25パッケージ) および `go vet ./...` を実行し、全通過を確認。

### Phase 4: 検証記録と PR 作成
- `specs/014-terminal-line-wrap-and-layout/verification.md` を作成。
- リモートへ push し、Draft PR を作成。

---

## 2. リスクと対策
- **リスク**: プロンプトの文字数目安（30〜35文字）に過剰に引っ張られて、1行が不自然に短くなりすぎたり、リズムが壊れること。
- **対策**: 「目安」とし、文節や句点（。）のキリの良い位置で自然に改行するよう促す。ポエムのような極端な短行（15文字未満）の多用を明示的に禁止する。
