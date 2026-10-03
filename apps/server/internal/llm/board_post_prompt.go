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
	if req.FreeformFromSubject {
		return buildFreeformBoardPostPrompt(req)
	}
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
	additional := make([]string, 0, 2)
	if requiredReferent != "" && !strings.Contains(subject, requiredReferent) {
		additional = append(additional, "本文で示すcanonical referent: "+requiredReferent)
	}
	if quote := strings.TrimSpace(req.QuoteText); quote != "" {
		additional = append(additional, "引用資料（引用時は原文を > 付きで使用）:\n"+quote)
	}
	extra := "(追加なし)"
	if len(additional) > 0 {
		extra = strings.Join(additional, "\n")
	}
	minChars, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)

	return fmt.Sprintf(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界で、確定済みSituationを会員が読む記事本文として文章化し、保存・表示します。

世界日付: %s
局: %s
掲示板: %s
確定済み件名: %s
投稿者:
%s

canonical Situation / thread facts:
%s

時代資料:
%s

親記事:
%s

追加の文章化材料:
%s

この記事を書いている本人の自然な文章にしてください。投稿目的と人物の書き方を材料に、確定済みの出来事・対象・親記事の文脈を表現します。読者には件名や投稿者が表示されています。
時代: %s
本文の目安: %d〜%d文字
時代背景: %s

JSON:
{"author":"...","subject":"%s","body":"..."}`, req.WorldDate, req.HostName, req.BoardTopic, subject, persona, intent, facts, parent, extra, req.WorldDate, minChars, maxChars, compactBoardPostEraRules(req.EraRules), subject)
}

func compactBoardPostEraRules(raw string) string {
	first := strings.TrimSpace(raw)
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	if first == "" {
		return "世界日付に沿った当時の本人の視点"
	}
	return first
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