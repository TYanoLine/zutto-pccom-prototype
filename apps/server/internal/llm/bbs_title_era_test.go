package llm

import "testing"

func TestValidateBBSTitleEraReview(t *testing.T) {
	req := BBSTitleEraRequest{WorldDate: "1996-08-13", BoardName: "ゲーム", Titles: []string{"最近何してる？", "ポケモン赤・緑"}}
	valid := BBSTitleEraReview{Decisions: []BBSTitleEraDecision{
		{Candidate: 1, Status: BBSTitleEraOK, Reason: "外部年代事実なし"},
		{Candidate: 2, Status: BBSTitleEraResearch, Reason: "実在作品の発売時期確認が必要"},
	}}
	if err := ValidateBBSTitleEraReview(req, valid); err != nil {
		t.Fatal(err)
	}
	cases := []BBSTitleEraReview{
		{Decisions: valid.Decisions[:1]},
		{Decisions: []BBSTitleEraDecision{{Candidate: 1, Status: BBSTitleEraOK, Reason: "ok"}, {Candidate: 1, Status: BBSTitleEraResearch, Reason: "dup"}}},
		{Decisions: []BBSTitleEraDecision{{Candidate: 1, Status: "maybe", Reason: "bad"}, {Candidate: 2, Status: BBSTitleEraOK, Reason: "ok"}}},
		{Decisions: []BBSTitleEraDecision{{Candidate: 1, Status: BBSTitleEraOK, Reason: ""}, {Candidate: 2, Status: BBSTitleEraOK, Reason: "ok"}}},
	}
	for _, draft := range cases {
		if ValidateBBSTitleEraReview(req, draft) == nil {
			t.Fatalf("accepted invalid era review: %+v", draft)
		}
	}
}
