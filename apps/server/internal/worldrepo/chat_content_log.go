package worldrepo

import (
	"encoding/json"
	"log"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// chatContentLog is operational telemetry for a resident's chat line, never a
// world fact (spec 017).
type chatContentLog struct {
	Host       string `json:"host"`
	Room       int    `json:"room"`
	Slot       string `json:"slot"`
	Kind       string `json:"kind"`
	Speaker    string `json:"speaker"`
	Addressee  string `json:"addressee,omitempty"`
	RespondsTo string `json:"responds_to,omitempty"`
	Body       string `json:"body"`
	At         string `json:"at"`
	ElapsedMS  int64  `json:"elapsed_ms"`
}

// logChatGeneratedContent logs a recorded resident line on hosts that opted
// in to the generated content log.
func (r *Repository) logChatGeneratedContent(job chatJob, u world.ChatUtterance) {
	if !r.shouldLogGeneratedContent(job.host) {
		return
	}
	entry := chatContentLog{
		Host: job.host.ID, Room: u.Room, Slot: job.slot.Key, Kind: string(job.slot.Kind),
		Speaker: u.Handle, RespondsTo: job.respondsTo, Body: u.Body, At: u.At.Format(time.RFC3339),
		ElapsedMS: time.Since(job.started).Milliseconds(),
	}
	if job.req.Addressee != nil {
		entry.Addressee = job.req.Addressee.Handle
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Chat generated content audit encoding failed: host=%s slot=%s err=%v", job.host.ID, job.slot.Key, err)
		return
	}
	log.Printf("Chat generated content: %s", encoded)
}
