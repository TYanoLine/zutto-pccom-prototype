package worldrepo

import (
	"context"
	"fmt"
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

// This registry exists only for the development materialization host. Keeping it
// outside canonical Post state prevents provider billing metadata from leaking
// into the simulated 1996 world. Production telemetry should eventually move to
// normal observability/persistence rather than this process-local registry.
var developmentGenerationUsage sync.Map

// MaterializationArticleWithDebug mirrors the normal lazy body materialization
// path but also captures renderer token usage when the configured materializer
// exposes it. The body itself is still the only newly committed world fact.
func (r *Repository) MaterializationArticleWithDebug(host world.Host, board world.Board, postID int64) (world.Post, bool, bool, string) {
	var selected world.Post
	found := false
	for _, p := range r.Base.ListPosts(host.ID) {
		if p.ID == postID && p.BoardID == board.ID {
			selected = p
			found = true
			break
		}
	}
	if !found {
		return world.Post{}, false, false, ""
	}
	renderContext, contextStats := r.materializationBBSRenderContext(host, board, selected)
	if selected.Body != "" {
		usage, _ := r.MaterializationGenerationUsage(postID)
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

	// The provider itself has a 30 second HTTP timeout. The old 20 second outer
	// context could cancel a healthy request first, producing intermittent silent
	// fallback. Give the provider enough room to finish and still keep this bounded.
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind:        historicalkb.KnowledgeCulturalSignal,
		Subject:     board.Name,
		WorldDate:   r.WorldDate,
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

	// Context is reconstructed from the canonical BBS store for this rendering
	// attempt. It is attached only to the local request copy and never persisted as
	// part of the post envelope.
	renderIntent := selected.Intent
	renderIntent.RenderContext = renderContext
	req := BoardMaterializationRequest{
		Host:             host,
		BoardID:          board.ID,
		BoardTopic:       selected.Subject,
		WorldDate:        r.WorldDate,
		Persona:          persona,
		Intent:           renderIntent,
		CanonicalSubject: selected.Subject,
	}
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
		developmentGenerationUsage.Store(generationUsageKey{repo: r, postID: postID}, usage)
	}
	diagnostic := joinDevelopmentDiagnostics(formatGenerationUsage(usage), contextStats.String())
	if updater, ok := r.Base.(world.PostUpdater); ok {
		if updated, ok := updater.UpdatePost(host.ID, selected); ok {
			return updated, true, true, diagnostic
		}
	}
	return selected, true, true, diagnostic
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
		usage, ok := value.(GenerationUsage)
		if !ok {
			return true
		}
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
		return true
	})
	return total
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
