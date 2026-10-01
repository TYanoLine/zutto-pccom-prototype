package llm

import (
    "strings"
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
        "本文の目安: 161〜320文字",
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
    if !strings.Contains(prompt,`"author":"KOJI\\"1"`) || !strings.Contains(prompt,`"subject":"「test\\"quote」"`){
        t.Fatalf("unsafe JSON template: %s",prompt)
    }
}
