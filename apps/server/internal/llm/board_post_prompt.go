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
	intent, requiredReferent := extractArticleReferentControl(req.PostIntent)
	intent = strings.TrimSpace(intent)
	if intent == "" {
		intent = "(追加のcanonical Situationなし)"
	}

	facts := "(なし)"
	if len(req.HistoricalFacts) > 0 {
		facts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	persona := "(通常会員。追加プロフィールなし)"
	if strings.TrimSpace(req.AuthorHandle) != "" {
		persona = fmt.Sprintf("handle=%s\n%s", req.AuthorHandle, strings.TrimSpace(req.PersonaProfile))
	}
	subject := strings.TrimSpace(req.CanonicalSubject)
	if subject == "" {
		subject = strings.TrimSpace(req.BoardTopic)
	}
	parent := "(root記事なのでなし)"
	if strings.TrimSpace(req.ParentSubject) != "" || strings.TrimSpace(req.ParentBody) != "" {
		parent = fmt.Sprintf("subject=%s\nbody:\n%s", strings.TrimSpace(req.ParentSubject), strings.TrimSpace(req.ParentBody))
	}
	quoteRule := "引用は不要です。"
	if strings.TrimSpace(req.QuoteText) != "" {
		quoteRule = fmt.Sprintf("引用する場合は次の原文を > 付きで一字一句そのまま使うこと:\n%s", strings.TrimSpace(req.QuoteText))
	}
	referentRule := ""
	if requiredReferent != "" {
		referentRule = fmt.Sprintf("\n- canonical referent「%s」。表示件名がこの対象名を明示していない場合、本文の自然な位置で対象名を少なくとも一度明示する。対象を別の作品・製品・店等へ置き換えない。", requiredReferent)
	}
	minChars, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "new_post"
	}
	eraRules := compactBoardPostEraRules(req.EraRules)

	return fmt.Sprintf(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界で、すでに正本化された記事Situationを、その人物がBBSへ実際に投稿した本文として文章化します。
あなたの出力は会員が読む記事本文として保存・表示されます。入力されたcanonical Situationとthread factsが本文の材料です。

世界日付: %s
局: %s
掲示板: %s
件名: %s
投稿者:
%s

canonical Situation / thread facts:
%s

使ってよい公開史実:
%s

親記事:
%s

ルール:
- canonical Situation はすでに世界で起きた事実。内容・人物・対象・因果を変えず、書かれていない新しい出来事を足さない。
- 事実を全部説明する必要はない。この人物がその瞬間に実際に口にしそうな部分だけを書く。
- ヘッダを読み上げない。件名・投稿者・日時・掲示板は読者に見えているので、本文は用件から自然に始める。
- discourse_mode と typed Situation の形をそのまま文章行為にする。ask_peersだけが質問を主目的にし、それ以外へ「みなさんは？」等の質問を付け足さない。
- replyなら親記事の文脈へ反応する。root/replyを変更しない。
- supplied historical facts と canonical Situation にない実在固有名詞、仕様、価格、発売時期、攻略情報などを追加しない。
- personaのwriting傾向があれば従う。文章をFAQ・解説・結論付きの整った記事へ無理に仕上げない。
- 当時の本人として普通に書く。現代からの懐古・時代解説・AI/プロンプト/DB等のメタ発言は禁止。
- 世界日付は %s。この日より未来の知識や出来事を使わない。
- 非引用部分は%d〜%d文字を目安にする。水増ししない。%s
- %s
- 時代制約: %s

JSONだけを返す:
{"author":"...","subject":"%s","body":"..."}`, req.WorldDate, req.HostName, req.BoardTopic, subject, persona, intent, facts, parent, req.WorldDate, minChars, maxChars, referentRule, quoteRule, eraRules, subject)
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
	_, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	if err := validateBoardPostDraftWithBodyLimit(d, maxChars); err != nil {
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


func extractArticleReferentControl(raw string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	kept := make([]string, 0, len(lines))
	referent := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "article_referent_required=") {
			if referent == "" {
				referent = strings.TrimSpace(strings.TrimPrefix(trimmed, "article_referent_required="))
			}
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), referent
}