package llm

import (
	"fmt"
	"testing"
)

func testBBSArticleBatchDraft(req BBSArticleBatchRequest) BBSArticleBatchDraft {
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = fmt.Sprintf("候補タイトル%02d", i+1)
	}
	posts := make([]BBSArticleBatchPost, 0, len(req.Slots))
	nextCandidate := 1
	for _, slot := range req.Slots {
		post := BBSArticleBatchPost{
			SlotIndex:        slot.Index,
			Body:             "本文です。",
			Topic:            "topic",
			Motivation:       "motivation",
			Stance:           "neutral",
			Goal:             "share",
			SituationSummary: "summary",
		}
		if slot.Kind == "reply" {
			post.Candidate = 0
		} else {
			post.Candidate = nextCandidate
			post.Subject = candidates[nextCandidate-1]
			nextCandidate++
		}
		posts = append(posts, post)
	}
	return BBSArticleBatchDraft{Candidates: candidates, Posts: posts}
}

func TestValidateBBSArticleBatchAcceptsRootAndReplySlots(t *testing.T) {
	req := BBSArticleBatchRequest{
		Slots: []BBSArticleBatchSlot{
			{Index: 1, Kind: "root"},
			{Index: 2, Kind: "root"},
			{Index: 3, Kind: "reply", ReplyToPostID: 42, ReplyToSubject: "元記事"},
		},
		RecentSubjects: []string{"まったく別の昔の記事"},
	}
	draft := testBBSArticleBatchDraft(req)
	if err := ValidateBBSArticleBatch(req, draft); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBBSArticleBatchRejectsRecentNearDuplicate(t *testing.T) {
	req := BBSArticleBatchRequest{
		Slots:          []BBSArticleBatchSlot{{Index: 1, Kind: "root"}},
		RecentSubjects: []string{"セガサターンのおすすめソフトありますか？"},
	}
	draft := testBBSArticleBatchDraft(req)
	draft.Candidates[0] = "セガサターンのおすすめソフトありますか"
	draft.Posts[0].Subject = draft.Candidates[0]
	if err := ValidateBBSArticleBatch(req, draft); err == nil {
		t.Fatal("near-duplicate recent title was accepted")
	}
}

func TestValidateBBSArticleBatchRejectsRewrittenCandidate(t *testing.T) {
	req := BBSArticleBatchRequest{Slots: []BBSArticleBatchSlot{{Index: 1, Kind: "root"}}}
	draft := testBBSArticleBatchDraft(req)
	draft.Posts[0].Subject = "候補を勝手に書き換え"
	if err := ValidateBBSArticleBatch(req, draft); err == nil {
		t.Fatal("rewritten candidate was accepted")
	}
}
