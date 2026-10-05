package hostcatalog

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const erikaKProgramID = "erika-k"

type ErikaKDetail struct {
	Texts  ErikaKTexts   `yaml:"texts"`
	Boards []ErikaKBoard `yaml:"boards"`
}

type ErikaKTexts struct {
	LoginBanner   []string `yaml:"login_banner"`
	LoginGreeting string   `yaml:"login_greeting"`
	MainMenuTitle string   `yaml:"main_menu_title"`
	Goodbye       string   `yaml:"goodbye"`
}

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
	Unread               bool    `yaml:"unread"`
}

var (
	boardPathPattern   = regexp.MustCompile(`^[0-9]+(/[0-9]+)*$`)
	placeholderPattern = regexp.MustCompile(`\{[^}]*\}`)
)

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
	checkText := func(field, text string) {
		for _, r := range text {
			if r < 0x20 || r == 0x7f {
				add(field, fmt.Errorf("must not contain the control character %q", r))
				break
			}
		}
		for _, found := range placeholderPattern.FindAllString(text, -1) {
			if found != "{handle}" {
				add(field, fmt.Errorf("unknown placeholder %s (only {handle} is supported)", found))
			}
		}
	}
	for i, line := range d.Texts.LoginBanner {
		checkText(fmt.Sprintf("texts.login_banner[%d]", i), line)
	}
	checkText("texts.login_greeting", d.Texts.LoginGreeting)
	checkText("texts.main_menu_title", d.Texts.MainMenuTitle)
	checkText("texts.goodbye", d.Texts.Goodbye)

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
