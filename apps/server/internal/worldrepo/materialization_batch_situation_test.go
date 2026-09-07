package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestDevelopmentBatchSituationValidatorRejectsSameBoardObject(t *testing.T) {
	roots := []developmentWindowShell{
		{eventID: "board-3:event-0001", board: world.Board{ID: "3"}, shell: developmentTimelineShell{createdAt: time.Now()}},
		{eventID: "board-3:event-0002", board: world.Board{ID: "3"}, shell: developmentTimelineShell{createdAt: time.Now().Add(time.Hour)}},
	}
	proposals := map[string]developmentSituationProposal{
		roots[0].eventID: {eventID: roots[0].eventID, objectClass: "資源回収", changeClass: "曜日変更", occurrence: "資源回収の曜日が変わった", actorObservation: "掲示を見た", noveltyKey: "resource-day"},
		roots[1].eventID: {eventID: roots[1].eventID, objectClass: "資源回収", changeClass: "場所変更", occurrence: "資源回収の場所が変わった", actorObservation: "掲示を見た", noveltyKey: "resource-place"},
	}
	accepted, rejected := developmentValidateSituationProposalBatch(roots, proposals, nil)
	if len(accepted) != 1 || len(rejected) != 1 {
		t.Fatalf("accepted=%d rejected=%v", len(accepted), rejected)
	}
	if !strings.Contains(rejected[roots[1].eventID], "object_class") {
		t.Fatalf("wrong rejection: %v", rejected)
	}
}

func TestDevelopmentBatchSituationValidatorAllowsDifferentObjects(t *testing.T) {
	roots := []developmentWindowShell{
		{eventID: "board-1:event-0001", board: world.Board{ID: "1"}},
		{eventID: "board-1:event-0002", board: world.Board{ID: "1"}},
	}
	proposals := map[string]developmentSituationProposal{
		roots[0].eventID: {eventID: roots[0].eventID, objectClass: "ゲーム内の名前", changeClass: "決めかねた", occurrence: "名前入力で少し迷った", actorObservation: "入力画面で手が止まった", noveltyKey: "game-name"},
		roots[1].eventID: {eventID: roots[1].eventID, objectClass: "帰宅時の雨", changeClass: "急に降った", occurrence: "帰り道で急に雨が強くなった", actorObservation: "歩いていて濡れた", noveltyKey: "rain-walk"},
	}
	accepted, rejected := developmentValidateSituationProposalBatch(roots, proposals, nil)
	if len(accepted) != 2 || len(rejected) != 0 {
		t.Fatalf("accepted=%d rejected=%v", len(accepted), rejected)
	}
}

func TestSparseSituationFromBatchProposalIsCanonicalBeforeProse(t *testing.T) {
	got := developmentSparseSituationFromProposal(developmentTimelineShell{discourseMode: "ask_peers"}, developmentSituationProposal{objectClass: "接続後の最初の画面", changeClass: "表示待ち", occurrence: "接続後に最初の画面が出るまで少し待った", actorObservation: "端末上で待ち時間を見た", uncertainty: "時間帯によるか不明", noveltyKey: "first-screen-wait"})
	joined := got.summary + "\n" + strings.Join(got.facts, "\n")
	for _, want := range []string{"batch_proposed", "accepted_batch_proposal_before_prose", "object_class=接続後の最初の画面", "answerability="} {
		if !strings.Contains(got.kind+"\n"+joined, want) {
			t.Fatalf("missing %q in %s", want, got.kind+"\n"+joined)
		}
	}
}
