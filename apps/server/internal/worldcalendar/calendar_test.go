package worldcalendar

import (
	"reflect"
	"testing"
	"time"
)

// japan1996Holidays mirrors JAPAN_1996_HOLIDAYS in
// apps/web/src/billing/pseudoTariffs.ts; both come from the Cabinet Office list.
var japan1996Holidays = []string{
	"1996-01-01", "1996-01-15", "1996-02-11", "1996-02-12", "1996-03-20",
	"1996-04-29", "1996-05-03", "1996-05-04", "1996-05-05", "1996-05-06",
	"1996-07-20", "1996-09-15", "1996-09-16", "1996-09-23", "1996-10-10",
	"1996-11-03", "1996-11-04", "1996-11-23", "1996-12-23",
}

var jst = time.FixedZone("JST", 9*60*60)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, jst)
	if err != nil {
		panic(err)
	}
	return t
}

func TestHolidaysMatchClientSet(t *testing.T) {
	if got := Default().Holidays(); !reflect.DeepEqual(got, japan1996Holidays) {
		t.Fatalf("holidays = %v, want %v", got, japan1996Holidays)
	}
}

func TestDayFacts(t *testing.T) {
	c := Default()
	if d := c.Day(at("1996-11-04 12:00")); !d.Holiday || !d.Known || d.Weekday != time.Monday {
		t.Fatalf("1996-11-04 = %+v, want a known Monday holiday", d)
	}
	if d := c.Day(at("1996-12-24 21:00")); !d.Has("christmas_eve") || d.Weekend() {
		t.Fatalf("1996-12-24 = %+v, want christmas_eve on a weekday", d)
	}
	if d := c.Day(at("1997-01-02 10:00")); !d.Has("year_end_new_year") {
		t.Fatalf("1997-01-02 = %+v, want year_end_new_year", d)
	}
	if d := c.Day(at("1998-05-01 10:00")); d.Known {
		t.Fatalf("1998 should have no calendar data: %+v", d)
	}
	if d := c.Day(at("1996-10-12 10:00")); !d.Weekend() || d.Holiday {
		t.Fatalf("1996-10-12 = %+v, want a Saturday that is not a holiday", d)
	}
}

func TestEffects(t *testing.T) {
	c := Default()
	if f := c.Factor(at("1996-10-15 21:00"), "office_worker"); f != 1 {
		t.Fatalf("ordinary day factor = %v, want 1", f)
	}
	if f := c.Factor(at("1996-12-24 21:00"), "office_worker"); f >= 1 {
		t.Fatalf("christmas eve evening factor = %v, want < 1", f)
	}
	if f := c.Factor(at("1996-12-24 12:00"), "office_worker"); f != 1 {
		t.Fatalf("christmas eve noon factor = %v, want 1", f)
	}
	nye := c.Factor(at("1996-12-31 23:30"), "office_worker")
	pastMidnight := c.Factor(at("1997-01-01 00:30"), "office_worker")
	if nye <= c.Factor(at("1996-12-31 20:00"), "office_worker") || pastMidnight <= c.Factor(at("1997-01-01 02:00"), "office_worker") {
		t.Fatalf("new year's eve 23-25h should be raised: 23:30=%v 00:30=%v", nye, pastMidnight)
	}
	if c.Factor(at("1996-12-30 21:00"), "university_student") <= c.Factor(at("1996-12-30 21:00"), "office_worker") {
		t.Fatal("students should be less reduced than workers over the new year")
	}
}

func TestHolidayProfile(t *testing.T) {
	c := Default()
	weekdayInSummer := at("1996-08-06 14:00") // Tuesday
	if !c.UseHolidayProfile(weekdayInSummer, "secondary_student") {
		t.Fatal("students follow the holiday rhythm during summer break")
	}
	if c.UseHolidayProfile(weekdayInSummer, "office_worker") {
		t.Fatal("office workers keep the weekday rhythm during school summer break")
	}
	if !c.UseHolidayProfile(at("1996-11-04 14:00"), "office_worker") {
		t.Fatal("a substitute holiday uses the holiday rhythm")
	}
}
