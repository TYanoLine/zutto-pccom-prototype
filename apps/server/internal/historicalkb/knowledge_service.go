package historicalkb

import (
	"context"
	"fmt"
	"time"
)

type KnowledgeService struct {
	Store      *Store
	Researcher Researcher
	Now        func() time.Time
}

func (s KnowledgeService) Resolve(ctx context.Context, q KnowledgeQuery) (KnowledgeResult, error) {
	q.Region = normalizeRegion(q.Region)
	if q.RequiredEvidence == "" { q.RequiredEvidence = EvidencePlausible }
	if q.Kind == "" { q.Kind = KnowledgeGeneral }
	key := KnowledgeKey(q)

	if q.RequiredEvidence == EvidenceAtmospheric {
		return KnowledgeResult{Query:q,Coverage:1,Confidence:0.5,CanUse:true}, nil
	}
	if s.Store == nil { return KnowledgeResult{}, ErrNotConfigured }

	facts, err := s.Store.FindFacts(ctx,key,q.WorldDate)
	if err != nil { return KnowledgeResult{}, err }
	current := summarizeKnowledge(q,facts)
	if Sufficient(current,q.RequiredEvidence) { current.CanUse=true; return current,nil }

	if q.RequiredEvidence == EvidencePlausible {
		current.CanUse = true
		if len(current.Missing)==0 { current.Missing=[]KnowledgeGap{{Description:"共有KBに十分な裏付けがないためモデル既知知識で暫定生成可能"}} }
		return current,nil
	}

	leaseKey:=ResearchKey(q)
	owner := fmt.Sprintf("resolve-%d",s.now().UnixNano())
	acquired,err:=s.Store.TryAcquireResearchLease(ctx,leaseKey,owner,2*time.Minute)
	if err!=nil{return KnowledgeResult{},err}
	if !acquired { current.ResearchPending=true; current.CanUse=false; return current,nil }
	defer s.Store.ReleaseResearchLease(context.Background(),leaseKey,owner)

	question:=q.Need
	if question=="" { question=fmt.Sprintf("%sについて、%s時点の%sで利用できる世界事実として必要な範囲を確認",q.Subject,q.WorldDate,q.Region) }
	r,err:=s.Researcher.Research(ctx,q.Subject,question,q.WorldDate,fmt.Sprintf("knowledge kind=%s region=%s audience=%v; verify only what is needed for a persistent world fact",q.Kind,q.Region,q.Audience))
	if err!=nil{return current,err}

	now:=s.now(); rid:=NewCaseID(now)
	caseStatus:=StatusNeedsReview
	if len(r.MissingInfo)==0 && r.Confidence>=0.8 { caseStatus=StatusProvisional }
	c:=ResearchCase{ID:rid,Topic:q.Subject,Question:question,WorldDate:q.WorldDate,Status:caseStatus,Summary:r.Summary,ProvisionalAnswer:r.ProvisionalAnswer,MissingInfo:r.MissingInfo,Confidence:r.Confidence,Sources:r.Sources,Messages:[]Message{{Role:"assistant",Body:r.Summary+"\n\n暫定結論:\n"+r.ProvisionalAnswer,CreatedAt:now}},CreatedAt:now,UpdatedAt:now}
	if err:=s.Store.Upsert(ctx,c);err!=nil{return current,err}

	factStatus:=FactProvisional
	if len(r.MissingInfo)==0 && r.Confidence>=0.8 && len(r.Sources)>0 { factStatus=FactVerified }
	f:=HistoricalFact{ID:"fact-"+rid,KnowledgeKey:key,Kind:q.Kind,Subject:q.Subject,Claim:r.ProvisionalAnswer,ValidFrom:q.WorldDate,Region:q.Region,Audience:q.Audience,Confidence:r.Confidence,Status:factStatus,Sources:r.Sources,ResearchID:rid,CreatedAt:now,UpdatedAt:now}
	if err:=s.Store.UpsertFact(ctx,f);err!=nil{return current,err}

	facts,err=s.Store.FindFacts(ctx,key,q.WorldDate);if err!=nil{return KnowledgeResult{},err}
	out:=summarizeKnowledge(q,facts);out.Researched=true;out.ResearchID=rid;out.CanUse=Sufficient(out,q.RequiredEvidence)
	if !out.CanUse { out.Missing=append(out.Missing,KnowledgeGap{Description:"自動調査は完了したが、Verified世界事実としては運営レビューまたは追加資料が必要"}) }
	return out,nil
}

func summarizeKnowledge(q KnowledgeQuery,facts []HistoricalFact) KnowledgeResult {
	r:=KnowledgeResult{Query:q,Facts:facts}
	if len(facts)==0 { r.Missing=[]KnowledgeGap{{Description:"共有Historical KBに一致するFactがない"}}; return r }
	total:=0.0; best:=0.0; strong:=0
	for _,f:=range facts { if f.Confidence>best{best=f.Confidence}; total+=f.Confidence; if f.Status==FactVerified||f.Status==FactOperatorVerified||f.Status==FactCanonical{strong++} }
	r.Confidence=best
	r.Coverage=total/float64(len(facts)); if r.Coverage>1{r.Coverage=1}
	if strong==0 { r.Coverage*=0.65; r.Missing=[]KnowledgeGap{{Description:"検証済みFactがまだない"}} }
	return r
}

func (s KnowledgeService) now() time.Time { if s.Now!=nil{return s.Now()}; return time.Now().UTC() }
