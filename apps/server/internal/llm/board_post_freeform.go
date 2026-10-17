package llm

import (
    "encoding/json"
    "fmt"
    "strings"
)

// This temporary evaluation prompt deliberately leaves article-local
// wording choices to the model. Only accepted summary/required identities,
// reply causality and the already displayed header remain binding. It never
// selects or rewrites a World event.
func buildFreeformBoardPostPrompt(req BoardPostRequest) string {
    subject := strings.TrimSpace(req.CanonicalSubject)
    displayedSubject := subject
    outputSubject := subject
    if displayedSubject == "" {
        displayedSubject = "(返信・ホスト上の表示件名なし)"
        // Some renderer JSON validators require a nonempty subject field. This
        // is internal output metadata, not a newly adopted host-visible title.
        outputSubject = strings.TrimSpace(req.ParentSubject)
        if outputSubject == "" { outputSubject = req.BoardTopic }
    }
    persona := strings.TrimSpace(req.PersonaProfile)
    if handle := strings.TrimSpace(req.AuthorHandle); handle != "" {
        persona = "handle=" + handle + "\n" + persona
    }
    if persona == "" { persona = "(追加プロフィールなし)" }
    intent, requiredReferent := extractArticleReferentControl(req.PostIntent)
    if len(req.ThreadContext) > 0 {
        intent = stripLegacyThreadContext(intent)
    }
    var extra []string
    if strings.TrimSpace(intent) != "" {
        extra = append(extra, "記事の起点となる確定情報（矛盾させず、すべてを説明する必要はありません）:\n"+strings.TrimSpace(intent))
    }
    if requiredReferent != "" && !strings.Contains(subject, requiredReferent) {
        extra = append(extra, "この記事の確定済み参照対象: "+requiredReferent)
    }
    if len(req.ThreadContext) > 0 {
        extra = append(extra, formatThreadContext(req.ThreadContext))
        if parent := strings.TrimSpace(req.ParentBody); parent != "" || strings.TrimSpace(req.ParentSubject) != "" {
            extra = append(extra, fmt.Sprintf("直接の返信先（あなたが返信する対象）:\n件名: %s\n本文:\n%s", strings.TrimSpace(req.ParentSubject), parent))
        }
    } else if parent := strings.TrimSpace(req.ParentBody); parent != "" || strings.TrimSpace(req.ParentSubject) != "" {
        extra = append(extra, fmt.Sprintf("返信先（この記事の著者が応じる相手の文章）:\n件名: %s\n本文:\n%s", strings.TrimSpace(req.ParentSubject), parent))
    }
    if quote := strings.TrimSpace(req.QuoteText); quote != "" {
        extra = append(extra, "本文に引用する確定済み原文（引用するときは > 付きでそのまま）:\n"+quote)
    }
    if len(req.HistoricalFacts) > 0 {
        extra = append(extra, "確定済み時代情報:\n"+strings.Join(req.HistoricalFacts, "\n"))
    }
    if rel := strings.TrimSpace(req.RelationshipContext); rel != "" {
        extra = append(extra, "相手との関係性:\n"+rel)
    }
    context := ""
    if len(extra) > 0 { context = "\n\n"+strings.Join(extra, "\n\n") }
    _, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
    authorJSON, _ := json.Marshal(req.AuthorHandle)
    subjectJSON, _ := json.Marshal(outputSubject)
    return fmt.Sprintf(`これは「ずっとパソコン通信」の内部生成です。1996年前後の日本の草の根パソコン通信世界で、会員が読む記事本文を文章化し、保存・表示します。

世界日付: %s
局: %s
掲示板: %s
確定済み件名: %s
投稿者:
%s%s

この記事を書いている本人の自然な文章にしてください。件名、投稿目的と人物の書き方を材料に、会員が自分の言葉で書いた本文にしてください。確定済みの世界事実や返信先の文章と矛盾させず、細部の言い回しや文章の展開は自由です。
当時のパソコン通信らしい口語や感情表現（(笑)、(^^;、(爆) など）を使うか、どれを使うかは、投稿者の人物像と「文体・口調」に任せてください。特定の語尾や顔文字を全員が使う必要はなく、まったく使わない人がいても構いません。相手との関係性に応じた距離感で語ってください。
スレッドに先行する他の発言や返信がある場合は、スレッド全体の会話の流れを踏まえて自然に書き込んでください。先行する他の返信者と同じ定型的な挨拶を単にオウム返しするのではなく、先行発言の言い換えや同意の繰り返しは避けてください。同意するだけで終わらず、自分の体験や具体的な事例、疑問、別の見方や反対意見、小さな訂正、少し脱線した余談など、この人らしく自然なものを自分で選んで加えてください。短い一言だけの返信でも、先行返信と結論が同じなら書かない部分があっても構いません。
返信の際、親記事や先行発言の文章に言及したい部分がある場合は、必要に応じて本文冒頭などに「>」を付けて自然に引用し、それに対する返信を続けてください。結びの挨拶（「よろしくお願いします」等）だけの引用や機械的な全文引用は避け、言及したい核心部分だけを引用するか、引用なしで直接返答しても構いません。
当時の80桁端末（全角40文字画面）で読みやすいよう、1行の長さは全角30〜35文字程度を目安とし、句点（。）やキリの良い文節で自然に改行を入れてください。改行を入れずに画面端で機械的に折り返される長大な行や、逆に15文字前後の極端に細切れなポエム調改行の多用は避けてください。
時代: %s
本文の長さ: 最大%d文字。目安の上限であり、埋める必要はありません。内容に必要な分だけ書き、質問や話題を無理に足さないでください。質問は多くても1〜3点にとどめてください。
時代背景: %s

JSON:
{"author":%s,"subject":%s,"body":"..."}`,
        req.WorldDate, req.HostName, req.BoardTopic, displayedSubject,
        persona, context, req.WorldDate, maxChars,
        compactBoardPostEraRules(req.EraRules), string(authorJSON), string(subjectJSON),
    )
}
