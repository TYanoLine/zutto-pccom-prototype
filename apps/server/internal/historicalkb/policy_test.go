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

func TestKnowledgeKeyReusesConceptAcrossDates(t *testing.T){
	a:=KnowledgeQuery{Kind:KnowledgeTechnicalCapability,Subject:"V.34 modem",WorldDate:"1996-08-29",Region:"JP",Need:"speed"}
	b:=a;b.Need="availability";b.WorldDate="1997-08-29"
	if KnowledgeKey(a)!=KnowledgeKey(b){t.Fatal("same concept should reuse shared fact bucket across dates")}
	if ResearchKey(a)==ResearchKey(b){t.Fatal("different historical slices must have different research leases")}
}
