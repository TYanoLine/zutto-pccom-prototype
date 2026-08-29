package worldengine

import (
	"context"

	"zutto-pccom/apps/server/internal/historicalkb"
)

type KnowledgeResolver interface {
	Resolve(context.Context, historicalkb.KnowledgeQuery) (historicalkb.KnowledgeResult, error)
}

type Engine struct { Knowledge KnowledgeResolver }

type EvidenceRequest struct {
	Kind          historicalkb.KnowledgeKind
	Subject       string
	WorldDate     string
	Region        string
	Audience      []string
	Need          string
	Persistence   bool
	Importance    float64
	Specificity   float64
	HasExactDate  bool
	HasExactNumber bool
	HasProductModel bool
	HasTechnicalSpec bool
}

type EvidenceDecision struct {
	Level     historicalkb.EvidenceLevel `json:"level"`
	Knowledge historicalkb.KnowledgeResult `json:"knowledge"`
	ModelFirst bool `json:"modelFirst"`
}

// ResolveEvidence is the WorldEngine boundary. It decides whether a decision is
// atmosphere/model-first or deserves shared historical retrieval/research.
func (e Engine) ResolveEvidence(ctx context.Context, r EvidenceRequest) (EvidenceDecision,error) {
	level:=historicalkb.RequiredEvidence(historicalkb.DecisionContext{Persistence:r.Persistence,Importance:r.Importance,Specificity:r.Specificity,HasExactDate:r.HasExactDate,HasExactNumber:r.HasExactNumber,HasProductModel:r.HasProductModel,HasTechnicalSpec:r.HasTechnicalSpec})
	if level==historicalkb.EvidenceAtmospheric || e.Knowledge==nil {
		return EvidenceDecision{Level:level,ModelFirst:true,Knowledge:historicalkb.KnowledgeResult{CanUse:true}},nil
	}
	k,err:=e.Knowledge.Resolve(ctx,historicalkb.KnowledgeQuery{Kind:r.Kind,Subject:r.Subject,WorldDate:r.WorldDate,Region:r.Region,Audience:r.Audience,Need:r.Need,RequiredEvidence:level})
	if err!=nil {
		// Plausible prose is allowed to degrade to model-first. Verified persistent
		// facts are not silently invented when the research subsystem fails.
		if level==historicalkb.EvidencePlausible { return EvidenceDecision{Level:level,ModelFirst:true,Knowledge:k},nil }
		return EvidenceDecision{Level:level,Knowledge:k},err
	}
	return EvidenceDecision{Level:level,Knowledge:k,ModelFirst:level==historicalkb.EvidencePlausible && len(k.Facts)==0},nil
}
