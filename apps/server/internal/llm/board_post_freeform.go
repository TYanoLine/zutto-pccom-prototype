package llm

import (
    "encoding/json"
    "fmt"
    "strings"
)

// This temporary HAKATA evaluation prompt deliberately leaves article-local
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
    var extra []string
    if strings.TrimSpace(intent) != "" {
        extra = append(extra, "記事の起点となる確定情報（矛盾させず、すべてを説明する必要はありません）:\n"+strings.TrimSpace(intent))
    }
    if requiredReferent != "" && !strings.Contains(subject, requiredReferent) {
        extra = append(extra, "この記事の確定済み参照対象: "+requiredReferent)
    }
    if parent := strings.TrimSpace(req.ParentBody); parent != "" || strings.TrimSpace(req.ParentSubject) != "" {
        extra = append(extra, fmt.Sprintf("返信先（この記事の著者が応じる相手の文章）:\n件名: %s\n本文:\n%s", strings.TrimSpace(req.ParentSubject), parent))
    }
    if quote := strings.TrimSpace(req.QuoteText); quote != "" {
        extra = append(extra, "本文に引用する確定済み原文（引用するときは > 付きでそのまま）:\n"+quote)
    }
    if len(req.HistoricalFacts) > 0 {
        extra = append(extra, "確定済み時代情報:\n"+strings.Join(req.HistoricalFacts, "\n"))
    }
    context := ""
    if len(extra) > 0 { context = "\n\n"+strings.Join(extra, "\n\n") }
    minChars, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
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
時代: %s
本文の目安: %d〜%d文字
時代背景: %s

JSON:
{"author":%s,"subject":%s,"body":"..."}`,
        req.WorldDate, req.HostName, req.BoardTopic, displayedSubject,
        persona, context, req.WorldDate, minChars, maxChars,
        compactBoardPostEraRules(req.EraRules), string(authorJSON), string(subjectJSON),
    )
}
