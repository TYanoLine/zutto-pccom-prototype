package worldrepo

import (
    "context"
    "fmt"
    "log"
    "strings"
    "time"

    "zutto-pccom/apps/server/internal/bbsengine"
    "zutto-pccom/apps/server/internal/historicalkb"
    "zutto-pccom/apps/server/internal/llm"
    "zutto-pccom/apps/server/internal/world"
    "zutto-pccom/apps/server/internal/worldengine"
)

type repositoryBBSBatchPlanner struct { repo *Repository }

// Production has one planning path: World chooses the article slots, then
// Situation and title generators express those selected events.
func (r *Repository) sharedBBSArticleEngineEnabled(host world.Host) bool {
    if r == nil { return false }
    var materializer LLMMaterializer
    switch m := r.Materializer.(type) {
    case LLMMaterializer: materializer=m
    case *LLMMaterializer:
        if m == nil { return false }
        materializer=*m
    default: return false
    }
    _, hasSituation:=materializer.Renderer.(llm.BBSWorldSituationProposer)
    _, hasTitles:=materializer.Renderer.(llm.BBSSituationTitlePlanner)
    return hasSituation && hasTitles
}

func (p repositoryBBSBatchPlanner) PlanBBSBatch(ctx context.Context, req bbsengine.BatchRequest) ([]bbsengine.PlannedPost, error) {
    started:=time.Now()
    defer func() { log.Printf("BBS timing: host=%s board=%s phase=planner_total duration=%s slots=%d",req.Host.ID,req.Board.ID,time.Since(started),len(req.Slots)) }()
    if p.repo==nil { return nil,fmt.Errorf("BBS batch planner repository is nil") }
    var materializer LLMMaterializer
    switch m:=p.repo.Materializer.(type) {
    case LLMMaterializer: materializer=m
    case *LLMMaterializer:
        if m==nil { return nil,fmt.Errorf("BBS materializer is nil") }
        materializer=*m
    default: return nil,fmt.Errorf("BBS batch requires LLMMaterializer")
    }
    situationProposer,hasSituation:=materializer.Renderer.(llm.BBSWorldSituationProposer)
    titlePlanner,hasTitles:=materializer.Renderer.(llm.BBSSituationTitlePlanner)
    if !hasSituation || !hasTitles {
        return nil,fmt.Errorf("BBS production requires Situation and title planner; legacy title experiments are retired")
    }
    titleAsOf:=req.WorldNow.Format(time.DateOnly)
    for _,slot:=range req.Slots {
        if slot.CreatedAt.IsZero() { continue }
        date:=slot.CreatedAt.Format(time.DateOnly)
        if date<titleAsOf { titleAsOf=date }
    }
    // Historical references remain disabled in the normal live materializer.
    materializer=materializer.withPeriodReferents(req.WorldNow.Format(time.DateOnly),titleAsOf)
    decision:=worldengine.EvidenceDecision{}
    if p.repo.Engine!=nil && materializer.HistoricalReferencesEnabled {
        var err error
        decision,err=p.repo.Engine.ResolveEvidence(ctx,worldengine.EvidenceRequest{
            Kind:historicalkb.KnowledgeCulturalSignal,
            Subject:req.Board.Name,WorldDate:titleAsOf,Region:req.Host.Region,
            Audience:[]string{"Japanese dial-up BBS users"},
            Need:fmt.Sprintf("%s の「%s」で時代整合性に必要な背景",req.Host.Name,req.Board.Name),
            Persistence:true,Importance:.3,Specificity:.3,
        })
        if err!=nil {return nil,fmt.Errorf("resolve BBS historical context: %w",err)}
    }
    return p.planSituationFirstBatch(ctx,materializer,decision,situationProposer,titlePlanner,req,titleAsOf)
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
		if world.IsSemanticRoot(post) && strings.TrimSpace(post.Subject) != "" {
			out = append(out, strings.TrimSpace(post.Subject))
		}
	}
	return out
}

