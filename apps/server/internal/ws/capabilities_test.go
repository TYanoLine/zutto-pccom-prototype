package ws

import (
	"encoding/json"
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

// TestCapabilitiesFollowTheHostFlag checks that capabilitiesFor returns the
// correct GenerationTrace flag based on the host's Debug field.
func TestCapabilitiesFollowTheHostFlag(t *testing.T) {
	hostA := world.Host{ID: "a", Debug: hostcatalog.DebugFlags{GenerationTrace: true}}
	hostB := world.Host{ID: "b"}

	capA := capabilitiesFor(hostA)
	capB := capabilitiesFor(hostB)

	if !capA.GenerationTrace {
		t.Errorf("expected capabilitiesFor(hostA with GenerationTrace=true).GenerationTrace = true, got false")
	}
	if capB.GenerationTrace {
		t.Errorf("expected capabilitiesFor(hostB with GenerationTrace=false).GenerationTrace = false, got true")
	}
}

// TestCapabilitiesIgnoreTheRoleAndTheIdentity checks that only the Debug flag
// determines the capabilities, not the host's role, phone, or other attributes.
func TestCapabilitiesIgnoreTheRoleAndTheIdentity(t *testing.T) {
	host := world.Host{
		ID:    "hakata-canal-net",
		Phone: "0920000196",
		Role:  hostcatalog.RoleExperiment,
		Debug: hostcatalog.DebugFlags{GenerationTrace: false},
	}

	cap := capabilitiesFor(host)

	if cap.GenerationTrace {
		t.Errorf("expected capabilitiesFor(host without GenerationTrace flag).GenerationTrace = false, got true")
	}
}

// TestServerMessageEncodesCapabilities checks that capabilities are correctly
// JSON-marshaled and omitted when nil.
func TestServerMessageEncodesCapabilities(t *testing.T) {
	msgWithCap := serverMessage{
		Type:         "dial_result",
		Capabilities: &hostCapabilities{GenerationTrace: true},
	}
	msgWithoutCap := serverMessage{
		Type:         "dial_result",
		Capabilities: nil,
	}

	b1, err := json.Marshal(msgWithCap)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s1 := string(b1)

	b2, err := json.Marshal(msgWithoutCap)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	s2 := string(b2)

	if !strings.Contains(s1, `"capabilities":{"generation_trace":true}`) {
		t.Errorf("expected JSON to contain capabilities, got: %s", s1)
	}
	if strings.Contains(s2, `"capabilities"`) {
		t.Errorf("expected nil capabilities to be omitted, got: %s", s2)
	}
}
