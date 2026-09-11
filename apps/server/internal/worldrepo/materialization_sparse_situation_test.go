package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestSparseSituationAvoidsImmediateBoardFacetRepeat(t *testing.T) {
	host := world.Host{ID: "host-test"}
	board := world.Board{ID: "3", Name: "地域の話題"}
	persona := world.Persona{ID: "persona-a", Handle: "A"}
	at := time.Date(1996, 8, 20, 20, 0, 0, 0, time.Local)
	firstShell := developmentTimelineShell{
		index:         1,
		persona:       persona,
		createdAt:     at.Add(-time.Hour),
		action:        "thread_start",
		anchorKey:     "local",
		causeKind:     "recent_salience",
		discourseMode: "share_observation",
	}
	first := developmentSelectRootSituation(host, board, firstShell, nil)
	prior := []world.Post{{
		ID:              1,
		BoardID:         board.ID,
		Author:          "B",
		AuthorPersonaID: "persona-b",
		CreatedAt:       firstShell.createdAt,
		Intent:          world.PostIntent{SituationKind: first.kind},
	}}
	secondShell := firstShell
	secondShell.index = 2
	secondShell.createdAt = at
	second := developmentSelectRootSituation(host, board, secondShell, prior)
	if second.kind == first.kind {
		t.Fatalf("immediate board situation facet repeated: %q", second.kind)
	}
}

func TestSparseSituationAvoidsSameActorFacetWithinTenDays(t *testing.T) {
	host := world.Host{ID: "host-test"}
	board := world.Board{ID: "1", Name: "フリートーク"}
	persona := world.Persona{ID: "persona-a", Handle: "A"}
	at := time.Date(1996, 8, 20, 20, 0, 0, 0, time.Local)
	seedShell := developmentTimelineShell{
		index:         1,
		persona:       persona,
		createdAt:     at.Add(-5 * 24 * time.Hour),
		action:        "thread_start",
		anchorKey:     "games",
		causeKind:     "recent_salience",
		discourseMode: "share_experience",
	}
	first := developmentSelectRootSituation(host, board, seedShell, nil)
	prior := []world.Post{{
		ID:              1,
		BoardID:         board.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		CreatedAt:       seedShell.createdAt,
		Intent:          world.PostIntent{SituationKind: first.kind},
	}}
	later := seedShell
	later.index = 2
	later.createdAt = at
	second := developmentSelectRootSituation(host, board, later, prior)
	if second.kind == first.kind {
		t.Fatalf("same actor reused situation facet within ten days: %q", second.kind)
	}
}

func TestSparseSituationReplyInheritsCanonicalSource(t *testing.T) {
	host := world.Host{ID: "host-test"}
	board := world.Board{ID: "2", Name: "パソコン通信・モデム"}
	source := world.Post{
		ID:        17,
		BoardID:   board.ID,
		CreatedAt: time.Date(1996, 8, 18, 22, 0, 0, 0, time.Local),
		Intent: world.PostIntent{
			SituationKind:    "communications_post_confirmation",
			SituationSummary: "source situation",
			SituationFacts:   []string{"occurrence=source fact"},
		},
	}
	shell := developmentTimelineShell{
		index:       2,
		persona:     world.Persona{ID: "persona-b", Handle: "B"},
		createdAt:   source.CreatedAt.Add(time.Hour),
		action:      "reply",
		anchorKey:   "communications",
		causeKind:   "observed_thread",
		parentIndex: 1,
		sourceIndex: 1,
	}
	got := developmentSituationForShell(host, board, shell, nil, &source)
	if got.kind != source.Intent.SituationKind || !strings.Contains(got.summary, "source situation") {
		t.Fatalf("reply lost canonical source situation: %#v", got)
	}
	joined := strings.Join(got.facts, "\n")
	if !strings.Contains(joined, "reply_binding=") {
		t.Fatalf("reply situation missing binding guard: %s", joined)
	}
}

func TestSparseAskPeersSituationRequiresAnswerableReferent(t *testing.T) {
	host := world.Host{ID: "host-test"}
	board := world.Board{ID: "1", Name: "フリートーク"}
	shell := developmentTimelineShell{
		index:         1,
		persona:       world.Persona{ID: "persona-a", Handle: "A"},
		createdAt:     time.Date(1996, 8, 20, 20, 0, 0, 0, time.Local),
		action:        "thread_start",
		anchorKey:     "games",
		causeKind:     "recent_salience",
		discourseMode: "ask_peers",
	}
	got := developmentSelectRootSituation(host, board, shell, nil)
	text := got.summary + "\n" + strings.Join(got.facts, "\n")
	if !strings.Contains(text, "answerable") || !strings.Contains(text, "answerability=") {
		t.Fatalf("ask_peers situation lacks answerability guard: %s", text)
	}
}
