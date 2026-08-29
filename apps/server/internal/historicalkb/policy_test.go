package historicalkb

import "testing"

func TestRequiredEvidence(t *testing.T) {
	cases:=[]struct{name string; in DecisionContext; want EvidenceLevel}{
		{"ordinary atmosphere",DecisionContext{Importance:.2,Specificity:.2},EvidenceAtmospheric},
		{"plausible context",DecisionContext{Importance:.6,Specificity:.4},EvidencePlausible},
		{"persistent exact number",DecisionContext{Persistence:true,HasExactNumber:true},EvidenceVerified},
		{"persistent product model",DecisionContext{Persistence:true,HasProductModel:true},EvidenceVerified},
	}
	for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){if got:=RequiredEvidence(tc.in);got!=tc.want{t.Fatalf("got %s want %s",got,tc.want)}})}
}

func TestKnowledgeKeyIgnoresNeedButKeepsHistoricalSlice(t *testing.T){
	a:=KnowledgeQuery{Kind:KnowledgeTechnicalCapability,Subject:"V.34 modem",WorldDate:"1996-08-29",Region:"JP",Need:"speed"}
	b:=a;b.Need="availability"
	if KnowledgeKey(a)!=KnowledgeKey(b){t.Fatal("need wording should not split shared fact bucket")}
	b.WorldDate="1997-08-29"
	if KnowledgeKey(a)==KnowledgeKey(b){t.Fatal("different world date must not share historical slice")}
}
