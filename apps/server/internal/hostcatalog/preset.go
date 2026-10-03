package hostcatalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// PresetSchemaVersion is the only preset file schema the loader accepts.
const PresetSchemaVersion = 1

// The YAML shapes. Optional values are pointers (or empty strings) so that
// "not specified" can be told apart from a zero value. Decoding is strict:
// unknown keys are errors, so a typo never silently becomes a default.
type presetFile struct {
	Schema     int             `yaml:"schema"`
	Key        string          `yaml:"key"`
	Revision   int             `yaml:"revision"`
	Listed     *bool           `yaml:"listed"`
	Role       string          `yaml:"role"`
	Host       presetHost      `yaml:"host"`
	Dial       presetDial      `yaml:"dial"`
	Debug      DebugFlags      `yaml:"debug"`
	Generation GenerationFlags `yaml:"generation"`
	Detail     PresetDetail    `yaml:"detail"`
}

type presetHost struct {
	Name          string        `yaml:"name"`
	Phone         string        `yaml:"phone"`
	Region        *presetRegion `yaml:"region"`
	Program       string        `yaml:"program"`
	SoftwareLabel string        `yaml:"software_label"`
	Lines         *int          `yaml:"lines"`
	MaxBaud       *int          `yaml:"max_baud"`
	FoundedOn     string        `yaml:"founded_on"`
	Popularity    *float64      `yaml:"popularity"`
	Members       *int          `yaml:"members"`
	Traits        presetTraits  `yaml:"traits"`
}

type presetRegion struct {
	Prefecture string `yaml:"prefecture"`
	City       string `yaml:"city"`
}

type presetTraits struct {
	ANSI           *bool `yaml:"ansi"`
	GuestAllowed   *bool `yaml:"guest_allowed"`
	TelehoFriendly *bool `yaml:"teleho_friendly"`
}

type presetDial struct {
	Mode       string `yaml:"mode"`
	Behavior   string `yaml:"behavior"`
	BusyFirstN int    `yaml:"busy_first_n"`
}

// PresetDetail holds detail fixed by the preset itself. Anything left out is
// generated on first access. Only the keys below are accepted until the
// per-program detail schemas exist.
type PresetDetail struct {
	Welcome string `yaml:"welcome"`
}

// Preset is a validated preset file. Unspecified optional values stay nil/empty
// and are meant to be filled deterministically from the world seed later.
type Preset struct {
	Source   string // file name
	Key      string
	Revision int
	Listed   bool
	Role     string

	Name           string
	Phone          string // "" = unspecified
	Region         *Region
	Program        string
	SoftwareLabel  string
	Lines          *int
	MaxBaud        *int
	FoundedOn      string // "" = unspecified
	Popularity     *float64
	Members        *int
	ANSI           *bool
	GuestAllowed   *bool
	TelehoFriendly *bool

	DialMode string
	Dial     DialBehavior

	// Debug and GenerationFlags are opt-in per host; absent means off.
	Debug           DebugFlags
	GenerationFlags GenerationFlags

	Detail PresetDetail
}

// ParsePreset decodes and validates one preset file. file is the base name
// (e.g. "hakata-canal-net.yaml") and must match the preset key.
func ParsePreset(file string, data []byte, o Options) (Preset, error) {
	var f presetFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return Preset{}, fmt.Errorf("%s: empty file", file)
		}
		return Preset{}, fmt.Errorf("%s: %w", file, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Preset{}, fmt.Errorf("%s: only one YAML document is allowed per file", file)
	}

	var errs []error
	add := func(field string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s: %w", file, field, err))
		}
	}

	if f.Schema != PresetSchemaVersion {
		add("schema", fmt.Errorf("%d is unsupported (want %d)", f.Schema, PresetSchemaVersion))
	}
	if err := checkKey(f.Key); err != nil {
		add("key", err)
	} else if want := strings.TrimSuffix(file, ".yaml"); f.Key != want {
		add("key", fmt.Errorf("%q must equal the file name %q", f.Key, want))
	}
	if f.Revision < 1 {
		add("revision", fmt.Errorf("%d must be at least 1", f.Revision))
	}
	if f.Listed == nil {
		add("listed", errors.New("is required (true or false)"))
	}
	listed := f.Listed != nil && *f.Listed
	add("role", checkRole(f.Role, listed))

	h := f.Host
	if strings.TrimSpace(h.Name) == "" {
		add("host.name", errors.New("is required"))
	}
	if h.Program == "" {
		add("host.program", errors.New("is required"))
	} else {
		add("host.program", checkProgram(h.Program, o))
	}
	if h.Phone != "" {
		add("host.phone", checkPhone(h.Phone, o))
	}
	var region *Region
	if h.Region != nil {
		r := Region{Prefecture: h.Region.Prefecture, City: h.Region.City}
		if r.Prefecture == "" {
			add("host.region.prefecture", errors.New("is required when region is given"))
		} else {
			add("host.region", checkRegion(r))
		}
		region = &r
	}
	if h.Lines != nil {
		add("host.lines", checkLines(*h.Lines))
	}
	if h.MaxBaud != nil {
		add("host.max_baud", checkBaud(*h.MaxBaud))
	}
	if h.FoundedOn != "" {
		add("host.founded_on", checkFoundedOn(h.FoundedOn))
	}
	if h.Popularity != nil {
		add("host.popularity", checkPopularity(*h.Popularity))
	}
	if h.Members != nil {
		add("host.members", checkMembers(*h.Members))
	}

	dialMode := f.Dial.Mode
	if dialMode == "" {
		dialMode = "tone"
	}
	add("dial.mode", checkDialMode(dialMode))
	dial := DialBehavior{Kind: DialKind(f.Dial.Behavior), BusyFirstN: f.Dial.BusyFirstN}
	if dial.Kind == "" {
		dial.Kind = DialNormal
	}
	add("dial", checkDial(dial))

	if len(errs) > 0 {
		return Preset{}, errors.Join(errs...)
	}
	return Preset{
		Source:          file,
		Key:             f.Key,
		Revision:        f.Revision,
		Listed:          listed,
		Role:            f.Role,
		Name:            strings.TrimSpace(h.Name),
		Phone:           h.Phone,
		Region:          region,
		Program:         h.Program,
		SoftwareLabel:   strings.TrimSpace(h.SoftwareLabel),
		Lines:           h.Lines,
		MaxBaud:         h.MaxBaud,
		FoundedOn:       h.FoundedOn,
		Popularity:      h.Popularity,
		Members:         h.Members,
		ANSI:            h.Traits.ANSI,
		GuestAllowed:    h.Traits.GuestAllowed,
		TelehoFriendly:  h.Traits.TelehoFriendly,
		DialMode:        dialMode,
		Dial:            dial,
		Debug:           f.Debug,
		GenerationFlags: f.Generation,
		Detail:          f.Detail,
	}, nil
}

// Missing lists the descriptor fields the preset leaves unspecified. Those are
// the fields a generator will have to fill from the world seed.
func (p Preset) Missing() []string {
	var m []string
	add := func(missing bool, name string) {
		if missing {
			m = append(m, name)
		}
	}
	add(p.Phone == "", "host.phone")
	add(p.Region == nil, "host.region")
	add(p.Lines == nil, "host.lines")
	add(p.MaxBaud == nil, "host.max_baud")
	add(p.FoundedOn == "", "host.founded_on")
	add(p.Popularity == nil, "host.popularity")
	add(p.Members == nil, "host.members")
	add(p.ANSI == nil, "host.traits.ansi")
	add(p.GuestAllowed == nil, "host.traits.guest_allowed")
	add(p.TelehoFriendly == nil, "host.traits.teleho_friendly")
	return m
}

// IncompleteError is returned by Preset.Descriptor when the preset still needs
// generated values.
type IncompleteError struct {
	Key    string
	Fields []string
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("preset %q is incomplete: %s not specified", e.Key, strings.Join(e.Fields, ", "))
}

// Descriptor converts a fully specified preset into a HostDescriptor. A preset
// that relies on generated values returns *IncompleteError.
func (p Preset) Descriptor() (HostDescriptor, error) {
	if missing := p.Missing(); len(missing) > 0 {
		return HostDescriptor{}, &IncompleteError{Key: p.Key, Fields: missing}
	}
	return HostDescriptor{
		Key:             p.Key,
		Origin:          OriginPreset,
		Listed:          p.Listed,
		Role:            p.Role,
		PresetRevision:  p.Revision,
		Phone:           p.Phone,
		Name:            p.Name,
		Program:         p.Program,
		SoftwareLabel:   p.SoftwareLabel,
		Region:          *p.Region,
		Lines:           *p.Lines,
		MaxBaud:         *p.MaxBaud,
		FoundedOn:       p.FoundedOn,
		Popularity:      *p.Popularity,
		Members:         *p.Members,
		Traits:          Traits{ANSI: *p.ANSI, GuestAllowed: *p.GuestAllowed, TelehoFriendly: *p.TelehoFriendly},
		DialMode:        p.DialMode,
		Dial:            p.Dial,
		Debug:           p.Debug,
		GenerationFlags: p.GenerationFlags,
	}, nil
}
