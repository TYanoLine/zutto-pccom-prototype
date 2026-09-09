# Title-first Era Validator PoC

`title-first` の自然な件名生成と、年代の正しさの検証を分離するための fresh Materialization Lab 専用 PoC。

## パイプライン

```text
20件の自然なタイトル候補生成
  -> Era Validator（ok / research / ng）
  -> research のみ Historical Knowledge Engine でWeb史料確認
  -> ok / verified のみ人物・投稿枠へ割当
  -> 本文生成
```

最初のタイトル生成プロンプトは従来どおり最小のままにし、年代制約・人物設定・投稿目的を混ぜない。Era Validator は文章の自然さや人物適合を評価せず、外部の年代事実を確認する必要があるかだけを振り分ける。

## Era 判定

- `ok`: 外部年代事実を参照しなくても安全な一般的・局内・日常的題材。Web検索なしで人物割当へ進める。
- `research`: 実在の製品、作品、サービス、規格、機種、人名、番組、曲、イベント等、または時点依存の技術・文化事実を含む候補。モデル記憶で可否を確定せず、既存 Historical Knowledge Engine へ送る。
- `ng`: world_date だけから論理的に成立しないことが明白な候補。モデル知識に依存する疑わしい候補は `ng` ではなく `research` にする。

Web史料確認後は、検証済み Historical Fact の `ERA_OK:` / `ERA_NG:` マーカーだけを採用する。調査失敗、史料不足、同名曖昧、マーカー欠落、研究中は `unverified` とし、そのrunでは安全側に除外する。

## コスト境界

1回の `title-first` run でWeb史料確認する候補は最大12件、同時実行は3件まで。上限を超えた `research` 候補は未検証として除外し、件数を埋めるための再生成はしない。

Historical Knowledge Engine の既存 Fact / research lease を再利用する。title-era 用 Knowledge subject には基準日を含め、ある日付での `ERA_NG` を後日の世界日付へ誤適用しない。

## 日付

板ごとに、その生成windowで最も早い独立rootの投稿日時を検証基準日にする。その日までに成立する実在物なら同じwindowの後続rootにも年代上は使用可能とみなす。これは保守的な方式で、window途中に成立する実在物は今回のPoCでは除外され得る。

## 人物整合との分離

Era検証を通過した候補だけを従来の人物・投稿枠 matcher へ渡す。matcher は発売日・サービス開始時期などをモデル記憶から再判定せず、人物の既存事実、投稿日時、board、routing domain、discourse mode、過去BBS状態との整合だけを見る。

候補に合わせて所有、購入、利用、視聴、プレイ歴などの人物事実を追加しない。

## Viewer

`/poc/materialization-lab-viewer` のタイトル候補表で以下を確認できる。

- 生成直後タイトル
- Era判定
- Era理由
- Web史料検証結果（あれば）
- 採用後タイトル
- 発言者
- 最終判定
- 採否理由

このPoCは通常世界へ書き戻さない。最終的な史実品質は、Era Validator の分類精度と Historical Knowledge Engine が返す検証済みFactの品質を別々に評価する。
