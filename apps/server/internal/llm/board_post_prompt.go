package llm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	workerMSGPattern              = regexp.MustCompile(`(?i)\bMSG\s*#?\s*\d+\b`)
	workerOpeningTimestampPattern = regexp.MustCompile(`(?m)^\s*(?:\d{1,2}/\d{1,2}\s+\d{1,2}:\d{2}|\d{1,2}月\d{1,2}日\s*\d{1,2}時(?:\d{1,2}分)?)`)
)

// BuildBoardPostPrompt is intentionally shared by all prose-worker providers.
// World/planning metadata is already resolved before this point. The worker sees
// only the facts needed to write the post, not internal routing/debug machinery.
func BuildBoardPostPrompt(req BoardPostRequest) string {
	facts := "(none supplied)"
	if len(req.HistoricalFacts) > 0 {
		facts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	persona := "(ordinary member; no persistent profile supplied)"
	if strings.TrimSpace(req.AuthorHandle) != "" {
		persona = fmt.Sprintf("handle=%s\n%s", req.AuthorHandle, strings.TrimSpace(req.PersonaProfile))
	}
	intent := strings.TrimSpace(req.PostIntent)
	if intent == "" {
		intent = "(no extra canonical facts supplied)"
	}
	subject := strings.TrimSpace(req.CanonicalSubject)
	if subject == "" {
		subject = strings.TrimSpace(req.BoardTopic)
	}
	eraRules := compactBoardPostEraRules(req.EraRules)

	return fmt.Sprintf(`1996年前後の日本の草の根パソコン通信BBSに、指定された人物として1件だけ自然な本文を書いてください。

件名: %s
掲示板: %s
投稿者:
%s

この記事について既に確定している事実:
%s

本文で使用してよい公開史実:
%s

時代制約:
- 世界日付は %s。未来の知識は使わない。
- %s

書き方:
- 読者はすでにヘッダ（件名・投稿者・日時・掲示板）を見ています。本文でヘッダを読み上げないでください。
- MSG番号、記事番号、投稿時刻、日付、board id、内部状態、cause、routing、world slot、「新スレです」「～板から失礼します」など、投稿システム側のメタ情報を書かないでください。
- 「この記事について既に確定している事実」は世界事実の許可境界であって、本文に全部書くチェックリストではありません。この人物がこの瞬間に実際に口にしそうな部分だけを書いてください。
- 投稿を「役に立つ記事」「きれいな説明」「完成した回答」に仕上げる必要はありません。自然なら一言の感想、短い報告、相手の一部分への反応だけで終わって構いません。
- 確定事実に必要性がない限り、まとめ、教訓、結論、網羅的な比較、読者への質問を付け足さないでください。特に毎回「みなさんはどうですか？」型で締めないでください。
- 返信では、相手の記事の全部に答えたり会話を必ず発展させたりする必要はありません。共感、驚き、一点だけの補足、短い経験談だけでも人物と状況に自然なら十分です。
- 「この件について書きます」「おすすめを教えてください」のように件名を言い換えるだけで終わらず、確定事実の中に具体的な対象・観察・条件があれば、必要なものだけ普通に使ってください。
- 記事の書き方を説明せず、最初の文から用件そのものに入ってください。
- article_detail は投稿者について確定済みのローカル事実です。ただし本文で全detailを列挙する義務はありません。source_article_detail は相手の記事の事実で、返信者自身の経験へ移してはいけません。
- supplied historical facts は公開世界について使ってよい事実です。そこから未提示の価格・発売日・仕様・作品内容などを連想で追加しないでください。
- 新しい所有歴、購入歴、職歴、家族事情、長期的な嗜好や習慣を勝手に作らないでください。
- 文章は自然なら短くて構いません。1〜3文でもよく、長くなる理由がある人物・話題だけ長くしてください。上限は1〜5段落、500字程度。顔文字は人物に合う場合だけ。
- 1990年代らしさを小道具で演出せず、その時代の本人として普通に書いてください。

JSONだけを返してください:
{"author":"...","subject":"...","body":"..."}`, subject, req.BoardTopic, persona, intent, facts, req.WorldDate, eraRules)
}

func compactBoardPostEraRules(raw string) string {
	raw = strings.TrimSpace(raw)
	first := raw
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	if first == "" {
		first = "世界日付より未来の知識や出来事を使わない。"
	}
	return first + "\n当時の本人として普通に書く。現代から振り返る説明、レトロ・懐古演出、時代解説はしない。"
}

func validateBoardPostWorkerDraft(req BoardPostRequest, d BoardPostDraft) error {
	if err := validateBoardPostDraft(d); err != nil {
		return err
	}
	body := strings.TrimSpace(d.Body)
	if workerMSGPattern.MatchString(body) {
		return fmt.Errorf("article worker leaked MSG metadata")
	}
	if workerOpeningTimestampPattern.MatchString(body) {
		return fmt.Errorf("article worker opened by restating header timestamp")
	}
	for _, marker := range []string{"新スレです", "新規スレです", "板から失礼します", "板のMSG", "board id", "routing_domain", "cause_kind", "world slot"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(marker)) {
			return fmt.Errorf("article worker leaked/meta-narrated %q", marker)
		}
	}
	return nil
}
