// Package hostcatalog defines the canonical description of a BBS host
// (HostDescriptor) and the data-driven preset hosts that are loaded from YAML.
//
// A host is described in three layers (see README.md): the Descriptor defined
// here is the "skeleton" fixed when the host directory is created. Per-program
// detail (welcome text, menus, boards) and mutable operational state live
// elsewhere.
package hostcatalog

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Origin says where a host definition came from.
type Origin string

const (
	OriginPreset    Origin = "preset"
	OriginGenerated Origin = "generated"
)

// Role is an optional operational tag. Hosts tagged debug or test must never be
// listed in the public directory; this is enforced by validation.
const (
	RoleDebug      = "debug"
	RoleTest       = "test"
	RoleExperiment = "experiment"
	RoleEvent      = "event"
)

// DialKind selects a special dial behavior that used to be hard-coded in
// telephone.Network.
type DialKind string

const (
	DialNormal        DialKind = "normal"
	DialAlwaysConnect DialKind = "always_connect"
	DialBusyFirstN    DialKind = "busy_first_n"
)

type DialBehavior struct {
	Kind       DialKind
	BusyFirstN int // only for DialBusyFirstN: attempts numbered below this are BUSY (dials are numbered from 1, so 5 means four BUSY dials)
}

type Region struct {
	Prefecture string
	City       string
}

func (r Region) String() string { return r.Prefecture + r.City }
func (r Region) IsZero() bool   { return r == Region{} }

type Traits struct {
	ANSI           bool
	GuestAllowed   bool
	TelehoFriendly bool
}

// DefaultTraits mirrors the convention already used for hosts that have no trait
// data (worldrepo.completeDevelopmentHost). It is a stopgap until traits are
// generated as part of the host skeleton.
func DefaultTraits() Traits { return Traits{GuestAllowed: true, TelehoFriendly: true} }

// HostDescriptor is the immutable skeleton of a host. It deliberately contains
// no derived values: the busy rate is computed from Popularity/Lines/time of
// day, and time-varying membership belongs to the board activity model.
type HostDescriptor struct {
	ID             string // persistence ID (uuid); empty until stored
	Key            string // stable, human-readable key: "hakata-canal-net", "world-001"
	Origin         Origin
	Listed         bool // shown in the host directory; does NOT affect dialability
	Role           string
	Generation     int
	PresetRevision int // revision of the preset definition this host was created from

	Phone         string // digits only
	Name          string
	Program       string // host-program ID
	SoftwareLabel string // display name; falls back to Program
	Region        Region
	Lines         int
	MaxBaud       int
	FoundedOn     string // YYYY-MM-DD
	Popularity    float64
	Members       int // snapshot at world date
	Traits        Traits
	DialMode      string // "tone" | "pulse"
	Dial          DialBehavior
}

// SoftwareDisplay returns the label shown to users for the host program.
func (d HostDescriptor) SoftwareDisplay() string {
	if strings.TrimSpace(d.SoftwareLabel) != "" {
		return d.SoftwareLabel
	}
	return d.Program
}

// Options injects the environment validation depends on.
type Options struct {
	// Programs reports whether a host-program ID exists. nil uses KnownProgram.
	Programs func(id string) bool
	// ReservedPhones are numbers no host may use (e.g. future special numbers).
	ReservedPhones map[string]struct{}
}

func (o Options) programKnown(id string) bool {
	if o.Programs != nil {
		return o.Programs(id)
	}
	return KnownProgram(id)
}

func (o Options) phoneReserved(p string) bool {
	_, ok := o.ReservedPhones[p]
	return ok
}

var keyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var supportedBauds = []int{2400, 9600, 14400, 28800}

// NormalizePhone strips everything but ASCII digits.
func NormalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidPhone reports whether p is a plausible domestic number: 10 or 11 digits,
// a leading 0 followed by a non-zero digit.
func ValidPhone(p string) bool {
	if len(p) != 10 && len(p) != 11 {
		return false
	}
	return NormalizePhone(p) == p && p[0] == '0' && p[1] != '0'
}

func checkKey(k string) error {
	if !keyPattern.MatchString(k) {
		return fmt.Errorf("%q must be lower-case words joined by hyphens (a-z, 0-9)", k)
	}
	return nil
}

func checkPhone(p string, o Options) error {
	if p != NormalizePhone(p) {
		return fmt.Errorf("%q must contain digits only", p)
	}
	if !ValidPhone(p) {
		return fmt.Errorf("%q must be 10 or 11 digits starting with 0", p)
	}
	if o.phoneReserved(p) {
		return fmt.Errorf("%q is a reserved number", p)
	}
	return nil
}

func checkProgram(id string, o Options) error {
	if !o.programKnown(id) {
		return fmt.Errorf("unknown host program %q", id)
	}
	return nil
}

func checkRegion(r Region) error {
	if r.IsZero() {
		return nil
	}
	if !KnownPrefecture(r.Prefecture) {
		return fmt.Errorf("unknown prefecture %q", r.Prefecture)
	}
	return nil
}

func checkLines(n int) error {
	if n < 1 {
		return fmt.Errorf("%d must be at least 1", n)
	}
	return nil
}

func checkBaud(n int) error {
	if !slices.Contains(supportedBauds, n) {
		return fmt.Errorf("%d is not one of %v", n, supportedBauds)
	}
	return nil
}

func checkPopularity(v float64) error {
	if math.IsNaN(v) || v < 0 || v > 1 {
		return fmt.Errorf("%v must be between 0 and 1", v)
	}
	return nil
}

func checkMembers(n int) error {
	if n < 0 {
		return fmt.Errorf("%d must not be negative", n)
	}
	return nil
}

func checkFoundedOn(s string) error {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("%q must be a valid YYYY-MM-DD date", s)
	}
	return nil
}

func checkDialMode(m string) error {
	if m != "tone" && m != "pulse" {
		return fmt.Errorf("%q must be tone or pulse", m)
	}
	return nil
}

func checkDial(d DialBehavior) error {
	switch d.Kind {
	case DialNormal, DialAlwaysConnect:
		if d.BusyFirstN != 0 {
			return fmt.Errorf("busy_first_n is only valid with behavior %q", DialBusyFirstN)
		}
	case DialBusyFirstN:
		if d.BusyFirstN < 1 {
			return fmt.Errorf("behavior %q requires busy_first_n >= 1", DialBusyFirstN)
		}
	default:
		return fmt.Errorf("unknown dial behavior %q", d.Kind)
	}
	return nil
}

func checkRole(role string, listed bool) error {
	switch role {
	case "", RoleExperiment, RoleEvent:
		return nil
	case RoleDebug, RoleTest:
		if listed {
			return fmt.Errorf("role %q hosts must not be listed", role)
		}
		return nil
	}
	return fmt.Errorf("unknown role %q (want debug, test, experiment or event)", role)
}

// Validate checks every invariant of the descriptor and reports all violations.
// Region is optional (generated hosts do not have one yet) but must be valid
// when present.
func (d HostDescriptor) Validate(o Options) error {
	var errs []error
	add := func(field string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", field, err))
		}
	}
	add("key", checkKey(d.Key))
	if d.Origin != OriginPreset && d.Origin != OriginGenerated {
		add("origin", fmt.Errorf("unknown origin %q", d.Origin))
	}
	add("role", checkRole(d.Role, d.Listed))
	add("phone", checkPhone(d.Phone, o))
	if strings.TrimSpace(d.Name) == "" {
		add("name", errors.New("must not be empty"))
	}
	add("program", checkProgram(d.Program, o))
	add("region", checkRegion(d.Region))
	add("lines", checkLines(d.Lines))
	add("max_baud", checkBaud(d.MaxBaud))
	add("founded_on", checkFoundedOn(d.FoundedOn))
	add("popularity", checkPopularity(d.Popularity))
	add("members", checkMembers(d.Members))
	add("dial_mode", checkDialMode(d.DialMode))
	add("dial", checkDial(d.Dial))
	if d.Generation < 0 {
		add("generation", errors.New("must not be negative"))
	}
	return errors.Join(errs...)
}
