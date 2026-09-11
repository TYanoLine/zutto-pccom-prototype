package worldrepo

import (
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestEnableDevelopmentInteractiveTitleFirstPoCUsesLargerObservationCap(t *testing.T) {
	repo := New(world.NewMemoryStore(), nil, nil, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	if !developmentConversationViewPoCEnabled(repo) {
		t.Fatal("conversation view must be enabled for ordinary ATDT development-host access")
	}
	if !developmentTitleFirstEnabled(repo) {
		t.Fatal("title-first must be enabled for ordinary ATDT development-host access")
	}
	if got := developmentConversationShellLimit(repo); got != 24 {
		t.Fatalf("shell limit=%d, want 24 for ordinary ATDT observation", got)
	}
}

func TestResetMaterializationConversationRearmsInteractiveTitleFirst(t *testing.T) {
	base := world.NewMemoryStore()
	repo := New(base, nil, nil, "1996-08-29")
	repo.EnableDevelopmentInteractiveTitleFirstPoC()
	host, err := repo.HostByPhone("0450000196")
	if err != nil {
		t.Fatal(err)
	}
	stateValue, ok := developmentTitleFirst.Load(repo)
	if !ok {
		t.Fatal("title-first state missing")
	}
	old := stateValue.(*developmentTitleFirstState)
	old.attempted = true
	old.rows = []DevelopmentTitleCandidate{{BoardID: "1", Candidate: 1, Original: "old"}}

	if _, _, ok := repo.ResetMaterializationConversation(host); !ok {
		t.Fatal("development reset unsupported")
	}
	stateValue, ok = developmentTitleFirst.Load(repo)
	if !ok {
		t.Fatal("title-first state disappeared after reset")
	}
	next := stateValue.(*developmentTitleFirstState)
	if next == old || next.attempted || len(next.rows) != 0 {
		t.Fatalf("title-first was not rearmed: %+v", next)
	}
}
