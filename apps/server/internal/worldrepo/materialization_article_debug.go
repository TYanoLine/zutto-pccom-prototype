package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// GenerationUsage is development-only operational telemetry. It must never be
// interpreted as a world fact or influence NPC decisions.
type GenerationUsage struct {
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
	ReasoningTokens   int
	TotalTokens       int
	Model             string
}

type usageAwareMaterializer interface {
	GenerateBoardPostsWithUsage(context.Context, BoardMaterializationRequest, worldengine.EvidenceDecision) ([]world.Post, GenerationUsage, error)
}

type generationUsageKey struct {
	repo   *Repository
	postID int64
}

var developmentGenerationUsage sync.Map

// MaterializationArticleWithDebug preserves lazy materialization while enforcing
// causal prose order. If a user opens a later reply first, earlier posts in that
// same thread are rendered and committed chronologically before the selected
// reply. Thus observation order cannot rewrite the textual history of a thread.
func (r *Repository) MaterializationArticleWithDebug(host world.Host, board world.Board, postID int64) (world.Post, bool, bool, string) {
	selected, found := r.findMaterializationPost(host.ID, board.ID, postID)
	if !found {
		return world.Post{}, false, false, ""
	}
	if selected.Body != "" {
		renderContext, contextStats := r.materializationRenderContext(host, board, selected)
		_ = renderContext
		usage, _ := r.MaterializationGenerationUsage(postID)
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())
	}

	for _, predecessor := range r.materializationThreadPredecessors(host.ID, board.ID, selected) {
		if predecessor.Body != "" {
			continue
		}
		_, ok, _, diagnostic := r.materializeArticleBodyOnce(host, board, predecessor)
		if !ok || strings.Contains(diagnostic, "error stage=") {
			return selected, true, false, joinDevelopmentDiagnostics(fmt.Sprintf("error stage=dependency msg=%04d", predecessor.ID), diagnostic)
		}
	}

	selected, found = r.findMaterializationPost(host.ID, board.ID, postID)
	if !found {
		return world.Post{}, false, false, ""
	}
	return r.materializeArticleBodyOnce(host, board, selected)
}

func (r *Repository) materializeArticleBodyOnce(host world.Host, board world.Board, selected world.Post) (world.Post, bool, bool, string) {
	detailDiagnostic := ""
	if developmentInteractiveTitleFirstEnabled(r) {
		var detailErr error
		selected, detailDiagnostic, detailErr = r.materializeInteractiveTitleArticleDetails(host, board, selected)
		if detailErr != nil {
			return selected, true, false, detailDiagnostic
		}
	}
	_, contextStats := r.materializationRenderContext(host, board, selected)
	if selected.Body != "" {
		usage, _ := r.MaterializationGenerationUsage(selected.ID)
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())
	}
	if r.Engine == nil || r.Materializer == nil {
		return selected, true, false, joinDevelopmentDiagnostics("error stage=setup detail=world engine or materializer unavailable", contextStats.String())
	}

	var persona *world.Persona
	if ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {
		if p, found := ps.PersonaByID(selected.AuthorPersonaID); found {
			copy := p
			persona = &copy
		}
	}

	topicLabel := strings.TrimSpace(selected.Subject)
	if topicLabel == "" {
		topicLabel = strings.TrimSpace(selected.Intent.Topic)
	}
	if topicLabel == "" {
		topicLabel = board.Name
	}

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     board.Name,
		WorldDate:   selected.CreatedAt.Format("2006-01-02"),
		Region:      host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボード、%sによる話題『%s』の記事本文を、確定済みの投稿意図を変えず1996年の自然なパソコン通信文体で補完する", host.Name, board.Name, selected.Author, topicLabel),
		Persistence: true,
		Importance:  .30,
		Specificity: .30,
	})
	if err != nil {
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationError("evidence", err), contextStats.String())
	}

	// Subject is host-native surface data and may legitimately be empty on a
	// response (Erika-K append). Intent.Topic carries the semantic conversation
	// topic for prose generation without manufacturing a host-visible subject.
	boardTopic := topicLabel
	canonicalSubject := selected.Subject
	if developmentConversationViewPoCEnabled(r) {
		boardTopic = board.Name
		canonicalSubject = ""
	}
	if fixed := titleFirstSubject(selected.Intent.SituationFacts); fixed != "" {
		canonicalSubject = fixed
	}
	renderIntent := selected.Intent
	renderIntent.RenderContext = r.materializationArticleWorkerContext(host, board, selected)
	req := BoardMaterializationRequest{
		Host:             host,
		BoardID:          board.ID,
		BoardTopic:       boardTopic,
		WorldDate:        selected.CreatedAt.Format("2006-01-02"),
		Persona:          persona,
		Intent:           renderIntent,
		CanonicalSubject: canonicalSubject,
	}
	req = r.prepareBoardComposition(req, selected)
	var posts []world.Post
	var usage GenerationUsage
	for attempt := 0; attempt < 2; attempt++ {
		posts = nil
		usage = GenerationUsage{}
		if materializer, ok := r.Materializer.(usageAwareMaterializer); ok {
			posts, usage, err = materializer.GenerateBoardPostsWithUsage(ctx, req, decision)
		} else {
			posts, err = r.Materializer.GenerateBoardPosts(ctx, req, decision)
		}
		if err == nil && len(posts) > 0 && strings.TrimSpace(posts[0].Body) != "" {
			break
		}
		if err == nil {
			if len(posts) == 0 {
				err = fmt.Errorf("no post returned")
			} else {
				err = fmt.Errorf("empty body returned")
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationError("renderer", err), contextStats.String())
	}
	if len(posts) == 0 {
		return selected, true, false, joinDevelopmentDiagnostics("error stage=renderer detail=no post returned", contextStats.String())
	}
	if developmentConversationViewPoCEnabled(r) {
		if target := topicTargetFact(selected.Intent.SituationFacts); world.IsSemanticRoot(selected) && target != "" && !TopicTargetInSubject(posts[0].Subject, target) {
			if usage.TotalTokens > 0 || usage.Model != "" {
				developmentGenerationUsage.Store(generationUsageKey{repo: r, postID: selected.ID}, usage)
			}
			return selected, true, false, joinDevelopmentDiagnostics("error stage=subject detail=selected topic target missing", contextStats.String())
		}
		selected.Subject = r.developmentConversationRenderedSubject(host.ID, selected, posts[0].Subject)
	}
	selected.Body = posts[0].Body
	if selected.Body == "" {
		return selected, true, false, joinDevelopmentDiagnostics("error stage=renderer detail=empty body returned", contextStats.String())
	}
	if usage.TotalTokens > 0 || usage.Model != "" {
		developmentGenerationUsage.Store(generationUsageKey{repo: r, postID: selected.ID}, usage)
	}
	diagnostic := joinDevelopmentDiagnostics(detailDiagnostic, formatGenerationUsage(usage), contextStats.String())
	if updater, ok := r.Base.(world.PostUpdater); ok {
		if updated, ok := updater.UpdatePost(host.ID, selected); ok {
			return updated, true, true, diagnostic
		}
	}
	return selected, true, true, diagnostic
}

func (r *Repository) findMaterializationPost(hostID, boardID string, postID int64) (world.Post, bool) {
	for _, post := range r.Base.ListPosts(hostID) {
		if post.ID == postID && post.BoardID == boardID {
			return post, true
		}
	}
	return world.Post{}, false
}

func (r *Repository) materializationThreadPredecessors(hostID, boardID string, selected world.Post) []world.Post {
	if world.IsSemanticRoot(selected) {
		return nil
	}
	all := r.Base.ListPosts(hostID)
	postsByID := make(map[int64]world.Post, len(all))
	for _, post := range all {
		postsByID[post.ID] = post
	}
	rootID := threadRootID(postsByID, selected)
	out := make([]world.Post, 0, 8)
	for _, post := range all {
		if post.BoardID != boardID || post.ID == selected.ID || !postBefore(post, selected) {
			continue
		}
		if threadRootID(postsByID, post) == rootID {
			out = append(out, post)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (r *Repository) MaterializationGenerationUsage(postID int64) (GenerationUsage, bool) {
	value, ok := developmentGenerationUsage.Load(generationUsageKey{repo: r, postID: postID})
	if !ok {
		return GenerationUsage{}, false
	}
	usage, ok := value.(GenerationUsage)
	return usage, ok
}

func (r *Repository) MaterializationUsageTotal() GenerationUsage {
	var total GenerationUsage
	developmentGenerationUsage.Range(func(key, value any) bool {
		usageKey, ok := key.(generationUsageKey)
		if !ok || usageKey.repo != r {
			return true
		}
		if usage, ok := value.(GenerationUsage); ok {
			addGenerationUsage(&total, usage)
		}
		return true
	})
	developmentPlanningUsage.Range(func(key, value any) bool {
		planningKey, ok := key.(developmentPlanningKey)
		if !ok || planningKey.repo != r {
			return true
		}
		if usage, ok := value.(GenerationUsage); ok {
			addGenerationUsage(&total, usage)
		}
		return true
	})
	return total
}

func addGenerationUsage(total *GenerationUsage, usage GenerationUsage) {
	total.InputTokens += usage.InputTokens
	total.CachedInputTokens += usage.CachedInputTokens
	total.OutputTokens += usage.OutputTokens
	total.ReasoningTokens += usage.ReasoningTokens
	total.TotalTokens += usage.TotalTokens
	if usage.Model != "" {
		if total.Model == "" {
			total.Model = usage.Model
		} else if total.Model != usage.Model {
			total.Model = "mixed"
		}
	}
}

func (r *Repository) MaterializationUsageTotalText() string {
	total := r.MaterializationUsageTotal()
	if total.TotalTokens == 0 {
		return ""
	}
	return formatGenerationUsage(total)
}

func formatGenerationUsage(usage GenerationUsage) string {
	if usage.TotalTokens == 0 && usage.Model == "" {
		return ""
	}
	model := usage.Model
	if model == "" {
		model = "unknown"
	}
	return fmt.Sprintf("model=%s input=%d cached=%d output=%d reasoning=%d total=%d", model, usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningTokens, usage.TotalTokens)
}

func formatGenerationError(stage string, err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	runes := []rune(message)
	if len(runes) > 240 {
		message = string(runes[:240]) + "..."
	}
	return fmt.Sprintf("error stage=%s detail=%s", stage, message)
}

func joinDevelopmentDiagnostics(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, " ")
}
