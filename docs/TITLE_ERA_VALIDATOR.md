# Title-first Era Validator PoC

`title-first` の自然な件名生成と、年代の正しさの検証を分離するための fresh Materialization Lab 専用 PoC。

## パイプライン

```text
20件の自然なタイトル候補生成
  -> Era Router（ok / research / ng）
  -> ok / research を人物・投稿枠へ仮割当
  -> 仮採用された research だけ Historical Knowledge Engine でWeb史料確認
  -> ok / verified の仮採用だけ確定
  -> 本文生成
```

最初のタイトル生成プロンプトは従来どおり最小のままにし、年代制約・人物設定・投稿目的を混ぜない。Era Router は文章の自然さや人物適合を評価せず、外部の年代事実を確認する必要があるかだけを振り分ける。

20候補すべてを先にWeb検索しない。人物・投稿枠 matcher が実際の投稿候補として選んだものだけが、必要なら史料検索を消費する。これにより使われない候補が検索予算を奪うのを防ぐ。

## Era 判定

- `ok`: タイトル成立のために world_date 時点の外部年代事実を確認する必要がない題材。一般的な日常話題、一般技術カテゴリ、単なる地名などは固有名詞があっても `ok` になり得る。
- `research`: タイトルの成立が、具体的な製品・作品・サービス・規格・機種の発売・発表・提供時期や、現実の当日イベントなどに実質的に依存する候補。モデル記憶だけで可否を確定しない。
- `ng`: world_date だけから論理的に成立しないことが明白な候補。モデル知識に依存する疑わしい候補は `ng` ではなく `research` にする。

`research` でも人物・投稿枠に選ばれなかった候補はWeb検索せず、Viewer上では `not_needed` として区別する。

Web史料確認後は、検証済み Historical Fact の `ERA_OK:` / `ERA_NG:` マーカーだけを採用する。調査失敗、史料不足、同名曖昧、マーカー欠落、研究中は `unverified` とし、そのrunでは安全側に除外する。

## コスト境界

1回の `title-first` run でWeb史料確認する**仮採用候補**は最大16件、同時実行は3件まで。標準4板Labでは残り板数で公平配分し、未使用枠は後続板へ繰り越す。

この16件は20候補×4板=80件に対する上限ではなく、人物matcherが実際の投稿枠へ仮採用した `research` 候補だけに対する上限。したがって通常は上限よりかなり少ない検索回数になることを期待する。

Historical Knowledge Engine の既存 Fact / research lease を再利用する。title-era 用 Knowledge subject には基準日を含め、ある日付での `ERA_NG` を後日の世界日付へ誤適用しない。

## 日付

板ごとに、その生成windowで最も早い独立rootの投稿日時を検証基準日にする。その日までに成立する実在物なら同じwindowの後続rootにも年代上は使用可能とみなす。これは保守的な方式で、window途中に成立する実在物は今回のPoCでは除外され得る。

## 人物整合との分離

`ng` 以外の候補を人物・投稿枠 matcher へ渡す。matcher は発売日・サービス開始時期などをモデル記憶から再判定せず、人物の既存事実、投稿日時、board、routing domain、discourse mode、過去BBS状態との整合だけを見る。

matcherが `research` 候補を仮採用した場合のみ、その最終subjectをEra史料検証する。`verified` なら確定し、`ng` / `unverified` ならその投稿枠への割当を破棄する。件数を埋めるための事実捏造や自動的な別題材への書き換えはしない。

候補に合わせて所有、購入、利用、視聴、プレイ歴などの人物事実を追加しない。

## Viewer

`/poc/materialization-lab-viewer` のタイトル候補表で以下を確認できる。

- 生成直後タイトル
- Era判定
- Era理由
- Web史料検証結果（あれば）
- `not_needed`（人物割当に使われず検索不要だった候補）
- 採用後タイトル
- 発言者
- 最終判定
- 採否理由

このPoCは通常世界へ書き戻さない。最終的な史実品質は、Era Router の分類精度、人物matcherの選択品質、Historical Knowledge Engine が返す検証済みFactの品質を別々に評価する。
