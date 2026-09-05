package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func participationPersona(id, handle string) world.Persona {
	return world.Persona{
		ID:               id,
		Handle:           handle,
		ReplyTendency:    .70,
		NewcomerOpenness: .70,
		Interests:        map[string]float64{"local": .8},
	}
}

func participationShell(index, parent int, persona world.Persona, at time.Time) developmentTimelineShell {
	action := "reply"
	if parent == 0 {
		action = "thread_start"
	}
	return developmentTimelineShell{
		index:       index,
		persona:     persona,
		createdAt:   at,
		action:      action,
		parentIndex: parent,
		anchorKey:   "local",
	}
}

func TestParticipationStateBlocksRepeatWithoutNewContribution(t *testing.T) {
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)
	taka := participationPersona("taka", "TAKA")
	mari := participationPersona("mari", "MARI")
	root := participationShell(1, 0, taka, base)
	firstMari := participationShell(2, 1, mari, base.Add(time.Hour))

	_, returning, ok := demoParticipationReplySource(mari, root, []developmentTimelineShell{root, firstMari})
	if !returning {
		t.Fatal("expected MARI to be recognized as a returning participant")
	}
	if ok {
		t.Fatal("unchanged thread must not authorize a second MARI contribution")
	}
}

func TestParticipationStateRequiresAnotherActorAfterLastContribution(t *testing.T) {
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)
	taka := participationPersona("taka", "TAKA")
	mari := participationPersona("mari", "MARI")
	sysop := participationPersona("sysop", "SYSOP")
	root := participationShell(1, 0, taka, base)
	firstMari := participationShell(2, 1, mari, base.Add(time.Hour))
	laterSysop := participationShell(3, 1, sysop, base.Add(2*time.Hour))

	source, returning, ok := demoParticipationReplySource(mari, root, []developmentTimelineShell{root, firstMari, laterSysop})
	if !ok || !returning {
		t.Fatalf("later contribution should make a return eligible before sparse gate: ok=%v returning=%v", ok, returning)
	}
	if source.index != laterSysop.index {
		t.Fatalf("return source=%d, want latest other contribution %d", source.index, laterSysop.index)
	}
}

func TestParticipationStateFirstContributionUsesLatestThreadSource(t *testing.T) {
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)
	taka := participationPersona("taka", "TAKA")
	mari := participationPersona("mari", "MARI")
	sysop := participationPersona("sysop", "SYSOP")
	root := participationShell(1, 0, taka, base)
	laterSysop := participationShell(2, 1, sysop, base.Add(time.Hour))

	source, returning, ok := demoParticipationReplySource(mari, root, []developmentTimelineShell{root, laterSysop})
	if !ok || returning {
		t.Fatalf("first contribution should be eligible and non-returning: ok=%v returning=%v", ok, returning)
	}
	if source.index != laterSysop.index {
		t.Fatalf("first contribution source=%d, want latest thread contribution %d", source.index, laterSysop.index)
	}
}

func TestParticipationStateRootAuthorCannotReplyToUnchangedOwnRoot(t *testing.T) {
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)
	taka := participationPersona("taka", "TAKA")
	root := participationShell(1, 0, taka, base)

	_, returning, ok := demoParticipationReplySource(taka, root, []developmentTimelineShell{root})
	if !returning {
		t.Fatal("root author is already a participant")
	}
	if ok {
		t.Fatal("root author must not manufacture a reply before anyone else contributes")
	}
}

func TestParticipationReturnGateIsSparseRatherThanAutomatic(t *testing.T) {
	host := world.Host{ID: "materialize-demo"}
	board := world.Board{ID: "3", Name: "地域の話題"}
	mari := participationPersona("mari", "MARI")
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)

	passes := 0
	for i := 0; i < 200; i++ {
		if demoShouldReturnToThread(host, board, mari, base.Add(time.Duration(i)*time.Hour), 1, 3, i) {
			passes++
		}
	}
	if passes == 0 {
		t.Fatal("return gate should still permit occasional natural back-and-forth")
	}
	if passes >= 80 {
		t.Fatalf("return gate is too permissive: %d/200 passed", passes)
	}
}

func TestParticipationReturnCauseNamesNewSourceAndForbidsRestatement(t *testing.T) {
	base := time.Date(1996, 8, 20, 21, 0, 0, 0, time.UTC)
	taka := participationPersona("taka", "TAKA")
	sysop := participationPersona("sysop", "SYSOP")
	root := participationShell(1, 0, taka, base)
	source := participationShell(3, 1, sysop, base.Add(2*time.Hour))

	summary := developmentReplyCauseSummary(developmentReplyTarget{root: root, source: source, returning: true})
	for _, want := range []string{"event 0003", "already contributed", "do not merely restate"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("return cause summary missing %q: %s", want, summary)
		}
	}
}
