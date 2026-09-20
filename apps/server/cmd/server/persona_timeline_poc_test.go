package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolvePersonaTimelinePocHidesFutureAndChangesOccupation(t *testing.T) {
	person := personaTimelinePocPeople()["worker"]

	august, err := resolvePersonaTimelinePoc(person, "1996-08-15")
	if err != nil {
		t.Fatal(err)
	}
	if august.Occupation != "会社員" {
		t.Fatalf("August occupation = %q, want 会社員", august.Occupation)
	}
	for _, line := range august.RendererContext {
		if strings.Contains(line, "退職") || strings.Contains(line, "販売・サービス業の仕事を始めた") {
			t.Fatalf("future event leaked into August renderer context: %q", line)
		}
	}
	if august.HiddenFutureEvents != 2 {
		t.Fatalf("August hidden future events = %d, want 2", august.HiddenFutureEvents)
	}

	october, err := resolvePersonaTimelinePoc(person, "1996-10-10")
	if err != nil {
		t.Fatal(err)
	}
	if october.Occupation != "無職" {
		t.Fatalf("October occupation = %q, want 無職", october.Occupation)
	}
	if october.HiddenFutureEvents != 1 {
		t.Fatalf("October hidden future events = %d, want 1", october.HiddenFutureEvents)
	}

	november, err := resolvePersonaTimelinePoc(person, "1996-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if november.Occupation != "販売・サービス業" {
		t.Fatalf("November occupation = %q, want 販売・サービス業", november.Occupation)
	}
	if november.HiddenFutureEvents != 0 {
		t.Fatalf("November hidden future events = %d, want 0", november.HiddenFutureEvents)
	}
}

func TestResolvePersonaTimelinePocTemporaryStates(t *testing.T) {
	worker := personaTimelinePocPeople()["worker"]
	sick, err := resolvePersonaTimelinePoc(worker, "1996-07-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(sick.ActiveStates) != 1 || sick.ActiveStates[0].Key != "health.common_cold" {
		t.Fatalf("cold state = %#v", sick.ActiveStates)
	}
	recovered, err := resolvePersonaTimelinePoc(worker, "1996-08-15")
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.ActiveStates) != 0 {
		t.Fatalf("cold state remained active: %#v", recovered.ActiveStates)
	}

	student := personaTimelinePocPeople()["student"]
	summer, err := resolvePersonaTimelinePoc(student, "1996-08-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(summer.ActiveStates) != 1 || summer.ActiveStates[0].Value != "夏季休業" {
		t.Fatalf("summer state = %#v", summer.ActiveStates)
	}
	autumn, err := resolvePersonaTimelinePoc(student, "1996-09-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(autumn.ActiveStates) != 0 {
		t.Fatalf("summer break remained active: %#v", autumn.ActiveStates)
	}
}

func TestPersonaTimelinePocViewerHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/poc/persona-timeline", nil)
	rr := httptest.NewRecorder()
	newPersonaTimelinePocViewerHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"人物を「ある日付」で解決する", "FUTURE ISOLATED", "/api/debug/persona-timeline"} {
		if !strings.Contains(body, want) {
			t.Fatalf("viewer missing %q", want)
		}
	}
}
