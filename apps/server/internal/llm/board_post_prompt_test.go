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

func TestBuildBoardPostPromptAllowsSparseUnfinishedHumanPosts(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{BoardTopic: "ゲーム", WorldDate: "1996-06-07", AuthorHandle: "YUKI", PersonaProfile: "writing=勢いのある短文が多い。", CanonicalSubject: "最近こればかりやってます"})
	for _, want := range []string{
		"canonical Situation はすでに世界で起きた事実",
		"事実を全部説明する必要はない",
		"本文は用件から自然に始める",
		"ask_peersだけが質問を主目的",
		"文章をFAQ・解説・結論付きの整った記事へ無理に仕上げない",
		"writing=勢いのある短文が多い。",
		"世界日付は 1996-06-07",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing concise Situation-rendering guidance %q", want)
		}
	}
	for _, legacy := range []string{"utterance_attention", "10〜20文字程度ごと", "端末側の80桁級表示", "同じ三文構成"} {
		if strings.Contains(prompt, legacy) {
			t.Fatalf("legacy prose micromanagement survived refresh: %q", legacy)
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


func TestBuildBoardPostPromptRequiresRootReferentToSurface(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		BoardTopic:       "ANIME/MANGA",
		WorldDate:        "1996-08-12",
		AuthorHandle:     "KOJI.B",
		CanonicalSubject: "伏線に気づいた？",
		PostIntent: strings.Join([]string{
			"world_adopted_summary=前の回で聞き流した台詞が後の回を見て気になった",
			"article_detail=referent:新世紀エヴァンゲリオン",
			"article_detail=observation:前の回では聞き流した台詞が、後の回を見てから気になった",
			"article_referent_required=新世紀エヴァンゲリオン",
		}, "\n"),
	})
	for _, want := range []string{
		"canonical referent「新世紀エヴァンゲリオン」",
		"表示件名がこの対象名を明示していない場合",
		"本文の自然な位置で対象名を少なくとも一度",
		"対象を別の作品・製品・店等へ置き換えない",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("required referent guidance missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "article_referent_required=") {
		t.Fatalf("render-control metadata leaked into canonical fact block:\n%s", prompt)
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
