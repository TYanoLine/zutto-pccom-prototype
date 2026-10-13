package worldrepo

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/hostprogram/erikak"
	"zutto-pccom/apps/server/internal/world"
)

// Spec 019 story 6: one user on two calls at once. Each call knows which
// screen is its own; a line typed on one call is another screen's line on the
// other, and WHO marks only the caller's own line.
func TestOneUserOnTwoCalls(t *testing.T) {
	repo, _, host, at, room := talkTestRepo(t, nil)
	current := at
	repo.SetWorldNow(func() time.Time { return current })
	host.Lines = 16

	call := func(id string) *erikak.Runtime {
		line, ok := repo.BeginHumanCall(host, id, "u1", 14400)
		if !ok {
			t.Fatalf("no free line for %s", id)
		}
		rt := erikak.New(host, repo)
		rt.SetCallInfo(world.CallInfo{CallID: id, HumanID: "u1", Line: line, Baud: 14400})
		rt.HandleLine("GUEST")
		rt.HandleLine("4")
		rt.HandleLine("X")
		if out, _ := rt.HandleLine(fmt.Sprint(room)); !strings.Contains(out, "## chat on  GUEST ##") {
			t.Fatalf("room entry of %s:\n%s", id, out)
		}
		return rt
	}
	a, b := call("call-A"), call("call-B")

	current = at.Add(10 * time.Second)
	if out, _ := a.HandleLine("やあ"); !strings.Contains(out, "GUEST   :やあ") {
		t.Fatalf("own line:\n%s", out)
	}
	current = at.Add(20 * time.Second)
	if got := b.Poll(current); !strings.Contains(got, "GUEST   :やあ") {
		t.Errorf("the other call does not see the line:\n%s", got)
	}
	if got := a.Poll(current); strings.Contains(got, "やあ") {
		t.Errorf("the call that typed the line sees it again:\n%s", got)
	}

	out, _ := a.HandleLine("..")
	var rows []string
	for _, l := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(l, "#") && strings.Contains(l, "GUEST") {
			rows = append(rows, l)
		}
	}
	own := 0
	for _, l := range rows {
		if strings.Contains(l, " WHO") {
			own++
		}
	}
	if len(rows) != 2 || own != 1 {
		t.Errorf("WHO shows %d rows of the user's calls with %d marked as the caller's own, want 2 and 1:\n%s", len(rows), own, out)
	}
}
