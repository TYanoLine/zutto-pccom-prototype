package config

import "testing"

func TestHistoricalReferencesDefaultOff(t *testing.T) {
	t.Setenv("HISTORICAL_REFERENCES_ENABLED", "")
	if Load().HistoricalReferencesEnabled {
		t.Fatal("historical references must default to OFF")
	}
}

func TestHistoricalReferencesCanBeEnabledAndDisabled(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "on", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "off", want: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("HISTORICAL_REFERENCES_ENABLED", tc.value)
			if got := Load().HistoricalReferencesEnabled; got != tc.want {
				t.Fatalf("value=%q got=%v want=%v", tc.value, got, tc.want)
			}
		})
	}
}
