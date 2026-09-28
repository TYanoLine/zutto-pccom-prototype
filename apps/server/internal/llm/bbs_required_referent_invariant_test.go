package llm

import (
	"strings"
	"testing"
)

func TestArticleDetailNeedsForcedWebSearchForRequiredRootWithMissingGrounding(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{
		EventID: "e1", Subject: "次号の展開を予想", Summary: "前号から残る謎の次の展開を予想する",
	}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{
		EventID: "e1", ReferentRequirement: "required", ReferentStatus: "unresolved", ReferentGrounding: "not_applicable",
		Details: []BBSArticleDetail{{Kind: "reaction_context", Fact: "前号から残る謎の次の展開を予想している"}},
	}}}
	if !articleDetailNeedsForcedWebSearch(req, draft) {
		t.Fatal("required unresolved root must enter the recovery search even when the first pass mislabeled grounding as not_applicable")
	}
}

func TestValidateBBSTitleArticleDetailsForCommitRejectsUnresolvedRequiredRoot(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{
		EventID: "e1", Subject: "次号の展開を予想", Summary: "前号から残る謎の次の展開を予想する",
	}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{
		EventID: "e1", ReferentRequirement: "required", ReferentStatus: "unresolved", ReferentGrounding: "external_history",
		Details: []BBSArticleDetail{{Kind: "reaction_context", Fact: "前号から残る謎の次の展開を予想している"}},
	}}}
	if err := ValidateBBSTitleArticleDetails(req, draft); err != nil {
		t.Fatalf("first-pass validator must still allow unresolved state for recovery: %v", err)
	}
	if err := ValidateBBSTitleArticleDetailsForCommit(req, draft); err == nil {
		t.Fatal("commit validator must reject an unresolved required root")
	}
}

func TestValidateBBSTitleArticleDetailsForCommitRejectsPlaceholderReferent(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{
		EventID: "e1", IsReply: true, Subject: "次号の展開を予想", Summary: "先行記事への返信",
	}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{
		EventID: "e1", ReferentRequirement: "required", ReferentStatus: "resolved", ReferentGrounding: "world_local",
		Details: []BBSArticleDetail{
			{Kind: "referent", Fact: "EMI.Mが予想している、題名不詳の連載漫画。"},
			{Kind: "observation", Fact: "前号を読み返すと気になる描写が一つあった"},
		},
	}}}
	if err := ValidateBBSTitleArticleDetailsForCommit(req, draft); err == nil {
		t.Fatal("placeholder such as 題名不詳 must not satisfy a required referent")
	}
}

func TestValidateBBSTitleArticleDetailsForCommitAcceptsConcreteWorldLocalReferent(t *testing.T) {
	req := BBSTitleArticleDetailRequest{Articles: []BBSTitleArticleDetailSeed{{
		EventID: "e1", Subject: "近所の店で見つけた", Summary: "近所の店で見つけた物について話す",
	}}}
	draft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{
		EventID: "e1", ReferentRequirement: "required", ReferentStatus: "resolved", ReferentGrounding: "world_local",
		Details: []BBSArticleDetail{{Kind: "referent", Fact: "自宅近くの小さな中古本屋"}},
	}}}
	if err := ValidateBBSTitleArticleDetailsForCommit(req, draft); err != nil {
		t.Fatalf("concrete anonymous world-local referent should be committable: %v", err)
	}
}

func TestContextualTitleCandidatePromptRejectsUnanchoredSpecificRoots(t *testing.T) {
	prompt := contextualTitleCandidatePrompt(BBSContextualTitleCandidateRequest{
		WorldDate: "1996-08-26",
		BoardName: "ＡＮＩＭＥ／ＭＡＮＧＡ",
		BoardScope: "アニメ、漫画、関連する雑談や感想。",
		CandidateCount: 100,
		RemainingNeeded: 10,
	})
	for _, want := range []string{"次号の展開を予想", "台詞の間が好き", "後段Article Detailが補ってくれる前提"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("title candidate specificity rule missing %q", want)
		}
	}
}
