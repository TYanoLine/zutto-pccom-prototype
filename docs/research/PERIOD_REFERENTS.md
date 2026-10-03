# 固有名詞不足の分析と出典付き参照対象

## 2026-09-08: main 1546293 のコード上の原因

1. 通常のLLMMaterializerはHISTORICAL_REFERENCES_ENABLEDがfalseなら新しい実在名称を禁止。根拠を供給するHistoricalTextureはfresh Labの明示指定時だけで、通常配線には存在しなかった。
2. batch Situationと本文にはtextureを渡す一方、host-wide Producerと旧timeline plannerには具体的な参照対象を渡していなかった。本文だけを具体化しても、先に確定した一般的な意味計画に拘束される。
3. 日常の機械・作品を懐古的な話題にしない規則が、通常の対象を名前で呼ぶことまで抑制しうる。状況生成時の名称確定と、本文生成時の後付け命名を区別する必要がある。
4. Webのfresh proxyがsituation_mode / historical_texture / board_count / shell_limitを転送しておらず、Web経由のA/B設定がバックエンドに届かなかった。
5. Labのfacetsは手書きの一般的な状況を先に固定する。出典付き名称を渡しても既存状況を勝手に改名しないため、自由な状況生成の評価にはbatchを明示する。

以上はコード分析。生成結果の改善率を示す実測ではない。

## 変更

- historicalkb.PeriodReferentsは名称・使用可能日・限定的なclaim・出典URLを持つ**bootstrap seed**。実在名の許可リストでも話題リストでもない。2026-09-26以降、共有BBSの件名候補生成やJevの話題適合入力へカタログ全体を渡さない。未収録を含む実在名は未確定候補として提案でき、採用候補だけを後段Historical KBで検証する。PeriodReferentsはその検証・存在確認を高速化するために使う。
- 日付不正は追加供給なし。複数記事の計画には世界日付と全記事日付の最古日を使う。窓途中の発売を遅い記事にも供給しない保守的な初期実装。
- 本文とevidenceの日時は観測日ではなく記事日。永続化済み記事や人物の事実は書き換えない。
- 状況・Producer・timeline・本文へ同じ根拠を供給する。世界側の投稿者・日時・board・root/reply・原因選択は維持する。
- 名称は話題のノルマではない。既に選ばれた原因・board・人物の関心に合う対象の名称を、世界事実の提案段階で確定してよい。本文workerは確定済み対象を保持する。
- 実在名称の存在は、人物の所有・購入・経験・互換性や作品の詳細を証明しない。詳細は別の根拠が必要。

## オンデマンド調査で育つHistorical KB

通常のタイトル生成では、PeriodReferents未収録の実在名を候補段階で禁止しない。候補はまだworld factではなく、Azure OpenAIは各候補と同時に `historical_claims` を返す。claimはタイトル全文ではなく、`PC-9821Xa` のような再利用可能なsubjectと、`product_availability` / `technical_capability` 等のknowledge kind、最小限の確認事項を持つ。

採用候補の流れは次の順序に固定する。

1. Historical KBをsubject/kind/region/audience単位で**DB lookupのみ**実行する。ここではWeb検索を開始しない。
2. Verified / operator_verified / canonical の既存Factで足りれば、そのFactを再利用する。
3. missしたclaimだけをWeb調査へ送る。調査結果はResearchCaseとHistoricalFactとしてPostgresへ保存する。
4. 対話UIが待つ同期時間はboard全体で6秒まで。ただし6秒経過は**待機の打切り**であり、開始済みWeb調査のキャンセル理由にはしない。調査はbounded background jobとして継続し、完了すれば次回以降のKB hitになる。
5. 1回のboard materializationで新規background researchへ送るselected title job数には上限を置き、候補20件すべてを無差別に検索しない。

この構造では、たとえば `PC-9821Xa使ってる人います？` の確認結果を、後日の `PC-9821Xaのメモリについて` でも再利用できる。タイトル全文＋日付をKnowledgeKeyにする旧方式は、claim metadataを持たないlegacy/test rendererの互換フォールバックに限定する。

historical_claims自体はモデルが出した**調査ヒント**であって証拠ではない。metadataに書かれた名称・分類・needだけでcanonical採用してはいけない。最終採用には既存の検証済みHistorical Fact、またはHistorical Knowledge Engineによる調査成功が必要。

## 出典と証拠範囲

全項目はメーカー・運営元の公式資料に直接記載された範囲をConfirmedとして採用。資料の後年の懐古表現、後続製品、売上、人気評価はプロンプトへ渡さない。年/月しか確認していない項目の使用開始日は次年/次月の1日とし、発売日の断定には使わない。

|名称|使用可能日|確認した範囲|出典|
|---|---|---|---|
|PC-9801|1983-01-01|NECのパソコン、1982年発売|https://jpn.nec.com/profile/corp/history.html|
|NIFTY-Serve|1987-04-15|パソコン通信サービスの開始|https://www.nifty.co.jp/company/history/|
|SC-55|1992-01-01|1991年のSOUND CANVAS音源|https://www.roland.com/jp/company/history/|
|スーパーメトロイド|1994-03-19|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|ファイナルファンタジーVI|1994-04-02|スーパーファミコン版発売|https://support.jp.square-enix.com/faqarticle.php?id=195&kid=45701&la=0&ret=main|
|スーパーストリートファイターII|1994-06-25|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|セガサターン|1994-11-22|家庭用ゲーム機の発売|https://www.sega.jp/history/hard/column/column_05.html|
|スーパードンキーコング|1994-11-26|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|PlayStation|1994-12-03|日本での家庭用ゲーム機発売|https://www.playstation.com/ja-jp/playstation-history/1994-ps-one/|
|ときめきメモリアル|1995-01-01|1994年にPCエンジン向け第1作が登場。年のみの資料なので翌年1月から使用|https://www.konami.com/games/corporate/ja/news/topics/20250203/|
|一太郎Ver.6|1995-02-01|1995年1月のWindows向けワープロ|https://www.ichitaro.com/history/tw06.html|
|パンツァードラグーン|1995-03-10|サターン向けシューティング|https://www.sega.jp/history/hard/segasaturn/software.html|
|クロノ・トリガー|1995-03-11|スーパーファミコン用RPG発売|https://www.nintendo.co.jp/wii/vc/vc_chr/vc_chr_01.html|
|スーパーマリオ ヨッシーアイランド|1995-08-05|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|パネルでポン|1995-10-27|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|Windows 95|1995-11-23|日本語版OSの発売|https://news.microsoft.com/source/1998/06/17/windows-98-available-in-japanese/|
|バーチャファイター２|1995-12-01|サターン向けアクションゲーム|https://www.sega.jp/history/hard/segasaturn/software.html|
|ドラゴンクエストVI 幻の大地|1996-01-01|1995年12月にスーパーファミコン版発売。月のみの資料なので翌月1日から使用|https://www.jp.square-enix.com/game/detail/dq6/|
|Jリーグ実況ウイニングイレブン|1996-01-01|1995年のPlayStation向けタイトル。年のみの資料なので翌年1月から使用|https://www.konami.com/corporate/ja/history/product.html|
|幻想水滸伝|1996-01-01|1995年に日本でリリース。年のみの資料なので翌年1月から使用|https://www.konami.com/games/suikoden/ja/cp/suki|
|ポケットモンスター 赤・緑|1996-02-27|ゲームボーイ向けソフト発売|https://www.nintendo.co.jp/ds/interview/ipkj/vol1/index.html|
|スーパーマリオRPG|1996-03-09|スーパーファミコン用RPG発売|https://www.nintendo.co.jp/clvs/soft/mario_rpg.html|
|星のカービィ スーパーデラックス|1996-03-21|スーパーファミコン用ソフト発売|https://www.nintendo.co.jp/corporate/release/2017/170627.html|
|バイオハザード|1996-03-22|PlayStation版発売|https://www.capcom.co.jp/ir/news/html/200612b.html|

人物の出来事・好みはサービス独自の架空設定であり、上記史実とは区別する。

## 検証方法と残る限界

freshに `situation_mode=batch&historical_texture=sourced&board_count=6&shell_limit=5` を指定し、同じ条件で `historical_texture=off` と比較する。件数だけでなく、名称の種類、特定名称への集中、対象が分かる質問、人物の関心との適合、根拠のない仕様や経験の追加を読む。旧1996-08-curatedはfixture比較用に残す。

自動テストは全項目の使用開始日前後・不正日付・共有カタログの不変性・状況/Producer/本文への伝播・過去記事への未来名称混入防止・off維持を確認する。生成文の自然さを自動テスト成功だけで保証しない。

初期カタログは技術・ゲーム・DTM寄りであり、これを候補生成へ直接供給すると一般Q&A/雑談までその分野へ引っ張ることが実測された。そのためカタログの偏りを生成分布へ転写しないことを設計上の制約とする。音楽作品、雑誌、地域の店舗・駅、文化・ニュースの網羅性は不足しているが、カタログを均等な話題辞書へ拡張すること自体を解決策にはしない。固有名詞の出現率は強制せず、未供給の作品の仕様・攻略なども生成を許可しない。
