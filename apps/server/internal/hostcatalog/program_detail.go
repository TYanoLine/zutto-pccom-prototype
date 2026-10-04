package hostcatalog

import (
	"fmt"
	"regexp"
	"strings"
)

type ErikaKDetail struct {
	Login  ErikaKLogin   `yaml:"login"`
	Boards []ErikaKBoard `yaml:"boards"`
}

type ErikaKLogin struct {
	StationMessage []string `yaml:"station_message"`
	MemberGreeting string   `yaml:"member_greeting"`
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

var erikaKBoardPath = regexp.MustCompile(`^[0-9]+(/[0-9]+)*$`)
var erikaKPlaceholders = regexp.MustCompile(`\{([^}]*)\}`)

func validateErikaKDetail(d *ErikaKDetail) error {
	if d == nil {
		return nil
	}
	var errs []string
	seen := map[string]bool{}
	for i, b := range d.Boards {
		prefix := fmt.Sprintf("detail.erika_k.boards[%d]", i)
		if !erikaKBoardPath.MatchString(b.Path) {
			errs = append(errs, prefix+".path must match ^[0-9]+(/[0-9]+)*$")
		}
		if seen[b.Path] {
			errs = append(errs, prefix+".path is duplicated")
		}
		seen[b.Path] = true
		if strings.TrimSpace(b.Name) == "" {
			errs = append(errs, prefix+".name is required")
		}
		if b.RootAuthorPolicy != "" && b.RootAuthorPolicy != "sysop_only" {
			errs = append(errs, prefix+".root_author_policy must be empty or sysop_only")
		}
		if b.ActivityWeight < 0 || b.ReplyRate < 0 || b.RetainedRootCap < 0 || b.VerifiedReferentRate < 0 || b.VerifiedReferentRate > 1 {
			errs = append(errs, prefix+" numeric values must be non-negative and verified_referent_rate must be between 0 and 1")
		}
	}
	for _, b := range d.Boards {
		if i := strings.LastIndexByte(b.Path, '/'); i >= 0 && !seen[b.Path[:i]] {
			errs = append(errs, "detail.erika_k.boards parent "+b.Path[:i]+" is missing")
		}
	}
	for i, line := range d.Login.StationMessage {
		if strings.TrimSpace(line) == "" {
			errs = append(errs, fmt.Sprintf("detail.erika_k.login.station_message[%d] is required", i))
		}
	}
	for _, match := range erikaKPlaceholders.FindAllStringSubmatch(d.Login.MemberGreeting, -1) {
		if match[1] != "handle" {
			errs = append(errs, "detail.erika_k.login.member_greeting contains an unsupported placeholder")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
