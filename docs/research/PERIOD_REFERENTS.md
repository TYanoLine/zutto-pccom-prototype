# 固有名詞不足の分析と出典付き参照対象

## 2026-09-08: main 1546293 のコード上の原因

1. 通常のLLMMaterializerはHISTORICAL_REFERENCES_ENABLEDがfalseなら新しい実在名称を禁止。根拠を供給するHistoricalTextureはfresh Labの明示指定時だけで、通常配線には存在しなかった。
2. batch Situationと本文にはtextureを渡す一方、host-wide Producerと旧timeline plannerには具体的な参照対象を渡していなかった。本文だけを具体化しても、先に確定した一般的な意味計画に拘束される。
3. 日常の機械・作品を懐古的な話題にしない規則が、通常の対象を名前で呼ぶことまで抑制しうる。状況生成時の名称確定と、本文生成時の後付け命名を区別する必要がある。
4. Webのfresh proxyがsituation_mode / historical_texture / board_count / shell_limitを転送しておらず、Web経由のA/B設定がバックエンドに届かなかった。
5. Labのfacetsは手書きの一般的な状況を先に固定する。出典付き名称を渡しても既存状況を勝手に改名しないため、自由な状況生成の評価にはbatchを明示する。

以上はコード分析。生成結果の改善率を示す実測ではない。

## 変更

- historicalkb.PeriodReferentsは名称・使用可能日・限定的なclaim・出典URLを持つ小さなカタログ。通常の配線で有効化し、検索APIやモデル記憶を無制限に許可する設定とは分ける。
- 日付不正は追加供給なし。複数記事の計画には世界日付と全記事日付の最古日を使う。窓途中の発売を遅い記事にも供給しない保守的な初期実装。
- 本文とevidenceの日時は観測日ではなく記事日。永続化済み記事や人物の事実は書き換えない。
- 状況・Producer・timeline・本文へ同じ根拠を供給する。世界側の投稿者・日時・board・root/reply・原因選択は維持する。
- 名称は話題のノルマではない。既に選ばれた原因・board・人物の関心に合う対象の名称を、世界事実の提案段階で確定してよい。本文workerは確定済み対象を保持する。
- 実在名称の存在は、人物の所有・購入・経験・互換性や作品の詳細を証明しない。詳細は別の根拠が必要。

## 出典と証拠範囲

全項目はメーカー・運営元の公式資料に直接記載された範囲をConfirmedとして採用。資料の後年の懐古表現、後続製品、売上、人気評価はプロンプトへ渡さない。年/月しか確認していない項目の使用開始日は次年/次月の1日とし、発売日の断定には使わない。

|名称|使用可能日|確認した範囲|出典|
|---|---|---|---|
|PC-9801|1983-01-01|NECのパソコン、1982年発売|https://jpn.nec.com/profile/corp/history.html|
|NIFTY-Serve|1987-04-15|パソコン通信サービスの開始|https://www.nifty.co.jp/company/history/|
|SC-55|1992-01-01|1991年のSOUND CANVAS音源|https://www.roland.com/jp/company/history/|
|セガサターン|1994-11-22|家庭用ゲーム機の発売|https://www.sega.jp/history/hard/column/column_05.html|
|PlayStation|1994-12-03|日本での家庭用ゲーム機発売|https://www.playstation.com/ja-jp/playstation-history/1994-ps-one/|
|一太郎Ver.6|1995-02-01|1995年1月のWindows向けワープロ|https://www.ichitaro.com/history/tw06.html|
|パンツァードラグーン|1995-03-10|サターン向けシューティング|https://www.sega.jp/history/hard/segasaturn/software.html|
|Windows 95|1995-11-23|日本語版OSの発売|https://news.microsoft.com/source/1998/06/17/windows-98-available-in-japanese/|
|バーチャファイター２|1995-12-01|サターン向けアクションゲーム|https://www.sega.jp/history/hard/segasaturn/software.html|
|ポケットモンスター 赤・緑|1996-02-27|ゲームボーイ向けソフト発売|https://www.nintendo.co.jp/ds/interview/ipkj/vol1/index.html|

人物の出来事・好みはサービス独自の架空設定であり、上記史実とは区別する。

## 検証方法と残る限界

freshに `situation_mode=batch&historical_texture=sourced&board_count=6&shell_limit=5` を指定し、同じ条件で `historical_texture=off` と比較する。件数だけでなく、名称の種類、特定名称への集中、対象が分かる質問、人物の関心との適合、根拠のない仕様や経験の追加を読む。旧1996-08-curatedはfixture比較用に残す。

自動テストは全項目の使用開始日前後・不正日付・共有カタログの不変性・状況/Producer/本文への伝播・過去記事への未来名称混入防止・off維持を確認する。生成文の自然さを自動テスト成功だけで保証しない。

初期カタログは技術・ゲーム・DTM寄り。音楽作品、雑誌、地域の店舗・駅、文化・ニュースの網羅性は不足している。固有名詞の出現率は強制せず、未供給の作品の仕様・攻略なども生成を許可しない。日付とboard・人物への適合のうち、後者は現状プロンプトによる制約であり、全史実を機械検証する仕組みではない。
