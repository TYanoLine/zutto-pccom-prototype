package hostcatalog

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const erikaKProgramID = "erika-k"

// ErikaKDetail is the station-specific data of a host that runs the Erika-K host
// program: the station's own texts and its boards. The program itself (the state
// machine, the commands and the screen layout) lives in the erikak package. The
// slices are read-only: callers must not modify them.
type ErikaKDetail struct {
	Texts  ErikaKTexts   `yaml:"texts"`
	Boards []ErikaKBoard `yaml:"boards"`
}

// ErikaKTexts are named parts of Erika-K's screens that the station writes
// itself. The host program prints them as written, without building, padding or
// converting anything, and prints nothing for a part that is left out: the
// program has no default wording. The only substitution is "{handle}", the handle
// of the user who logged in, and it is replaced in LoginBanner and LoginGreeting
// only. New parts (for example menus) are added as new keys.
type ErikaKTexts struct {
	// LoginBanner lines follow the "last access" line after login. One element is
	// one line, rules and headings included.
	LoginBanner []string `yaml:"login_banner"`
	// LoginGreeting follows the banner, between blank lines.
	LoginGreeting string `yaml:"login_greeting"`
	// MainMenuTitle is the first line of the main menu.
	MainMenuTitle string `yaml:"main_menu_title"`
	// Goodbye is the line after the standard thanks when the user ends the call.
	Goodbye string `yaml:"goodbye"`
}

// ErikaKBoard is one entry of the station's board tree. The tree is given in
// display order. Key and parent are derived from Path ("10/1" is board 1 under
// forum 10).
type ErikaKBoard struct {
	Path                 string  `yaml:"path"`
	Alias                string  `yaml:"alias"`
	Name                 string  `yaml:"name"`
	Hidden               bool    `yaml:"hidden"`
	Scope                string  `yaml:"scope"`
	RootAuthorPolicy     string  `yaml:"root_author_policy"`
	ActivityWeight       float64 `yaml:"activity_weight"`
	ReplyRate            float64 `yaml:"reply_rate"`
	RetainedRootCap      int     `yaml:"retained_root_cap"`
	VerifiedReferentRate float64 `yaml:"verified_referent_rate"`
	// Unread marks the board as having unread messages in the station's fixed
	// prototype display, until real read state exists.
	Unread bool `yaml:"unread"`
}

var (
	boardPathPattern   = regexp.MustCompile(`^[0-9]+(/[0-9]+)*$`)
	placeholderPattern = regexp.MustCompile(`\{[^}]*\}`)
)

// validateErikaKDetail checks the structure of an Erika-K detail. How wide a
// line may be on screen is checked by the erikak package (ValidateDetail).
func validateErikaKDetail(d *ErikaKDetail) error {
	if d == nil {
		return nil
	}
	var errs []error
	add := func(field string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", field, err))
		}
	}
	// checkText rejects control characters and placeholders. "{handle}" is only
	// accepted where the host program replaces it; anywhere else it would be
	// printed literally, so it is an error.
	checkText := func(field, text string, handleReplaced bool) {
		for _, r := range text {
			if r < 0x20 || r == 0x7f {
				add(field, fmt.Errorf("must not contain the control character %q", r))
				break
			}
		}
		for _, found := range placeholderPattern.FindAllString(text, -1) {
			switch {
			case found == "{handle}" && handleReplaced:
			case found == "{handle}":
				add(field, errors.New("{handle} is not replaced in this text; it is only supported in login_banner and login_greeting"))
			default:
				add(field, fmt.Errorf("unknown placeholder %s (only {handle} is supported)", found))
			}
		}
	}
	for i, line := range d.Texts.LoginBanner {
		checkText(fmt.Sprintf("texts.login_banner[%d]", i), line, true)
	}
	checkText("texts.login_greeting", d.Texts.LoginGreeting, true)
	checkText("texts.main_menu_title", d.Texts.MainMenuTitle, false)
	checkText("texts.goodbye", d.Texts.Goodbye, false)

	defined := make(map[string]bool, len(d.Boards))
	for _, b := range d.Boards {
		if boardPathPattern.MatchString(b.Path) {
			defined[b.Path] = true
		}
	}
	seen := make(map[string]bool, len(d.Boards))
	for i, b := range d.Boards {
		field := fmt.Sprintf("boards[%d]", i)
		if strings.TrimSpace(b.Path) == "" {
			add(field+".path", errors.New("is required"))
			continue
		}
		if !boardPathPattern.MatchString(b.Path) {
			add(field+".path", fmt.Errorf("%q must be numbers separated by \"/\" (for example \"10/1\")", b.Path))
			continue
		}
		if seen[b.Path] {
			add(field+".path", fmt.Errorf("%q is listed twice", b.Path))
			continue
		}
		seen[b.Path] = true
		if cut := strings.LastIndex(b.Path, "/"); cut >= 0 && !defined[b.Path[:cut]] {
			add(field+".path", fmt.Errorf("the parent board %q of %q is not defined", b.Path[:cut], b.Path))
		}
		if strings.TrimSpace(b.Name) == "" {
			add(field+".name", errors.New("is required"))
		}
		if b.RootAuthorPolicy != "" && b.RootAuthorPolicy != "sysop_only" {
			add(field+".root_author_policy", fmt.Errorf("%q must be empty or \"sysop_only\"", b.RootAuthorPolicy))
		}
		if math.IsNaN(b.ActivityWeight) || b.ActivityWeight < 0 {
			add(field+".activity_weight", fmt.Errorf("%v must not be negative", b.ActivityWeight))
		}
		if math.IsNaN(b.ReplyRate) || b.ReplyRate < 0 {
			add(field+".reply_rate", fmt.Errorf("%v must not be negative", b.ReplyRate))
		}
		if b.RetainedRootCap < 0 {
			add(field+".retained_root_cap", fmt.Errorf("%d must not be negative", b.RetainedRootCap))
		}
		if math.IsNaN(b.VerifiedReferentRate) || b.VerifiedReferentRate < 0 || b.VerifiedReferentRate > 1 {
			add(field+".verified_referent_rate", fmt.Errorf("%v must be between 0 and 1", b.VerifiedReferentRate))
		}
	}
	return errors.Join(errs...)
}
