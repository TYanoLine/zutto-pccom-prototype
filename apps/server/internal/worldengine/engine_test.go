package worldengine

import (
	"context"
	"testing"

	"zutto-pccom/apps/server/internal/historicalkb"
)

type fakeResolver struct{ calls, lookupCalls int }
func (f *fakeResolver) Resolve(_ context.Context,q historicalkb.KnowledgeQuery)(historicalkb.KnowledgeResult,error){f.calls++;return historicalkb.KnowledgeResult{Query:q,CanUse:true,Facts:[]historicalkb.HistoricalFact{{Claim:"28.8kbps V.34",Status:historicalkb.FactVerified,Confidence:.9}},Coverage:.9,Confidence:.9},nil}
func (f *fakeResolver) Lookup(_ context.Context,q historicalkb.KnowledgeQuery)(historicalkb.KnowledgeResult,error){f.lookupCalls++;return historicalkb.KnowledgeResult{Query:q,CanUse:true,Facts:[]historicalkb.HistoricalFact{{Claim:"cached "+q.Subject,Status:historicalkb.FactVerified,Confidence:.95}},Coverage:.95,Confidence:.95},nil}

func TestAtmosphericDoesNotTouchHistoricalKnowledge(t *testing.T){f:=&fakeResolver{};e:=Engine{Knowledge:f};d,err:=e.ResolveEvidence(context.Background(),EvidenceRequest{Subject:"夏休みの雑談",Importance:.1,Specificity:.1});if err!=nil{t.Fatal(err)};if !d.ModelFirst{t.Fatal("atmospheric prose should be model-first")};if f.calls!=0{t.Fatalf("knowledge resolver called %d times",f.calls)}}

func TestPersistentTechnicalClaimUsesHistoricalKnowledge(t *testing.T){f:=&fakeResolver{};e:=Engine{Knowledge:f};d,err:=e.ResolveEvidence(context.Background(),EvidenceRequest{Kind:historicalkb.KnowledgeTechnicalCapability,Subject:"V.34 modem",WorldDate:"1996-08-29",Region:"JP",Persistence:true,HasTechnicalSpec:true});if err!=nil{t.Fatal(err)};if d.Level!=historicalkb.EvidenceVerified{t.Fatalf("level=%s",d.Level)};if f.calls!=1{t.Fatalf("knowledge resolver called %d times",f.calls)};if !d.Knowledge.CanUse{t.Fatal("verified fact should be usable")}}


func TestLookupEvidenceUsesCacheOnlyResolverPath(t *testing.T) {
	f := &fakeResolver{}
	e := Engine{Knowledge: f}
	d, err := e.LookupEvidence(context.Background(), EvidenceRequest{
		Kind: historicalkb.KnowledgeProductAvailability,
		Subject: "PC-9821Xa",
		WorldDate: "1996-08-26",
		Region: "JP",
		Persistence: true,
		HasProductModel: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lookupCalls != 1 {
		t.Fatalf("lookup calls=%d, want 1", f.lookupCalls)
	}
	if f.calls != 0 {
		t.Fatalf("Resolve was called during cache-only lookup: %d", f.calls)
	}
	if !d.Knowledge.CanUse || len(d.Knowledge.Facts) != 1 {
		t.Fatalf("lookup decision=%+v", d)
	}
}
