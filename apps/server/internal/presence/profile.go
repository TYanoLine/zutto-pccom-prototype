package presence

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// Lifestyle values (open strings shared with worldcalendar effects.yaml).
const (
	SecondaryStudent  = "secondary_student"
	UniversityStudent = "university_student"
	OfficeWorker      = "office_worker"
	SelfEmployed      = "self_employed"
	Homemaker         = "homemaker"
	ShiftWorker       = "shift_worker"
)

// Persona fact keys written when a resident's presence is observed.
const (
	FactLifestyle  = "presence.lifestyle"
	FactPhoneLine  = "presence.phone_line"
	FactNightOwl   = "presence.night_owl"
	FactModemBPS   = "presence.modem_bps"
	factTelehoPref = "presence.teleho:"
	FactSourceKind = "presence_profile"
)

// FactTeleho is the per-host Telehodai registration key.
func FactTeleho(hostID string) string { return factTelehoPref + hostID }

// Profile is a resident's connection circumstances for one host.
type Profile struct {
	Lifestyle   string
	SharedPhone bool
	NightOwl    float64 // 0..1
	ModemBPS    int
	Teleho      bool // this host is one of the resident's Telehodai numbers
}

var lifestyleShares = []struct {
	name  string
	share float64
}{
	{SecondaryStudent, .15}, {UniversityStudent, .25}, {OfficeWorker, .40},
	{SelfEmployed, .08}, {Homemaker, .05}, {ShiftWorker, .07},
}

// lifestyleFromOccupation maps an already materialized occupation onto a
// lifestyle. An empty result means the occupation says nothing.
func lifestyleFromOccupation(occupation string) string {
	o := strings.TrimSpace(occupation)
	if o == "" {
		return ""
	}
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(o, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("中学", "高校"):
		return SecondaryStudent
	case has("大学", "専門学校", "院生", "大学院", "浪人"):
		return UniversityStudent
	case has("主婦", "主夫"):
		return Homemaker
	case has("自営", "店"):
		return SelfEmployed
	case has("夜勤", "看護", "警備"):
		return ShiftWorker
	}
	return OfficeWorker
}

// ProfileFor returns the resident's profile for the host. Stored presence
// facts win; otherwise the occupation, then stable draws, decide.
func ProfileFor(p world.Persona, facts []world.PersonaFact, host world.Host) Profile {
	stored := map[string]string{}
	for _, f := range facts {
		stored[f.Key] = f.Value
	}
	var prof Profile

	prof.Lifestyle = stored[FactLifestyle]
	if prof.Lifestyle == "" {
		prof.Lifestyle = lifestyleFromOccupation(p.Occupation)
	}
	if prof.Lifestyle == "" {
		u := unit("lifestyle", p.ID)
		acc := 0.0
		for _, s := range lifestyleShares {
			acc += s.share
			if u < acc {
				prof.Lifestyle = s.name
				break
			}
		}
		if prof.Lifestyle == "" {
			prof.Lifestyle = OfficeWorker
		}
	}

	if v, ok := stored[FactPhoneLine]; ok {
		prof.SharedPhone = v == "shared"
	} else {
		shared := .55
		if prof.Lifestyle == SecondaryStudent {
			shared = .85
		}
		prof.SharedPhone = unit("phone", p.ID) < shared
	}

	if v, err := strconv.ParseFloat(stored[FactNightOwl], 64); err == nil {
		prof.NightOwl = v
	} else {
		n := unit("night", p.ID)
		if prof.Lifestyle == UniversityStudent {
			n += .2
		}
		if n > 1 {
			n = 1
		}
		prof.NightOwl = float64(int(n*100)) / 100
	}

	if v, err := strconv.Atoi(stored[FactModemBPS]); err == nil {
		prof.ModemBPS = v
	} else {
		u := unit("modem", p.ID)
		switch {
		case u < .05:
			prof.ModemBPS = 2400
		case u < .30:
			prof.ModemBPS = 9600
		case u < .75:
			prof.ModemBPS = 14400
		default:
			prof.ModemBPS = 28800
		}
	}

	if v, ok := stored[FactTeleho(host.ID)]; ok {
		prof.Teleho = v == "yes"
	} else {
		share := .25
		if host.TelehoFriendly {
			share = .55
		}
		prof.Teleho = unit("teleho", p.ID, host.ID) < share
	}
	return prof
}

// Facts returns the profile as persona facts to store on first observation.
func (prof Profile) Facts(personaID, hostID string, at time.Time) []world.PersonaFact {
	phone := "own"
	if prof.SharedPhone {
		phone = "shared"
	}
	teleho := "no"
	if prof.Teleho {
		teleho = "yes"
	}
	mk := func(key, value string) world.PersonaFact {
		return world.PersonaFact{PersonaID: personaID, Key: key, Value: value, MaterializedAt: at, SourceKind: FactSourceKind}
	}
	return []world.PersonaFact{
		mk(FactLifestyle, prof.Lifestyle),
		mk(FactPhoneLine, phone),
		mk(FactNightOwl, fmt.Sprintf("%.2f", prof.NightOwl)),
		mk(FactModemBPS, strconv.Itoa(prof.ModemBPS)),
		mk(FactTeleho(hostID), teleho),
	}
}

// Baud is the speed the resident's modem negotiates with the host.
func (prof Profile) Baud(host world.Host) int {
	if host.MaxBaud > 0 && prof.ModemBPS > host.MaxBaud {
		return host.MaxBaud
	}
	return prof.ModemBPS
}
