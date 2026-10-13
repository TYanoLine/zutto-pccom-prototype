package ws

import (
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

const keyA = "0123456789abcdef0123456789abcdef"
const keyB = "fedcba9876543210fedcba9876543210"

func TestHumanIDFollowsTheBrowserKey(t *testing.T) {
	a1, a2, b := humanID(keyA), humanID(keyA), humanID(keyB)
	if a1 != a2 || a1 == b {
		t.Fatalf("same key %q %q, other key %q", a1, a2, b)
	}
	if strings.Contains(a1, keyA[:8]) || strings.Contains(a1, keyA) {
		t.Fatalf("the human ID %q carries the key", a1)
	}
	for _, bad := range []string{"", "short", strings.ToUpper(keyA), keyA + "0", "g123456789abcdef0123456789abcdef"} {
		x, y := humanID(bad), humanID(bad)
		if x == "" || x == y {
			t.Errorf("key %q: IDs %q %q must be one per call", bad, x, y)
		}
	}
}

type humanLines struct{ humans map[string]string }

func (l *humanLines) DialLine(world.Host, time.Time) (int, bool) { return 1, true }
func (l *humanLines) BeginHumanCall(_ world.Host, callID, humanID string, _ int) (int, bool) {
	l.humans[callID] = humanID
	return 1, true
}
func (l *humanLines) SetHumanCallHandle(string, string) {}
func (l *humanLines) EndHumanCall(string)               {}

func TestCallsOfOneUserShareTheHumanID(t *testing.T) {
	lines := &humanLines{humans: map[string]string{}}
	m := NewSessionManager(time.Minute)
	m.SetLineOccupancy(lines)
	host := world.Host{ID: "h"}
	s1, _, err := m.Create(host, 9600, 1, nil, humanID(keyA))
	if err != nil {
		t.Fatal(err)
	}
	s2, _, err := m.Create(host, 9600, 1, nil, humanID(keyA))
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == s2.ID || s1.HumanID == "" || s1.HumanID != s2.HumanID {
		t.Fatalf("calls %q %q, humans %q %q", s1.ID, s2.ID, s1.HumanID, s2.HumanID)
	}
	if lines.humans[s1.ID] != s1.HumanID || lines.humans[s2.ID] != s2.HumanID {
		t.Fatalf("the world was told %v", lines.humans)
	}
}
