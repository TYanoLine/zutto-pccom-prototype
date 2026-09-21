package worldrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

type repositoryBBSBatchPlanner struct {
	repo *Repository
}

func (p repositoryBBSBatchPlanner) PlanBBSBatch(ctx context.Context, req bbsengine.BatchRequest) ([]bbsengine.PlannedPost, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("bbs batch planner repository is nil")
	}
	var materializer LLMMaterializer
	switch m := p.repo.Materializer.(type) {
	case LLMMaterializer:
		materializer = m
	case *LLMMaterializer:
		materializer = *m
	default:
		return nil, fmt.Errorf("shared BBS article engine requires LLMMaterializer")
	}
	planner, ok := materializer.Renderer.(llm.BBSArticleBatchPlanner)
	if !ok {
		return nil, fmt.Errorf("configured renderer does not support batched BBS article planning")
	}

	worldDate := req.WorldNow.Format(time.DateOnly)
	decision := worldengine.EvidenceDecision{}
	if p.repo.Engine != nil {
		var err error
		decision, err = p.repo.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
			Kind:        historicalkb.KnowledgeCulturalSignal,
			Subject:     req.Board.Name,
			WorldDate:   worldDate,
			Region:      req.Host.Region,
			Audience:    []string{"Japanese dial-up BBS users"},
			Need:        fmt.Sprintf("%s の「%s」で、この期間に自然に起きる複数投稿の時代背景", req.Host.Name, req.Board.Name),
			Persistence: true,
			Importance:  .3,
			Specificity: .3,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve BBS batch historical context: %w", err)
		}
	}

	slots := make([]llm.BBSArticleBatchSlot, 0, len(req.Slots))
	for _, slot := range req.Slots {
		kind := "root"
		if slot.ReplyToPostID != 0 {
			kind = "reply"
		}
		slots = append(slots, llm.BBSArticleBatchSlot{
			Index:          slot.Index,
			AuthorHandle:   slot.Author,
			CreatedAt:      slot.CreatedAt.Format(time.RFC3339),
			Kind:           kind,
			ReplyToPostID:  slot.ReplyToPostID,
			ReplyToSubject: slot.ReplyToSubject,
			ReplyToAuthor:  slot.ReplyToAuthor,
		})
	}

	recentSubjects := make([]string, 0, len(req.RecentPosts))
	for _, post := range req.RecentPosts {
		if post.ParentID == 0 && strings.TrimSpace(post.Subject) != "" {
			recentSubjects = append(recentSubjects, post.Subject)
		}
	}
	draft, err := planner.GenerateBBSArticleBatch(ctx, llm.BBSArticleBatchRequest{
		HostName:        req.Host.Name,
		HostRegion:      req.Host.Region,
		HostSoftware:    req.Host.Software,
		BoardID:         req.Board.ID,
		BoardName:       req.Board.Name,
		WorldDate:       worldDate,
		RecentBBSState:  planningBBSStateWithBodyExcerpts(req.RecentPosts, 48, 8),
		RecentSubjects:  recentSubjects,
		HistoricalFacts: materializer.historicalFacts(decision),
		EraRules:        materializer.eraRules(),
		Slots:           slots,
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(draft.Posts, func(i, j int) bool { return draft.Posts[i].SlotIndex < draft.Posts[j].SlotIndex })
	out := make([]bbsengine.PlannedPost, 0, len(draft.Posts))
	for _, post := range draft.Posts {
		out = append(out, bbsengine.PlannedPost{
			SlotIndex:        post.SlotIndex,
			Subject:          post.Subject,
			Body:             post.Body,
			Topic:            post.Topic,
			Motivation:       post.Motivation,
			Stance:           post.Stance,
			Goal:             post.Goal,
			SituationSummary: post.SituationSummary,
		})
	}
	return out, nil
}

func planningBBSStateWithBodyExcerpts(posts []world.Post, titleLimit, bodyLimit int) string {
	base := strings.TrimSpace(planningBBSState(posts, titleLimit))
	if len(posts) == 0 || bodyLimit <= 0 {
		return base
	}
	ordered := append([]world.Post(nil), posts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})
	withBody := make([]world.Post, 0, bodyLimit)
	for i := len(ordered) - 1; i >= 0 && len(withBody) < bodyLimit; i-- {
		if strings.TrimSpace(ordered[i].Body) != "" {
			withBody = append(withBody, ordered[i])
		}
	}
	if len(withBody) == 0 {
		return base
	}
	var b strings.Builder
	if base != "" {
		b.WriteString(base)
		b.WriteString("\n")
	}
	b.WriteString("RECENT BODY EXCERPTS (context only):\n")
	for i := len(withBody) - 1; i >= 0; i-- {
		post := withBody[i]
		body := compactBBSExcerpt(post.Body, 220)
		fmt.Fprintf(&b, "MSG %04d %s %s / %s: %s\n", post.ID, post.CreatedAt.Format("01/02 15:04"), post.Author, post.Subject, body)
	}
	return strings.TrimSpace(b.String())
}

func compactBBSExcerpt(s string, maxRunes int) string {
	s = strings.ReplaceAll(s, "\r\n", " / ")
	s = strings.ReplaceAll(s, "\n", " / ")
	s = strings.ReplaceAll(s, "\r", " / ")
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if maxRunes > 0 && len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}
