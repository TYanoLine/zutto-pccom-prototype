# 検証結果: BBS本文の人物別具体化

**実施日**: 2026-09-28

## 自動テスト

`apps/server` で次を実行し、すべて成功した。

```text
go test ./internal/world ./internal/worldrepo ./internal/llm ./internal/hostprogram/erikak ./cmd/server
go test ./...
```

確認した内容:

- Article Detail 0〜2件の保存と、正常0件の完了状態
- Detail生成・検証の再試行、planner不在・保存失敗時の本文生成中断
- 完了状態が本文より先に保存され、再閲覧で再提案されないこと
- title-firstではヘッダー時にDetailを作らず、本文閲覧時に共有処理で一度だけ確定すること
- 返信者自身の履歴とスレッド内の他人の発言の分離
- 人物事実の投稿時刻フィルタ、旧本文・旧Detailの互換性
- snapshotの保存・復元、並行閲覧、Erika-Kのアペンド失敗表示
- 通常閲覧とfresh Labが同じDetail処理を利用すること

## 手動品質評価

このcheckout内で利用できる生成済みfresh Labアーカイブまたはサンプル本文は見つからなかった。Live Labはprovider利用量を発生させるため起動していない。利用可能な標本数は0件で、意味的反復・一人称事実・未来漏洩・1996年境界の人手判定は未実施。標本不足は実装テストのブロッカーではなく、結果を追加取得できた時点で追記する。
