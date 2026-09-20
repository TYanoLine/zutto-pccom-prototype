package main

import "testing"

func TestUpdatePersonaProfileDiversity(t *testing.T) {
	results := []personaProfileResult{
		{
			DistinctiveHook: "質問されると急に説明が細かくなる",
			CoreTraits: []string{"慎重", "面倒見がよい", "長話は苦手"},
			SocialDynamics: []string{"新人には丁寧"},
			ParticipationHabits: []string{"質問には返事をする"},
			Profile: "普段は聞き役だが、質問されると説明が細かくなる。新人には丁寧に返す。",
		},
		{
			DistinctiveHook: "軽口が多いのにトラブル相談では急に真面目になる",
			CoreTraits: []string{"冗談好き", "飽きっぽい", "困り事には真面目"},
			SocialDynamics: []string{"常連には軽口が多い"},
			ParticipationHabits: []string{"議論が長いと途中で抜ける"},
			Profile: "普段は軽口が多い。ところがトラブル相談では急に真面目になり、経験談を短く返す。",
		},
		{
			DistinctiveHook: "ROMが長いが書く日はまとめて長文になる",
			CoreTraits: []string{"観察好き", "慎重", "思い込みが強い"},
			SocialDynamics: []string{"訂正されると素直に引く"},
			ParticipationHabits: []string{"数日分をまとめて書く"},
			Profile: "読むだけの日が続くが、書く日はまとめて長文になる。自分の環境を基準に考えがちだが、訂正されると素直に引く。",
		},
	}
	var summary personaProfileSummary
	updatePersonaProfileDiversity(&summary, results)
	if summary.UniqueHookRatio != 1 {
		t.Fatalf("hook uniqueness = %v, want 1", summary.UniqueHookRatio)
	}
	if summary.UniqueTraitRatio != 1 {
		t.Fatalf("trait uniqueness = %v, want 1", summary.UniqueTraitRatio)
	}
	if summary.AvgNearestSimilarity <= 0 || summary.AvgNearestSimilarity >= 0.8 {
		t.Fatalf("unexpected average nearest similarity: %v", summary.AvgNearestSimilarity)
	}
	if summary.MaxProfileSimilarity <= 0 || summary.MaxProfileSimilarity >= 0.8 {
		t.Fatalf("unexpected max similarity: %v", summary.MaxProfileSimilarity)
	}
}

func TestUpdatePersonaProfileDiversityDetectsDuplicates(t *testing.T) {
	results := []personaProfileResult{
		{
			DistinctiveHook: "同じフック",
			CoreTraits: []string{"慎重", "世話好き", "短気"},
			SocialDynamics: []string{"新人には丁寧"},
			ParticipationHabits: []string{"質問に返す"},
			Profile: "同じ文章をほぼそのまま書いている人物です。",
		},
		{
			DistinctiveHook: "同じフック",
			CoreTraits: []string{"慎重", "世話好き", "短気"},
			SocialDynamics: []string{"新人には丁寧"},
			ParticipationHabits: []string{"質問に返す"},
			Profile: "同じ文章をほぼそのまま書いている人物です。",
		},
	}
	var summary personaProfileSummary
	updatePersonaProfileDiversity(&summary, results)
	if summary.UniqueHookRatio != 0.5 {
		t.Fatalf("hook uniqueness = %v, want .5", summary.UniqueHookRatio)
	}
	if summary.UniqueTraitRatio != 0.5 {
		t.Fatalf("trait uniqueness = %v, want .5", summary.UniqueTraitRatio)
	}
	if summary.MaxProfileSimilarity != 1 || summary.AvgNearestSimilarity != 1 {
		t.Fatalf("duplicate profile similarity = max %v avg %v, want 1/1", summary.MaxProfileSimilarity, summary.AvgNearestSimilarity)
	}
}
