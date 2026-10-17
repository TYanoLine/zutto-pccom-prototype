package llm

import (
    "strings"
    "strconv"
    "testing"
)

func TestHAKATAFreeformPromptFollowsTitleAndUsesRealBoardName(t *testing.T) {
    prompt:=BuildBoardPostPrompt(BoardPostRequest{
        FreeformFromSubject:true,
        HostName:"HAKATA CANAL NET",
        BoardTopic:"アニメ・漫画",
        WorldDate:"1996-02-17",
        AuthorHandle:"KOJI.I",
        PersonaProfile:"writing=気さくな口調; interests=[anime_manga,games]",
        CanonicalSubject:"プリンセスメーカー2、月末結果の違い",
        PostIntent:"確定した状況: 月末の結果を比較して気づいたことを話す",
        EraRules:"世界日付の日本に暮らす当時の会員の視点。追加の内部ルール",
        BodyMinChars:161, BodyMaxChars:320,
    })
    for _, want:=range []string{
        "世界日付: 1996-02-17",
        "局: HAKATA CANAL NET",
        "掲示板: アニメ・漫画",
        "確定済み件名: プリンセスメーカー2、月末結果の違い",
        "handle=KOJI.I",
        "writing=気さくな口調",
        "月末の結果を比較して気づいた",
        "本人の自然な文章",
        "本文の長さ: 最大320文字。",
        `"subject":"プリンセスメーカー2、月末結果の違い"`,
    }{
        if !strings.Contains(prompt,want){t.Fatalf("missing %q in freeform prompt:\n%s",want,prompt)}
    }
    for _, bad:=range []string{
        "掲示板: プリンセスメーカー2",
        "canonical Situation / thread facts",
        "article_detail_contract",
        "時代資料:\n(なし)",
        "親記事:\n(root記事なのでなし)",
    }{
        if strings.Contains(prompt,bad){t.Fatalf("unnecessary material %q in freeform prompt:\n%s",bad,prompt)}
    }
}

func TestHAKATAFreeformPromptPreservesReplyTargetAndRequiredName(t *testing.T){
    prompt:=BuildBoardPostPrompt(BoardPostRequest{
        FreeformFromSubject:true,
        WorldDate:"1996-02-17",
        BoardTopic:"アニメ・漫画",
        AuthorHandle:"REI",
        ParentSubject:"月末結果の話",
        ParentBody:"私は同じ条件でも少し違う結果でした。",
        PostIntent:"確定した状況: 比較への返事\narticle_referent_required=プリンセスメーカー2",
        QuoteText:"私は同じ条件でも少し違う結果でした。",
    })
    for _, want:=range []string{
        "掲示板: アニメ・漫画",
        "確定済み件名: (返信・ホスト上の表示件名なし)",
        "返信先",
        "私は同じ条件でも少し違う結果でした。",
        "この記事の確定済み参照対象: プリンセスメーカー2",
        `"subject":"月末結果の話"`,
    }{
        if !strings.Contains(prompt,want){t.Fatalf("missing %q:\n%s",want,prompt)}
    }
    if strings.Contains(prompt,"article_referent_required="){t.Fatalf("internal control leaked:\n%s",prompt)}
}

func TestHAKATAFreeformPromptEscapesSubjectJSON(t *testing.T){
    prompt:=BuildBoardPostPrompt(BoardPostRequest{
        FreeformFromSubject:true,
        AuthorHandle:`KOJI"1`,
        BoardTopic:"GAME",
        CanonicalSubject:`「test"quote」`,
    })
    if !strings.Contains(prompt, `"author":`+strconv.Quote(`KOJI"1`)) || !strings.Contains(prompt, `"subject":`+strconv.Quote(`「test"quote」`)){
        t.Fatalf("unsafe JSON template: %s",prompt)
    }
}

func TestBuildBoardPostPromptIncludesTerminalLineLayoutGuidance(t *testing.T) {
	freeformPrompt := BuildBoardPostPrompt(BoardPostRequest{
		FreeformFromSubject: true,
		BoardTopic:          "GAME",
		CanonicalSubject:    "テスト件名",
	})
	if !strings.Contains(freeformPrompt, "全角30〜35文字程度を目安") {
		t.Fatalf("freeform prompt missing terminal line layout guidance:\n%s", freeformPrompt)
	}

	standardPrompt := BuildBoardPostPrompt(BoardPostRequest{
		FreeformFromSubject: false,
		BoardTopic:          "GAME",
		CanonicalSubject:    "テスト件名",
	})
	if !strings.Contains(standardPrompt, "全角30〜35文字程度を目安") {
		t.Fatalf("standard prompt missing terminal line layout guidance:\n%s", standardPrompt)
	}
}

func TestFreeformPromptRendersThreadContextChronologically(t *testing.T) {
	prompt := BuildBoardPostPrompt(BoardPostRequest{
		FreeformFromSubject: true,
		BoardTopic:          "新人歓迎・自己紹介",
		WorldDate:           "1996-02-04",
		AuthorHandle:        "MEG.N",
		ParentSubject:       "新しく参加した方、気軽にどうぞ（SHIN）",
		ParentBody:          "はじめまして、SHINです。新しく参加されたみなさん、ようこそ！",
		ThreadContext: []ThreadPostContext{
			{
				PostID:  2119,
				Author:  "SHIN",
				Subject: "新しく参加した方、気軽にどうぞ（SHIN）",
				Body:    "はじめまして、SHINです。新しく参加されたみなさん、ようこそ！",
				IsRoot:  true,
			},
			{
				PostID:  2120,
				Author:  "MEG.M",
				Body:    "SHINさん、はじめまして。MEG.Mです。歓迎の言葉、ありがとうございます。",
				IsRoot:  false,
			},
		},
		PostIntent: "確定した状況: MEG.N が SHIN の件名「新しく参加した方、気軽にどうぞ（SHIN）」の少し前の記事へ返信する\nこれまでの会話:\nTHREAD CONTEXT (canonical article content only):\n[SHIN] 新しく参加した方、気軽にどうぞ（SHIN）\n[MEG.M] 歓迎の言葉、ありがとうございます。",
	})
	for _, want := range []string{
		"スレッドの文脈（これまでの発言の流れ・時系列）:",
		"[1] 発言者: SHIN (親記事/スレッド起点) / 件名: 新しく参加した方、気軽にどうぞ（SHIN）",
		"[2] 発言者: MEG.M",
		"直接の返信先（あなたが返信する対象）:",
		"スレッドに先行する他の発言や返信がある場合は、スレッド全体の会話の流れを踏まえて自然に書き込んでください。",
		"先行する他の返信者と同じ定型的な挨拶を単にオウム返しするのではなく",
		"スレッドの先行返信が呼びかけで始まっていても、同じ形で書き出さず",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in prompt:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "THREAD CONTEXT (canonical article content only)") {
		t.Fatalf("legacy raw thread context not stripped from intent:\n%s", prompt)
	}
}

