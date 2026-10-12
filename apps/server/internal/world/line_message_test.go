package world

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLineMessageReadsWithoutSlotFields(t *testing.T) {
	old := `{"id":"1","host_id":"h","from":{"Kind":"member","ID":"x"},"to":{"Kind":"persona","ID":"p"},"to_line":2,"body":"やあ","sent_at":"1996-04-01T10:00:00Z","deliver_at":"1996-04-01T10:00:00Z"}`
	var m LineMessage
	if err := json.Unmarshal([]byte(old), &m); err != nil {
		t.Fatal(err)
	}
	if m.Slot != "" || m.RespondsTo != "" || m.Body != "やあ" {
		t.Fatalf("old message read as %+v", m)
	}
}

func TestMemoryStoreKeepsLineMessageSlotFields(t *testing.T) {
	s := NewMemoryStore()
	at := time.Date(1996, 4, 1, 10, 0, 0, 0, time.UTC)
	saved := s.SaveLineMessage(LineMessage{HostID: "h", Body: "どうも", SentAt: at, DeliverAt: at, Slot: "answer|h", RespondsTo: "7"})
	got := s.ListLineMessages("h")
	if len(got) != 1 || got[0].ID != saved.ID || got[0].Slot != "answer|h" || got[0].RespondsTo != "7" {
		t.Fatalf("stored %+v", got)
	}
}
