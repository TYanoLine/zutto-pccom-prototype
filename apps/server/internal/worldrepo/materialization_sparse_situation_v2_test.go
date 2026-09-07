package worldrepo

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestSparseSituationV2AvoidsBoardFacetRepeatForOneWeek(t *testing.T) {
	host := world.Host{ID: "host-test"}
	board := world.Board{ID: "3", Name: "地域の話題"}
	persona := world.Persona{ID: "persona-a", Handle: "A"}
	at := time.Date(1996, 8, 20, 20, 0, 0, 0, time.Local)
	seed := developmentTimelineShell{
		index:         1,
		persona:       persona,
		createdAt:     at.Add(-5 * 24 * time.Hour),
		action:        "thread_start",
		anchorKey:     "local",
		causeKind:     "recent_salience",
		discourseMode: "share_observation",
	}
	first := developmentSelectRootSituationV2(host, board, seed, nil)
	prior := []world.Post{{
		ID:              1,
		BoardID:         board.ID,
		Author:          "OTHER",
		AuthorPersonaID: "persona-other",
		CreatedAt:       seed.createdAt,
		Intent:          world.PostIntent{SituationKind: first.kind},
	}}
	later := seed
	later.index = 2
	later.createdAt = at
	later.persona = world.Persona{ID: "persona-b", Handle: "B"}
	second := developmentSelectRootSituationV2(host, board, later, prior)
	if second.kind == first.kind {
		t.Fatalf("board reused facet within five days despite alternatives: %q", second.kind)
	}
}

func TestSparseSituationV2ReplyChainCarriesOnlyCoreSituationFacts(t *testing.T) {
	root := world.Post{
		ID:      100,
		BoardID: "2",
		Intent: world.PostIntent{
			SituationKind:    "communications_line_coordination",
			SituationSummary: "The actor has one concrete, answerable uncertainty involving household line timing.",
			SituationFacts: []string{
				"focus=sharing an ordinary household telephone line",
				"occurrence=A planned voice call changed when the actor chose to connect.",
				"scope_boundary=Keep it to scheduling and availability.",
				"root_independence=Do not import another root event.",
				"answerability=State enough detail for peers to answer.",
			},
		},
	}
	firstReplySituation := developmentSituationFromSourceV2(root, false)
	firstReply := world.Post{
		ID:      101,
		BoardID: "2",
		Intent: world.PostIntent{
			SituationKind:    firstReplySituation.kind,
			SituationSummary: firstReplySituation.summary,
			SituationFacts:   append([]string(nil), firstReplySituation.facts...),
		},
	}
	secondReplySituation := developmentSituationFromSourceV2(firstReply, false)
	joined := strings.Join(secondReplySituation.facts, "\n")
	if got := strings.Count(joined, "reply_binding="); got != 1 {
		t.Fatalf("reply chain accumulated reply bindings: count=%d facts=%s", got, joined)
	}
	for _, forbidden := range []string{"root_independence=", "answerability="} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("reply inherited root-only renderer guard %q: %s", forbidden, joined)
		}
	}
	if !strings.Contains(secondReplySituation.summary, "reply inside the canonical source situation") {
		t.Fatalf("reply summary does not describe reply relation: %q", secondReplySituation.summary)
	}
	if strings.Contains(strings.ToLower(secondReplySituation.summary), "answerable uncertainty") {
		t.Fatalf("root discourse summary leaked into reply: %q", secondReplySituation.summary)
	}
}

func TestSparseSituationV2ContinuationCarriesCoreFactsAndOneProgressGuard(t *testing.T) {
	source := world.Post{
		ID: 200,
		Intent: world.PostIntent{
			SituationKind: "games_progress_setback",
			SituationFacts: []string{
				"focus=a recent progress setback followed by another attempt",
				"occurrence=A small mistake ruined a promising attempt, but a retry went smoothly.",
				"scope_boundary=Keep gameplay self-contained.",
				"root_independence=old guard",
			},
		},
	}
	got := developmentSituationFromSourceV2(source, true)
	joined := strings.Join(got.facts, "\n")
	if strings.Count(joined, "continuation=") != 1 {
		t.Fatalf("continuation guard count wrong: %s", joined)
	}
	if strings.Contains(joined, "root_independence=") || strings.Contains(joined, "reply_binding=") {
		t.Fatalf("continuation inherited non-core guards: %s", joined)
	}
	if !strings.Contains(got.summary, "materially new development") {
		t.Fatalf("continuation summary missing progress requirement: %q", got.summary)
	}
}
