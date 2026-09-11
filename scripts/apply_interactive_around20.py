from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:180]!r}")
    p.write_text(text.replace(old, new, 1))


# Ordinary ATDT observation should have enough world-selected shells that, after
# title-first rejection, roughly twenty committed articles remain per board.
path = "apps/server/internal/worldrepo/materialization_interactive_title_first.go"
replace_once(path, "\tr.SetDevelopmentConversationShellLimit(12)\n", "\tr.SetDevelopmentConversationShellLimit(24)\n")

path = "apps/server/internal/worldrepo/materialization_conversation_view.go"
replace_once(
    path,
    "\tif limit > 12 {\n\t\tlimit = 12\n\t}\n",
    "\tif limit > 24 {\n\t\tlimit = 24\n\t}\n",
)

path = "apps/server/internal/worldrepo/materialization_demo_persona.go"
replace_once(
    path,
    "const developmentInteractiveActivityLookbackDays = 28\n",
    "const developmentInteractiveActivityLookbackDays = 120\n",
)

# Specialized scale-fixture boards should use their own interest for visit
# affinity. They previously fell through to free-talk affinity even though root
# eligibility was already strict, which produced lots of irrelevant visits and
# too few usable roots on games/music/software.
path = "apps/server/internal/worldrepo/materialization_demo_dense.go"
replace_once(
    path,
    '''\tcase "3":\n\t\tvalues = []float64{interest("local"), interest("chat") * .55, interest("games") * .20}\n\tdefault:\n''',
    '''\tcase "3":\n\t\tvalues = []float64{interest("local"), interest("chat") * .55, interest("games") * .20}\n\tcase "4":\n\t\tvalues = []float64{interest("games")}\n\tcase "5":\n\t\tvalues = []float64{interest("music")}\n\tcase "6":\n\t\tvalues = []float64{interest("software")}\n\tdefault:\n''',
)

# Interactive browsing gets multiple independent 20-title pools, four root slots
# at a time. The isolated Lab intentionally keeps the one-pool experiment.
path = "apps/server/internal/worldrepo/materialization_interactive_board.go"
replace_once(
    path,
    '''\t// The Lab keeps one host-wide planning state. Interactive browsing is board\n\t// local: rearm title planning with already-committed posts as conversation\n\t// history so opening board 2 never forces board 1 to be regenerated.\n\tr.EnableDevelopmentTitleFirstPoC(r.Base.ListPosts(host.ID))\n\tbatchSituations, err := r.developmentPlanTitleFirst(host, windowShells, personas)\n''',
    '''\t// The Lab keeps one host-wide, one-pool planning state. Interactive browsing\n\t// is board-local and may refill the uncommitted wording pool so a larger world\n\t// sample is not starved merely because one set of 20 titles missed its roots.\n\tbatchSituations, err := r.developmentPlanInteractiveTitleFirst(host, windowShells, personas)\n''',
)

Path("apps/server/internal/worldrepo/materialization_interactive_title_refill.go").write_text(r'''package worldrepo

import (
    "sort"

    "zutto-pccom/apps/server/internal/world"
)

const developmentInteractiveTitleRootsPerPool = 4

// developmentPlanInteractiveTitleFirst keeps the original title-first invariant:
// every accepted title starts as uncommitted wording and becomes a world fact only
// after slot/persona/Era validation. The ordinary ATDT observation path simply
// gives later root slots another independent 20-title pool instead of asking one
// pool to cover an entire ~20-article board sample. Fresh Lab remains one-pool.
func (r *Repository) developmentPlanInteractiveTitleFirst(host world.Host, window []developmentWindowShell, personas []world.Persona) (map[string]developmentSparseSituation, error) {
    roots := make([]developmentWindowShell, 0, len(window))
    for _, item := range window {
        shell := item.shell
        if shell.action == "thread_start" && shell.parentIndex == 0 && shell.sourceIndex == 0 {
            roots = append(roots, item)
        }
    }
    sort.SliceStable(roots, func(i, j int) bool {
        if roots[i].shell.createdAt.Equal(roots[j].shell.createdAt) {
            return roots[i].eventID < roots[j].eventID
        }
        return roots[i].shell.createdAt.Before(roots[j].shell.createdAt)
    })

    merged := map[string]developmentSparseSituation{}
    if len(roots) == 0 {
        return merged, nil
    }

    history := append([]world.Post(nil), r.Base.ListPosts(host.ID)...)
    allRows := make([]DevelopmentTitleCandidate, 0)
    researchUsed := 0

    for start := 0; start < len(roots); start += developmentInteractiveTitleRootsPerPool {
        end := start + developmentInteractiveTitleRootsPerPool
        if end > len(roots) {
            end = len(roots)
        }
        batch := append([]developmentWindowShell(nil), roots[start:end]...)

        r.EnableDevelopmentTitleFirstPoC(history)
        stateValue, _ := developmentTitleFirst.Load(r)
        state := stateValue.(*developmentTitleFirstState)
        state.eraResearchUsed = researchUsed

        planned, err := r.developmentPlanTitleFirst(host, batch, personas)
        allRows = append(allRows, state.rows...)
        researchUsed = state.eraResearchUsed
        if err != nil {
            developmentTitleFirst.Store(r, &developmentTitleFirstState{
                history: append([]world.Post(nil), history...), attempted: true,
                result: merged, err: err, rows: allRows, eraResearchUsed: researchUsed,
            })
            return nil, err
        }
        for eventID, situation := range planned {
            merged[eventID] = situation
        }

        // Accepted earlier roots are canonical within this planning operation.
        // Feed only their accepted title/situation back as transient BBS history
        // so later pools can avoid obvious repetition without treating rejected
        // candidates as facts.
        for _, item := range batch {
            situation, ok := planned[item.eventID]
            if !ok {
                continue
            }
            subject := titleFirstSubject(situation.facts)
            if subject == "" {
                continue
            }
            history = append(history, world.Post{
                BoardID: item.board.ID,
                Author: item.shell.persona.Handle,
                AuthorPersonaID: item.shell.persona.ID,
                Subject: subject,
                CreatedAt: item.shell.createdAt,
                Intent: world.PostIntent{
                    Action: item.shell.action,
                    AnchorKey: item.shell.anchorKey,
                    CauseKind: item.shell.causeKind,
                    DiscourseMode: item.shell.discourseMode,
                    SituationKind: situation.kind,
                    SituationSummary: situation.summary,
                    SituationFacts: append([]string(nil), situation.facts...),
                    Topic: item.shell.anchorKey,
                    Motivation: item.shell.causeSummary,
                },
            })
        }
    }

    developmentTitleFirst.Store(r, &developmentTitleFirstState{
        history: append([]world.Post(nil), history...), attempted: true,
        result: merged, rows: allRows, eraResearchUsed: researchUsed,
    })
    return merged, nil
}
''')

# Update regression expectations and cover specialized-board affinity.
path = "apps/server/internal/worldrepo/materialization_interactive_title_first_test.go"
replace_once(
    path,
    '''\tif got := developmentConversationShellLimit(repo); got != 12 {\n\t\tt.Fatalf("shell limit=%d, want 12 for ordinary ATDT observation", got)\n\t}\n''',
    '''\tif got := developmentConversationShellLimit(repo); got != 24 {\n\t\tt.Fatalf("shell limit=%d, want 24 for ordinary ATDT observation", got)\n\t}\n''',
)

path = "apps/server/internal/worldrepo/materialization_interactive_scale_test.go"
replace_once(
    path,
    '''\tif got := developmentConversationShellLimit(repo); got != 12 {\n\t\tt.Fatalf("interactive shell limit=%d, want 12", got)\n\t}\n''',
    '''\tif got := developmentConversationShellLimit(repo); got != 24 {\n\t\tt.Fatalf("interactive shell limit=%d, want 24", got)\n\t}\n''',
)
with Path(path).open("a") as f:
    f.write(r'''

func TestSpecializedBoardAffinityUsesMatchingInterest(t *testing.T) {
    p := world.Persona{Interests: map[string]float64{"games": .8, "music": .6, "software": .9}}
    tests := []struct {
        board world.Board
        want  float64
    }{
        {world.Board{ID: "4", Name: "ゲーム"}, .8},
        {world.Board{ID: "5", Name: "音楽"}, .6},
        {world.Board{ID: "6", Name: "ソフトウェア"}, .9},
    }
    for _, tc := range tests {
        if got := demoBoardAffinity(p, tc.board); got != tc.want {
            t.Fatalf("board %s affinity=%v want=%v", tc.board.ID, got, tc.want)
        }
    }
    unrelated := world.Persona{Interests: map[string]float64{"chat": 1, "local": 1}}
    if got := demoBoardAffinity(unrelated, world.Board{ID: "6", Name: "ソフトウェア"}); got != 0 {
        t.Fatalf("unrelated software affinity=%v want=0", got)
    }
}
''')

# Document the observation scale. This does not redefine normal world density.
path = "docs/MATERIALIZATION_LAB.md"
replace_once(
    path,
    '''傾向確認用の通常runtimeだけは、板を6種（フリートーク／パソコン通信・モデム／地域の話題／ゲーム／音楽／ソフトウェア）に広げ、過去28日の活動から1板最大12 Envelopeを選ぶ。fresh Labの `board_count` / `shell_limit` と14日活動窓は比較条件として従来どおり維持する。''',
    '''傾向確認用の通常runtimeだけは、板を6種（フリートーク／パソコン通信・モデム／地域の話題／ゲーム／音楽／ソフトウェア）に広げ、最大120日の活動から1板最大24のworld-selected shellを選ぶ。title-firstは4 rootごとに独立した20候補poolを補充し、1回の候補偏りだけで大きな観察標本が空洞化しないようにする。これは開発ホストの観察用スケールであり、通常世界の投稿密度を20件固定する仕様ではない。fresh Labの `board_count` / `shell_limit` と14日活動窓・1板1候補poolは比較条件として従来どおり維持する。''',
)
