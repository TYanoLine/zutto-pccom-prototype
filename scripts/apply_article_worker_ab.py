from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"pattern not found in {path}: {old[:120]!r}")
    p.write_text(s.replace(old, new, 1))

# Config: Gemini is comparison-only for now.
path = "apps/server/internal/config/config.go"
replace_once(path, "\tOpenAIModel                 string\n", "\tOpenAIModel                 string\n\tGeminiKey                   string\n\tGeminiModel                 string\n")
replace_once(path, "\t\tOpenAIModel:                 env(\"OPENAI_MODEL\", \"gpt-5.6-luna\"),\n", "\t\tOpenAIModel:                 env(\"OPENAI_MODEL\", \"gpt-5.6-luna\"),\n\t\tGeminiKey:                   os.Getenv(\"GEMINI_API_KEY\"),\n\t\tGeminiModel:                 env(\"GEMINI_MODEL\", \"gemini-3.8-flash\"),\n")

# Shared, deliberately compact article-worker prompt + output guard.
Path("apps/server/internal/llm/board_post_prompt.go").write_text(r'''package llm

import (
    "fmt"
    "regexp"
    "strings"
)

var (
    workerMSGPattern = regexp.MustCompile(`(?i)\\bMSG\\s*#?\\s*\\d+\\b`)
    workerOpeningTimestampPattern = regexp.MustCompile(`(?m)^\\s*(?:\\d{1,2}/\\d{1,2}\\s+\\d{1,2}:\\d{2}|\\d{1,2}月\\d{1,2}日\\s*\\d{1,2}時(?:\\d{1,2}分)?)`)
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
    eraRules := withDiegeticWorldFrame(req.EraRules)

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
- 「この件について書きます」「おすすめを教えてください」のように件名を言い換えるだけで終わらず、確定事実の中に具体的な対象・観察・条件・比較・順序があれば本文で普通に使ってください。
- 記事の書き方を説明せず、最初の文から用件そのものに入ってください。
- article_detail は投稿者について確定済みのローカル事実です。source_article_detail は相手の記事の事実で、返信者自身の経験へ移してはいけません。
- supplied historical facts は公開世界について使ってよい事実です。そこから未提示の価格・発売日・仕様・作品内容などを連想で追加しないでください。
- 新しい所有歴、購入歴、職歴、家族事情、長期的な嗜好や習慣を勝手に作らないでください。
- 文章は自然なら短くて構いません。1～5段落、500字程度まで。顔文字は人物に合う場合だけ。
- 1990年代らしさを小道具で演出せず、その時代の本人として普通に書いてください。

JSONだけを返してください:
{"author":"...","subject":"...","body":"..."}`, subject, req.BoardTopic, persona, intent, facts, req.WorldDate, eraRules)
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
''')

# Replace OpenAI article worker body with the shared prompt. Planning methods in
# this file remain untouched.
path = "apps/server/internal/llm/openai.go"
p = Path(path)
s = p.read_text()
start = s.index("func (p OpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {")
end = s.index("\ntype responseTextResult struct", start)
newfunc = r'''func (p OpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	prompt := BuildBoardPostPrompt(req)
	result, err := p.responseText(ctx, prompt, "low")
	if err != nil {
		return BoardPostDraft{}, err
	}
	var draft BoardPostDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BoardPostDraft{}, fmt.Errorf("decode board post JSON: %w", err)
	}
	if err := validateBoardPostWorkerDraft(req, draft); err != nil {
		return BoardPostDraft{}, err
	}
	draft.Author = strings.ToUpper(strings.TrimSpace(draft.Author))
	draft.Subject = strings.TrimSpace(draft.Subject)
	draft.Body = normalizeCRLF(draft.Body)
	draft.Usage = result.Usage
	return draft, nil
}
'''
p.write_text(s[:start] + newfunc + s[end:])

# Native Gemini Interactions API provider. Only BoardPostRenderer is needed for
# the A/B experiment; world planning stays on the existing OpenAI renderer.
Path("apps/server/internal/llm/gemini.go").write_text(r'''package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

type GeminiProvider struct {
    APIKey   string
    Model    string
    Client   *http.Client
    Endpoint string
}

type geminiInteractionResponse struct {
    Status string `json:"status"`
    Steps []struct {
        Type string `json:"type"`
        Content []struct {
            Type string `json:"type"`
            Text string `json:"text"`
        } `json:"content"`
    } `json:"steps"`
    Usage struct {
        TotalInputTokens   int `json:"total_input_tokens"`
        TotalOutputTokens  int `json:"total_output_tokens"`
        TotalThoughtTokens int `json:"total_thought_tokens"`
        TotalTokens        int `json:"total_tokens"`
    } `json:"usage"`
}

func (p GeminiProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
    if strings.TrimSpace(p.APIKey) == "" {
        return BoardPostDraft{}, errors.New("GEMINI_API_KEY is not set")
    }
    model := strings.TrimSpace(p.Model)
    if model == "" {
        model = "gemini-3.8-flash"
    }
    endpoint := strings.TrimSpace(p.Endpoint)
    if endpoint == "" {
        endpoint = "https://generativelanguage.googleapis.com/v1beta/interactions"
    }
    schema := map[string]any{
        "type": "object",
        "properties": map[string]any{
            "author": map[string]any{"type": "string"},
            "subject": map[string]any{"type": "string"},
            "body": map[string]any{"type": "string"},
        },
        "required": []string{"author", "subject", "body"},
    }
    payload := map[string]any{
        "model": model,
        "input": BuildBoardPostPrompt(req),
        "generation_config": map[string]any{"thinking_level": "low"},
        "response_format": map[string]any{
            "type": "text", "mime_type": "application/json", "schema": schema,
        },
    }
    raw, err := json.Marshal(payload)
    if err != nil { return BoardPostDraft{}, err }
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
    if err != nil { return BoardPostDraft{}, err }
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-goog-api-key", p.APIKey)
    client := p.Client
    if client == nil { client = &http.Client{Timeout: 90 * time.Second} }
    resp, err := client.Do(httpReq)
    if err != nil { return BoardPostDraft{}, err }
    defer resp.Body.Close()
    body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
    if err != nil { return BoardPostDraft{}, err }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return BoardPostDraft{}, fmt.Errorf("gemini interactions HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
    }
    var interaction geminiInteractionResponse
    if err := json.Unmarshal(body, &interaction); err != nil {
        return BoardPostDraft{}, fmt.Errorf("decode gemini interaction: %w", err)
    }
    var texts []string
    for _, step := range interaction.Steps {
        if step.Type != "model_output" { continue }
        for _, content := range step.Content {
            if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
                texts = append(texts, content.Text)
            }
        }
    }
    if len(texts) == 0 { return BoardPostDraft{}, fmt.Errorf("gemini interaction returned no model text (status=%s)", interaction.Status) }
    var draft BoardPostDraft
    if err := json.Unmarshal([]byte(strings.Join(texts, "\n")), &draft); err != nil {
        return BoardPostDraft{}, fmt.Errorf("decode gemini board post JSON: %w", err)
    }
    if err := validateBoardPostWorkerDraft(req, draft); err != nil { return BoardPostDraft{}, err }
    draft.Author = strings.ToUpper(strings.TrimSpace(draft.Author))
    draft.Subject = strings.TrimSpace(draft.Subject)
    draft.Body = normalizeCRLF(draft.Body)
    total := interaction.Usage.TotalTokens
    if total == 0 { total = interaction.Usage.TotalInputTokens + interaction.Usage.TotalOutputTokens + interaction.Usage.TotalThoughtTokens }
    draft.Usage = TokenUsage{
        InputTokens: interaction.Usage.TotalInputTokens,
        OutputTokens: interaction.Usage.TotalOutputTokens,
        ReasoningTokens: interaction.Usage.TotalThoughtTokens,
        TotalTokens: total,
        Model: model,
    }
    return draft, nil
}
''')

# Give both providers the exact same producer/worker contract.
Path("apps/server/internal/llm/structured_board_worker.go").write_text(r'''package llm

import (
    "context"
    "strings"
)

func prepareStructuredBoardPostRequest(req BoardPostRequest) BoardPostRequest {
    if strings.Contains(req.PostIntent, "producer_event_id=") {
        req.PostIntent = `ARTICLE WORKER CONTRACT:
The host-window PRODUCER already coordinated this article with the rest of the world window. All producer_* fields below are canonical production instructions, not suggestions.
Use them as content facts, but do not narrate these field names or any internal planning metadata.
Do not replace producer_episode, invent a different referent, give the actor knowledge outside producer_actor_knowledge/context, omit producer_required_contribution, or violate producer_must_not.
If producer_audience_context says a referent is not shared, do not use unexplained shorthand as though readers already know it.
If a concrete external product/place name is absent from the brief and historical facts, do not invent one merely for specificity.

` + req.PostIntent
    }
    return req
}

func (p StructuredOpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
    return p.OpenAIProvider.GenerateBoardPost(ctx, prepareStructuredBoardPostRequest(req))
}

type StructuredGeminiProvider struct{ GeminiProvider }

func (p StructuredGeminiProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
    return p.GeminiProvider.GenerateBoardPost(ctx, prepareStructuredBoardPostRequest(req))
}
''')

# Worker context contains only conversation content. The diagnostic context can
# stay rich, but is no longer sent to the prose model.
Path("apps/server/internal/worldrepo/materialization_worker_context.go").write_text(r'''package worldrepo

import (
    "fmt"
    "strings"

    "zutto-pccom/apps/server/internal/world"
)

func (r *Repository) materializationArticleWorkerContext(host world.Host, board world.Board, selected world.Post) string {
    if selected.ParentID == 0 && selected.Intent.SourcePostID == 0 {
        return ""
    }
    all := r.Base.ListPosts(host.ID)
    seen := map[int64]bool{}
    var b strings.Builder
    b.WriteString("THREAD CONTEXT (canonical article content only):\n")
    add := func(post world.Post) {
        if post.ID == selected.ID || seen[post.ID] { return }
        seen[post.ID] = true
        fmt.Fprintf(&b, "[%s] %s\n", post.Author, strings.TrimSpace(post.Subject))
        if body := strings.TrimSpace(post.Body); body != "" {
            b.WriteString(truncateDemoContext(body, 420))
            b.WriteString("\n")
        } else if summary := strings.TrimSpace(post.Intent.SituationSummary); summary != "" {
            b.WriteString(summary)
            b.WriteString("\n")
        }
    }
    if selected.Intent.SourcePostID != 0 {
        if source, ok := developmentConversationFindPost(all, selected.Intent.SourcePostID); ok && postBefore(source, selected) { add(source) }
    }
    if selected.ParentID != 0 {
        for _, prior := range r.materializationThreadPredecessors(host.ID, board.ID, selected) { add(prior) }
    }
    return strings.TrimSpace(b.String())
}

func workerRelevantSituationFact(fact string) bool {
    fact = strings.TrimSpace(fact)
    for _, prefix := range []string{
        "article_detail=", "source_article_detail=", "world_adopted_summary=", "source_world_adopted_summary=",
        "focus=", "occurrence=", "scope_boundary=", "source_focus=", "source_occurrence=", "source_scope_boundary=",
        "continuation=", "source_fact=",
    } {
        if strings.HasPrefix(fact, prefix) { return true }
    }
    return false
}
''')

# Replace the worker intent packet: no IDs, timestamps, routing labels or causes.
path = "apps/server/internal/worldrepo/llm_materializer.go"
p = Path(path)
s = p.read_text()
start = s.index("func intentSummary(i world.PostIntent) string {")
end = s.index("\nfunc usableClaims", start)
newfunc = r'''func intentSummary(i world.PostIntent) string {
	parts := make([]string, 0, 20)
	if i.DiscourseMode != "" {
		parts = append(parts, "discourse_mode="+i.DiscourseMode)
	}
	if summary := strings.TrimSpace(i.SituationSummary); summary != "" {
		parts = append(parts, "canonical_event="+summary)
	}
	for _, fact := range i.SituationFacts {
		if workerRelevantSituationFact(fact) {
			parts = append(parts, strings.TrimSpace(fact))
		}
	}
	if i.Goal != "" {
		parts = append(parts, "goal="+i.Goal)
	}
	if len(i.Claims) > 0 {
		parts = append(parts, "claims="+strings.Join(i.Claims, " / "))
	}
	if len(i.RespondsToClaims) > 0 {
		parts = append(parts, "responds_to_claims="+strings.Join(i.RespondsToClaims, " / "))
	}
	if i.ProducerEventID != "" {
		parts = append(parts, "producer_event_id="+i.ProducerEventID)
	}
	if i.ProducerEpisode != "" {
		parts = append(parts, "producer_episode="+i.ProducerEpisode)
	}
	if len(i.ProducerReferents) > 0 {
		parts = append(parts, "producer_referents="+strings.Join(i.ProducerReferents, " / "))
	}
	if len(i.ProducerActorKnowledge) > 0 {
		parts = append(parts, "producer_actor_knowledge="+strings.Join(i.ProducerActorKnowledge, " / "))
	}
	if len(i.ProducerAudienceContext) > 0 {
		parts = append(parts, "producer_audience_context="+strings.Join(i.ProducerAudienceContext, " / "))
	}
	if len(i.ProducerContribution) > 0 {
		parts = append(parts, "producer_required_contribution="+strings.Join(i.ProducerContribution, " / "))
	}
	if len(i.ProducerMustNot) > 0 {
		parts = append(parts, "producer_must_not="+strings.Join(i.ProducerMustNot, " / "))
	}
	if strings.TrimSpace(i.RenderContext) != "" {
		parts = append(parts, "conversation_context:\n"+strings.TrimSpace(i.RenderContext))
	}
	return strings.Join(parts, "\n")
}
'''
p.write_text(s[:start] + newfunc + s[end:])

# Production prose now receives the compact content-only thread context. Keep the
# full context around for diagnostics and counters.
path = "apps/server/internal/worldrepo/materialization_article_debug.go"
replace_once(path, "\trenderIntent.RenderContext = renderContext\n", "\trenderIntent.RenderContext = r.materializationArticleWorkerContext(host, board, selected)\n")

# Public helper for a non-persisting A/B render of one existing canonical article.
Path("apps/server/internal/worldrepo/materialization_article_ab.go").write_text(r'''package worldrepo

import (
    "context"
    "fmt"
    "strings"
    "time"

    "zutto-pccom/apps/server/internal/historicalkb"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldengine"
)

// MaterializationArticleWorkerABInput prepares exactly the worker input production
// would use, but does not render or persist a body. Article-detail materialization
// may persist canonical detail because that is world state shared by both arms.
func (r *Repository) MaterializationArticleWorkerABInput(host world.Host, board world.Board, postID int64) (BoardMaterializationRequest, worldengine.EvidenceDecision, world.Post, error) {
    selected, found := r.findMaterializationPost(host.ID, board.ID, postID)
    if !found { return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, world.Post{}, fmt.Errorf("post not found") }
    if developmentInteractiveTitleFirstEnabled(r) {
        var err error
        selected, _, err = r.materializeInteractiveTitleArticleDetails(host, board, selected)
        if err != nil { return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, err }
    }
    if r.Engine == nil { return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, fmt.Errorf("world engine unavailable") }
    var persona *world.Persona
    if ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {
        if p, found := ps.PersonaByID(selected.AuthorPersonaID); found { copy := p; persona = &copy }
    }
    ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
    defer cancel()
    decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
        Kind: historicalkb.KnowledgeCulturalSignal, Subject: board.Name,
        WorldDate: selected.CreatedAt.Format("2006-01-02"), Region: host.Region,
        Audience: []string{host.SoftwareID},
        Need: fmt.Sprintf("%s の %s ボード、%sによる件名『%s』の記事本文を、確定済みの投稿意図を変えず1996年の自然なパソコン通信文体で補完する", host.Name, board.Name, selected.Author, selected.Subject),
        Persistence: true, Importance: .30, Specificity: .30,
    })
    if err != nil { return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, err }
    boardTopic := selected.Subject
    canonicalSubject := selected.Subject
    if developmentConversationViewPoCEnabled(r) { boardTopic = board.Name; canonicalSubject = "" }
    if fixed := titleFirstSubject(selected.Intent.SituationFacts); fixed != "" { canonicalSubject = fixed }
    renderIntent := selected.Intent
    renderIntent.RenderContext = r.materializationArticleWorkerContext(host, board, selected)
    // Explicitly keep the A/B packet free of diagnostic context.
    if strings.Contains(renderIntent.RenderContext, "MSG ") { return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, fmt.Errorf("worker context unexpectedly contains MSG metadata") }
    req := BoardMaterializationRequest{Host: host, BoardID: board.ID, BoardTopic: boardTopic, WorldDate: selected.CreatedAt.Format("2006-01-02"), Persona: persona, Intent: renderIntent, CanonicalSubject: canonicalSubject}
    return req, decision, selected, nil
}
''')

# Server-side A/B endpoint. It never writes either model's body to the world DB.
Path("apps/server/cmd/server/article_worker_ab.go").write_text(r'''package main

import (
    "context"
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "time"

    "zutto-pccom/apps/server/internal/llm"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldrepo"
)

type articleWorkerABArm struct {
    Model string `json:"model"`
    Body string `json:"body,omitempty"`
    Subject string `json:"subject,omitempty"`
    LatencyMS int64 `json:"latency_ms"`
    Error string `json:"error,omitempty"`
}

func newArticleWorkerABHandler(repo *worldrepo.Repository, base worldrepo.LLMMaterializer, gemini llm.BoardPostRenderer, geminiConfigured bool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if !geminiConfigured { w.WriteHeader(http.StatusServiceUnavailable); _ = json.NewEncoder(w).Encode(map[string]any{"error":"GEMINI_API_KEY is not configured"}); return }
        phone := strings.TrimSpace(r.URL.Query().Get("phone")); if phone == "" { phone = "0450000196" }
        boardID := strings.TrimSpace(r.URL.Query().Get("board"))
        postID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("post")), 10, 64)
        if err != nil || boardID == "" { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(map[string]any{"error":"board and numeric post are required"}); return }
        host, err := repo.HostByPhone(phone); if err != nil { w.WriteHeader(http.StatusNotFound); _ = json.NewEncoder(w).Encode(map[string]any{"error":err.Error()}); return }
        boards, _ := repo.MaterializationBoards(host)
        var board world.Board; found := false
        for _, b := range boards { if b.ID == boardID { board = b; found = true; break } }
        if !found { w.WriteHeader(http.StatusNotFound); _ = json.NewEncoder(w).Encode(map[string]any{"error":"board not found"}); return }
        req, decision, selected, err := repo.MaterializationArticleWorkerABInput(host, board, postID)
        if err != nil { w.WriteHeader(http.StatusBadGateway); _ = json.NewEncoder(w).Encode(map[string]any{"error":err.Error()}); return }

        openAI := base
        geminiMat := base; geminiMat.Renderer = gemini
        type armResult struct { name string; arm articleWorkerABArm }
        ch := make(chan armResult, 2)
        run := func(name string, mat worldrepo.LLMMaterializer) {
            start := time.Now(); ctx, cancel := context.WithTimeout(r.Context(), 70*time.Second); defer cancel()
            posts, usage, err := mat.GenerateBoardPostsWithUsage(ctx, req, decision)
            arm := articleWorkerABArm{Model: usage.Model, LatencyMS: time.Since(start).Milliseconds()}
            if err != nil { arm.Error = err.Error() } else if len(posts) == 0 { arm.Error = "no post returned" } else { arm.Subject = posts[0].Subject; arm.Body = posts[0].Body }
            ch <- armResult{name:name, arm:arm}
        }
        var wg sync.WaitGroup; wg.Add(2)
        go func(){ defer wg.Done(); run("luna", openAI) }()
        go func(){ defer wg.Done(); run("gemini", geminiMat) }()
        go func(){ wg.Wait(); close(ch) }()
        arms := map[string]articleWorkerABArm{}
        for result := range ch { arms[result.name] = result.arm }
        _ = json.NewEncoder(w).Encode(map[string]any{
            "canonical": map[string]any{"post":selected.ID,"board":board.Name,"author":selected.Author,"subject":selected.Subject,"worker_intent":worldrepo.DebugIntentSummary(req.Intent)},
            "luna": arms["luna"], "gemini": arms["gemini"], "persisted": false,
        })
    }
}
''')

# Tiny exported diagnostic helper so cmd/server does not duplicate intent packet logic.
Path("apps/server/internal/worldrepo/materialization_worker_debug.go").write_text(r'''package worldrepo

import "zutto-pccom/apps/server/internal/world"

func DebugIntentSummary(intent world.PostIntent) string { return intentSummary(intent) }
''')

# Wire Gemini comparison provider and endpoint.
path = "apps/server/cmd/server/main.go"
replace_once(path,
    "\tpostRenderer := llm.StructuredOpenAIProvider{OpenAIProvider: llm.OpenAIProvider{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, Client: &http.Client{Timeout: 90 * time.Second}}}\n\tpostMaterializer := worldrepo.LLMMaterializer{Renderer: postRenderer, Fallback: worldrepo.FallbackMaterializer{}, HistoricalReferencesEnabled: cfg.HistoricalReferencesEnabled, CuratedHistoricalReferences: true}\n",
    "\tpostRenderer := llm.StructuredOpenAIProvider{OpenAIProvider: llm.OpenAIProvider{APIKey: cfg.OpenAIKey, Model: cfg.OpenAIModel, Client: &http.Client{Timeout: 90 * time.Second}}}\n\tgeminiRenderer := llm.StructuredGeminiProvider{GeminiProvider: llm.GeminiProvider{APIKey: cfg.GeminiKey, Model: cfg.GeminiModel, Client: &http.Client{Timeout: 90 * time.Second}}}\n\tpostMaterializer := worldrepo.LLMMaterializer{Renderer: postRenderer, Fallback: worldrepo.FallbackMaterializer{}, HistoricalReferencesEnabled: cfg.HistoricalReferencesEnabled, CuratedHistoricalReferences: true}\n")
replace_once(path,
    "\tmux.HandleFunc(\"/api/debug/materialization-lab-fresh-view\", materializationLab.freshViewerHandler())\n",
    "\tmux.HandleFunc(\"/api/debug/materialization-lab-fresh-view\", materializationLab.freshViewerHandler())\n\tmux.HandleFunc(\"/api/debug/article-worker-ab\", newArticleWorkerABHandler(runtimeStore, postMaterializer, geminiRenderer, cfg.GeminiKey != \"\"))\n")
replace_once(path,
    "\"openai_model\":cfg.OpenAIModel,\"research_auth\"",
    "\"openai_model\":cfg.OpenAIModel,\"gemini_model\":cfg.GeminiModel,\"gemini_article_worker_ab\":cfg.GeminiKey!=\"\",\"research_auth\"")

# Env docs.
path = ".env.example"
replace_once(path, "OPENAI_MODEL=gpt-5.6-luna\n", "OPENAI_MODEL=gpt-5.6-luna\nGEMINI_API_KEY=\nGEMINI_MODEL=gemini-3.8-flash\n")

# Tests: shared prompt, metadata validator, Gemini wire format, compact worker packet.
Path("apps/server/internal/llm/board_post_prompt_test.go").write_text(r'''package llm

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestBuildBoardPostPromptOmitsInfrastructureMetadata(t *testing.T) {
    prompt := BuildBoardPostPrompt(BoardPostRequest{BoardTopic:"音楽", WorldDate:"1996-06-07", AuthorHandle:"MARI", CanonicalSubject:"YMOを聴き直しています", PostIntent:"article_detail=observation:音の重なり方が以前より気になった"})
    for _, bad := range []string{"Host software family", "Board ID", "CURRENT WORLD SLOT", "routing_domain"} {
        if strings.Contains(prompt, bad) { t.Fatalf("prompt leaked infrastructure metadata %q", bad) }
    }
    if !strings.Contains(prompt, "YMOを聴き直しています") || !strings.Contains(prompt, "article_detail=") { t.Fatalf("canonical content missing: %s", prompt) }
}

func TestValidateBoardPostWorkerDraftRejectsHeaderNarration(t *testing.T) {
    req := BoardPostRequest{}
    for _, body := range []string{"5/22 00:27、新スレです。おすすめありますか。", "音楽板のMSG 1201です。YMOの話です。", "音楽板から失礼します。"} {
        if err := validateBoardPostWorkerDraft(req, BoardPostDraft{Author:"MARI", Subject:"x", Body:body}); err == nil { t.Fatalf("accepted metadata narration: %q", body) }
    }
}

func TestGeminiProviderUsesInteractionsStructuredOutput(t *testing.T) {
    var got map[string]any
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("x-goog-api-key") != "secret" { t.Errorf("missing gemini key header") }
        _ = json.NewDecoder(r.Body).Decode(&got)
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"{\\"author\\":\\"MARI\\",\\"subject\\":\\"YMOを聴き直しています\\",\\"body\\":\\"最近またYMOを聴いています。音の重なり方が前より気になります。\\"}"}]}],"usage":{"total_input_tokens":10,"total_output_tokens":20,"total_thought_tokens":3,"total_tokens":33}}`))
    })); defer srv.Close()
    p := GeminiProvider{APIKey:"secret", Model:"gemini-3.8-flash", Endpoint:srv.URL, Client:srv.Client()}
    draft, err := p.GenerateBoardPost(context.Background(), BoardPostRequest{BoardTopic:"音楽", WorldDate:"1996-06-07", AuthorHandle:"MARI", CanonicalSubject:"YMOを聴き直しています"})
    if err != nil { t.Fatal(err) }
    if draft.Usage.Model != "gemini-3.8-flash" || draft.Usage.TotalTokens != 33 { t.Fatalf("usage=%+v", draft.Usage) }
    if got["model"] != "gemini-3.8-flash" { t.Fatalf("model=%v", got["model"]) }
    cfg := got["generation_config"].(map[string]any); if cfg["thinking_level"] != "low" { t.Fatalf("thinking=%v", cfg) }
    rf := got["response_format"].(map[string]any); if rf["mime_type"] != "application/json" { t.Fatalf("response_format=%v", rf) }
}
''')

Path("apps/server/internal/worldrepo/materialization_worker_context_test.go").write_text(r'''package worldrepo

import (
    "strings"
    "testing"

    "zutto-pccom/apps/server/internal/world"
)

func TestIntentSummaryForWorkerDropsPlanningMetadataButKeepsArticleFacts(t *testing.T) {
    i := world.PostIntent{Action:"thread_start", AnchorKey:"music", CauseKind:"recent_salience", Topic:"music", Motivation:"internal cause", DiscourseMode:"share_experience", SituationSummary:"YMOを最近また聴いている", SituationFacts:[]string{"article_detail=observation:音の重なり方が以前より気になった", "title_first_subject=YMOを聴き直しています"}}
    got := intentSummary(i)
    for _, bad := range []string{"action=", "internal_routing_domain", "world_cause=", "motivation="} { if strings.Contains(got,bad) { t.Fatalf("worker packet leaked %q: %s", bad, got) } }
    if !strings.Contains(got,"canonical_event=") || !strings.Contains(got,"article_detail=") { t.Fatalf("worker facts missing: %s", got) }
    if strings.Contains(got,"title_first_subject=") { t.Fatalf("subject metadata leaked: %s", got) }
}

func TestArticleWorkerContextHasNoMSGIdsOrTimestamps(t *testing.T) {
    base := world.NewMemoryStore(); repo := New(base,nil,nil,"1996-08-29")
    host := world.Host{ID:"h"}; root := base.AddPost(host.ID, world.Post{BoardID:"5", Author:"MARI", Subject:"YMOを聴き直しています", Body:"最近また聴いています。"})
    reply := world.Post{ID:root.ID+1, BoardID:"5", ParentID:root.ID, Author:"YUKI", Subject:"Re: YMOを聴き直しています", Intent:world.PostIntent{SourcePostID:root.ID}}
    got := repo.materializationArticleWorkerContext(host, world.Board{ID:"5",Name:"音楽"}, reply)
    if strings.Contains(got,"MSG") || strings.Contains(got,"board=") || strings.Contains(got,"01/02") { t.Fatalf("metadata leaked: %s", got) }
    if !strings.Contains(got,"[MARI] YMOを聴き直しています") { t.Fatalf("source content missing: %s", got) }
}
''')

# Update old test contract that explicitly expected MSG debug context in the prose packet.
path = "apps/server/internal/worldrepo/materialization_bbs_context_test.go"
p = Path(path)
s = p.read_text()
s = s.replace("func TestIntentSummaryCarriesFreeFormGoalAndTransientBBSContext", "func TestIntentSummaryCarriesFreeFormGoalAndContentOnlyBBSContext")
s = s.replace('RenderContext: "THREAD SO FAR\\n[MSG 1002 TAKA] ..."', 'RenderContext: "THREAD CONTEXT\\n[TAKA] 件名\\n本文"')
s = s.replace('!strings.Contains(summary, "MSG 1002 TAKA")', '!strings.Contains(summary, "[TAKA] 件名")')
s = s.replace('"bbs_context:"', '"conversation_context:"')
p.write_text(s)

# Document A/B boundary.
with Path("docs/MATERIALIZATION_LAB.md").open("a") as f:
    f.write(r'''

## Article worker A/B (OpenAI / Gemini)

通常の `0450000196` に既に存在するcanonical記事について、`GET /api/debug/article-worker-ab?phone=0450000196&board=<id>&post=<msg>` で最終本文workerだけを比較できる。Article Detail・人物・件名・史実判定は1つのworld inputを共有し、OpenAI側と `gemini-3.8-flash` 側の本文はどちらもDBへ保存しない。

本文workerへ渡すcontextはworld/planningの正本そのものではなく、文章化に必要なcanonical event/detail/source conversationだけへ縮約する。MSG番号、日時、board id、routing/cause等の開発メタデータは本文workerへ渡さず、出力validatorでもMSG番号・ヘッダ日時の読み上げ・「新スレです」「〜板から失礼します」等を拒否する。通常runtimeの採用workerは比較結果が出るまでOpenAIのまま。
''')
