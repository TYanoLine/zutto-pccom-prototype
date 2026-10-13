package worldrepo

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/lineslot"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

// Residents' line messages to human callers (spec 018). The World decides who
// writes, when and in answer to what (internal/lineslot); this file writes the
// decided messages through an optional generator, records them and hands them
// to the host program that delivers them. Messages are written only while the
// human's call is live.

// Timing of line message writing. Fictional tuning values.
const (
	// lineLead is how long before its time a slot's message starts being
	// written.
	lineLead = 20 * time.Second
	// lineStale is how late a slot's message may still be recorded.
	lineStale = time.Minute
	// lineContext bounds how far back slots are computed on each call.
	lineContext = 10 * time.Minute
	// lineHistory is how many earlier messages between the two the writer sees.
	lineHistory = 6
	// lineJobTimeout bounds one message's writing, retry included.
	lineJobTimeout = 20 * time.Second
)

// lineTalk is the repository's spec 018 state.
type lineTalk struct {
	mu     sync.Mutex
	writer llm.LineMessageWriter
	// started holds the slots whose messages were started or given up.
	started map[string]bool
	// shown is the end of the last window handed to a call; a message is never
	// recorded at or before it.
	shown map[string]time.Time
}

// SetLineMessageWriter gives residents words in line messages. Without a
// writer residents never send one.
func (r *Repository) SetLineMessageWriter(w llm.LineMessageWriter) {
	t := &r.lineTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writer = w
}

// lineJob is one slot's message being written.
type lineJob struct {
	host       world.Host
	call       world.LineMessageCall
	slot       lineslot.Slot
	req        llm.LineMessageRequest
	stayTo     time.Time
	respondsTo string
	started    time.Time
}

// LineMessageEvents implements world.LineMessageHost.
func (r *Repository) LineMessageEvents(host world.Host, call world.LineMessageCall, from, to time.Time) []world.LineMessage {
	records, ok := r.Base.(world.ChatRecordStore)
	ps, havePresence := r.presenceStore()
	if !ok || !havePresence || !to.After(from) || call.CallID == "" {
		return nil
	}
	lock := presenceLock(host.ID)
	lock.Lock()
	defer lock.Unlock()
	hc, live := ps.HumanCall(call.CallID)
	if !live || hc.HostID != host.ID {
		return nil
	}

	t := &r.lineTalk
	t.mu.Lock()
	writer := t.writer
	if t.shown == nil {
		t.shown = map[string]time.Time{}
	}
	if to.After(t.shown[call.CallID]) {
		t.shown[call.CallID] = to
	}
	t.mu.Unlock()
	if writer != nil {
		r.startLineJobs(writer, host, call, hc, ps, records, to)
	}

	var out []world.LineMessage
	for _, m := range records.ListLineMessages(host.ID) {
		if m.Slot != "" && m.To == call.Human && m.ToLine == call.Line && m.DeliverAt.After(from) && !m.DeliverAt.After(to) {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].DeliverAt.Equal(out[j].DeliverAt) {
			return out[i].DeliverAt.Before(out[j].DeliverAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// startLineJobs computes the slots due and starts writing them. The caller
// holds the host's presence lock.
func (r *Repository) startLineJobs(writer llm.LineMessageWriter, host world.Host, call world.LineMessageCall, hc world.HumanCall, ps world.PresenceStore, records world.ChatRecordStore, now time.Time) {
	known := world.KnownAs(call.Human, call.Login)
	windowFrom := now.Add(-lineContext)
	if windowFrom.Before(hc.Start) {
		windowFrom = hc.Start.Add(-time.Nanosecond)
	}

	personas := map[string]world.Persona{}
	if store, ok := r.Base.(world.PersonaStore); ok {
		for _, p := range store.ListHostPersonas(host.ID) {
			personas[p.ID] = p
		}
	}
	ctx := r.gatherCalls(ps, host, now.Add(lineLead), windowFrom, now)
	var residents []lineslot.Resident
	for _, s := range ctx.sessions {
		if s.Actor.Kind != world.ActorKindPersona || s.End.IsZero() || !s.End.After(windowFrom) {
			continue
		}
		handle := s.Handle
		if handle == "" {
			handle = r.handleOf(host, s.Actor)
		}
		residents = append(residents, lineslot.Resident{Actor: s.Actor, Handle: handle, Persona: personas[s.Actor.ID],
			Line: s.Line, Call: world.CallActivity{Actor: s.Actor, Start: s.Start}.Key(), From: s.Start, To: s.End})
	}
	if len(residents) == 0 {
		return
	}

	var sent, recorded, all []world.LineMessage
	for _, m := range records.ListLineMessages(host.ID) {
		switch {
		case m.From == call.Human && m.SentAt.After(hc.Start.Add(-time.Nanosecond)):
			sent = append(sent, m)
			all = append(all, m)
		case m.To == call.Human && m.Slot != "" && m.SentAt.After(hc.Start.Add(-time.Nanosecond)):
			recorded = append(recorded, m)
			all = append(all, m)
		}
	}
	events := map[string]lineslot.WorldEvent{}
	var evs []lineslot.WorldEvent
	for _, e := range r.chatWorldEvents(host, map[world.ActorRef]world.ActorRef{call.Human: known}, now) {
		if e.Subject == call.Human {
			ev := lineslot.WorldEvent{ID: e.ID, At: e.At, Kind: e.Kind, Board: e.Board, Title: e.Title, By: e.By}
			events[e.ID] = ev
			evs = append(evs, ev)
		}
	}
	in := lineslot.Input{
		Host: host, From: windowFrom, To: now.Add(lineLead),
		Caller:    lineslot.Caller{Actor: call.Human, Known: known, Line: call.Line, Call: call.CallID, From: hc.Start},
		Residents: residents, Sent: sent, Recorded: recorded, Events: evs,
		Relation: func(a, b world.ActorRef) int { return r.chatRelation(host, nil, a, b) },
	}

	t := &r.lineTalk
	c := &r.chatTalk
	for _, s := range lineslot.Slots(in) {
		t.mu.Lock()
		done := t.started[s.Key]
		if !done && s.At.Before(now.Add(-lineStale)) {
			if t.started == nil {
				t.started = map[string]bool{}
			}
			t.started[s.Key] = true // Too late to send; the resident kept quiet.
			done = true
		}
		t.mu.Unlock()
		if done {
			continue
		}
		c.mu.Lock()
		if c.running == nil {
			c.running = map[string]int{}
		}
		full := c.running[host.ID] >= chatJobsPerHost
		if !full {
			c.running[host.ID]++
		}
		c.mu.Unlock()
		if full {
			break
		}
		t.mu.Lock()
		if t.started == nil {
			t.started = map[string]bool{}
		}
		t.started[s.Key] = true
		t.mu.Unlock()
		job := r.lineJobFor(host, call, in, s, events, all, personas)
		go r.runLineJob(writer, job)
	}
}

// lineJobFor gathers the materials of a slot's message.
func (r *Repository) lineJobFor(host world.Host, call world.LineMessageCall, in lineslot.Input, s lineslot.Slot, events map[string]lineslot.WorldEvent, all []world.LineMessage, personas map[string]world.Persona) lineJob {
	job := lineJob{host: host, call: call, slot: s, started: time.Now()}
	for _, res := range in.Residents {
		if res.Actor == s.Speaker && res.Line == s.Line && !s.At.Before(res.From) && s.At.Before(res.To) {
			job.stayTo = res.To
		}
	}
	req := llm.LineMessageRequest{
		WorldTime: s.At, Station: host.Name, Kind: string(s.Kind),
		Speaker:   llm.ChatLinePerson{Handle: s.Handle, Summary: personaSummary(personas[s.Speaker.ID])},
		Recipient: llm.ChatLinePerson{Handle: call.Login},
		Relation:  relationLabel(in.Relation(s.Speaker, in.Caller.Known)),
		MaxCells:  llm.LineMessageCells,
	}
	var turns []world.LineMessage
	for _, m := range all {
		fromMe := m.From == s.Speaker && m.To == call.Human
		toMe := m.To == s.Speaker && m.From == call.Human
		if (fromMe || toMe) && m.SentAt.Before(s.At) {
			turns = append(turns, m)
		}
		if m.ID == s.RespondsTo && s.RespondsTo != "" {
			req.Incoming = m.Body
			if m.FromHandle != "" {
				req.Recipient.Handle = m.FromHandle // As the resident saw it on the screen.
			}
			job.respondsTo = world.ChannelLineMessage + ":" + m.ID
		}
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].SentAt.Before(turns[j].SentAt) })
	if n := len(turns); n > lineHistory {
		turns = turns[n-lineHistory:]
	}
	for _, m := range turns {
		from := m.FromHandle
		if m.From == s.Speaker {
			from = s.Handle
		}
		req.History = append(req.History, llm.LineMessageTurn{From: from, Body: m.Body})
	}
	if e, ok := events[s.Event]; ok {
		req.Event = &llm.ChatLineEvent{Subject: call.Login, Kind: e.Kind, Board: e.Board, Title: e.Title, At: e.At}
		if !e.By.IsZero() {
			req.Event.By = r.handleOf(host, e.By)
			if e.By.Kind == world.ActorKindLogin {
				req.Event.By = e.By.ID
			}
		}
		job.respondsTo, _, _ = strings.Cut(e.ID, ">")
	}
	job.req = req
	return job
}

// runLineJob writes a slot's message and records it, or lets the resident
// keep quiet when writing fails.
func (r *Repository) runLineJob(writer llm.LineMessageWriter, job lineJob) {
	defer func() {
		c := &r.chatTalk
		c.mu.Lock()
		c.running[job.host.ID]--
		c.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), lineJobTimeout)
	defer cancel()
	var body string
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		body, err = writer.WriteLineMessage(ctx, job.req)
		if err == nil {
			body = llm.NormalizeLineMessage(body)
			err = llm.ValidateLineMessage(body, job.req)
		}
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		log.Printf("Line message generation failed: host=%s slot=%s err=%v", job.host.ID, job.slot.Key, err)
		return
	}
	if m, ok := r.recordLineMessage(job, body); ok {
		r.logLineMessageContent(job, m)
	}
}

// recordLineMessage saves a written message at its slot time, or now if that
// has passed, unless the slot is already recorded, the human has hung up, the
// resident has left or the message is too late.
func (r *Repository) recordLineMessage(job lineJob, body string) (world.LineMessage, bool) {
	records, ok := r.Base.(world.ChatRecordStore)
	ps, havePresence := r.presenceStore()
	if !ok || !havePresence {
		return world.LineMessage{}, false
	}
	lock := presenceLock(job.host.ID)
	lock.Lock()
	defer lock.Unlock()
	if _, live := ps.HumanCall(job.call.CallID); !live {
		return world.LineMessage{}, false
	}
	s := job.slot
	for _, m := range records.ListLineMessages(job.host.ID) {
		if m.Slot == s.Key {
			return world.LineMessage{}, false
		}
	}
	at := s.At
	if now := r.currentWorldTime(); now.After(at) {
		at = now
	}
	// A call may already have been handed the slot's time; the message comes
	// after it.
	r.lineTalk.mu.Lock()
	shown := r.lineTalk.shown[job.call.CallID]
	r.lineTalk.mu.Unlock()
	if !at.After(shown) {
		at = shown.Add(time.Nanosecond)
	}
	if at.Sub(s.At) > lineStale || (!job.stayTo.IsZero() && !at.Before(job.stayTo)) {
		return world.LineMessage{}, false
	}
	saved := records.SaveLineMessage(world.LineMessage{
		HostID: job.host.ID, From: s.Speaker, FromHandle: s.Handle, FromLine: s.Line, To: job.call.Human, ToLine: job.call.Line, Body: body,
		SentAt: at, DeliverAt: at, Slot: s.Key, RespondsTo: s.RespondsTo,
	})
	if err := r.appendLedger(world.InteractionEvent{
		EventID:           world.ChannelLineMessage + ":" + saved.ID,
		HostID:            job.host.ID,
		Channel:           world.ChannelLineMessage,
		Actor:             saved.From,
		AudienceScope:     world.PrivateScope(job.host.ID),
		AddressedTo:       []world.ActorRef{saved.To},
		AddressedLogins:   []string{job.call.Login},
		RespondsToEventID: job.respondsTo,
		ContentRef:        "line_message:" + saved.ID,
		OccurredAt:        saved.SentAt,
		SourceKind:        sourceKindOf(saved.From),
	}); err != nil {
		log.Printf("Line message ledger append failed: host=%s slot=%s err=%v", job.host.ID, s.Key, err)
	}
	return saved, true
}

// lineMessageLog is operational telemetry for a resident's line message,
// never a world fact (spec 018).
type lineMessageLog struct {
	Host       string `json:"host"`
	Slot       string `json:"slot"`
	Kind       string `json:"kind"`
	Speaker    string `json:"speaker"`
	Recipient  string `json:"recipient"`
	RespondsTo string `json:"responds_to,omitempty"`
	Body       string `json:"body"`
	At         string `json:"at"`
	ElapsedMS  int64  `json:"elapsed_ms"`
}

// logLineMessageContent logs a recorded resident message on hosts that opted
// in to the generated content log.
func (r *Repository) logLineMessageContent(job lineJob, m world.LineMessage) {
	if !r.shouldLogGeneratedContent(job.host) {
		return
	}
	encoded, err := json.Marshal(lineMessageLog{
		Host: job.host.ID, Slot: job.slot.Key, Kind: string(job.slot.Kind), Speaker: job.slot.Handle,
		Recipient: job.req.Recipient.Handle, RespondsTo: job.respondsTo, Body: m.Body, At: m.SentAt.Format(time.RFC3339),
		ElapsedMS: time.Since(job.started).Milliseconds(),
	})
	if err != nil {
		log.Printf("Line message audit encoding failed: host=%s slot=%s err=%v", job.host.ID, job.slot.Key, err)
		return
	}
	log.Printf("Line message generated content: %s", encoded)
}
