package llm

import (
	"strings"
	"testing"
)

func TestBuildBoardPostPromptOmitsInfrastructureMetadata(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{BoardTopic: "音楽", WorldDate: "1996-06-07", AuthorHandle: "MARI", CanonicalSubject: "YMOを聴き直しています", PostIntent: "article_detail=observation:音の重なり方が以前より気になった"})
	for _, bad := range []string{"Host software family", "Board ID", "CURRENT WORLD SLOT", "routing_domain"} {
		if strings.Contains(prompt, bad) {
			t.Fatalf("prompt leaked infrastructure metadata %q", bad)
		}
	}
	if !strings.Contains(prompt, "YMOを聴き直しています") || !strings.Contains(prompt, "article_detail=") {
		t.Fatalf("canonical content missing: %s", prompt)
	}
}

func TestBuildBoardPostPromptUsesPurposeAndMaterialsInsteadOfRuleStack(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic: "ゲーム", WorldDate: "1996-06-07", AuthorHandle: "YUKI",
		PersonaProfile: "writing=勢いのある短文が多い。", CanonicalSubject: "最近こればかりやってます",
	})
	for _, want := range []string{
		"ずっとパソコン通信", "会員が読む記事本文として文章化し、保存・表示",
		"canonical Situation / thread facts", "この記事を書いている本人の自然な文章",
		"writing=勢いのある短文が多い。", "時代: 1996-06-07", "確定済み件名: 最近こればかりやってます",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing production material or purpose %q: %s", want, prompt)
		}
	}
	for _, old := range []string{
		"ルール:", "水増ししない", "文章をFAQ", "みなさんは？",
		"supplied historical facts", "同じ三文構成",
	} {
		if strings.Contains(prompt, old) {
			t.Fatalf("wording rule stack reintroduced %q: %s", old, prompt)
		}
	}
}

func TestBuildBoardPostPromptKeepsCanonicalSubjectInsteadOfRewritingIt(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic:       "パソコン通信・モデム",
		WorldDate:        "1996-08-29",
		AuthorHandle:     "NORI",
		PersonaProfile:   "writing=短く要点を書くこともある",
		CanonicalSubject: "Windows 95でモデムが認識されません",
		PostIntent:       "discourse_mode=ask_peers\noccurrence=Windows 95でモデムが認識されず相談する",
	})
	for _, want := range []string{
		"件名: Windows 95でモデムが認識されません",
		"canonical Situation / thread facts",
		`{"author":"...","subject":"Windows 95でモデムが認識されません","body":"..."}`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("canonical subject contract missing %q: %s", want, prompt)
		}
	}
	for _, legacy := range []string{"意味判定用タイトル", "短縮、口語化、省略", "固定パターン化しない"} {
		if strings.Contains(prompt, legacy) {
			t.Fatalf("legacy title-rewrite guidance survived refresh: %q", legacy)
		}
	}
}

func TestValidateBoardPostWorkerDraftRejectsHeaderNarration(t *testing.T) {
	req := BoardPostRequest{}
	for _, body := range []string{"5/22 00:27、新スレです。おすすめありますか。", "音楽板のMSG 1201です。YMOの話です。", "音楽板から失礼します。"} {
		if err := validateBoardPostWorkerDraft(req, BoardPostDraft{Author: "MARI", Subject: "x", Body: body}); err == nil {
			t.Fatalf("accepted metadata narration: %q", body)
		}
	}
}


func TestBuildBoardPostPromptCarriesCanonicalReferentAsMaterial(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic: "ANIME/MANGA", WorldDate: "1996-08-12",
		AuthorHandle: "KOJI.B", CanonicalSubject: "伏線に気づいた？",
		PostIntent: strings.Join([]string{
			"world_adopted_summary=前の回で聞き流した台詞が後の回を見て気になった",
			"article_detail=referent:新世紀エヴァンゲリオン",
			"article_referent_required=新世紀エヴァンゲリオン",
		}, "\n"),
	})
	if !strings.Contains(prompt, "本文で示すcanonical referent: 新世紀エヴァンゲリオン") {
		t.Fatalf("accepted referent missing from worker materials: %s", prompt)
	}
	if strings.Contains(prompt, "article_referent_required=") {
		t.Fatalf("internal render control leaked into worker materials: %s", prompt)
	}
	alreadyNamed := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic: "ANIME/MANGA", WorldDate: "1996-08-12",
		CanonicalSubject: "新世紀エヴァンゲリオンの台詞",
		PostIntent: "article_referent_required=新世紀エヴァンゲリオン",
	})
	if strings.Contains(alreadyNamed, "本文で示すcanonical referent:") {
		t.Fatalf("unnecessary repeated referent requirement: %s", alreadyNamed)
	}
}

func TestExtractArticleReferentControlPreservesOtherFacts(t *testing.T) {
	intent, referent := extractArticleReferentControl(strings.Join([]string{
		"discourse_mode=share_observation",
		"article_referent_required=新世紀エヴァンゲリオン",
		"article_detail=observation:前の回の台詞が気になった",
	}, "\n"))
	if referent != "新世紀エヴァンゲリオン" {
		t.Fatalf("referent=%q", referent)
	}
	if strings.Contains(intent, "article_referent_required=") || !strings.Contains(intent, "article_detail=observation:") {
		t.Fatalf("unexpected cleaned intent: %q", intent)
	}
}
