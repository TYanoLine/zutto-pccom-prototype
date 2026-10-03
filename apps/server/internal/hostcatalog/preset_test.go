package hostcatalog

import (
	"errors"
	"strings"
	"testing"
)

const samplePreset = `schema: 1
key: sample-bbs
revision: 2
listed: true
host:
  name: SAMPLE BBS
  phone: "0312345678"
  region: { prefecture: 東京都, city: 渋谷区 }
  program: ktbbs
  lines: 2
  max_baud: 9600
  founded_on: 1995-01-02
  popularity: 0.4
  members: 10
  traits: { ansi: true, guest_allowed: false, teleho_friendly: true }
`

func TestParsePresetValid(t *testing.T) {
	p, err := ParsePreset("sample-bbs.yaml", []byte(samplePreset), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Missing()) != 0 {
		t.Fatalf("unexpected missing fields: %v", p.Missing())
	}
	d, err := p.Descriptor()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(Options{}); err != nil {
		t.Fatalf("descriptor from preset invalid: %v", err)
	}
	if d.Origin != OriginPreset || d.PresetRevision != 2 || !d.Listed || d.Key != "sample-bbs" {
		t.Fatalf("unexpected identity fields: %+v", d)
	}
	if d.DialMode != "tone" || d.Dial.Kind != DialNormal {
		t.Fatalf("dial defaults: %q %+v", d.DialMode, d.Dial)
	}
	if !d.Traits.ANSI || d.Traits.GuestAllowed || !d.Traits.TelehoFriendly {
		t.Fatalf("traits: %+v", d.Traits)
	}
	if d.Region.String() != "東京都渋谷区" {
		t.Fatalf("region: %v", d.Region)
	}
}

func TestParsePresetRejects(t *testing.T) {
	replace := func(old, new string) func(string) string {
		return func(s string) string {
			if !strings.Contains(s, old) {
				t.Fatalf("test bug: %q not in sample", old)
			}
			return strings.Replace(s, old, new, 1)
		}
	}
	cases := []struct {
		name string
		edit func(string) string
		file string
		opts Options
		want string
	}{
		{"missing listed", replace("listed: true\n", ""), "", Options{}, "listed: is required"},
		{"unknown key", replace("  lines: 2\n", "  lines: 2\n  colour: red\n"), "", Options{}, "colour"},
		{"file name mismatch", replace("key: sample-bbs", "key: sample-bbs"), "other.yaml", Options{}, "must equal the file name"},
		{"schema", replace("schema: 1", "schema: 2"), "", Options{}, "schema:"},
		{"revision", replace("revision: 2", "revision: 0"), "", Options{}, "revision:"},
		{"bad key", replace("key: sample-bbs", "key: Sample_BBS"), "Sample_BBS.yaml", Options{}, "key:"},
		{"phone formatting", replace(`"0312345678"`, `"03-1234-5678"`), "", Options{}, "digits only"},
		{"phone length", replace(`"0312345678"`, `"031234567"`), "", Options{}, "host.phone"},
		{"reserved phone", func(s string) string { return s }, "", Options{ReservedPhones: map[string]struct{}{"0312345678": {}}}, "reserved"},
		{"unknown program", replace("program: ktbbs", "program: nope"), "", Options{}, "unknown host program"},
		{"missing program", replace("  program: ktbbs\n", ""), "", Options{}, "host.program: is required"},
		{"missing name", replace("  name: SAMPLE BBS\n", ""), "", Options{}, "host.name: is required"},
		{"prefecture", replace("東京都", "Atlantis"), "", Options{}, "unknown prefecture"},
		{"region without prefecture", replace("{ prefecture: 東京都, city: 渋谷区 }", "{ city: 渋谷区 }"), "", Options{}, "prefecture"},
		{"lines", replace("lines: 2", "lines: 0"), "", Options{}, "host.lines"},
		{"baud", replace("max_baud: 9600", "max_baud: 12345"), "", Options{}, "host.max_baud"},
		{"popularity", replace("popularity: 0.4", "popularity: 1.5"), "", Options{}, "host.popularity"},
		{"members", replace("members: 10", "members: -1"), "", Options{}, "host.members"},
		{"date", replace("1995-01-02", "1995-13-40"), "", Options{}, "host.founded_on"},
		{"debug listed", replace("listed: true\n", "listed: true\nrole: debug\n"), "", Options{}, "must not be listed"},
		{"unknown role", replace("listed: true\n", "listed: true\nrole: vip\n"), "", Options{}, "unknown role"},
		{"unknown dial behavior", func(s string) string { return s + "dial:\n  behavior: explode\n" }, "", Options{}, "unknown dial behavior"},
		{"busy_first_n without behavior", func(s string) string { return s + "dial:\n  busy_first_n: 3\n" }, "", Options{}, "only valid with behavior"},
		{"busy_first_n behavior without n", func(s string) string { return s + "dial:\n  behavior: busy_first_n\n" }, "", Options{}, "busy_first_n >= 1"},
		{"dial mode", func(s string) string { return s + "dial:\n  mode: laser\n" }, "", Options{}, "dial.mode"},
		{"unknown detail key", func(s string) string { return s + "detail:\n  menus: []\n" }, "", Options{}, "menus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := tc.file
			if file == "" {
				file = "sample-bbs.yaml"
			}
			_, err := ParsePreset(file, []byte(tc.edit(samplePreset)), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestParsePresetStructuralErrors(t *testing.T) {
	cases := map[string]string{
		"empty file":      "",
		"two documents":   samplePreset + "---\n" + samplePreset,
		"invalid yaml":    "schema: [1\n",
		"wrong top level": "- a\n- b\n",
	}
	for name, src := range cases {
		if _, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestParsePresetReportsAllErrors(t *testing.T) {
	src := strings.NewReplacer("lines: 2", "lines: 0", "max_baud: 9600", "max_baud: 5", "listed: true\n", "").Replace(samplePreset)
	_, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{})
	for _, want := range []string{"host.lines", "host.max_baud", "listed"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in %v", want, err)
		}
	}
}

func TestUnquotedPhoneAndDateKeepTheirText(t *testing.T) {
	src := strings.Replace(samplePreset, `"0312345678"`, "0312345678", 1)
	p, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Phone != "0312345678" || p.FoundedOn != "1995-01-02" {
		t.Fatalf("phone/date text changed: %q %q", p.Phone, p.FoundedOn)
	}
}

func TestPartialPresetReportsMissingFields(t *testing.T) {
	src := "schema: 1\nkey: partial-bbs\nrevision: 1\nlisted: true\nhost:\n  name: PARTIAL BBS\n  program: ktbbs\n"
	p, err := ParsePreset("partial-bbs.yaml", []byte(src), Options{})
	if err != nil {
		t.Fatalf("a partial preset must parse: %v", err)
	}
	if len(p.Missing()) != 10 {
		t.Fatalf("missing = %v", p.Missing())
	}
	_, err = p.Descriptor()
	var inc *IncompleteError
	if !errors.As(err, &inc) || inc.Key != "partial-bbs" || len(inc.Fields) != 10 {
		t.Fatalf("want *IncompleteError, got %v", err)
	}
}

func TestPresetDetailAndDialParsed(t *testing.T) {
	src := samplePreset + "dial:\n  behavior: busy_first_n\n  busy_first_n: 5\n  mode: pulse\ndetail:\n  welcome: |\n    hello\n"
	p, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Dial != (DialBehavior{Kind: DialBusyFirstN, BusyFirstN: 5}) || p.DialMode != "pulse" || p.Detail.Welcome != "hello\n" {
		t.Fatalf("dial/detail not parsed: %+v %q %q", p.Dial, p.DialMode, p.Detail.Welcome)
	}
}

func TestPresetAlwaysConnectBehaviorParses(t *testing.T) {
	src := samplePreset + "dial:\n  behavior: always_connect\n"
	p, err := ParsePreset("sample-bbs.yaml", []byte(src), Options{})
	if err != nil || p.Dial.Kind != DialAlwaysConnect {
		t.Fatalf("always_connect not parsed: kind=%q err=%v", p.Dial.Kind, err)
	}
}
