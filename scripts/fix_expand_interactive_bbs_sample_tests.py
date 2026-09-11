from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:180]!r}")
    p.write_text(text.replace(old, new, 1))


path = "apps/server/internal/worldrepo/materialization_interactive_title_first_test.go"
replace_once(
    path,
    "func TestEnableDevelopmentInteractiveTitleFirstPoCMatchesFreshLabMode(t *testing.T) {",
    "func TestEnableDevelopmentInteractiveTitleFirstPoCUsesLargerObservationCap(t *testing.T) {",
)
replace_once(
    path,
    '''\tif got := developmentConversationShellLimit(repo); got != 8 {\n\t\tt.Fatalf("shell limit=%d, want 8 to match the current fresh-Lab comparison path", got)\n\t}\n''',
    '''\tif got := developmentConversationShellLimit(repo); got != 12 {\n\t\tt.Fatalf("shell limit=%d, want 12 for ordinary ATDT observation", got)\n\t}\n''',
)

path = "apps/server/internal/worldrepo/materialization_interactive_scale_test.go"
replace_once(path, '    "time"\n', '')
replace_once(
    path,
    '''    stamp := worldTime(repo.WorldDate)\n    oldest := interactiveVisits[0].createdAt\n    if !oldest.Before(stamp.AddDate(0, 0, -(developmentDefaultActivityLookbackDays - 1))) {\n        t.Fatalf("oldest interactive visit=%s did not extend beyond default %d-day window", oldest.Format(time.RFC3339), developmentDefaultActivityLookbackDays)\n    }\n    if oldest.Before(stamp.AddDate(0, 0, -(developmentInteractiveActivityLookbackDays - 1))) {\n        t.Fatalf("oldest interactive visit=%s escaped %d-day window", oldest.Format(time.RFC3339), developmentInteractiveActivityLookbackDays)\n    }\n''',
    '''    if !interactiveVisits[0].createdAt.Before(labVisits[0].createdAt) {\n        t.Fatalf("interactive oldest=%s, default oldest=%s; longer window should expose earlier activity", interactiveVisits[0].createdAt, labVisits[0].createdAt)\n    }\n''',
)
