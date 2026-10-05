package erikak

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

// stationStore is a MemoryStore whose host detail is chosen by the test, so a
// station other than the sample one can be described.
type stationStore struct {
	*world.MemoryStore
	detail    hostcatalog.PresetDetail
	hasDetail bool
}

func (s stationStore) HostDetail(string) (hostcatalog.PresetDetail, bool) {
	return s.detail, s.hasDetail
}

// newStationRuntime returns a runtime for a made-up station. A nil detail models
// a host whose preset has no detail.erika_k.
func newStationRuntime(detail *hostcatalog.ErikaKDetail) *Runtime {
	store := stationStore{MemoryStore: world.NewMemoryStore()}
	if detail != nil {
		store.detail = hostcatalog.PresetDetail{ErikaK: detail}
		store.hasDetail = true
	}
	host := world.Host{ID: "other-station", Phone: "0450000099", Name: "OTHER STATION", Software: "K", SoftwareID: "erika-k"}
	return New(host, store)
}

func guestLogin(t *testing.T, r *Runtime) string {
	t.Helper()
	out, disconnect := r.HandleLine("GUEST")
	if disconnect {
		t.Fatal("the guest login disconnected")
	}
	return out
}

func TestAnotherStationShowsOnlyItsOwnDefinition(t *testing.T) {
	r := newStationRuntime(&hostcatalog.ErikaKDetail{
		Texts: hostcatalog.ErikaKTexts{
			LoginBanner:   []string{"*** HELLO {handle} ***"},
			LoginGreeting: "Nice to see you, {handle}.",
			MainMenuTitle: "OTHER MENU TITLE",
			Goodbye:       "Come back soon.",
		},
		Boards: []hostcatalog.ErikaKBoard{
			{Path: "1", Name: "FIRST BOARD"},
			{Path: "2", Name: "SECOND BOARD"},
			{Path: "10", Alias: "FORUM", Name: "THE FORUM"},
			{Path: "10/1", Name: "CHILD BOARD"},
		},
	})

	login := guestLogin(t, r)
	for _, want := range []string{"*** HELLO GUEST ***", "Nice to see you, GUEST.", "OTHER MENU TITLE", "前回アクセス"} {
		if !strings.Contains(login, want) {
			t.Fatalf("login output lacks %q:\n%s", want, login)
		}
	}
	// Nothing of the sample station leaks in.
	for _, leak := range []string{"HAKATA", "博多", "WELCOME TO", "ERIKA-K", "ＨＡＫＡＴＡ", "■"} {
		if strings.Contains(login, leak) {
			t.Fatalf("login output contains %q, which belongs to another station", leak)
		}
	}

	boards, _ := r.HandleLine("1")
	for _, want := range []string{"FIRST BOARD", "SECOND BOARD", "THE FORUM"} {
		if !strings.Contains(boards, want) {
			t.Fatalf("the board menu lacks %q:\n%s", want, boards)
		}
	}
	if strings.Contains(boards, "博多・天神広場") || strings.Contains(boards, "事務局からのお知らせ") {
		t.Fatalf("the board menu shows the sample station's boards:\n%s", boards)
	}

	r.HandleLine("/")
	forum, _ := r.HandleLine("BJ 10")
	if !strings.Contains(forum, "THE FORUM") || !strings.Contains(forum, "CHILD BOARD") {
		t.Fatalf("the forum menu is wrong:\n%s", forum)
	}

	r.HandleLine("/")
	bye, disconnect := r.HandleLine("9")
	if !disconnect || !strings.Contains(bye, "Come back soon.") || !strings.Contains(bye, "ご利用ありがとうございました。") {
		t.Fatalf("goodbye = %q (disconnect=%v)", bye, disconnect)
	}
}

func TestHostWithoutDetailHasNoBoardsAndPrintsNoStationTexts(t *testing.T) {
	r := newStationRuntime(nil)
	login := guestLogin(t, r)
	if !strings.Contains(login, "前回アクセス") {
		t.Fatalf("the standard last-access line is missing:\n%s", login)
	}
	for _, absent := range []string{"WELCOME", "■", "#"} {
		if strings.Contains(login, absent) {
			t.Fatalf("a host without detail printed %q:\n%s", absent, login)
		}
	}
	boards, _ := r.HandleLine("1")
	if !strings.Contains(boards, "ボード／フォーラムメニュー") {
		t.Fatalf("the board menu did not render:\n%s", boards)
	}
	if strings.Contains(boards, "[1]") {
		t.Fatalf("a host without detail has boards:\n%s", boards)
	}
	r.HandleLine("/")
	bye, _ := r.HandleLine("9")
	if bye != "\r\nご利用ありがとうございました。\r\n" {
		t.Fatalf("goodbye = %q", bye)
	}
}

func TestZeroValueRuntimeDoesNotPanic(t *testing.T) {
	r := &Runtime{}
	for name, f := range map[string]func() string{
		"main menu":      r.renderMainMenu,
		"board map":      r.renderBoardMap,
		"unread summary": r.renderUnreadSummary,
		"login":          r.finishLogin,
	} {
		if f() == "" {
			t.Fatalf("%s rendered nothing", name)
		}
	}
}

// A part that the station leaves out is not printed, and the program does not
// substitute wording of its own.
func TestOmittedTextsPrintNothing(t *testing.T) {
	full := hostcatalog.ErikaKTexts{
		LoginBanner:   []string{"BANNER-LINE"},
		LoginGreeting: "GREETING-LINE",
		MainMenuTitle: "MENU-TITLE-LINE",
		Goodbye:       "GOODBYE-LINE",
	}
	loginWith := func(texts hostcatalog.ErikaKTexts) string {
		return guestLogin(t, newStationRuntime(&hostcatalog.ErikaKDetail{Texts: texts}))
	}
	byeWith := func(texts hostcatalog.ErikaKTexts) string {
		r := newStationRuntime(&hostcatalog.ErikaKDetail{Texts: texts})
		guestLogin(t, r)
		out, disconnect := r.HandleLine("9")
		if !disconnect {
			t.Fatal("9 did not end the call")
		}
		return out
	}

	all := loginWith(full)
	banner, greeting, title := strings.Index(all, "BANNER-LINE"), strings.Index(all, "GREETING-LINE"), strings.Index(all, "MENU-TITLE-LINE")
	if banner < 0 || greeting < 0 || title < 0 || !(banner < greeting && greeting < title) {
		t.Fatalf("banner, greeting and title must all appear, in that order:\n%s", all)
	}
	if !strings.Contains(byeWith(full), "GOODBYE-LINE") {
		t.Fatal("goodbye text missing")
	}

	noBanner := full
	noBanner.LoginBanner = nil
	if out := loginWith(noBanner); strings.Contains(out, "BANNER-LINE") || !strings.Contains(out, "GREETING-LINE") || !strings.Contains(out, "MENU-TITLE-LINE") {
		t.Fatalf("without a banner only the banner may disappear:\n%s", out)
	}

	noGreeting := full
	noGreeting.LoginGreeting = ""
	if out := loginWith(noGreeting); strings.Contains(out, "GREETING-LINE") || !strings.Contains(out, "BANNER-LINE") || !strings.Contains(out, "MENU-TITLE-LINE") {
		t.Fatalf("without a greeting only the greeting may disappear:\n%s", out)
	}

	noTitle := full
	noTitle.MainMenuTitle = ""
	out := loginWith(noTitle)
	if strings.Contains(out, "MENU-TITLE-LINE") || !strings.Contains(out, "BANNER-LINE") || !strings.Contains(out, "GREETING-LINE") {
		t.Fatalf("without a title only the title may disappear:\n%s", out)
	}
	lines := strings.Split(out, "\r\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "―") {
			if i == 0 || lines[i-1] != "" {
				t.Fatalf("without a title the menu must start at its rule; the line before it is %q", lines[i-1])
			}
			break
		}
	}

	noGoodbye := full
	noGoodbye.Goodbye = ""
	if got := byeWith(noGoodbye); got != "\r\nご利用ありがとうございました。\r\n" {
		t.Fatalf("without a goodbye text only the standard thanks may print, got %q", got)
	}
}

func TestLoginTextsPrintExactlyAsWritten(t *testing.T) {
	r := newStationRuntime(&hostcatalog.ErikaKDetail{Texts: hostcatalog.ErikaKTexts{
		LoginBanner: []string{"  padded line   ", "", "{handle} / {handle}"},
	}})
	out := guestLogin(t, r)
	for _, want := range []string{"\r\n  padded line   \r\n", "\r\n\r\nGUEST / GUEST\r\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("login output lacks %q (texts must be printed as written):\n%q", want, out)
		}
	}
	// The station name and software name are not woven into any text.
	if strings.Contains(out, "OTHER STATION") {
		t.Fatalf("the host name was inserted into a station text:\n%s", out)
	}
}

func TestHandleIsTheLoggedInMember(t *testing.T) {
	r := newStationRuntime(&hostcatalog.ErikaKDetail{Texts: hostcatalog.ErikaKTexts{
		LoginBanner:   []string{"banner for {handle}"},
		LoginGreeting: "greeting for {handle}",
	}})
	if out, _ := r.HandleLine("MIKI"); !strings.Contains(out, "PASSWORD") {
		t.Fatalf("expected a password prompt, got %q", out)
	}
	out, _ := r.HandleLine("secret")
	for _, want := range []string{"banner for MIKI", "greeting for MIKI"} {
		if !strings.Contains(out, want) {
			t.Fatalf("login output lacks %q:\n%s", want, out)
		}
	}
}

func TestValidateDetailLimitsEachLineToEightyCells(t *testing.T) {
	valid := hostcatalog.ErikaKDetail{Texts: hostcatalog.ErikaKTexts{
		LoginBanner:   []string{strings.Repeat("■", 40), strings.Repeat("a", 80)},
		LoginGreeting: "{handle}" + strings.Repeat("a", 72), // {handle} counts as 8 cells
		MainMenuTitle: strings.Repeat("あ", 40),
		Goodbye:       strings.Repeat("a", 80),
	}}
	if err := ValidateDetail(valid); err != nil {
		t.Fatalf("lines of exactly 80 cells were rejected: %v", err)
	}

	for name, tc := range map[string]struct {
		texts hostcatalog.ErikaKTexts
		field string
	}{
		"banner":   {hostcatalog.ErikaKTexts{LoginBanner: []string{"ok", strings.Repeat("■", 41)}}, "texts.login_banner[1]"},
		"greeting": {hostcatalog.ErikaKTexts{LoginGreeting: "{handle}" + strings.Repeat("a", 73)}, "texts.login_greeting"},
		"title":    {hostcatalog.ErikaKTexts{MainMenuTitle: strings.Repeat("a", 81)}, "texts.main_menu_title"},
		"goodbye":  {hostcatalog.ErikaKTexts{Goodbye: strings.Repeat("あ", 41)}, "texts.goodbye"},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateDetail(hostcatalog.ErikaKDetail{Texts: tc.texts})
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("error = %v, want one that mentions %s", err, tc.field)
			}
		})
	}
}

func TestEveryEmbeddedErikaKDetailFitsTheScreen(t *testing.T) {
	presets, err := hostcatalog.LoadPresets(hostcatalog.Options{})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, p := range presets {
		if p.Detail.ErikaK == nil {
			continue
		}
		checked++
		if err := ValidateDetail(*p.Detail.ErikaK); err != nil {
			t.Errorf("%s: %v", p.Key, err)
		}
	}
	if checked == 0 {
		t.Fatal("no preset has an Erika-K detail to check")
	}
}
