package worldrepo

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"zutto-pccom/apps/server/internal/chatslot"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

// Residents' lines in chat rooms (spec 017). The World decides who speaks,
// when and to what (internal/chatslot); this file keeps who is watching each
// room, writes the decided lines through an optional generator and records
// them. Lines are written only while a caller is in the room.

// staleViewer is how long a caller's stay in a room stays open without a
// ChatEvents call for that room. A dropped line never says it left the room;
// with nobody polling nothing is written anyway.
const staleViewer = time.Minute

// chatTalk is the repository's spec 017 state.
type chatTalk struct {
	mu sync.Mutex
	// viewers are the callers' stays, by host and room.
	viewers map[viewerRoom][]viewerStay
	writer  llm.ChatLineWriter
	// started holds the slots whose lines were started or given up, and
	// running counts writer calls per host.
	started map[string]bool
	running map[string]int
	// shown is the end of the last window ChatEvents returned, by host and
	// room; a line is never recorded at or before it.
	shown map[viewerRoom]time.Time
}

type viewerRoom struct {
	host string
	room int
}

// viewerStay is a caller in a room from From until To (zero while open).
// Seen is the last world time the room was polled during the stay.
type viewerStay struct {
	Actor    world.ActorRef
	Handle   string
	From, To time.Time
	Seen     time.Time
}

// EnterChatRoom records that a caller entered a room at a world time.
func (r *Repository) EnterChatRoom(host world.Host, room int, actor world.ActorRef, handle string, at time.Time) {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.viewers == nil {
		t.viewers = map[viewerRoom][]viewerStay{}
	}
	key := viewerRoom{host.ID, room}
	stays := closeStale(t.viewers[key], at)
	for i, s := range stays {
		if s.Actor == actor && s.To.IsZero() {
			// Entering again without leaving closes the earlier stay.
			stays[i].To = at
		}
	}
	t.viewers[key] = append(stays, viewerStay{Actor: actor, Handle: handle, From: at, Seen: at})
}

// LeaveChatRoom records that a caller left a room at a world time.
func (r *Repository) LeaveChatRoom(host world.Host, room int, actor world.ActorRef, at time.Time) {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	key := viewerRoom{host.ID, room}
	stays := t.viewers[key]
	for i, s := range stays {
		if s.Actor == actor && s.To.IsZero() {
			stays[i].To = at
		}
	}
}

// watchRoom marks room as polled at now and returns the callers' stays in it
// that overlap [since, now], oldest first. A stay unpolled for staleViewer is
// closed at its last poll.
func (r *Repository) watchRoom(host world.Host, room int, since, now time.Time) []viewerStay {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	key := viewerRoom{host.ID, room}
	if len(t.viewers[key]) == 0 {
		return nil
	}
	stays := closeStale(t.viewers[key], now)
	var out []viewerStay
	for i, s := range stays {
		if s.To.IsZero() && now.After(s.Seen) {
			stays[i].Seen = now
			s.Seen = now
		}
		if s.From.After(now) || (!s.To.IsZero() && s.To.Before(since)) {
			continue
		}
		out = append(out, s)
	}
	t.viewers[key] = stays
	sort.SliceStable(out, func(i, j int) bool { return out[i].From.Before(out[j].From) })
	return out
}

// closeStale closes the open stays last polled more than staleViewer before
// now, at their last poll.
func closeStale(stays []viewerStay, now time.Time) []viewerStay {
	for i, s := range stays {
		if s.To.IsZero() && now.Sub(s.Seen) > staleViewer {
			stays[i].To = s.Seen
		}
	}
	return stays
}

// Timing of line writing. Fictional tuning values.
const (
	// chatLead is how long before its time a slot's line starts being
	// written, so a typical generation finishes on time.
	chatLead = 20 * time.Second
	// chatStale is how late a slot's line may still be shown.
	chatStale = time.Minute
	// chatContext bounds how far back slots are computed on each poll.
	chatContext = 10 * time.Minute
	// chatTranscript is how many recent lines the writer sees.
	chatTranscript = 12
	// chatJobTimeout bounds one line's writing, retry included.
	chatJobTimeout = 20 * time.Second
	// chatJobsPerHost bounds concurrent writer calls per host.
	chatJobsPerHost = 2
	// chatLedgerScan is how many recent ledger events news looks through.
	chatLedgerScan = 500
)

// SetChatLineWriter gives residents words in chat rooms. Without a writer
// residents never speak.
func (r *Repository) SetChatLineWriter(w llm.ChatLineWriter) {
	t := &r.chatTalk
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writer = w
}

// chatJob is one slot's line being written.
type chatJob struct {
	host     world.Host
	slot     chatslot.Slot
	req      llm.ChatLineRequest
	stayTo   time.Time
	audience []world.ActorRef
	// respondsTo is the ledger event the line answers.
	respondsTo string
	started    time.Time
}

// chatLines returns the recorded lines of room in (from, to] as say events
// and starts writing the residents' lines that are due. It also returns the
// resident plans a caller watched in the room. The caller holds the host's
// presence lock.
func (r *Repository) chatLines(host world.Host, room int, from, to time.Time, plans map[string]world.CallActivity, sessions map[string]world.PresenceSession) ([]world.ChatEvent, []world.CallActivity) {
	records, ok := r.Base.(world.ChatRecordStore)
	if !ok {
		return nil, nil
	}
	var says []world.ChatEvent
	for _, u := range records.ListChatUtterances(host.ID, room, from, to.Add(time.Nanosecond)) {
		if u.At.After(from) {
			says = append(says, world.ChatEvent{At: u.At, Kind: world.ChatSay, Room: room, Actor: u.Actor, Handle: u.Handle, Body: u.Body, UtteranceID: u.ID})
		}
	}

	viewers := r.watchRoom(host, room, to.Add(-chatContext), to)
	t := &r.chatTalk
	t.mu.Lock()
	writer := t.writer
	if t.shown == nil {
		t.shown = map[viewerRoom]time.Time{}
	}
	if key := (viewerRoom{host.ID, room}); to.After(t.shown[key]) {
		t.shown[key] = to
	}
	t.mu.Unlock()
	if len(viewers) == 0 {
		return says, nil
	}

	windowFrom := viewers[0].From.Add(-time.Second)
	if c := to.Add(-chatContext); c.After(windowFrom) {
		windowFrom = c
	}
	personas := map[string]world.Persona{}
	if store, ok := r.Base.(world.PersonaStore); ok {
		for _, p := range store.ListHostPersonas(host.ID) {
			personas[p.ID] = p
		}
	}
	var stays []chatslot.ResidentStay
	var watched []world.CallActivity
	for key, a := range plans {
		inRoom := false
		for _, seg := range a.Segments {
			if seg.State != world.CallChat || seg.Room != room || !seg.To.After(windowFrom) || seg.From.After(to.Add(chatLead)) {
				continue
			}
			handle := sessions[key].Handle
			if handle == "" {
				handle = r.handleOf(host, a.Actor)
			}
			stays = append(stays, chatslot.ResidentStay{Actor: a.Actor, Handle: handle, Persona: personas[a.Actor.ID], Call: key, From: seg.From, To: seg.To})
			for _, v := range viewers {
				if seg.From.Before(to) && (v.To.IsZero() || seg.From.Before(v.To)) && seg.To.After(v.From) {
					inRoom = true
				}
			}
		}
		if inRoom {
			watched = append(watched, a)
		}
	}
	sort.Slice(stays, func(i, j int) bool {
		if stays[i].Call != stays[j].Call {
			return stays[i].Call < stays[j].Call
		}
		return stays[i].From.Before(stays[j].From)
	})
	if writer == nil {
		return says, watched
	}

	var callers []chatslot.Stay
	for _, v := range viewers {
		callers = append(callers, chatslot.Stay{Actor: v.Actor, Handle: v.Handle, From: v.From, To: v.To})
	}
	utterances := records.ListChatUtterances(host.ID, room, windowFrom.Add(-chatContext), time.Time{})
	known := viewerKnown(viewers)
	in := chatslot.Input{
		Host: host, Room: room, From: windowFrom, To: to.Add(chatLead),
		Viewers: callers, Residents: stays, Utterances: utterances,
		Relation: func(a, b world.ActorRef) int { return r.chatRelation(host, known, a, b) },
		Events:   r.chatWorldEvents(host, known, to),
	}
	events := map[string]chatslot.WorldEvent{}
	for _, e := range in.Events {
		events[e.ID] = e
	}
	handles := map[world.ActorRef]string{}
	for _, s := range stays {
		handles[s.Actor] = s.Handle
	}
	for _, v := range viewers {
		handles[v.Actor] = v.Handle
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started == nil {
		t.started = map[string]bool{}
		t.running = map[string]int{}
	}
	for _, s := range chatslot.Slots(in) {
		if t.started[s.Key] {
			continue
		}
		if s.At.Before(to.Add(-chatStale)) {
			t.started[s.Key] = true // Too late to say; the resident kept quiet.
			continue
		}
		if t.running[host.ID] >= chatJobsPerHost {
			break
		}
		job := r.chatJobFor(host, in, s, handles, personas, events, utterances, to)
		t.started[s.Key] = true
		t.running[host.ID]++
		go r.runChatJob(writer, job)
	}
	return says, watched
}

// chatJobFor gathers the materials of a slot's line.
func (r *Repository) chatJobFor(host world.Host, in chatslot.Input, s chatslot.Slot, handles map[world.ActorRef]string, personas map[string]world.Persona, events map[string]chatslot.WorldEvent, utterances []world.ChatUtterance, now time.Time) chatJob {
	job := chatJob{host: host, slot: s, started: time.Now()}
	req := llm.ChatLineRequest{
		WorldTime: s.At, Station: host.Name, Room: s.Room, Kind: string(s.Kind),
		Speaker:  llm.ChatLinePerson{Handle: s.Handle, Summary: personaSummary(personas[s.Speaker.ID])},
		MaxCells: llm.DefaultChatLineCells,
	}
	if !s.Addressee.IsZero() {
		req.Addressee = &llm.ChatLinePerson{Handle: handles[s.Addressee]}
		if p, ok := personas[s.Addressee.ID]; ok && s.Addressee.Kind == world.ActorKindPersona {
			req.Addressee.Summary = personaSummary(p)
		}
		req.Relation = relationLabel(in.Relation(s.Speaker, s.Addressee))
	}
	since := time.Time{}
	for _, st := range in.Residents {
		if st.Actor == s.Speaker && !s.At.Before(st.From) && s.At.Before(st.To) {
			since = st.From
			job.stayTo = st.To
		}
	}
	for _, v := range in.Viewers {
		if since.Before(v.From) && !s.At.Before(v.From) {
			since = v.From
		}
	}
	for _, st := range in.Residents {
		if !s.At.Before(st.From) && s.At.Before(st.To) {
			req.Present = append(req.Present, st.Handle)
			if st.Actor != s.Speaker {
				job.audience = append(job.audience, st.Actor)
			}
		}
	}
	for _, v := range in.Viewers {
		if !s.At.Before(v.From) && (v.To.IsZero() || s.At.Before(v.To)) {
			req.Present = append(req.Present, v.Handle)
			job.audience = append(job.audience, v.Actor)
		}
	}
	for _, u := range utterances {
		if !u.At.Before(since) && !u.At.After(now) {
			req.Transcript = append(req.Transcript, llm.ChatLineTurn{Handle: u.Handle, Body: u.Body})
		}
	}
	if n := len(req.Transcript); n > chatTranscript {
		req.Transcript = req.Transcript[n-chatTranscript:]
	}
	if s.RespondsTo != "" {
		job.respondsTo = world.ChannelChatUtterance + ":" + s.RespondsTo
	}
	if e, ok := events[s.Event]; ok {
		req.Event = &llm.ChatLineEvent{Subject: handles[e.Subject], Kind: e.Kind, Board: e.Board, Title: e.Title, At: e.At}
		if !e.By.IsZero() {
			req.Event.By = handles[e.By]
			if e.By.Kind == world.ActorKindLogin {
				req.Event.By = e.By.ID // Residents know a caller by the login ID shown.
			} else if req.Event.By == "" {
				req.Event.By = r.handleOf(host, e.By)
			}
		}
		job.respondsTo, _, _ = strings.Cut(e.ID, ">")
	}
	if !s.Addressee.IsZero() {
		job.audience = []world.ActorRef{s.Addressee}
	}
	job.req = req
	return job
}

// runChatJob writes a slot's line and records it, or lets the resident keep
// quiet when writing fails.
func (r *Repository) runChatJob(writer llm.ChatLineWriter, job chatJob) {
	defer func() {
		t := &r.chatTalk
		t.mu.Lock()
		t.running[job.host.ID]--
		t.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), chatJobTimeout)
	defer cancel()
	var line string
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		line, err = writer.WriteChatLine(ctx, job.req)
		if err == nil {
			line = llm.NormalizeChatLine(line)
			err = llm.ValidateChatLine(line, job.req)
		}
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		log.Printf("Chat line generation failed: host=%s slot=%s err=%v", job.host.ID, job.slot.Key, err)
		return
	}
	if u, ok := r.recordResidentLine(job, line); ok {
		r.logChatGeneratedContent(job, u)
	}
}

// recordResidentLine saves a written line at its slot time, or now if that
// has passed, unless the slot is already recorded, the resident has left or
// the line is too late.
func (r *Repository) recordResidentLine(job chatJob, line string) (world.ChatUtterance, bool) {
	records, ok := r.Base.(world.ChatRecordStore)
	if !ok {
		return world.ChatUtterance{}, false
	}
	lock := presenceLock(job.host.ID)
	lock.Lock()
	defer lock.Unlock()
	s := job.slot
	for _, u := range records.ListChatUtterances(job.host.ID, s.Room, s.At.Add(-chatStale), time.Time{}) {
		if u.Slot == s.Key {
			return world.ChatUtterance{}, false
		}
	}
	at := s.At
	if now := r.currentWorldTime(); now.After(at) {
		at = now
	}
	// A poll may already have shown the slot's time; the line comes after it.
	r.chatTalk.mu.Lock()
	shown := r.chatTalk.shown[viewerRoom{job.host.ID, s.Room}]
	r.chatTalk.mu.Unlock()
	if !at.After(shown) {
		at = shown.Add(time.Nanosecond)
	}
	if at.Sub(s.At) > chatStale || (!job.stayTo.IsZero() && !at.Before(job.stayTo)) {
		return world.ChatUtterance{}, false
	}
	saved := records.SaveChatUtterance(world.ChatUtterance{
		HostID: job.host.ID, Room: s.Room, Actor: s.Speaker, Handle: s.Handle, Body: line, At: at,
		Slot: s.Key, RespondsTo: s.RespondsTo,
	})
	audience := append([]world.ActorRef(nil), job.audience...)
	sort.Slice(audience, func(i, j int) bool { return audience[i].String() < audience[j].String() })
	if err := r.appendLedger(world.InteractionEvent{
		EventID:           world.ChannelChatUtterance + ":" + saved.ID,
		HostID:            job.host.ID,
		Channel:           world.ChannelChatUtterance,
		Actor:             saved.Actor,
		AudienceScope:     world.ChatRoomScope(job.host.ID, saved.Room),
		AddressedTo:       audience,
		RespondsToEventID: job.respondsTo,
		ContentRef:        "chat:" + saved.ID,
		OccurredAt:        saved.At,
		SourceKind:        sourceKindOf(saved.Actor),
	}); err != nil {
		log.Printf("Chat line ledger append failed: host=%s slot=%s err=%v", job.host.ID, s.Key, err)
	}
	return saved, true
}

// viewerKnown maps each caller in the room to the person residents know them
// as: the login ID they showed on entering (spec 019). A guest maps to nobody.
func viewerKnown(viewers []viewerStay) map[world.ActorRef]world.ActorRef {
	out := map[world.ActorRef]world.ActorRef{}
	for _, v := range viewers {
		out[v.Actor] = world.KnownAs(v.Actor, v.Handle)
	}
	return out
}

// chatRelation is spec 013's measure between two people: 0 strangers, 1 have
// talked before, 2 regulars, counted from board replies both ways. People are
// the ones residents know; a caller who is nobody they know is a stranger.
func (r *Repository) chatRelation(host world.Host, known map[world.ActorRef]world.ActorRef, a, b world.ActorRef) int {
	rel, ok := r.Base.(world.RelationshipStore)
	if !ok {
		return 0
	}
	if k, ok := known[a]; ok {
		a = k
	}
	if k, ok := known[b]; ok {
		b = k
	}
	if a.IsZero() || b.IsZero() {
		return 0
	}
	n := 0
	if e, ok := rel.GetRelationship(host.ID, a, b); ok {
		n += e.BoardReplyCount
	}
	if e, ok := rel.GetRelationship(host.ID, b, a); ok {
		n += e.BoardReplyCount
	}
	return min(n, 2)
}

func relationLabel(n int) string {
	switch {
	case n >= 2:
		return "常連仲間"
	case n == 1:
		return "以前にもやり取り"
	}
	return "初対面"
}

// chatWorldEvents lists recent board events about people: what they posted
// and the replies their posts received. Reading headers never materializes a
// post body.
func (r *Repository) chatWorldEvents(host world.Host, known map[world.ActorRef]world.ActorRef, now time.Time) []chatslot.WorldEvent {
	ledger, ok := r.Base.(world.InteractionLedgerStore)
	if !ok {
		return nil
	}
	cutoff := now.Add(-chatslot.NewsWindow)
	var recent []world.InteractionEvent
	for _, e := range ledger.ListInteractionEvents(host.ID, chatLedgerScan) {
		if (e.Channel == "board_root" || e.Channel == "board_reply") && !e.OccurredAt.Before(cutoff) && !e.OccurredAt.After(now) {
			recent = append(recent, e)
		}
	}
	if len(recent) == 0 {
		return nil
	}
	posts := map[string]world.Post{}
	for _, p := range r.Base.ListPosts(host.ID) {
		posts[strconv.FormatInt(p.ID, 10)] = p
	}
	boards := map[string]string{}
	if store, ok := r.Base.(world.BoardStore); ok {
		for _, b := range store.ListBoards(host.ID) {
			boards[b.ID] = b.Name
		}
	}
	// subjects are the callers in the room whom residents know as person, or
	// the resident itself. A guest, or someone not in the room, has none.
	subjects := func(person world.ActorRef) []world.ActorRef {
		switch {
		case person.IsZero():
			return nil
		case person.Kind == world.ActorKindPersona:
			return []world.ActorRef{person}
		}
		var out []world.ActorRef
		for viewer, k := range known {
			if k == person {
				out = append(out, viewer)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
		return out
	}
	var out []chatslot.WorldEvent
	for _, e := range recent {
		p := posts[e.ContentRef]
		board := boards[p.BoardID]
		if board == "" {
			board = p.BoardID
		}
		base := chatslot.WorldEvent{At: e.OccurredAt, Board: board, Title: p.Subject}
		by := e.KnownActor()
		for _, subject := range subjects(by) {
			posted := base
			posted.ID, posted.Subject, posted.Kind = e.EventID, subject, "posted"
			out = append(out, posted)
		}
		for i := range e.AddressedTo {
			for _, subject := range subjects(e.KnownAddressed(i)) {
				replied := base
				replied.ID, replied.Subject, replied.Kind, replied.By = fmt.Sprintf("%s>%d", e.EventID, i), subject, "replied_to", by
				out = append(out, replied)
			}
		}
	}
	return out
}
