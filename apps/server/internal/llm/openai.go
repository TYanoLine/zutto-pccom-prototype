package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
)

// OpenAIProvider targets Azure OpenAI's OpenAI-compatible v1 REST surface.
// It intentionally uses net/http so provider authentication and endpoint
// selection remain explicit at the infrastructure boundary.
type OpenAIProvider struct {
	Endpoint string
	APIKey string
	Model  string
	Client *http.Client
}

func (p OpenAIProvider) GenerateReply(ctx context.Context, req ReplyRequest) (string, error) {
	minChars, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	eraRules := withDiegeticWorldFrame(req.EraRules)
	prompt := fmt.Sprintf(`You are writing one Japanese grass-roots BBS post as the specified persona.
World date: %s. Never use knowledge, products, slang, or events after this date.
The human-controlled member is not special. Do not flatter them. It is acceptable to disagree, be terse, or have little to say.
Do not mention AI, simulation, prompts, or modern social media.
Use a plausible 1996 Japanese BBS writing style, but do not overuse emoticons.
Host: %s
Persona: %s
Era rules:
%s
Incoming subject: %s
Incoming body:
%s

返信制約:
- 新しく書く非引用部分は%d〜%d文字を目標にする。ただし水増ししない。
- 親記事件名: %s
- 親記事本文:
%s
- 選択済みの引用（改変禁止）:
%s

Return only the post body.`, req.WorldDate, req.HostName, req.Persona, eraRules, req.Subject, req.Body, minChars, maxChars, req.ParentSubject, req.ParentBody, req.QuoteText)
	result, err := p.responseTextWithLimit(ctx, prompt, "low", outputTokenBudget(maxChars))
	if err != nil {
		return "", err
	}
	text := result.Text
	if req.QuoteText != "" {
		text, err = ensureExactQuote(text, req.QuoteText)
		if err != nil {
			return "", err
		}
	}
	return normalizeCRLF(text), nil
}

// GenerateBBSTimelineIntent proposes semantic content for event shells already
// selected by the world layer. There is deliberately no topic menu, subject bank,
// information-slot catalog, or canned conversation-act enum in this request.
func (p OpenAIProvider) GenerateBBSTimelineIntent(ctx context.Context, req BBSTimelineIntentRequest) (BBSTimelineIntentDraft, error) {
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	recent := strings.TrimSpace(req.RecentBBSState)
	if recent == "" {
		recent = "(no earlier materialized BBS state supplied)"
	}
	eraRules := withDiegeticWorldFrame(req.EraRules)
	prompt := fmt.Sprintf(`Propose semantic content for a bounded sequence of events in a fictional Japanese grass-roots BBS world.

The WORLD LAYER has already decided every event's actor, time, action topology, routing anchor and cause. You MUST NOT change those facts or turn persona background into a new posting reason.

There is intentionally NO fixed topic list, subject template bank, information-slot checklist, or response-act menu. anchor_key is internal routing metadata, not the actor's wording and not evidence that the category itself is noteworthy.

WORLD / BOARD:
- world date: %s
- host: %s
- region: %s
- host software family: %s
- board id: %s
- board name: %s
- era rules: %s

EARLIER BBS STATE:
%s

WORLD-SELECTED EVENT SHELLS (JSON):
%s

Rules for each event:
- Preserve index, author, timestamp, action, parent/source topology, anchor_key, cause_kind, and canonical_subject from the event shell.
- Follow DIEGETIC PRESENT / ERA NORMALITY: the actor lives inside this date. Ordinary contemporary technology, media, BBS participation, local life and hobbies are normally unmarked, not retro props or things to rediscover.
- Existing persona facts, interests and everyday_baseline are background/consistency context. A broad interest key is not itself the event and should not be echoed as a generic topic.
- Do not invent a hiatus, nostalgia, purchase, upgrade, new arrival, membership growth, maintenance, compatibility surprise or other world change merely to make the broad routing domain post-worthy.
- For recent_salience, make the post about the concrete contemporaneously meaningful difference/problem/decision/interaction/question/observation described or permitted by cause_summary; ordinary use of the domain itself is not news.
- For continuation_progress, add materially new progress/change/observation rather than restating an old preference or baseline.
- For observed_thread, genuinely respond to the selected thread without inventing an unrelated event.
- For a root post, create a natural Japanese BBS subject of at most 36 characters. Do not select from a canned list and do not mechanically repeat anchor_key.
- For a reply, subject may follow the canonical reply subject supplied by the world layer.
- topic is a short free-form semantic description of the concrete event, not an enum and not a repetition of the routing label.
- motivation, stance, and goal are free-form descriptions of this exact event.
- facts contains zero to three durable FICTIONAL PERSONAL facts that genuinely need to become concrete for this post. Do not manufacture a fact merely to add color or to justify why the post exists.
- If existing_facts contains a key, never contradict its value.
- Facts must NOT assert unprovided real-world hardware limits, exact product specifications, release dates, prices, historical events, or other external historical facts.
- discourse_mode is selected by the world layer for root posts and is not editable semantic decoration. share_observation/share_experience/state_opinion/share_tip must remain non-question roots; do not add a request for replies or an “anyone else?” ending. ask_peers is the one mode where audience solicitation is the point.
- Do not invent a question merely to keep the thread alive.
- The human-controlled member is not special and need not be mentioned.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones, or anything after the world date.

Return ONLY JSON in this exact top-level shape:
{"events":[{"index":0,"subject":"...","topic":"...","motivation":"...","stance":"...","goal":"...","facts":[{"key":"...","value":"..."}]}]}
Return exactly one event object for every supplied event index.`, req.WorldDate, req.HostName, req.HostRegion, req.HostSoftware, req.BoardID, req.BoardName, eraRules, recent, string(eventsJSON))
	maxTokens := 1800 + len(req.Events)*220
	if maxTokens > 6000 {
		maxTokens = 6000
	}
	result, err := p.responseTextWithLimit(ctx, prompt, "low", maxTokens)
	if err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	var draft BBSTimelineIntentDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BBSTimelineIntentDraft{}, fmt.Errorf("decode BBS timeline intent JSON: %w", err)
	}
	if err := validateBBSTimelineIntentDraft(req, draft); err != nil {
		return BBSTimelineIntentDraft{}, err
	}
	for i := range draft.Events {
		draft.Events[i].Subject = strings.TrimSpace(draft.Events[i].Subject)
		draft.Events[i].Topic = strings.TrimSpace(draft.Events[i].Topic)
		draft.Events[i].Motivation = strings.TrimSpace(draft.Events[i].Motivation)
		draft.Events[i].Stance = strings.TrimSpace(draft.Events[i].Stance)
		draft.Events[i].Goal = strings.TrimSpace(draft.Events[i].Goal)
		for j := range draft.Events[i].Facts {
			draft.Events[i].Facts[j].Key = strings.TrimSpace(strings.ToLower(draft.Events[i].Facts[j].Key))
			draft.Events[i].Facts[j].Value = strings.TrimSpace(draft.Events[i].Facts[j].Value)
		}
	}
	draft.Usage = result.Usage
	return draft, nil
}

func (p OpenAIProvider) GenerateBoardPost(ctx context.Context, req BoardPostRequest) (BoardPostDraft, error) {
	prompt := BuildBoardPostPrompt(req)
	_, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	result, err := p.responseTextWithLimit(ctx, prompt, "low", outputTokenBudget(maxChars))
	if err != nil {
		return BoardPostDraft{}, err
	}
	var draft BoardPostDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BoardPostDraft{}, fmt.Errorf("decode board post JSON: %w", err)
	}
	if req.QuoteText != "" {
		draft.Body, err = ensureExactQuote(draft.Body, req.QuoteText)
		if err != nil {
			return BoardPostDraft{}, err
		}
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

type responseTextResult struct {
	Text             string
	Usage            TokenUsage
	WebSearchCalls   int
	WebSearchSources []string
}

func (p OpenAIProvider) responseText(ctx context.Context, prompt, verbosity string) (responseTextResult, error) {
	return p.responseTextWithLimit(ctx, prompt, verbosity, 1200)
}

func (p OpenAIProvider) responseTextWithLimit(ctx context.Context, prompt, verbosity string, maxOutputTokens int) (responseTextResult, error) {
	if p.APIKey == "" {
		return responseTextResult{}, errors.New("AZURE_OPENAI_API_KEY is not set")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1200
	}
	payload := map[string]any{"model": p.Model, "input": prompt, "text": map[string]any{"verbosity": verbosity}, "max_output_tokens": maxOutputTokens}
	body, _ := json.Marshal(payload)
	endpoint, err := azureopenai.URL(p.Endpoint, "responses")
	if err != nil { return responseTextResult{}, err }
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil { return responseTextResult{}, err }
	if err := azureopenai.ApplyAPIKey(httpReq, p.APIKey); err != nil { return responseTextResult{}, err }
	resp, err := client.Do(httpReq)
	if err != nil {
		return responseTextResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseTextResult{}, fmt.Errorf("Azure OpenAI responses API returned %s", resp.Status)
	}
	var decoded struct {
		Model  string `json:"model"`
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens       int `json:"input_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputTokens       int `json:"output_tokens"`
			OutputTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return responseTextResult{}, err
	}
	usage := TokenUsage{
		InputTokens:       decoded.Usage.InputTokens,
		CachedInputTokens: decoded.Usage.InputTokenDetails.CachedTokens,
		OutputTokens:      decoded.Usage.OutputTokens,
		ReasoningTokens:   decoded.Usage.OutputTokenDetails.ReasoningTokens,
		TotalTokens:       decoded.Usage.TotalTokens,
		Model:             decoded.Model,
	}
	if usage.Model == "" {
		usage.Model = p.Model
	}
	for _, out := range decoded.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				return responseTextResult{Text: c.Text, Usage: usage}, nil
			}
		}
	}
	return responseTextResult{}, errors.New("no output_text in Azure OpenAI response")
}

func validateBBSTimelineIntentDraft(req BBSTimelineIntentRequest, d BBSTimelineIntentDraft) error {
	if len(d.Events) != len(req.Events) {
		return fmt.Errorf("BBS intent event count=%d want=%d", len(d.Events), len(req.Events))
	}
	wanted := make(map[int]BBSIntentEvent, len(req.Events))
	for _, event := range req.Events {
		wanted[event.Index] = event
	}
	seen := map[int]bool{}
	for _, event := range d.Events {
		shell, ok := wanted[event.Index]
		if !ok || seen[event.Index] {
			return fmt.Errorf("BBS intent contains unexpected/duplicate event index %d", event.Index)
		}
		seen[event.Index] = true
		if strings.TrimSpace(event.Subject) == "" || len([]rune(strings.TrimSpace(event.Subject))) > 36 {
			return fmt.Errorf("BBS intent subject invalid for event %d", event.Index)
		}
		if shell.CanonicalSubject != "" && strings.TrimSpace(event.Subject) != strings.TrimSpace(shell.CanonicalSubject) {
			return fmt.Errorf("BBS intent changed canonical subject for event %d", event.Index)
		}
		if strings.TrimSpace(event.Topic) == "" || len([]rune(event.Topic)) > 120 {
			return fmt.Errorf("BBS intent topic invalid for event %d", event.Index)
		}
		if len([]rune(event.Motivation)) > 320 || len([]rune(event.Stance)) > 320 || strings.TrimSpace(event.Goal) == "" || len([]rune(event.Goal)) > 420 {
			return fmt.Errorf("BBS intent semantic text invalid for event %d", event.Index)
		}
		if len(event.Facts) > 3 {
			return fmt.Errorf("BBS intent has too many durable facts for event %d", event.Index)
		}
		for _, fact := range event.Facts {
			if !validOpenFactKey(fact.Key) {
				return fmt.Errorf("BBS intent fact key %q invalid for event %d", fact.Key, event.Index)
			}
			if strings.TrimSpace(fact.Value) == "" || len([]rune(fact.Value)) > 220 {
				return fmt.Errorf("BBS intent fact value invalid for event %d", event.Index)
			}
		}
		joined := strings.ToLower(event.Subject + " " + event.Topic + " " + event.Motivation + " " + event.Stance + " " + event.Goal)
		for _, fact := range event.Facts {
			joined += " " + strings.ToLower(fact.Value)
		}
		if forbiddenFutureMetaTerm(joined) != "" {
			return fmt.Errorf("BBS intent contains forbidden future/meta term %q", forbiddenFutureMetaTerm(joined))
		}
	}
	return nil
}

func validOpenFactKey(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || len(value) > 96 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func forbiddenFutureMetaTerm(lower string) string {
	for _, forbidden := range []string{"chatgpt", "openai", "twitter", "facebook", "instagram", "スマホ", "生成ai", "生成ａｉ", "生成ＡＩ"} {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			return forbidden
		}
	}
	return ""
}

func validateBoardPostDraft(d BoardPostDraft) error {
	return validateBoardPostDraftWithBodyLimit(d, 700)
}

func validateBoardPostDraftWithBodyLimit(d BoardPostDraft, maxBodyChars int) error {
	a := strings.TrimSpace(d.Author)
	s := strings.TrimSpace(d.Subject)
	b := strings.TrimSpace(d.Body)
	if len(a) < 2 || len(a) > 12 {
		return errors.New("board post author length is invalid")
	}
	for _, r := range a {
		// Station handles may use half-width ASCII punctuation as well as
		// alphanumerics. Space/control/non-ASCII remain invalid.
		if r < 0x21 || r > 0x7e {
			return errors.New("board post author must use visible half-width ASCII")
		}
	}
	if s == "" || len([]rune(s)) > 36 {
		return errors.New("board post subject is empty or too long")
	}
	if b == "" {
		return errors.New("board post body is empty or too long")
	}
	nonQuoted := bodyWithoutQuotes(b)
	if nonQuoted == "" || len([]rune(nonQuoted)) > maxBodyChars {
		return fmt.Errorf("board post body is empty or too long (max %d non-quoted characters)", maxBodyChars)
	}
	if len([]rune(b)) > 8192 {
		return errors.New("board post body is too long (max 8192 total characters)")
	}
	lower := strings.ToLower(b + " " + s)
	if forbidden := forbiddenFutureMetaTerm(lower); forbidden != "" {
		return fmt.Errorf("board post contains forbidden future/meta term %q", forbidden)
	}
	return nil
}

func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\r\n") + "\r\n"
}
