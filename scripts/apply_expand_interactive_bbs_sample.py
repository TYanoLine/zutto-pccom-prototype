from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:180]!r}")
    p.write_text(text.replace(old, new, 1))


# Ordinary ATDT observation uses a larger envelope cap than the isolated Lab.
path = "apps/server/internal/worldrepo/materialization_interactive_title_first.go"
replace_once(
    path,
    "\tr.SetDevelopmentConversationShellLimit(8)\n",
    "\tr.SetDevelopmentConversationShellLimit(12)\n",
)

path = "apps/server/internal/worldrepo/materialization_conversation_view.go"
replace_once(
    path,
    "\tif limit > 10 {\n\t\tlimit = 10\n\t}\n",
    "\tif limit > 12 {\n\t\tlimit = 12\n\t}\n",
)

# Keep the Lab/default world window at 14 days. Only the ordinary interactive
# dial-up sample looks back 28 days so a user can see enough posts to judge trends.
path = "apps/server/internal/worldrepo/materialization_demo_persona.go"
replace_once(
    path,
    '''func developmentVisitsForBoard(host world.Host, board world.Board, personas []world.Persona, worldDate string) []demoPostCandidate {\n\tstamp := worldTime(worldDate)\n\tvisits := make([]demoPostCandidate, 0, 32)\n\tfor _, persona := range personas {\n\t\tdays := make([]demoActivityDay, 0, 14)\n\t\texpected := 0.0\n\t\tfor dayBack := 13; dayBack >= 0; dayBack-- {\n''',
    '''const developmentDefaultActivityLookbackDays = 14\nconst developmentInteractiveActivityLookbackDays = 28\n\nfunc developmentVisitsForBoard(host world.Host, board world.Board, personas []world.Persona, worldDate string) []demoPostCandidate {\n\treturn developmentVisitsForBoardDays(host, board, personas, worldDate, developmentDefaultActivityLookbackDays)\n}\n\nfunc developmentVisitsForBoardDays(host world.Host, board world.Board, personas []world.Persona, worldDate string, lookbackDays int) []demoPostCandidate {\n\tif lookbackDays < 1 {\n\t\tlookbackDays = 1\n\t}\n\tstamp := worldTime(worldDate)\n\tvisits := make([]demoPostCandidate, 0, len(personas)*lookbackDays)\n\tfor _, persona := range personas {\n\t\tdays := make([]demoActivityDay, 0, lookbackDays)\n\t\texpected := 0.0\n\t\tfor dayBack := lookbackDays - 1; dayBack >= 0; dayBack-- {\n''',
)

path = "apps/server/internal/worldrepo/materialization_interactive_board.go"
replace_once(
    path,
    "\tvisits := developmentVisitsForBoard(host, board, personas, r.WorldDate)\n",
    "\tvisits := developmentVisitsForBoardDays(host, board, personas, r.WorldDate, developmentInteractiveActivityLookbackDays)\n",
)

# Interactive development host gets the same six board categories already used
# by the scale Lab. Existing stored three-board hosts are migrated by appending
# only missing board IDs; RESET can continue to preserve board definitions.
path = "apps/server/internal/worldrepo/repository.go"
replace_once(
    path,
    '''func (r *Repository) MaterializationBoards(host world.Host) ([]world.Board, bool) {\n\tbs, ok := r.Base.(world.BoardStore)\n\tif !ok {\n\t\treturn nil, false\n\t}\n\tif existing := bs.ListBoards(host.ID); len(existing) > 0 {\n\t\treturn existing, false\n\t}\n\tboards := []world.Board{{ID: "1", Name: "フリートーク"}, {ID: "2", Name: "パソコン通信・モデム"}, {ID: "3", Name: "地域の話題"}}\n\tbs.SaveBoards(host.ID, boards)\n\treturn boards, true\n}\n''',
    '''func developmentMaterializationBoardCatalog() []world.Board {\n\treturn []world.Board{\n\t\t{ID: "1", Name: "フリートーク"},\n\t\t{ID: "2", Name: "パソコン通信・モデム"},\n\t\t{ID: "3", Name: "地域の話題"},\n\t\t{ID: "4", Name: "ゲーム"},\n\t\t{ID: "5", Name: "音楽"},\n\t\t{ID: "6", Name: "ソフトウェア"},\n\t}\n}\n\nfunc (r *Repository) MaterializationBoards(host world.Host) ([]world.Board, bool) {\n\tbs, ok := r.Base.(world.BoardStore)\n\tif !ok {\n\t\treturn nil, false\n\t}\n\tcatalog := developmentMaterializationBoardCatalog()\n\tdesired := catalog[:3]\n\tif developmentInteractiveTitleFirstEnabled(r) {\n\t\tdesired = catalog\n\t}\n\tif existing := bs.ListBoards(host.ID); len(existing) > 0 {\n\t\tif !developmentInteractiveTitleFirstEnabled(r) {\n\t\t\treturn existing, false\n\t\t}\n\t\tseen := make(map[string]bool, len(existing))\n\t\tfor _, board := range existing {\n\t\t\tseen[board.ID] = true\n\t\t}\n\t\tmerged := append([]world.Board(nil), existing...)\n\t\tchanged := false\n\t\tfor _, board := range desired {\n\t\t\tif seen[board.ID] {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tmerged = append(merged, board)\n\t\t\tseen[board.ID] = true\n\t\t\tchanged = true\n\t\t}\n\t\tif changed {\n\t\t\tbs.SaveBoards(host.ID, merged)\n\t\t\treturn merged, true\n\t\t}\n\t\treturn existing, false\n\t}\n\tboards := append([]world.Board(nil), desired...)\n\tbs.SaveBoards(host.ID, boards)\n\treturn boards, true\n}\n''',
)

# Document the ordinary dial-up observation scale without changing fresh Lab limits.
path = "docs/MATERIALIZATION_LAB.md"
replace_once(
    path,
    '''通常runtimeでも開発ホスト `0450000196` は Conversation View + title-first を有効化する。Web端末から `ATDT0450000196` で接続し、`B` で未生成Envelopeを計画、記事番号を開いてArticle Detail込み本文を遅延生成できる。`ALLBODY` では全本文を一括生成でき、`RESET` 後はtitle-first候補・割当も新しいplanning passへ再初期化する。ほかのホストプログラムにはこの開発専用経路を適用しない。\n''',
    '''通常runtimeでも開発ホスト `0450000196` は Conversation View + title-first を有効化する。Web端末から `ATDT0450000196` で接続し、`B` で未生成Envelopeを計画、記事番号を開いてArticle Detail込み本文を遅延生成できる。傾向確認用の通常runtimeだけは、板を6種（フリートーク／パソコン通信・モデム／地域の話題／ゲーム／音楽／ソフトウェア）に広げ、過去28日の活動から1板最大12 Envelopeを選ぶ。fresh Labの `board_count` / `shell_limit` と14日活動窓は比較条件として従来どおり維持する。`ALLBODY` では全本文を一括生成でき、`RESET` 後はtitle-first候補・割当も新しいplanning passへ再初期化する。ほかのホストプログラムにはこの開発専用経路を適用しない。\n''',
)

# Regression tests for board migration and the larger interactive-only sample.
Path("apps/server/internal/worldrepo/materialization_interactive_scale_test.go").write_text(r'''package worldrepo

import (
    "testing"
    "time"

    "zutto-pccom/apps/server/internal/world"
)

func TestInteractiveMaterializationExpandsStoredBoardsToSix(t *testing.T) {
    base := world.NewMemoryStore()
    repo := New(base, nil, nil, "1996-08-29")
    host, err := repo.HostByPhone("0450000196")
    if err != nil {
        t.Fatal(err)
    }

    initial, created := repo.MaterializationBoards(host)
    if !created || len(initial) != 3 {
        t.Fatalf("initial boards created=%v len=%d, want 3", created, len(initial))
    }

    repo.EnableDevelopmentInteractiveTitleFirstPoC()
    expanded, changed := repo.MaterializationBoards(host)
    if !changed || len(expanded) != 6 {
        t.Fatalf("interactive boards changed=%v len=%d, want 6", changed, len(expanded))
    }
    want := []string{"フリートーク", "パソコン通信・モデム", "地域の話題", "ゲーム", "音楽", "ソフトウェア"}
    for i, name := range want {
        if expanded[i].Name != name {
            t.Fatalf("board[%d]=%q, want %q", i, expanded[i].Name, name)
        }
    }
    if got := developmentConversationShellLimit(repo); got != 12 {
        t.Fatalf("interactive shell limit=%d, want 12", got)
    }

    stored, changedAgain := repo.MaterializationBoards(host)
    if changedAgain || len(stored) != 6 {
        t.Fatalf("stored interactive boards changed=%v len=%d, want unchanged 6", changedAgain, len(stored))
    }
}

func TestInteractiveActivityWindowIsLongerThanDefaultLabWindow(t *testing.T) {
    base := world.NewMemoryStore()
    repo := New(base, nil, nil, "1996-08-29")
    host, err := repo.HostByPhone("0450000196")
    if err != nil {
        t.Fatal(err)
    }
    personas, _ := repo.MaterializationPersonas(host)
    board := world.Board{ID: "4", Name: "ゲーム"}

    labVisits := developmentVisitsForBoard(host, board, personas, repo.WorldDate)
    interactiveVisits := developmentVisitsForBoardDays(host, board, personas, repo.WorldDate, developmentInteractiveActivityLookbackDays)
    if len(interactiveVisits) <= len(labVisits) {
        t.Fatalf("interactive visits=%d, default visits=%d; longer observation window should add sample opportunities", len(interactiveVisits), len(labVisits))
    }
    if len(interactiveVisits) == 0 {
        t.Fatal("interactive visit sample is empty")
    }
    stamp := worldTime(repo.WorldDate)
    oldest := interactiveVisits[0].createdAt
    if !oldest.Before(stamp.AddDate(0, 0, -(developmentDefaultActivityLookbackDays - 1))) {
        t.Fatalf("oldest interactive visit=%s did not extend beyond default %d-day window", oldest.Format(time.RFC3339), developmentDefaultActivityLookbackDays)
    }
    if oldest.Before(stamp.AddDate(0, 0, -(developmentInteractiveActivityLookbackDays - 1))) {
        t.Fatalf("oldest interactive visit=%s escaped %d-day window", oldest.Format(time.RFC3339), developmentInteractiveActivityLookbackDays)
    }
}
''')
