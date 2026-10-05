package hostcatalog

import (
	"strings"
	"testing"
)

// erikaKPresetOptions accepts any program ID, so these tests do not depend on
// the list of programs the world catalog knows about.
var erikaKPresetOptions = Options{Programs: func(string) bool { return true }}

const erikaKPresetHead = `
schema: 1
key: sample
revision: 1
listed: true
host:
  name: Sample
  program: erika-k
`

func parseErikaKPreset(t *testing.T, program, detail string) (Preset, error) {
	t.Helper()
	head := strings.Replace(erikaKPresetHead, "program: erika-k", "program: "+program, 1)
	return ParsePreset("sample.yaml", []byte(head+detail), erikaKPresetOptions)
}

func mustParseErikaK(t *testing.T, detail string) *ErikaKDetail {
	t.Helper()
	p, err := parseErikaKPreset(t, "erika-k", detail)
	if err != nil {
		t.Fatalf("a valid detail was rejected: %v", err)
	}
	if p.Detail.ErikaK == nil {
		t.Fatal("the detail was parsed but is nil")
	}
	return p.Detail.ErikaK
}

func requireErikaKError(t *testing.T, detail string, wants ...string) {
	t.Helper()
	_, err := parseErikaKPreset(t, "erika-k", detail)
	if err == nil {
		t.Fatalf("an invalid detail was accepted:\n%s", detail)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestErikaKDetailIsParsed(t *testing.T) {
	d := mustParseErikaK(t, `
detail:
  erika_k:
    texts:
      login_banner: ["== BANNER ==", "welcome {handle}"]
      login_greeting: "hello {handle}"
      main_menu_title: "TITLE"
      goodbye: "see you"
    boards:
      - path: "1"
        name: "Notice"
        root_author_policy: "sysop_only"
        activity_weight: 0.10
        reply_rate: 0.20
        retained_root_cap: 24
        verified_referent_rate: 0.10
        unread: true
      - path: "10"
        alias: "FORUM"
        name: "Forum"
      - path: "10/1"
        name: "Child"
      - path: "99"
        name: "Hidden"
        hidden: true
`)
	if got := d.Texts.LoginBanner; len(got) != 2 || got[0] != "== BANNER ==" || got[1] != "welcome {handle}" {
		t.Fatalf("login_banner = %q", got)
	}
	if d.Texts.LoginGreeting != "hello {handle}" || d.Texts.MainMenuTitle != "TITLE" || d.Texts.Goodbye != "see you" {
		t.Fatalf("texts = %+v", d.Texts)
	}
	if len(d.Boards) != 4 {
		t.Fatalf("boards = %d, want 4", len(d.Boards))
	}
	notice := d.Boards[0]
	if notice.Path != "1" || notice.Name != "Notice" || notice.RootAuthorPolicy != "sysop_only" || !notice.Unread ||
		notice.ActivityWeight != 0.10 || notice.ReplyRate != 0.20 || notice.RetainedRootCap != 24 || notice.VerifiedReferentRate != 0.10 {
		t.Fatalf("board 1 = %+v", notice)
	}
	if d.Boards[1].Alias != "FORUM" || !d.Boards[3].Hidden {
		t.Fatalf("alias/hidden not read: %+v %+v", d.Boards[1], d.Boards[3])
	}
}

func TestErikaKDetailIsOptional(t *testing.T) {
	p, err := parseErikaKPreset(t, "erika-k", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Detail.ErikaK != nil {
		t.Fatalf("a preset without detail.erika_k has %+v", p.Detail.ErikaK)
	}
}

// Left-out texts stay empty; the host prints nothing for them.
func TestOmittedErikaKTextsStayEmpty(t *testing.T) {
	d := mustParseErikaK(t, `
detail:
  erika_k:
    boards:
      - path: "1"
        name: "Only"
`)
	if len(d.Texts.LoginBanner) != 0 || d.Texts.LoginGreeting != "" || d.Texts.MainMenuTitle != "" || d.Texts.Goodbye != "" {
		t.Fatalf("omitted texts are not empty: %+v", d.Texts)
	}
}

func TestErikaKTextsKeepTrailingSpaces(t *testing.T) {
	d := mustParseErikaK(t, `
detail:
  erika_k:
    texts:
      login_banner: ["abc   ", ""]
`)
	if got := d.Texts.LoginBanner; len(got) != 2 || got[0] != "abc   " || got[1] != "" {
		t.Fatalf("banner lines were altered: %q", got)
	}
}

func TestErikaKDetailIsOnlyValidForErikaK(t *testing.T) {
	_, err := parseErikaKPreset(t, "other-program", `
detail:
  erika_k:
    boards:
      - path: "1"
        name: "A"
`)
	if err == nil || !strings.Contains(err.Error(), "detail.erika_k") {
		t.Fatalf("error = %v, want a detail.erika_k error", err)
	}
}

func TestErikaKBoardPathsAreChecked(t *testing.T) {
	for name, tc := range map[string]struct {
		boards string
		want   string
	}{
		"empty":         {`- {path: "", name: "A"}`, "path"},
		"not numeric":   {`- {path: "a", name: "A"}`, "path"},
		"double slash":  {`- {path: "1//2", name: "A"}`, "path"},
		"trailing":      {`- {path: "1/", name: "A"}`, "path"},
		"duplicate":     {"- {path: \"1\", name: \"A\"}\n      - {path: \"1\", name: \"B\"}", "listed twice"},
		"missing parent": {`- {path: "10/1", name: "A"}`, "parent"},
	} {
		t.Run(name, func(t *testing.T) {
			requireErikaKError(t, "\ndetail:\n  erika_k:\n    boards:\n      "+tc.boards+"\n", tc.want)
		})
	}
}

func TestErikaKChildMayComeBeforeItsParent(t *testing.T) {
	mustParseErikaK(t, `
detail:
  erika_k:
    boards:
      - {path: "10/1", name: "Child"}
      - {path: "10", name: "Forum"}
`)
}

func TestErikaKBoardFieldsAreChecked(t *testing.T) {
	for name, tc := range map[string]struct {
		board string
		want  string
	}{
		"blank name":       {`{path: "1", name: " "}`, "name"},
		"unknown policy":   {`{path: "1", name: "A", root_author_policy: "other"}`, "root_author_policy"},
		"negative weight":  {`{path: "1", name: "A", activity_weight: -0.1}`, "activity_weight"},
		"negative reply":   {`{path: "1", name: "A", reply_rate: -1}`, "reply_rate"},
		"negative cap":     {`{path: "1", name: "A", retained_root_cap: -1}`, "retained_root_cap"},
		"referent too big": {`{path: "1", name: "A", verified_referent_rate: 1.5}`, "verified_referent_rate"},
	} {
		t.Run(name, func(t *testing.T) {
			requireErikaKError(t, "\ndetail:\n  erika_k:\n    boards:\n      - "+tc.board+"\n", tc.want)
		})
	}
	// The allowed edge values are accepted.
	mustParseErikaK(t, `
detail:
  erika_k:
    boards:
      - {path: "1", name: "A", root_author_policy: "sysop_only", verified_referent_rate: 1}
      - {path: "2", name: "B", activity_weight: 0, reply_rate: 0, retained_root_cap: 0, verified_referent_rate: 0}
`)
}

func TestErikaKTextsRejectControlCharacters(t *testing.T) {
	for name, text := range map[string]string{
		"tab":     `"a\tb"`,
		"newline": `"a\nb"`,
		"escape":  `"a\eb"`,
		"cr":      `"a\rb"`,
	} {
		t.Run(name, func(t *testing.T) {
			requireErikaKError(t, "\ndetail:\n  erika_k:\n    texts:\n      goodbye: "+text+"\n", "control character", "texts.goodbye")
		})
	}
	requireErikaKError(t, "\ndetail:\n  erika_k:\n    texts:\n      login_banner: [\"ok\", \"bad\\tline\"]\n", "texts.login_banner[1]")
}

func TestErikaKTextsAcceptHandleOnlyWhereItIsReplaced(t *testing.T) {
	// Replaced by the host: login_banner and login_greeting.
	mustParseErikaK(t, `
detail:
  erika_k:
    texts:
      login_banner: ["hi {handle}"]
      login_greeting: "hello {handle}"
`)
	// Printed literally by the host, so rejected.
	for _, key := range []string{"main_menu_title", "goodbye"} {
		requireErikaKError(t, "\ndetail:\n  erika_k:\n    texts:\n      "+key+": \"x {handle}\"\n", "{handle} is not replaced", "texts."+key)
	}
}

func TestErikaKTextsRejectUnknownPlaceholders(t *testing.T) {
	requireErikaKError(t, "\ndetail:\n  erika_k:\n    texts:\n      login_greeting: \"hi {name}\"\n", "unknown placeholder {name}")
	requireErikaKError(t, "\ndetail:\n  erika_k:\n    texts:\n      login_banner: [\"{handle} {x}\"]\n", "unknown placeholder {x}")
}

func TestErikaKProblemsAreReportedTogether(t *testing.T) {
	_, err := parseErikaKPreset(t, "erika-k", `
detail:
  erika_k:
    texts:
      goodbye: "a\tb"
    boards:
      - {path: "x", name: "A"}
      - {path: "2", name: ""}
`)
	if err == nil {
		t.Fatal("an invalid detail was accepted")
	}
	for _, want := range []string{"texts.goodbye", "boards[0].path", "boards[1].name"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestErikaKDetailRejectsUnknownKeys(t *testing.T) {
	for name, detail := range map[string]string{
		"texts typo":  "\ndetail:\n  erika_k:\n    texts:\n      login_baner: [\"x\"]\n",
		"board typo":  "\ndetail:\n  erika_k:\n    boards:\n      - {path: \"1\", name: \"A\", activity_wieght: 1}\n",
		"detail typo": "\ndetail:\n  erika_k:\n    text: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseErikaKPreset(t, "erika-k", detail); err == nil {
				t.Fatalf("a typo in %s was accepted", name)
			}
		})
	}
}

// The embedded HAKATA preset is the one real definition: it must stay valid and
// complete, since the golden screens in the erikak package are built from it.
func TestEmbeddedErikaKDetail(t *testing.T) {
	presets, err := LoadPresets(Options{})
	if err != nil {
		t.Fatal(err)
	}
	var found *Preset
	for i := range presets {
		if presets[i].Key == "hakata-canal-net" {
			found = &presets[i]
		}
		if presets[i].Detail.ErikaK != nil && presets[i].Program != erikaKProgramID {
			t.Errorf("%s has detail.erika_k but runs %q", presets[i].Key, presets[i].Program)
		}
	}
	if found == nil || found.Detail.ErikaK == nil {
		t.Fatal("hakata-canal-net has no detail.erika_k")
	}
	d := found.Detail.ErikaK
	if len(d.Boards) != 29 {
		t.Errorf("boards = %d, want 29", len(d.Boards))
	}
	if len(d.Texts.LoginBanner) != 6 || d.Texts.LoginGreeting == "" || d.Texts.MainMenuTitle == "" || d.Texts.Goodbye == "" {
		t.Errorf("texts are incomplete: %+v", d.Texts)
	}
	unread := 0
	for _, b := range d.Boards {
		if b.Unread {
			unread++
		}
	}
	if unread != 5 {
		t.Errorf("unread boards = %d, want 5", unread)
	}
}
