package presence

import (
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcalendar"
)

var jst = time.FixedZone("JST", 9*60*60)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, jst)
	if err != nil {
		panic(err)
	}
	return t
}

func TestTelehoFactorJumpsAt2300(t *testing.T) {
	reg := Profile{Teleho: true}
	if telehoFactor(reg, at("1996-10-15 23:00")) <= telehoFactor(reg, at("1996-10-15 22:50")) {
		t.Fatal("registered callers should rush at 23:00")
	}
	if telehoFactor(reg, at("1996-10-15 23:30")) >= telehoFactor(reg, at("1996-10-15 23:10")) {
		t.Fatal("the rush is limited to the first twenty minutes")
	}
	if telehoFactor(reg, at("1996-10-15 08:00")) >= telehoFactor(reg, at("1996-10-15 07:50")) {
		t.Fatal("08:00 ends the discount window")
	}
	if telehoFactor(Profile{}, at("1996-10-15 14:00")) >= 1 {
		t.Fatal("unregistered callers avoid daytime rates")
	}
}

func TestPhoneFactorSharedEvening(t *testing.T) {
	if phoneFactor(Profile{SharedPhone: true}, at("1996-10-15 20:00")) >= 1 {
		t.Fatal("a shared line is busy in the evening")
	}
	if phoneFactor(Profile{SharedPhone: true}, at("1996-10-15 23:30")) != 1 || phoneFactor(Profile{}, at("1996-10-15 20:00")) != 1 {
		t.Fatal("only shared lines, only 19-23")
	}
}

func TestNightFactor(t *testing.T) {
	if nightFactor(Profile{NightOwl: 1}, at("1996-10-15 03:00")) <= nightFactor(Profile{NightOwl: 0}, at("1996-10-15 03:00")) {
		t.Fatal("night owls stay up")
	}
	if nightFactor(Profile{}, at("1996-10-15 01:00")) != 1 {
		t.Fatal("before 02:00 nobody is reduced")
	}
}

func TestHostOpen(t *testing.T) {
	night := world.Host{OperatingHours: [2]string{"22:00", "08:00"}}
	if hostOpen(night, at("1996-10-15 14:00")) || !hostOpen(night, at("1996-10-15 23:00")) || !hostOpen(night, at("1996-10-15 07:59")) {
		t.Fatal("night-only station hours wrong")
	}
	if !hostOpen(world.Host{}, at("1996-10-15 14:00")) {
		t.Fatal("no operating hours means always open")
	}
	p := world.Persona{ID: "p", ActivityPattern: "regular"}
	if StartProbability(p, Profile{Lifestyle: OfficeWorker}, night, worldcalendar.Default(), at("1996-10-15 14:00")) != 0 {
		t.Fatal("closed station gets no calls")
	}
}

func TestWeekdayVsHolidayRhythm(t *testing.T) {
	cal := worldcalendar.Default()
	p := world.Persona{ID: "p", ActivityPattern: "regular"}
	prof := Profile{Lifestyle: OfficeWorker}
	weekday := StartProbability(p, prof, world.Host{}, cal, at("1996-10-15 14:00"))
	saturday := StartProbability(p, prof, world.Host{}, cal, at("1996-10-19 14:00"))
	substitute := StartProbability(p, prof, world.Host{}, cal, at("1996-11-04 14:00"))
	if !(saturday > weekday && substitute > weekday) {
		t.Fatalf("weekday %v, saturday %v, substitute holiday %v", weekday, saturday, substitute)
	}
}
