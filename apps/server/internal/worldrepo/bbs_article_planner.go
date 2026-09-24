package worldrepo

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode"

	"zutto-pccom/apps/server/internal/bbsengine"
	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

const (
	sharedTitlePoolAttempts   = 3
	sharedTitleResearchBudget = 6 * time.Second
)

type repositoryBBSBatchPlanner struct {
	repo *Repository
}

func (r *Repository) sharedBBSArticleEngineEnabled(host world.Host) bool {
	if r == nil || host.SoftwareID == "materialization-demo" {
		return false
	}
	var materializer LLMMaterializer
	switch m := r.Materializer.(type) {
	case LLMMaterializer:
		materializer = m
	case *LLMMaterializer:
		materializer = *m
	default:
		return false
	}
	_, ok := materializer.Renderer.(llm.BBSTitleCandidatePlanner)
	return ok
}

// PlanBBSBatch intentionally keeps the World/wording boundary narrow.
//
// World Engine: actor, time and root/reply topology.
// Title candidate model: proposes many uncommitted subjects.
// Jev/OpenAI review: ranks/maps candidates to already-selected root slots.
// Historical gate: checks only selected ambiguous titles.
// World: adopts the winning title/summary as canonical.
// Body prose: remains lazy until the article is read.
func (p repositoryBBSBatchPlanner) PlanBBSBatch(ctx context.Context, req bbsengine.BatchRequest) ([]bbsengine.PlannedPost, error) {
	totalStarted := time.Now()
	defer func() {
		log.Printf("BBS timing: host=%s board=%s phase=planner_total duration=%s slots=%d", req.Host.ID, req.Board.ID, time.Since(totalStarted), len(req.Slots))
	}()
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
	titlePlanner, ok := materializer.Renderer.(llm.BBSTitleCandidatePlanner)
	if !ok {
		return nil, fmt.Errorf("configured renderer does not support title-first candidate planning")
	}

	worldDate := req.WorldNow.Format(time.DateOnly)
	materializer = materializer.withPeriodReferents(worldDate)
	decision := worldengine.EvidenceDecision{}
	if p.repo.Engine != nil {
		evidenceStarted := time.Now()
		var err error
		decision, err = p.repo.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
			Kind:        historicalkb.KnowledgeCulturalSignal,
			Subject:     req.Board.Name,
			WorldDate:   worldDate,
			Region:      req.Host.Region,
			Audience:    []string{"Japanese dial-up BBS users"},
			Need:        fmt.Sprintf("%s の「%s」で、その時点のBBS件名候補に使ってよい時代背景・参照対象", req.Host.Name, req.Board.Name),
			Persistence: true,
			Importance:  .3,
			Specificity: .3,
		})
		log.Printf("BBS timing: host=%s board=%s phase=evidence duration=%s model_first=%t facts=%d", req.Host.ID, req.Board.ID, time.Since(evidenceStarted), decision.ModelFirst, len(decision.Knowledge.Facts))
		if err != nil {
			return nil, fmt.Errorf("resolve BBS title historical context: %w", err)
		}
	}

	planned := make(map[int]bbsengine.PlannedPost, len(req.Slots))
	rootSlots := make([]bbsengine.Slot, 0, len(req.Slots))
	for _, slot := range req.Slots {
		if slot.ReplyToPostID == 0 {
			rootSlots = append(rootSlots, slot)
			continue
		}
		planned[slot.Index] = bbsengine.PlannedPost{
			SlotIndex:        slot.Index,
			Topic:            strings.TrimSpace(slot.ReplyToSubject),
			Motivation:       "reply_to_existing_thread",
			Goal:             "respond to the existing thread",
			SituationSummary: fmt.Sprintf("%s が %s の件名「%s」の既存記事へ返信する", slot.Author, slot.ReplyToAuthor, slot.ReplyToSubject),
		}
	}

	if len(rootSlots) > 0 {
		roots, err := p.planRootTitles(ctx, materializer, decision, titlePlanner, req, rootSlots, worldDate)
		if err != nil {
			return nil, err
		}
		for _, root := range roots {
			planned[root.SlotIndex] = root
		}
	}

	out := make([]bbsengine.PlannedPost, 0, len(req.Slots))
	for _, slot := range req.Slots {
		post, ok := planned[slot.Index]
		if !ok {
			return nil, fmt.Errorf("title-first batch omitted slot %d", slot.Index)
		}
		out = append(out, post)
	}
	return out, nil
}

func (p repositoryBBSBatchPlanner) planRootTitles(
	ctx context.Context,
	materializer LLMMaterializer,
	decision worldengine.EvidenceDecision,
	titlePlanner llm.BBSTitleCandidatePlanner,
	req bbsengine.BatchRequest,
	rootSlots []bbsengine.Slot,
	worldDate string,
) ([]bbsengine.PlannedPost, error) {
	recentState := planningBBSStateWithBodyExcerpts(req.RecentPosts, 48, 8)
	recentSubjects := rootSubjects(req.RecentPosts)
	avoid := append([]string(nil), recentSubjects...)

	events := make([]llm.BBSWorldWindowEvent, 0, len(rootSlots))
	slotByEvent := map[string]bbsengine.Slot{}
	for _, slot := range rootSlots {
		eventID := fmt.Sprintf("slot-%d", slot.Index)
		profile, facts := p.personaTitleContext(slot.AuthorPersonaID)
		event := llm.BBSWorldWindowEvent{
			EventID:        eventID,
			BoardID:        req.Board.ID,
			BoardName:      req.Board.Name,
			AuthorHandle:   slot.Author,
			CreatedAt:      slot.CreatedAt.Format(time.RFC3339),
			Action:         "thread_start",
			AnchorKey:      "board:" + req.Board.ID,
			CauseKind:      "board_activity_window",
			CauseSummary:   "World Engine selected a root-post opportunity in this board/time window.",
			DiscourseMode:  "thread_start",
			PersonaProfile: profile,
			ExistingFacts:  facts,
		}
		events = append(events, event)
		slotByEvent[eventID] = slot
	}

	remaining := append([]llm.BBSWorldWindowEvent(nil), events...)
	adopted := map[string]bbsengine.PlannedPost{}
	contextual, hasContextual := materializer.Renderer.(llm.BBSContextualTitleCandidatePlanner)
	eraFallback, hasEraFallback := materializer.Renderer.(llm.BBSTitleEraValidator)

	for attempt := 0; attempt < sharedTitlePoolAttempts && len(remaining) > 0; attempt++ {
		var pool llm.BBSTitleCandidates
		var err error
		poolStarted := time.Now()
		if hasContextual {
			pool, err = contextual.GenerateContextualBBSTitleCandidates(ctx, llm.BBSContextualTitleCandidateRequest{
				WorldDate:       worldDate,
				BoardName:       req.Board.Name,
				RecentBBSState:  recentState,
				RecentSubjects:  recentSubjects,
				AvoidSubjects:   avoid,
				HistoricalFacts: materializer.historicalFacts(decision),
				EraRules:        materializer.eraRules(),
			})
		} else {
			pool, err = titlePlanner.GenerateBBSTitleCandidates(ctx, worldDate, req.Board.Name)
		}
		log.Printf("BBS timing: host=%s board=%s phase=title_pool attempt=%d duration=%s titles=%d err=%t", req.Host.ID, req.Board.ID, attempt+1, time.Since(poolStarted), len(pool.Titles), err != nil)
		if err != nil {
			// The structured provider already retries transient transport/rate
			// failures with backoff. A pool attempt means a new semantic pool,
			// not another burst of identical HTTP retries.
			return nil, fmt.Errorf("generate title-first candidate pool: %w", err)
		}

		titles := uniqueUsableTitles(pool.Titles, avoid)
		if len(titles) == 0 {
			continue
		}
		avoid = append(avoid, titles...)

		jevStarted := time.Now()
		jevAdvice, jevAttempted, jevErr := p.repo.developmentJevTitleAdvice(
			ctx, req.Host, req.Board, worldDate, pool.Titles, remaining, recentState,
		)
		log.Printf("BBS timing: host=%s board=%s phase=jev attempt=%d duration=%s used=%t err=%t", req.Host.ID, req.Board.ID, attempt+1, time.Since(jevStarted), jevAttempted, jevErr != nil)
		if jevErr != nil {
			jevAttempted = false
		}

		eraStatus := map[string]string{}
		if jevAttempted {
			for i, title := range pool.Titles {
				prob := jevAdvice.Era[i+1]
				switch {
				case prob.LogicallyImpossible >= developmentJevTitleEraImpossibleThreshold:
					eraStatus[title] = "ng"
				case prob.SafeWithoutResearch >= developmentJevTitleEraSafeThreshold:
					eraStatus[title] = "ok"
				default:
					eraStatus[title] = "research"
				}
			}
		} else if hasEraFallback {
			eraStarted := time.Now()
			eraReq := llm.BBSTitleEraRequest{WorldDate: worldDate, BoardName: req.Board.Name, Titles: pool.Titles}
			eraReview, eraErr := eraFallback.ValidateBBSTitleEra(ctx, eraReq)
			log.Printf("BBS timing: host=%s board=%s phase=era_fallback attempt=%d duration=%s err=%t", req.Host.ID, req.Board.ID, attempt+1, time.Since(eraStarted), eraErr != nil)
			if eraErr == nil {
				for _, d := range eraReview.Decisions {
					if d.Candidate < 1 || d.Candidate > len(pool.Titles) {
						continue
					}
					switch d.Status {
					case llm.BBSTitleEraOK:
						eraStatus[pool.Titles[d.Candidate-1]] = "ok"
					case llm.BBSTitleEraNG:
						eraStatus[pool.Titles[d.Candidate-1]] = "ng"
					default:
						eraStatus[pool.Titles[d.Candidate-1]] = "research"
					}
				}
			}
		}
		for _, title := range pool.Titles {
			if _, ok := eraStatus[title]; !ok {
				eraStatus[title] = "research"
			}
		}

		eligible := make([]string, 0, len(titles))
		for _, title := range titles {
			if eraStatus[title] != "ng" {
				eligible = append(eligible, title)
			}
		}
		if len(eligible) == 0 {
			continue
		}

		reviewer := titlePlanner
		if jevAttempted {
			reviewer = developmentJevTitlePlanner{
				titles:           append([]string(nil), pool.Titles...),
				advice:           jevAdvice,
				fitFloor:         developmentJevTitleFitThreshold,
				rankingOnly:      attempt == sharedTitlePoolAttempts-1,
				specificityBonus: sourcedTitleSpecificityBonus(pool.Titles, worldDate),
			}
		}
		available := append([]string(nil), eligible...)
		for len(available) > 0 && len(remaining) > 0 {
			reviewReq := llm.BBSTitleReviewRequest{
				BoardName:      req.Board.Name,
				Titles:         available,
				Events:         remaining,
				RecentBBSState: recentState,
			}
			reviewStarted := time.Now()
			review, err := reviewer.ReviewBBSTitleCandidates(ctx, reviewReq)
			log.Printf("BBS timing: host=%s board=%s phase=title_review attempt=%d duration=%s titles=%d events=%d err=%t", req.Host.ID, req.Board.ID, attempt+1, time.Since(reviewStarted), len(available), len(remaining), err != nil)
			if err != nil {
				if attempt+1 < sharedTitlePoolAttempts {
					break
				}
				return nil, fmt.Errorf("review title-first candidates: %w", err)
			}

			// Every candidate that won a slot in this round is consumed from this
			// pool, even if later rejected by duplicate/era research. This is the
			// key candidate-first fallback: the next-best candidate can then be
			// tried for the still-unfilled world slot without asking the wording
			// model to invent a new pool.
			consumed := map[string]bool{}
			candidates := make([]llm.BBSTitleDecision, 0)
			for _, d := range review.Decisions {
				subject := strings.TrimSpace(d.Subject)
				if d.EventID == "" || subject == "" {
					continue
				}
				consumed[subject] = true
				if strings.TrimSpace(d.Summary) == "" {
					continue
				}
				if _, exists := adopted[d.EventID]; exists {
					continue
				}
				if titleTooSimilarToAny(subject, recentSubjects) || titleTooSimilarToAdopted(subject, adopted) {
					continue
				}
				candidates = append(candidates, d)
			}
			if len(consumed) == 0 {
				break
			}

			researchJobs := make([]developmentTitleEraResearchJob, 0)
			researchDecision := map[int]llm.BBSTitleDecision{}
			jobID := 1
			for _, d := range candidates {
				switch eraStatus[d.Subject] {
				case "ok":
					adopted[d.EventID] = adoptedRoot(slotByEvent[d.EventID], d)
				case "research":
					researchJobs = append(researchJobs, developmentTitleEraResearchJob{candidate: jobID, title: d.Subject})
					researchDecision[jobID] = d
					jobID++
				}
			}
			if len(researchJobs) > 0 {
				// Historical verification is an adoption gate, not a reason to
				// freeze the BBS UI. If uncached research cannot finish inside
				// this small foreground budget, leave the candidate unverified
				// and try the next already-generated candidate instead.
				researchStarted := time.Now()
				researchCtx, cancel := context.WithTimeout(ctx, sharedTitleResearchBudget)
				outcomes := p.repo.developmentResearchTitleEraBatch(researchCtx, req.Host, req.Board, worldDate, researchJobs)
				cancel()
				log.Printf("BBS timing: host=%s board=%s phase=title_research attempt=%d duration=%s jobs=%d", req.Host.ID, req.Board.ID, attempt+1, time.Since(researchStarted), len(researchJobs))
				for id, outcome := range outcomes {
					if outcome.status != "verified" {
						continue
					}
					d := researchDecision[id]
					adopted[d.EventID] = adoptedRoot(slotByEvent[d.EventID], d)
				}
			}

			nextAvailable := make([]string, 0, len(available))
			for _, title := range available {
				if !consumed[title] {
					nextAvailable = append(nextAvailable, title)
				}
			}
			available = nextAvailable

			nextRemaining := make([]llm.BBSWorldWindowEvent, 0, len(remaining))
			for _, event := range remaining {
				if _, ok := adopted[event.EventID]; !ok {
					nextRemaining = append(nextRemaining, event)
				}
			}
			remaining = nextRemaining
		}
	}

	// World-selected roots survive wording difficulty. If Jev/OpenAI could not
	// fill every slot after bounded pools, adopt unused era-safe candidates from
	// the last generated pools is preferable to erasing the world action.
	if len(remaining) > 0 {
		return nil, fmt.Errorf("title-first pools exhausted with %d unfilled root slots", len(remaining))
	}

	out := make([]bbsengine.PlannedPost, 0, len(rootSlots))
	for _, slot := range rootSlots {
		eventID := fmt.Sprintf("slot-%d", slot.Index)
		out = append(out, adopted[eventID])
	}
	return out, nil
}

func sourcedTitleSpecificityBonus(titles []string, worldDate string) map[int]float64 {
	out := map[int]float64{}
	referents := historicalkb.PeriodReferents(worldDate)
	for i, title := range titles {
		for _, ref := range referents {
			name := strings.TrimSpace(ref.Name)
			if name != "" && strings.Contains(strings.ToLower(title), strings.ToLower(name)) {
				// Deliberately small: it breaks near-ties among already-fitting
				// candidates, never substitutes for Jev's board/person fit gate.
				out[i+1] = 0.12
				break
			}
		}
	}
	return out
}

func adoptedRoot(slot bbsengine.Slot, d llm.BBSTitleDecision) bbsengine.PlannedPost {
	return bbsengine.PlannedPost{
		SlotIndex:        slot.Index,
		Subject:          d.Subject,
		Topic:            d.Subject,
		Motivation:       "world_selected_board_activity",
		Goal:             "share or ask about the adopted subject",
		SituationSummary: d.Summary,
	}
}

func (p repositoryBBSBatchPlanner) personaTitleContext(personaID string) (string, []string) {
	if p.repo == nil || personaID == "" {
		return "", nil
	}
	var profile string
	if store, ok := p.repo.Base.(world.PersonaStore); ok {
		if persona, found := store.PersonaByID(personaID); found {
			profile = personaSummary(persona)
		}
	}
	facts := []string{}
	if store, ok := p.repo.Base.(world.PersonaFactStore); ok {
		for _, fact := range store.ListPersonaFacts(personaID) {
			facts = append(facts, fact.Key+"="+fact.Value)
		}
	}
	return profile, facts
}

func rootSubjects(posts []world.Post) []string {
	out := make([]string, 0, len(posts))
	for _, post := range posts {
		if post.ParentID == 0 && strings.TrimSpace(post.Subject) != "" {
			out = append(out, strings.TrimSpace(post.Subject))
		}
	}
	return out
}

func uniqueUsableTitles(titles, avoid []string) []string {
	out := make([]string, 0, len(titles))
	seen := map[string]bool{}
	for _, title := range titles {
		title = strings.TrimSpace(title)
		key := normalizeTitleForSimilarity(title)
		if title == "" || seen[key] || titleTooSimilarToAny(title, avoid) {
			continue
		}
		seen[key] = true
		out = append(out, title)
	}
	return out
}

func titleTooSimilarToAdopted(title string, adopted map[string]bbsengine.PlannedPost) bool {
	for _, post := range adopted {
		if titleSimilarity(title, post.Subject) >= .78 {
			return true
		}
	}
	return false
}

func titleTooSimilarToAny(title string, others []string) bool {
	for _, other := range others {
		if titleSimilarity(title, other) >= .78 {
			return true
		}
	}
	return false
}

func normalizeTitleForSimilarity(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) || strings.ContainsRune("！？?!。、・「」『』（）()[]【】〜～", r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func titleSimilarity(a, b string) float64 {
	a = normalizeTitleForSimilarity(a)
	b = normalizeTitleForSimilarity(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	grams := func(s string) map[string]bool {
		r := []rune(s)
		out := map[string]bool{}
		if len(r) == 1 {
			out[s] = true
			return out
		}
		for i := 0; i+1 < len(r); i++ {
			out[string(r[i:i+2])] = true
		}
		return out
	}
	ga, gb := grams(a), grams(b)
	union := map[string]bool{}
	inter := 0
	for g := range ga {
		union[g] = true
		if gb[g] {
			inter++
		}
	}
	for g := range gb {
		union[g] = true
	}
	if len(union) == 0 {
		return 0
	}
	return float64(inter) / float64(len(union))
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
