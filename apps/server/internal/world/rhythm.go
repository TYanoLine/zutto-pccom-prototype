package world

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// RhythmContext is the input to a single RhythmLayer.
type RhythmContext struct {
	Host      Host
	Board     Board
	Persona   Persona
	Archetype string
	At        time.Time
}

// RhythmLayer is one independent multiplicative factor in a resident's
// activity rhythm.
type RhythmLayer interface {
	Name() string
	Factor(ctx RhythmContext) float64
}

// RhythmConfig holds the parsed data from rhythm-1996.yaml.
type RhythmConfig struct {
	TelecomTariff struct {
		TelehodaiStart string      `yaml:"telehodai_start"`
		Hourly         [24]float64 `yaml:"hourly"`
	} `yaml:"telecom_tariff"`
	Holidays1996    []string `yaml:"holidays_1996"`
	SeasonalWindows []struct {
		Name    string  `yaml:"name"`
		From    string  `yaml:"from"`
		To      string  `yaml:"to"`
		DayKind string  `yaml:"day_kind"`
		Factor  float64 `yaml:"factor"`
	} `yaml:"seasonal_windows"`
	WorldEvents []struct {
		Name   string  `yaml:"name"`
		From   string  `yaml:"from"`
		To     string  `yaml:"to"`
		Factor float64 `yaml:"factor"`
	} `yaml:"world_events"`
	DefaultMemberMix     map[string]float64               `yaml:"default_member_mix"`
	OccupationArchetypes map[string]string                `yaml:"occupation_archetypes"`
	Archetypes           map[string]ArchetypeScheduleYAML `yaml:"archetypes"`
	StationCongestion    struct {
		DefaultLineCapacity   int     `yaml:"default_line_capacity"`
		SingleLinePeakDamping float64 `yaml:"single_line_peak_damping"`
		MultiLinePeakBoost    float64 `yaml:"multi_line_peak_boost"`
	} `yaml:"station_congestion"`
	ActivityPatterns map[string]struct {
		OffPeakBoost float64 `yaml:"off_peak_boost"`
		PeakBoost    float64 `yaml:"peak_boost"`
	} `yaml:"activity_patterns"`

	holidaySet map[string]bool
}

// ArchetypeScheduleYAML holds 24-hour curves for weekday, saturday, and holiday.
type ArchetypeScheduleYAML struct {
	Weekday  [24]float64 `yaml:"weekday"`
	Saturday [24]float64 `yaml:"saturday"`
	Holiday  [24]float64 `yaml:"holiday"`
}

var (
	defaultRhythmOnce   sync.Once
	defaultRhythmConfig *RhythmConfig
	defaultRhythmLayers []RhythmLayer
	jstZone             = time.FixedZone("JST", 9*3600)
)

// DefaultRhythmConfig returns the embedded 1996 rhythm configuration.
func DefaultRhythmConfig() *RhythmConfig {
	defaultRhythmOnce.Do(initDefaultRhythm)
	return defaultRhythmConfig
}

// DefaultRhythmLayers returns the default 6-layer rhythm stack loaded from
// data/rhythm-1996.yaml.
func DefaultRhythmLayers() []RhythmLayer {
	defaultRhythmOnce.Do(initDefaultRhythm)
	out := make([]RhythmLayer, len(defaultRhythmLayers))
	copy(out, defaultRhythmLayers)
	return out
}

// DefaultMemberMix returns a copy of the default lifestyle archetype mix.
func DefaultMemberMix() map[string]float64 {
	cfg := DefaultRhythmConfig()
	if cfg == nil || len(cfg.DefaultMemberMix) == 0 {
		return map[string]float64{
			"student":   0.25,
			"office":    0.40,
			"shift":     0.12,
			"homemaker": 0.10,
			"night_owl": 0.13,
		}
	}
	out := make(map[string]float64, len(cfg.DefaultMemberMix))
	for k, v := range cfg.DefaultMemberMix {
		out[k] = v
	}
	return out
}

func initDefaultRhythm() {
	raw, err := worldDataFS.ReadFile("data/rhythm-1996.yaml")
	if err != nil {
		defaultRhythmConfig = &RhythmConfig{holidaySet: map[string]bool{}}
		return
	}
	cfg, err := LoadRhythmConfigYAML(raw)
	if err != nil {
		defaultRhythmConfig = &RhythmConfig{holidaySet: map[string]bool{}}
		return
	}
	defaultRhythmConfig = cfg
	defaultRhythmLayers = BuildRhythmLayers(cfg)
}

// LoadRhythmConfigYAML parses RhythmConfig from YAML bytes.
func LoadRhythmConfigYAML(data []byte) (*RhythmConfig, error) {
	var cfg RhythmConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse rhythm yaml: %w", err)
	}
	cfg.holidaySet = make(map[string]bool, len(cfg.Holidays1996))
	for _, h := range cfg.Holidays1996 {
		cfg.holidaySet[strings.TrimSpace(h)] = true
	}
	return &cfg, nil
}

// BuildRhythmLayers constructs the standard 6 rhythm layers from a RhythmConfig.
func BuildRhythmLayers(cfg *RhythmConfig) []RhythmLayer {
	if cfg == nil {
		return nil
	}
	return []RhythmLayer{
		telecomTariffLayer{cfg: cfg},
		calendarSeasonLayer{cfg: cfg},
		worldEventLayer{cfg: cfg},
		archetypeScheduleLayer{cfg: cfg},
		stationCongestionLayer{cfg: cfg},
		activityPatternLayer{cfg: cfg},
	}
}

func toJST(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(jstZone)
}

func (cfg *RhythmConfig) dayKind(t time.Time) string {
	if t.IsZero() {
		return "weekday"
	}
	jt := toJST(t)
	dateStr := jt.Format("2006-01-02")
	if cfg != nil {
		if cfg.holidaySet[dateStr] {
			return "holiday"
		}
		for _, sw := range cfg.SeasonalWindows {
			if sw.From != "" && dateStr < sw.From {
				continue
			}
			if sw.To != "" && dateStr > sw.To {
				continue
			}
			if sw.DayKind != "" {
				return sw.DayKind
			}
		}
	}
	switch jt.Weekday() {
	case time.Sunday:
		return "holiday"
	case time.Saturday:
		return "saturday"
	default:
		return "weekday"
	}
}

// Layer 1: Telecom tariff / Telehodai
type telecomTariffLayer struct {
	cfg *RhythmConfig
}

func (l telecomTariffLayer) Name() string { return "telecom_tariff" }

func (l telecomTariffLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	jt := toJST(ctx.At)
	if start := strings.TrimSpace(l.cfg.TelecomTariff.TelehodaiStart); start != "" {
		if jt.Format("2006-01-02") < start {
			return 1.0
		}
	}
	v := l.cfg.TelecomTariff.Hourly[jt.Hour()]
	if v <= 0 {
		return 1.0
	}
	return v
}

// Layer 2: Weekday / holiday / seasonal windows
type calendarSeasonLayer struct {
	cfg *RhythmConfig
}

func (l calendarSeasonLayer) Name() string { return "calendar_season" }

func (l calendarSeasonLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	dateStr := toJST(ctx.At).Format("2006-01-02")
	f := 1.0
	for _, sw := range l.cfg.SeasonalWindows {
		if sw.Factor <= 0 {
			continue
		}
		if sw.From != "" && dateStr < sw.From {
			continue
		}
		if sw.To != "" && dateStr > sw.To {
			continue
		}
		f *= sw.Factor
	}
	return f
}

// Layer 3: World events
type worldEventLayer struct {
	cfg *RhythmConfig
}

func (l worldEventLayer) Name() string { return "world_event" }

func (l worldEventLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	dateStr := toJST(ctx.At).Format("2006-01-02")
	f := 1.0
	for _, ev := range l.cfg.WorldEvents {
		if ev.Factor <= 0 {
			continue
		}
		if ev.From != "" && dateStr < ev.From {
			continue
		}
		if ev.To != "" && dateStr > ev.To {
			continue
		}
		f *= ev.Factor
	}
	return f
}

// Layer 4: Lifestyle archetype x day_kind x hour
type archetypeScheduleLayer struct {
	cfg *RhythmConfig
}

func (l archetypeScheduleLayer) Name() string { return "archetype_schedule" }

func (l archetypeScheduleLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	arch := strings.ToLower(strings.TrimSpace(ctx.Archetype))
	if arch == "" {
		arch = AssignArchetype(ctx.Persona, nil, DefaultMemberMix())
	}
	sched, ok := l.cfg.Archetypes[arch]
	if !ok {
		sched, ok = l.cfg.Archetypes["office"]
		if !ok {
			return 1.0
		}
	}
	jt := toJST(ctx.At)
	hour := jt.Hour()
	var v float64
	switch l.cfg.dayKind(ctx.At) {
	case "holiday":
		v = sched.Holiday[hour]
	case "saturday":
		v = sched.Saturday[hour]
	default:
		v = sched.Weekday[hour]
	}
	if v <= 0 {
		return 1.0
	}
	return v
}

// Layer 5: Station line capacity / congestion
type stationCongestionLayer struct {
	cfg *RhythmConfig
}

func (l stationCongestionLayer) Name() string { return "station_congestion" }

func (l stationCongestionLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	jt := toJST(ctx.At)
	hour := jt.Hour()
	isPeak := hour == 23 || hour == 0
	if !isPeak {
		return 1.0
	}
	lineCap := l.cfg.StationCongestion.DefaultLineCapacity
	if lineCap <= 0 {
		lineCap = 2
	}
	if lineCap <= 1 {
		if l.cfg.StationCongestion.SingleLinePeakDamping > 0 {
			return l.cfg.StationCongestion.SingleLinePeakDamping
		}
	}
	return 1.0
}

// Layer 6: Persona activity_pattern temporal dispersion
type activityPatternLayer struct {
	cfg *RhythmConfig
}

func (l activityPatternLayer) Name() string { return "activity_pattern" }

func (l activityPatternLayer) Factor(ctx RhythmContext) float64 {
	if l.cfg == nil || ctx.At.IsZero() {
		return 1.0
	}
	pat := strings.ToLower(strings.TrimSpace(ctx.Persona.ActivityPattern))
	entry, ok := l.cfg.ActivityPatterns[pat]
	if !ok {
		return 1.0
	}
	hour := toJST(ctx.At).Hour()
	isPeak := hour >= 21 || hour <= 1
	if isPeak {
		if entry.PeakBoost > 0 {
			return entry.PeakBoost
		}
		return 1.0
	}
	if entry.OffPeakBoost > 0 {
		return entry.OffPeakBoost
	}
	return 1.0
}

// ComposeRhythm multiplies the factors of all layers in log-space and clamps to
// [RhythmMinFactor, RhythmMaxFactor] (plan 4.5).
func ComposeRhythm(layers []RhythmLayer, ctx RhythmContext) float64 {
	if len(layers) == 0 {
		return 1.0
	}
	logSum := 0.0
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		f := layer.Factor(ctx)
		if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
			continue
		}
		logSum += math.Log(f)
	}
	return clampFloat(math.Exp(logSum), RhythmMinFactor, RhythmMaxFactor)
}

// AssignArchetype deterministically assigns a lifestyle archetype from a
// persona's confirmed facts/occupation or a hash of its ID against memberMix (plan 4.6).
func AssignArchetype(p Persona, facts []PersonaFact, memberMix map[string]float64) string {
	var occMap map[string]string
	if cfg := DefaultRhythmConfig(); cfg != nil {
		occMap = cfg.OccupationArchetypes
	}
	if len(memberMix) == 0 {
		memberMix = DefaultMemberMix()
	}
	return assignArchetypeWithData(p, facts, memberMix, occMap)
}

func assignArchetypeWithData(p Persona, facts []PersonaFact, memberMix map[string]float64, occMap map[string]string) string {
	if len(occMap) > 0 {
		occKeys := make([]string, 0, len(occMap))
		for k := range occMap {
			occKeys = append(occKeys, k)
		}
		sort.Slice(occKeys, func(i, j int) bool {
			if len(occKeys[i]) != len(occKeys[j]) {
				return len(occKeys[i]) > len(occKeys[j])
			}
			return occKeys[i] < occKeys[j]
		})
		for _, f := range facts {
			if f.PersonaID != "" && p.ID != "" && f.PersonaID != p.ID {
				continue
			}
			val := strings.ToLower(strings.TrimSpace(f.Value))
			if val == "" {
				continue
			}
			for _, kw := range occKeys {
				if strings.Contains(val, strings.ToLower(kw)) {
					return occMap[kw]
				}
			}
		}
		if occ := strings.ToLower(strings.TrimSpace(p.Occupation)); occ != "" {
			for _, kw := range occKeys {
				if strings.Contains(occ, strings.ToLower(kw)) {
					return occMap[kw]
				}
			}
		}
	}
	if len(memberMix) == 0 {
		return "office"
	}
	keys := make([]string, 0, len(memberMix))
	total := 0.0
	for k, w := range memberMix {
		if w > 0 {
			keys = append(keys, k)
			total += w
		}
	}
	if total <= 0 || len(keys) == 0 {
		return "office"
	}
	sort.Strings(keys)
	u := reactionUnit("archetype", p.ID) * total
	acc := 0.0
	for _, k := range keys {
		acc += memberMix[k]
		if u <= acc {
			return k
		}
	}
	return keys[len(keys)-1]
}
