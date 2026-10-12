package presence

import (
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldcalendar"
)

// Window is the granularity of call starts.
const Window = 10 * time.Minute

// maxStartProbability caps one window's start chance.
const maxStartProbability = .95

// inTeleho reports the Telehodai discount window, 23:00-07:59.
func inTeleho(t time.Time) bool { return t.Hour() >= 23 || t.Hour() < 8 }

// telehoFactor: residents who registered this host call at night and avoid the
// day; the first twenty minutes after 23:00 get the rush of waiting callers.
func telehoFactor(prof Profile, t time.Time) float64 {
	h := t.Hour()
	if prof.Teleho {
		switch {
		case h == 23 && t.Minute() < 20:
			return 2.5 * 3
		case inTeleho(t):
			return 2.5
		default:
			return .5
		}
	}
	switch {
	case inTeleho(t):
		return 1
	case h >= 19:
		return .7
	default:
		return .6
	}
}

// phoneFactor: a line shared with the family is busy in the evening.
func phoneFactor(prof Profile, t time.Time) float64 {
	if prof.SharedPhone && t.Hour() >= 19 && t.Hour() < 23 {
		return .4
	}
	return 1
}

// nightFactor: only night owls stay up through 02:00-05:59.
func nightFactor(prof Profile, t time.Time) float64 {
	if h := t.Hour(); h >= 2 && h < 6 {
		return .15 + .85*prof.NightOwl
	}
	return 1
}

// hostOpen reports whether the station answers calls at t.
func hostOpen(host world.Host, t time.Time) bool {
	if host.OperatingHours == [2]string{} {
		return true
	}
	from, err1 := hostcatalog.ParseClockMinutes(host.OperatingHours[0])
	to, err2 := hostcatalog.ParseClockMinutes(host.OperatingHours[1])
	if err1 != nil || err2 != nil {
		return true
	}
	m := t.Hour()*60 + t.Minute()
	if from < to {
		return from <= m && m < to
	}
	return m >= from || m < to
}

// StartProbability is the chance that the resident starts a call to the host
// in the window beginning at t: the product of the spec 008 FR-012 layers.
func StartProbability(p world.Persona, prof Profile, host world.Host, cal *worldcalendar.Calendar, t time.Time) float64 {
	if !hostOpen(host, t) {
		return 0
	}
	holiday := cal.UseHolidayProfile(t, prof.Lifestyle)
	perWindow := dailyCalls(p.ActivityPattern) * share(prof.Lifestyle, holiday, t.Hour()) / float64(time.Hour/Window)
	v := perWindow *
		telehoFactor(prof, t) *
		phoneFactor(prof, t) *
		nightFactor(prof, t) *
		cal.Factor(t, prof.Lifestyle)
	if v > maxStartProbability {
		return maxStartProbability
	}
	return v
}
