package hostcatalog

import (
	"strings"
	"testing"
)

func TestParsePresetRejectsErikaKDetailForOtherProgram(t *testing.T) {
	data := []byte(`
schema: 1
key: sample
revision: 1
listed: true
host: {name: Sample, program: turbobbs}
detail:
  erika_k:
    boards: [{path: "1", name: "掲示板"}]
`)
	_, err := ParsePreset("sample.yaml", data, Options{})
	if err == nil || !strings.Contains(err.Error(), "detail.erika_k") {
		t.Fatalf("error = %v, want Erika-K detail validation", err)
	}
}

func TestParsePresetRejectsInvalidErikaKDetail(t *testing.T) {
	data := []byte(`
schema: 1
key: sample
revision: 1
listed: true
host: {name: Sample, program: erika-k}
detail:
  erika_k:
    login:
      member_greeting: "{name}"
    boards:
      - {path: "2/1", name: " "}
      - {path: "2/1", name: "重複"}
`)
	_, err := ParsePreset("sample.yaml", data, Options{})
	if err == nil || !strings.Contains(err.Error(), "detail.erika_k") {
		t.Fatalf("error = %v, want Erika-K detail validation", err)
	}
}
