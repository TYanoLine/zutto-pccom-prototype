package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type personaTimelinePocInterval struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	From       string `json:"from"`
	Until      string `json:"until,omitempty"`
	SourceKind string `json:"source_kind"`
}

type personaTimelinePocEvent struct {
	ID      string `json:"id"`
	At      string `json:"at"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

type personaTimelinePocPerson struct {
	ID               string                       `json:"id"`
	Handle           string                       `json:"handle"`
	BirthDate        string                       `json:"birth_date"`
	BaselineFacts    []string                     `json:"baseline_facts"`
	Occupation       []personaTimelinePocInterval `json:"occupation_history"`
	States           []personaTimelinePocInterval `json:"state_intervals"`
	Events           []personaTimelinePocEvent    `json:"events"`
	ObservationDates []string                     `json:"observation_dates"`
}

type personaTimelinePocSnapshot struct {
	At                 string                       `json:"at"`
	Age                int                          `json:"age"`
	Occupation         string                       `json:"occupation"`
	ActiveStates       []personaTimelinePocInterval `json:"active_states"`
	KnownEvents        []personaTimelinePocEvent    `json:"known_events"`
	HiddenFutureEvents int                          `json:"hidden_future_events"`
	RendererContext    []string                     `json:"renderer_context"`
}

func newPersonaTimelinePocHandler() http.HandlerFunc {
	people := personaTimelinePocPeople()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
			return
		}

		personID := strings.TrimSpace(r.URL.Query().Get("persona"))
		if personID == "" {
			personID = "worker"
		}
		person, ok := people[personID]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown persona"})
			return
		}
		at := strings.TrimSpace(r.URL.Query().Get("at"))
		if at == "" {
			at = person.ObservationDates[0]
		}
		snapshot, err := resolvePersonaTimelinePoc(person, at)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		catalog := make([]map[string]any, 0, len(people))
		keys := make([]string, 0, len(people))
		for id := range people {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			p := people[id]
			catalog = append(catalog, map[string]any{
				"id": id, "handle": p.Handle, "observation_dates": p.ObservationDates,
			})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"canonical": false,
			"note": "development PoC only; dates and life events are fictional fixtures. The important part is temporal resolution and future isolation.",
			"persona": person,
			"snapshot": snapshot,
			"catalog": catalog,
		})
	}
}

func personaTimelinePocPeople() map[string]personaTimelinePocPerson {
	return map[string]personaTimelinePocPerson{
		"worker": {
			ID: "timeline-worker", Handle: "RYO-5", BirthDate: "1967-03-12",
			BaselineFacts: []string{
				"パソコン通信には時々接続する",
				"ファイルや通信の具体的な話題には反応しやすい",
			},
			Occupation: []personaTimelinePocInterval{
				{Key: "employment.occupation", Value: "会社員", From: "1994-04-01", Until: "1996-09-30", SourceKind: "fixture_history"},
				{Key: "employment.status", Value: "無職", From: "1996-10-01", Until: "1996-10-20", SourceKind: "fixture_event"},
				{Key: "employment.occupation", Value: "販売・サービス業", From: "1996-10-21", SourceKind: "fixture_event"},
			},
			States: []personaTimelinePocInterval{
				{Key: "health.common_cold", Value: "軽い風邪", From: "1996-07-18", Until: "1996-07-23", SourceKind: "fixture_event"},
			},
			Events: []personaTimelinePocEvent{
				{ID: "cold-start", At: "1996-07-18", Kind: "health.started", Summary: "軽い風邪をひいた"},
				{ID: "cold-end", At: "1996-07-24", Kind: "health.ended", Summary: "風邪から回復した"},
				{ID: "job-end", At: "1996-09-30", Kind: "employment.ended", Summary: "勤めていた会社を退職した"},
				{ID: "job-start", At: "1996-10-21", Kind: "employment.started", Summary: "販売・サービス業の仕事を始めた"},
			},
			ObservationDates: []string{"1996-07-20", "1996-08-15", "1996-10-10", "1996-11-01"},
		},
		"student": {
			ID: "timeline-student", Handle: "EMI-2", BirthDate: "1975-11-08",
			BaselineFacts: []string{
				"大学生",
				"ゲームと音楽の話題を読むことが多い",
			},
			Occupation: []personaTimelinePocInterval{
				{Key: "school.role", Value: "大学生", From: "1994-04-01", SourceKind: "fixture_history"},
			},
			States: []personaTimelinePocInterval{
				{Key: "school.calendar", Value: "夏季休業", From: "1996-07-20", Until: "1996-09-15", SourceKind: "fixture_calendar"},
			},
			Events: []personaTimelinePocEvent{
				{ID: "summer-start", At: "1996-07-20", Kind: "school.break.started", Summary: "学校カレンダー上の夏季休業に入った"},
				{ID: "summer-end", At: "1996-09-16", Kind: "school.break.ended", Summary: "夏季休業が終わった"},
			},
			ObservationDates: []string{"1996-07-10", "1996-08-10", "1996-09-20"},
		},
	}
}

func resolvePersonaTimelinePoc(person personaTimelinePocPerson, rawAt string) (personaTimelinePocSnapshot, error) {
	at, err := time.Parse("2006-01-02", rawAt)
	if err != nil {
		return personaTimelinePocSnapshot{}, err
	}
	birth, err := time.Parse("2006-01-02", person.BirthDate)
	if err != nil {
		return personaTimelinePocSnapshot{}, err
	}
	age := at.Year() - birth.Year()
	birthday := time.Date(at.Year(), birth.Month(), birth.Day(), 0, 0, 0, 0, time.UTC)
	if at.Before(birthday) {
		age--
	}

	occupation := "未設定"
	for _, interval := range person.Occupation {
		if timelinePocActive(interval, at) {
			occupation = interval.Value
			break
		}
	}

	active := make([]personaTimelinePocInterval, 0)
	for _, state := range person.States {
		if timelinePocActive(state, at) {
			active = append(active, state)
		}
	}

	known := make([]personaTimelinePocEvent, 0)
	hidden := 0
	for _, event := range person.Events {
		eventAt, parseErr := time.Parse("2006-01-02", event.At)
		if parseErr != nil {
			continue
		}
		if eventAt.After(at) {
			hidden++
			continue
		}
		known = append(known, event)
	}

	context := []string{
		"観測日: " + rawAt,
		"年齢: " + strconvItoa(age) + "歳",
		"現在の所属/職業: " + occupation,
	}
	for _, fact := range person.BaselineFacts {
		context = append(context, "既存事実: "+fact)
	}
	for _, state := range active {
		context = append(context, "現在状態: "+state.Value+" ("+state.From+"〜"+timelinePocUntil(state.Until)+")")
	}
	for _, event := range known {
		context = append(context, "既知の過去イベント: "+event.At+" "+event.Summary)
	}

	return personaTimelinePocSnapshot{
		At: rawAt, Age: age, Occupation: occupation, ActiveStates: active,
		KnownEvents: known, HiddenFutureEvents: hidden, RendererContext: context,
	}, nil
}

func timelinePocActive(interval personaTimelinePocInterval, at time.Time) bool {
	from, err := time.Parse("2006-01-02", interval.From)
	if err != nil || at.Before(from) {
		return false
	}
	if interval.Until == "" {
		return true
	}
	until, err := time.Parse("2006-01-02", interval.Until)
	return err == nil && !at.After(until)
}

func timelinePocUntil(value string) string {
	if value == "" {
		return "継続中"
	}
	return value
}

func strconvItoa(value int) string {
	// Keep this PoC self-contained without leaking formatting concerns into the model.
	return fmt.Sprintf("%d", value)
}
