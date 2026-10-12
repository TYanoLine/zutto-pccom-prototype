package presence

import (
	"reflect"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

func TestLifestyleFromOccupation(t *testing.T) {
	for occ, want := range map[string]string{
		"高校2年": SecondaryStudent, "大学生": UniversityStudent, "会社員": OfficeWorker,
		"専業主婦": Homemaker, "自営業": SelfEmployed, "看護師": ShiftWorker, "": "",
	} {
		if got := lifestyleFromOccupation(occ); got != want {
			t.Errorf("%q -> %q, want %q", occ, got, want)
		}
	}
}

func TestProfileIsStable(t *testing.T) {
	host := world.Host{ID: "h1", MaxBaud: 14400, TelehoFriendly: true}
	p := world.Persona{ID: "p-42"}
	a, b := ProfileFor(p, nil, host), ProfileFor(p, nil, host)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("profile differs: %+v vs %+v", a, b)
	}
	if a.Baud(host) > host.MaxBaud {
		t.Fatalf("baud %d above host max", a.Baud(host))
	}
}

func TestStoredFactsWin(t *testing.T) {
	host := world.Host{ID: "h1", MaxBaud: 28800}
	p := world.Persona{ID: "p-7", Occupation: "会社員"}
	want := Profile{Lifestyle: Homemaker, SharedPhone: true, NightOwl: .5, ModemBPS: 2400, Teleho: true}
	got := ProfileFor(p, want.Facts(p.ID, host.ID, time.Time{}), host)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestOccupationDecidesLifestyle(t *testing.T) {
	if got := ProfileFor(world.Persona{ID: "x", Occupation: "大学3年"}, nil, world.Host{ID: "h"}).Lifestyle; got != UniversityStudent {
		t.Fatalf("lifestyle = %q", got)
	}
}
