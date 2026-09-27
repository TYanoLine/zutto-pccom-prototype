package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// MaterializationArticleWithDebugTimeout is a development-lab variant of
// MaterializationArticleWithDebug. It runs the same evidence + renderer path but
// lets the experiment harness vary the per-article deadline without changing the
// production 35s policy. It is intended to be called only on an isolated store.
func (r *Repository) MaterializationArticleWithDebugTimeout(host world.Host, board world.Board, postID int64, timeout time.Duration) (world.Post, bool, bool, string) {
	if timeout <= 0 {
		timeout = 35 * time.Second
	}
	selected, found := r.findMaterializationPost(host.ID, board.ID, postID)
	if !found {
		return world.Post{}, false, false, ""
	}
	if selected.Body != "" {
		renderContext, contextStats := r.materializationBBSRenderContext(host, board, selected)
		_ = renderContext
		usage, _ := r.MaterializationGenerationUsage(postID)
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())
	}

	for _, predecessor := range r.materializationThreadPredecessors(host.ID, board.ID, selected) {
		if predecessor.Body != "" {
			continue
		}
		_, ok, _, diagnostic := r.materializeArticleBodyOnceWithTimeout(host, board, predecessor, timeout)
		if !ok || strings.Contains(diagnostic, "error stage=") {
			return selected, true, false, joinDevelopmentDiagnostics(fmt.Sprintf("error stage=dependency msg=%04d", predecessor.ID), diagnostic)
		}
	}

	selected, found = r.findMaterializationPost(host.ID, board.ID, postID)
	if !found {
		return world.Post{}, false, false, ""
	}
	return r.materializeArticleBodyOnceWithTimeout(host, board, selected, timeout)
}

func (r *Repository) materializeArticleBodyOnceWithTimeout(host world.Host, board world.Board, selected world.Post, timeout time.Duration) (world.Post, bool, bool, string) {
	renderContext, contextStats := r.materializationBBSRenderContext(host, board, selected)
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     board.Name,
		WorldDate:   selected.CreatedAt.Format("2006-01-02"),
		Region:      host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボード、%sによる件名『%s』の記事本文を、確定済みの投稿意図を変えず1996年の自然なパソコン通信文体で補完する", host.Name, board.Name, selected.Author, selected.Subject),
		Persistence: true,
		Importance:  .30,
		Specificity: .30,
	})
	if err != nil {
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationError("evidence", err), contextStats.String())
	}

	renderIntent := selected.Intent
	renderIntent.RenderContext = renderContext
	req := BoardMaterializationRequest{
		Host:             host,
		BoardID:          board.ID,
		BoardTopic:       selected.Subject,
		WorldDate:        selected.CreatedAt.Format("2006-01-02"),
		Persona:          persona,
		Intent:           renderIntent,
		CanonicalSubject: selected.Subject,
	}
	req = r.prepareBoardComposition(req, selected)
	var posts []world.Post
	var usage GenerationUsage
	if materializer, ok := r.Materializer.(usageAwareMaterializer); ok {
		posts, usage, err = materializer.GenerateBoardPostsWithUsage(ctx, req, decision)
	} else {
		posts, err = r.Materializer.GenerateBoardPosts(ctx, req, decision)
	}
	if err != nil {
		return selected, true, false, joinDevelopmentDiagnostics(formatGenerationError("renderer", err), contextStats.String())
	}
	if len(posts) == 0 {
		return selected, true, false, joinDevelopmentDiagnostics("error stage=renderer detail=no post returned", contextStats.String())
	}
	selected.Body = posts[0].Body
	if selected.Body == "" {
		return selected, true, false, joinDevelopmentDiagnostics("error stage=renderer detail=empty body returned", contextStats.String())
	}
	if usage.TotalTokens > 0 || usage.Model != "" {
		developmentGenerationUsage.Store(generationUsageKey{repo: r, postID: selected.ID}, usage)
	}
	diagnostic := joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())
	if updater, ok := r.Base.(world.PostUpdater); ok {
		if updated, ok := updater.UpdatePost(host.ID, selected); ok {
			return updated, true, true, diagnostic
		}
	}
	return selected, true, true, diagnostic
}
