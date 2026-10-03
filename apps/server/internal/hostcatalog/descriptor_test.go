package hostcatalog

import (
	"strings"
	"testing"
)

func validDescriptor() HostDescriptor {
	return HostDescriptor{
		Key: "sample-bbs", Origin: OriginPreset, Listed: true,
		Phone: "0312345678", Name: "SAMPLE BBS", Program: "ktbbs",
		Region: Region{Prefecture: "東京都", City: "渋谷区"},
		Lines:  2, MaxBaud: 9600, FoundedOn: "1995-01-02", Popularity: 0.4, Members: 10,
		Traits: DefaultTraits(), DialMode: "tone", Dial: DialBehavior{Kind: DialNormal},
	}
}

func TestDescriptorValidateAccepts(t *testing.T) {
	if err := validDescriptor().Validate(Options{}); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
	d := validDescriptor()
	d.Region = Region{} // region is optional for generated hosts
	if err := d.Validate(Options{}); err != nil {
		t.Fatalf("descriptor without region rejected: %v", err)
	}
}

func TestDescriptorValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(*HostDescriptor)
		want string
	}{
		{"key", func(d *HostDescriptor) { d.Key = "Bad Key" }, "key:"},
		{"origin", func(d *HostDescriptor) { d.Origin = "other" }, "origin:"},
		{"phone", func(d *HostDescriptor) { d.Phone = "110" }, "phone:"},
		{"name", func(d *HostDescriptor) { d.Name = "  " }, "name:"},
		{"program", func(d *HostDescriptor) { d.Program = "nope" }, "program:"},
		{"region", func(d *HostDescriptor) { d.Region.Prefecture = "Atlantis" }, "region:"},
		{"lines", func(d *HostDescriptor) { d.Lines = 0 }, "lines:"},
		{"baud", func(d *HostDescriptor) { d.MaxBaud = 1234 }, "max_baud:"},
		{"founded", func(d *HostDescriptor) { d.FoundedOn = "1995-02-30" }, "founded_on:"},
		{"popularity", func(d *HostDescriptor) { d.Popularity = 1.01 }, "popularity:"},
		{"members", func(d *HostDescriptor) { d.Members = -1 }, "members:"},
		{"dial mode", func(d *HostDescriptor) { d.DialMode = "" }, "dial_mode:"},
		{"dial", func(d *HostDescriptor) { d.Dial = DialBehavior{Kind: DialBusyFirstN} }, "dial:"},
		{"generation", func(d *HostDescriptor) { d.Generation = -1 }, "generation:"},
		{"debug listed", func(d *HostDescriptor) { d.Role = RoleDebug }, "must not be listed"},
		{"test listed", func(d *HostDescriptor) { d.Role = RoleTest }, "must not be listed"},
		{"role", func(d *HostDescriptor) { d.Role = "vip" }, "unknown role"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDescriptor()
			tc.edit(&d)
			err := d.Validate(Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestDescriptorUnlistedDebugHostIsValid(t *testing.T) {
	d := validDescriptor()
	d.Role, d.Listed = RoleDebug, false
	if err := d.Validate(Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestDescriptorReportsAllViolations(t *testing.T) {
	d := validDescriptor()
	d.Lines, d.MaxBaud, d.Members = 0, 1, -5
	err := d.Validate(Options{})
	for _, want := range []string{"lines:", "max_baud:", "members:"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in %v", want, err)
		}
	}
}

func TestReservedPhoneRejected(t *testing.T) {
	o := Options{ReservedPhones: map[string]struct{}{"0312345678": {}}}
	if err := validDescriptor().Validate(o); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved phone accepted: %v", err)
	}
}

func TestPhoneHelpers(t *testing.T) {
	if got := NormalizePhone("045-123-4567"); got != "0451234567" {
		t.Fatalf("NormalizePhone = %q", got)
	}
	for p, want := range map[string]bool{
		"0451234567": true, "09012345678": true, "0120123456": true,
		"110": false, "": false, "045123456": false, "4512345678": false,
		"0051234567": false, "045-1234567": false, "045123456789": false,
	} {
		if got := ValidPhone(p); got != want {
			t.Errorf("ValidPhone(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestSoftwareDisplayAndRegionString(t *testing.T) {
	d := validDescriptor()
	if d.SoftwareDisplay() != "ktbbs" {
		t.Fatalf("fallback label = %q", d.SoftwareDisplay())
	}
	d.SoftwareLabel = "KTBBS compatible"
	if d.SoftwareDisplay() != "KTBBS compatible" {
		t.Fatalf("label = %q", d.SoftwareDisplay())
	}
	if got := (Region{Prefecture: "千葉県"}).String(); got != "千葉県" {
		t.Fatalf("Region.String = %q", got)
	}
}
