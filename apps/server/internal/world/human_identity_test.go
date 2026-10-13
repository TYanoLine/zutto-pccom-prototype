package world

import "testing"

// Spec 019: a call made before human IDs existed still names its caller by
// call ID, and a call with a human ID names the user across calls.
func TestHumanCallSessionNamesTheUser(t *testing.T) {
	old := HumanCall{CallID: "c1", HostID: "h"}
	if got := old.Session().Actor; got != (ActorRef{Kind: ActorKindMember, ID: "c1"}) {
		t.Fatalf("old call actor = %+v", got)
	}
	a := HumanCall{CallID: "c1", HumanID: "u1", HostID: "h"}.Session()
	b := HumanCall{CallID: "c2", HumanID: "u1", HostID: "h"}.Session()
	if a.Actor != b.Actor || a.Actor.ID != "u1" {
		t.Fatalf("two calls of one user: %+v and %+v", a.Actor, b.Actor)
	}
	if a.CallID != "c1" || b.CallID != "c2" {
		t.Fatalf("call IDs = %q, %q", a.CallID, b.CallID)
	}
}
