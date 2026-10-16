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
	if len(req.ThreadContext) > 0 {
		intent = stripLegacyThreadContext(intent)
	}
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
	if len(req.ThreadContext) > 0 {
		parent = formatThreadContext(req.ThreadContext) + "\n\n直接の返信先:\n" + parent
	}
	additional := make([]string, 0, 3)
	if requiredReferent != "" && !strings.Contains(subject, requiredReferent) {
		additional = append(additional, "本文で示すcanonical referent: "+requiredReferent)
	}
	if quote := strings.TrimSpace(req.QuoteText); quote != "" {
		additional = append(additional, "引用資料（引用時は原文を > 付きで使用）:\n"+quote)
	}
	if rel := strings.TrimSpace(req.RelationshipContext); rel != "" {
		additional = append(additional, "相手との関係性:\n"+rel)
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

文体の指針:
- 1996年当時の草の根パソコン通信（趣味サークルや部室のような親しみやすさ）の空気感で書いてください。
- 業務連絡やフリマのような過度に堅苦しい定型敬語（「おめでとうございます」「ご相談させてください」の連打など）は避け、会話としての生きた口語表現を使ってください。
- 口語の崩し方や当時の感情表現（(笑)、(^^;、(爆) 等）を使うか、どれを使うかは、投稿者の文体特徴に任せてください。特定の語尾や顔文字を全員が使う必要はありません。
- 相手との関係性（初対面か、親しい常連仲間か）に応じた自然な距離感で応じてください。
- スレッドに先行する他の発言や返信がある場合は、スレッド全体の会話の流れを踏まえて自然に書き込んでください。先行する他の返信者と同じ定型的な挨拶を単にオウム返しするのではなく、先行発言の言い換えや同意の繰り返しは避けてください。同意するだけで終わらず、自分の体験や具体的な事例、疑問、別の見方や反対意見、小さな訂正、少し脱線した余談など、この人らしく自然なものを自分で選んで加えてください。短い一言だけの返信でも、先行返信と結論が同じなら書かない部分があっても構いません。
- 返信の際、親記事や先行発言の文章に言及したい部分がある場合は、必要に応じて本文冒頭などに「>」を付けて自然に引用し、それに対する返信を続けてください。結びの挨拶（「よろしくお願いします」等）だけの引用や機械的な全文引用は避け、言及したい核心部分だけを引用するか、引用なしで直接返答しても構いません。
- 当時の80桁端末（全角40文字画面）で読みやすいよう、1行の長さは全角30〜35文字程度を目安とし、句点（。）やキリの良い文節で自然に改行を入れてください。改行を入れずに画面端で機械的に折り返される長大な行や、逆に15文字前後の極端に細切れなポエム調改行の多用は避けてください。

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

func truncateThreadBody(body string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(body))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

// threadContextRecentReplies bounds how many non-root predecessors are shown
// verbatim. Showing every earlier reply lets a long thread collapse into
// restatements of its first answer.
const threadContextRecentReplies = 3

func formatThreadContext(thread []ThreadPostContext) string {
	if len(thread) == 0 {
		return ""
	}
	keep := make([]bool, len(thread))
	var others []int
	for i, post := range thread {
		if post.IsRoot || i == 0 {
			keep[i] = true
			continue
		}
		others = append(others, i)
	}
	omitted := 0
	if len(others) > threadContextRecentReplies {
		omitted = len(others) - threadContextRecentReplies
		others = others[omitted:]
	}
	for _, i := range others {
		keep[i] = true
	}
	var b strings.Builder
	b.WriteString("スレッドの文脈（これまでの発言の流れ・時系列）:\n")
	skippedNoted := false
	for i, post := range thread {
		if !keep[i] {
			if !skippedNoted {
				fmt.Fprintf(&b, "（間の返信 %d 件は省略。同様の趣旨の発言が続いています）\n\n", omitted)
				skippedNoted = true
			}
			continue
		}
		role := ""
		if post.IsRoot {
			role = " (親記事/スレッド起点)"
		}
		subj := strings.TrimSpace(post.Subject)
		if subj != "" {
			fmt.Fprintf(&b, "[%d] 発言者: %s%s / 件名: %s\n", i+1, post.Author, role, subj)
		} else {
			fmt.Fprintf(&b, "[%d] 発言者: %s%s\n", i+1, post.Author, role)
		}
		body := strings.TrimSpace(post.Body)
		if body != "" {
			b.WriteString(truncateThreadBody(body, 400))
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func stripLegacyThreadContext(intent string) string {
	for _, marker := range []string{"これまでの会話:\nTHREAD CONTEXT", "conversation_context:\nTHREAD CONTEXT"} {
		if idx := strings.Index(intent, marker); idx >= 0 {
			intent = strings.TrimSpace(intent[:idx])
		}
	}
	return intent
}