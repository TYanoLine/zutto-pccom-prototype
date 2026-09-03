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
)

// OpenAIProvider intentionally uses net/http so the starter remains decoupled
// from SDK release cadence. Codex can replace this with openai-go/v3 later.
type OpenAIProvider struct {
	APIKey string
	Model  string
	Client *http.Client
}

func (p OpenAIProvider) GenerateReply(ctx context.Context, req ReplyRequest) (string, error) {
	prompt := fmt.Sprintf(`You are writing one Japanese grass-roots BBS post as the specified persona.
World date: %s. Never use knowledge, products, slang, or events after this date.
The human-controlled member is not special. Do not flatter them. It is acceptable to disagree, be terse, or have little to say.
Do not mention AI, simulation, prompts, or modern social media.
Use a plausible 1996 Japanese BBS writing style, but do not overuse emoticons.
Host: %s
Persona: %s
Era rules: %s
Incoming subject: %s
Incoming body:
%s

Return only the post body.`, req.WorldDate, req.HostName, req.Persona, req.EraRules, req.Subject, req.Body)
	result, err := p.responseText(ctx, prompt, "low")
	if err != nil {
		return "", err
	}
	return normalizeCRLF(result.Text), nil
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
	prompt := fmt.Sprintf(`Propose semantic content for a bounded sequence of events in a fictional Japanese grass-roots BBS world.

The WORLD LAYER has already decided every event's actor, time, and whether it is a root post or reply. You MUST NOT change those facts. You are proposing content semantics only; the application validates and commits accepted results as world state.

There is intentionally NO fixed topic list, subject template bank, information-slot checklist, or response-act menu. Infer each concrete post from the board, the persistent persona, earlier BBS state, and the sequence itself.

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
- Preserve index, author, timestamp, action, parent topology, and canonical_subject from the event shell.
- For a root post, create a natural Japanese BBS subject of at most 36 characters that reflects what this person actually wants to talk about now. Do not select from a canned list.
- For a reply, subject may simply follow the canonical reply subject supplied by the world layer; content should naturally continue the existing thread rather than starting an unrelated topic.
- topic is a short free-form semantic summary, not an enum or catalog key.
- motivation, stance, and goal are free-form descriptions of this exact event. goal should say what the actor is trying to communicate or react to; it does not have to advance the conversation.
- facts contains zero to three durable FICTIONAL PERSONAL facts that genuinely need to become concrete for this post. Do not manufacture a fact merely to add color. A short reaction may have zero facts.
- Each fact key is an open lowercase semantic path such as household.shared_computer or computer.communication_usage. It is NOT chosen from a predefined schema. Use the same key when the same personal dimension already appears in existing_facts.
- If existing_facts contains a key, never contradict its value. Prefer reusing it when relevant.
- Facts may describe this fictional person's ordinary ownership, habits, preferences, household situation, or experience. They must NOT assert unprovided real-world hardware limits, exact product specifications, release dates, prices, historical events, or other external historical facts.
- Do not invent a question merely to keep the thread alive. Ask only if that is naturally the actor's actual goal.
- Repetition, silence-like brevity, disagreement, or a mundane tangent can be natural. Do not optimize every post for information density.
- The human-controlled member is not special and need not be mentioned.
- Never mention AI, simulation, prompts, databases, web searches, social media, smartphones, or anything after the world date.
- Keep the sequence mutually coherent: later events can react to earlier proposed semantics, but do not make every event mechanically answer the previous one.

Return ONLY JSON in this exact top-level shape:
{"events":[{"index":0,"subject":"...","topic":"...","motivation":"...","stance":"...","goal":"...","facts":[{"key":"...","value":"..."}]}]}
Return exactly one event object for every supplied event index.`, req.WorldDate, req.HostName, req.HostRegion, req.HostSoftware, req.BoardID, req.BoardName, req.EraRules, recent, string(eventsJSON))
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
	facts := "(none supplied; keep concrete historical claims generic)"
	if len(req.HistoricalFacts) > 0 {
		facts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	persona := "(no persona preselected; choose an ordinary member suitable for the board)"
	if req.AuthorHandle != "" {
		persona = fmt.Sprintf("Handle: %s\nPersistent profile: %s", req.AuthorHandle, req.PersonaProfile)
	}
	intent := "(no precommitted semantic intent; infer a mundane board post from the cue)"
	if strings.TrimSpace(req.PostIntent) != "" {
		intent = req.PostIntent
	}
	subjectRule := "Choose a natural Japanese subject of at most 36 characters."
	if strings.TrimSpace(req.CanonicalSubject) != "" {
		subjectRule = fmt.Sprintf("The subject is already canonical world state. Return it exactly as: %s", req.CanonicalSubject)
	}
	authorRule := "Author handle: 2-12 ASCII letters/digits only."
	if strings.TrimSpace(req.AuthorHandle) != "" {
		authorRule = fmt.Sprintf("The author is already canonical world state. Return exactly this handle: %s", req.AuthorHandle)
	}
	prompt := fmt.Sprintf(`Write exactly one natural message for a Japanese grass-roots personal-computer BBS.
The actor, article header and semantic intent may already be persistent world state. Your job is to render prose consistent with those facts, not to choose a different event.

PRIMARY CONTENT CUE:
- Current board/header cue: %s
- Write about what an ordinary member would naturally mean by this cue.
- Do not pad the message with unrelated setting details merely because they are listed below.

PRECOMMITTED ACTOR:
%s

PRECOMMITTED POST INTENT AND RETRIEVED BBS CONTEXT:
%s

SEMANTIC / CONVERSATION RULES:
- If the intent contains claims=..., those are concrete fictional-world facts already decided for this person/post. Express them materially when relevant instead of replacing them with generic filler.
- goal is the free-form conversational purpose of this exact post. Follow it naturally; it is not a member of a fixed response-act list and it does not imply that the thread must advance.
- If the intent contains responds_to_claims=..., those identify an earlier point this reply is especially reacting to. Make that connection natural, but do not mechanically quote it if the thread already makes the connection obvious.
- If bbs_context is supplied, read THREAD SO FAR like prior messages in a chat. Earlier body text is canonical prose; semantic-envelope entries are canonical meaning for posts whose prose has not been materialized yet. Use this context to avoid accidental repetition and to make references/replies coherent.
- RELATED EARLIER POSTS in bbs_context are retrieval hints for duplicate-topic awareness. They do not prove the actor personally read or remembers those posts, so do not refer to them as memories unless THREAD SO FAR supports that.
- There is no obligation to ask a new question, reveal a new category of information, or keep the conversation alive. A mundane, uneven, occasionally repetitive human BBS exchange is acceptable when it fits the actor and context.
- Do not invent a new owned machine, modem, software setup, family situation, job history, or other durable personal fact merely to make the prose more specific. Durable personal details must come from claims=...; ordinary connective wording, opinions, and transient feelings are fine.
- Do not introduce a new question solely as a conversation-progression device. If the committed goal is not to ask, a natural statement can simply end.
- Do not say vague things such as "everyone has interesting setups" unless the actual supplied context supports that statement.

CANONICAL HEADER RULES:
- %s
- %s

BACKGROUND CONSTRAINTS — THESE ARE GUARDRAILS, NOT TOPICS TO MENTION:
- World date: %s
- Host: %s
- Region: %s
- Host software family: %s
- Board ID: %s
- Era rules: %s

HISTORICAL FACTS ALLOWED AS CONCRETE FACTUAL SUPPORT:
%s

Important interpretation rules:
- If an actor, subject or intent is precommitted above, do not change it. Express it naturally in the body.
- Background constraints exist to prevent contradictions. They are NOT a checklist of details to mention.
- Do not mention the region, date, season, host name, host software, or period technology unless the actual message content naturally requires it.
- Historical facts are permission/constraints for concrete claims, not suggested talking points. Omit them entirely when irrelevant.
- Never add period props such as floppy disks, magazines, modems, heat/weather, or place names just to make the prose feel "1990s".
- Natural topic focus and persona consistency are more important than demonstrating that you understood the supplied context.

Rules:
- Write as the selected independent BBS member, not as an assistant or narrator.
- The human-controlled user is not the center of the world and need not be mentioned.
- Never mention AI, simulation, prompts, web searches, databases, social media, smartphones, or anything from after the world date.
- Do not invent exact release dates, prices, model-specific availability, technical specifications, historical events, or other concrete factual claims unless they are supported by the supplied historical facts.
- When no historical facts are supplied, ordinary personal chatter, impressions, questions, habits, and mundane details are fine, but durable personal facts must still respect the precommitted claims.
- Do not imply that all members share the same opinion or equipment.
- Use plausible mid-1990s Japanese BBS prose, but avoid conspicuous era cosplay. Emoticons are optional and should follow the persona rather than being added mechanically.
- Body: Japanese, 1-5 short paragraphs, at most about 500 Japanese characters.

Return ONLY JSON with exactly these keys:
{"author":"...","subject":"...","body":"..."}`, req.BoardTopic, persona, intent, authorRule, subjectRule, req.WorldDate, req.HostName, req.HostRegion, req.HostSoftware, req.BoardID, req.EraRules, facts)
	result, err := p.responseText(ctx, prompt, "low")
	if err != nil {
		return BoardPostDraft{}, err
	}
	var draft BoardPostDraft
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Text)), &draft); err != nil {
		return BoardPostDraft{}, fmt.Errorf("decode board post JSON: %w", err)
	}
	if err := validateBoardPostDraft(draft); err != nil {
		return BoardPostDraft{}, err
	}
	draft.Author = strings.ToUpper(strings.TrimSpace(draft.Author))
	draft.Subject = strings.TrimSpace(draft.Subject)
	draft.Body = normalizeCRLF(draft.Body)
	draft.Usage = result.Usage
	return draft, nil
}

type responseTextResult struct {
	Text  string
	Usage TokenUsage
}

func (p OpenAIProvider) responseText(ctx context.Context, prompt, verbosity string) (responseTextResult, error) {
	return p.responseTextWithLimit(ctx, prompt, verbosity, 1200)
}

func (p OpenAIProvider) responseTextWithLimit(ctx context.Context, prompt, verbosity string, maxOutputTokens int) (responseTextResult, error) {
	if p.APIKey == "" {
		return responseTextResult{}, errors.New("OPENAI_API_KEY is not set")
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return responseTextResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return responseTextResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseTextResult{}, fmt.Errorf("openai responses API returned %s", resp.Status)
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
	return responseTextResult{}, errors.New("no output_text in OpenAI response")
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
	a := strings.TrimSpace(d.Author)
	s := strings.TrimSpace(d.Subject)
	b := strings.TrimSpace(d.Body)
	if len(a) < 2 || len(a) > 12 {
		return errors.New("board post author length is invalid")
	}
	for _, r := range a {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return errors.New("board post author must be ASCII alphanumeric")
		}
	}
	if s == "" || len([]rune(s)) > 36 {
		return errors.New("board post subject is empty or too long")
	}
	if b == "" || len([]rune(b)) > 700 {
		return errors.New("board post body is empty or too long")
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
