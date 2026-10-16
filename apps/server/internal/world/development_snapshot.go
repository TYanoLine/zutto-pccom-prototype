package world

import "errors"

const DevelopmentHostSnapshotSchemaVersion = 1

// DevelopmentHostSnapshot is a compact, development-only serialization of one
// host's canonical materialized state. It exists so the PoC can survive process
// restarts without turning MemoryStore itself into a production persistence
// implementation.
type DevelopmentHostSnapshot struct {
	SchemaVersion int                      `json:"schema_version"`
	Host          Host                     `json:"host"`
	Boards        []Board                  `json:"boards"`
	BoardActivity []BoardActivityState     `json:"board_activity,omitempty"`
	ReactionPlans []ReactionPlan           `json:"reaction_plans,omitempty"`
	Posts         []Post                   `json:"posts"`
	Personas      []Persona                `json:"personas"`
	PersonaFacts  map[string][]PersonaFact `json:"persona_facts"`
	Memberships   []string                 `json:"memberships"`
	NextPostID    int64                    `json:"next_post_id"`
	// Presence holds the observed connection sessions (spec 015). Older
	// snapshots have none.
	Presence []PresenceSession `json:"presence,omitempty"`
	// CallActivities holds the observed plans of resident calls (spec 016).
	// Older snapshots have none.
	CallActivities []CallActivity `json:"call_activities,omitempty"`
	// ChatUtterances and LineMessages are what callers said in chat rooms and
	// sent to lines (spec 016). Older snapshots have none.
	ChatUtterances []ChatUtterance `json:"chat_utterances,omitempty"`
	LineMessages   []LineMessage   `json:"line_messages,omitempty"`
}

// DevelopmentSnapshot returns a detached copy of the state needed to reproduce
// future consequences for one development host.
func (s *MemoryStore) DevelopmentSnapshot(phone string) (DevelopmentHostSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, ok := s.hosts[phone]
	if !ok {
		return DevelopmentHostSnapshot{}, false
	}

	memberships := append([]string(nil), s.memberships[host.ID]...)
	personas := make([]Persona, 0, len(memberships))
	facts := make(map[string][]PersonaFact, len(memberships))
	for _, id := range memberships {
		if persona, exists := s.personas[id]; exists {
			personas = append(personas, clonePersona(persona))
		}
		if personaFacts := s.personaFacts[id]; len(personaFacts) > 0 {
			facts[id] = append([]PersonaFact(nil), personaFacts...)
		}
	}

	boards := append([]Board(nil), s.boards[host.ID]...)
	posts := make([]Post, len(s.posts[host.ID]))
	for i, post := range s.posts[host.ID] {
		posts[i] = clonePost(post)
	}

	snapshot := DevelopmentHostSnapshot{
		SchemaVersion: DevelopmentHostSnapshotSchemaVersion,
		Host:          host,
		Boards:        boards,
		BoardActivity: s.listBoardActivityStatesLocked(host.ID),
		ReactionPlans: s.listReactionPlansLocked(host.ID),
		Posts:         posts,
		Personas:      personas,
		PersonaFacts:  facts,
		Memberships:   memberships,
		NextPostID:    s.next,
		Presence:      append([]PresenceSession(nil), s.presence[host.ID]...),
	}
	snapshot.CallActivities = cloneCallActivities(s.callPlans[host.ID])
	snapshot.ChatUtterances = append([]ChatUtterance(nil), s.chatLines[host.ID]...)
	snapshot.LineMessages = append([]LineMessage(nil), s.lineMessages[host.ID]...)
	return snapshot, true
}

// RestoreDevelopmentSnapshot replaces only the addressed host's materialized
// state. Other fixture hosts in MemoryStore remain untouched.
func (s *MemoryStore) RestoreDevelopmentSnapshot(snapshot DevelopmentHostSnapshot) error {
	if snapshot.SchemaVersion != DevelopmentHostSnapshotSchemaVersion {
		return errors.New("unsupported development host snapshot schema")
	}
	if snapshot.Host.ID == "" || snapshot.Host.Phone == "" {
		return errors.New("development host snapshot is missing host identity")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hostID := snapshot.Host.ID
	s.hosts[snapshot.Host.Phone] = snapshot.Host
	s.boards[hostID] = append([]Board(nil), snapshot.Boards...)
	s.boardActivity[hostID] = map[string]BoardActivityState{}
	for _, state := range snapshot.BoardActivity {
		s.boardActivity[hostID][state.BoardID] = state
	}
	s.reactionPlans[hostID] = map[int64]ReactionPlan{}
	for _, plan := range snapshot.ReactionPlans {
		s.reactionPlans[hostID][plan.TargetPostID] = cloneReactionPlan(plan)
	}

	posts := make([]Post, len(snapshot.Posts))
	maxPostID := snapshot.NextPostID
	for i, post := range snapshot.Posts {
		posts[i] = clonePost(post)
		if post.ID > maxPostID {
			maxPostID = post.ID
		}
	}
	s.posts[hostID] = posts

	// Remove the old membership-owned persona state for this host before restore.
	for _, id := range s.memberships[hostID] {
		delete(s.personaFacts, id)
		delete(s.personas, id)
	}
	s.memberships[hostID] = append([]string(nil), snapshot.Memberships...)
	for _, persona := range snapshot.Personas {
		s.personas[persona.ID] = clonePersona(persona)
	}
	for id, personaFacts := range snapshot.PersonaFacts {
		s.personaFacts[id] = append([]PersonaFact(nil), personaFacts...)
	}
	if maxPostID > s.next {
		s.next = maxPostID
	}
	s.presence[hostID] = append([]PresenceSession(nil), snapshot.Presence...)
	s.callPlans[hostID] = cloneCallActivities(snapshot.CallActivities)
	s.chatLines[hostID] = append([]ChatUtterance(nil), snapshot.ChatUtterances...)
	s.lineMessages[hostID] = append([]LineMessage(nil), snapshot.LineMessages...)
	return nil
}

func (s *MemoryStore) listBoardActivityStatesLocked(hostID string) []BoardActivityState {
	states := s.boardActivity[hostID]
	out := make([]BoardActivityState, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	return out
}

func (s *MemoryStore) listReactionPlansLocked(hostID string) []ReactionPlan {
	plans := s.reactionPlans[hostID]
	if len(plans) == 0 {
		return nil
	}
	out := make([]ReactionPlan, 0, len(plans))
	for _, plan := range plans {
		out = append(out, cloneReactionPlan(plan))
	}
	return out
}

func clonePersona(persona Persona) Persona {
	persona.EverydayContext = append([]string(nil), persona.EverydayContext...)
	persona.Interests = cloneFloatMap(persona.Interests)
	persona.Opinions = cloneFloatMap(persona.Opinions)
	return persona
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	if in == nil {
		return nil
	}
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func clonePost(post Post) Post {
	post.Intent.Topics = append([]string(nil), post.Intent.Topics...)
	post.Intent.SituationFacts = append([]string(nil), post.Intent.SituationFacts...)
	post.Intent.Claims = append([]string(nil), post.Intent.Claims...)
	post.Intent.RespondsToClaims = append([]string(nil), post.Intent.RespondsToClaims...)
	post.Intent.ProducerReferents = append([]string(nil), post.Intent.ProducerReferents...)
	post.Intent.ProducerActorKnowledge = append([]string(nil), post.Intent.ProducerActorKnowledge...)
	post.Intent.ProducerAudienceContext = append([]string(nil), post.Intent.ProducerAudienceContext...)
	post.Intent.ProducerContribution = append([]string(nil), post.Intent.ProducerContribution...)
	post.Intent.ProducerMustNot = append([]string(nil), post.Intent.ProducerMustNot...)
	post.Intent.RenderContext = ""
	return post
}
