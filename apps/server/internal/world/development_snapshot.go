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
	Posts         []Post                   `json:"posts"`
	Personas      []Persona                `json:"personas"`
	PersonaFacts  map[string][]PersonaFact `json:"persona_facts"`
	Memberships   []string                 `json:"memberships"`
	NextPostID    int64                    `json:"next_post_id"`
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

	return DevelopmentHostSnapshot{
		SchemaVersion: DevelopmentHostSnapshotSchemaVersion,
		Host:          host,
		Boards:        boards,
		Posts:         posts,
		Personas:      personas,
		PersonaFacts:  facts,
		Memberships:   memberships,
		NextPostID:    s.next,
	}, true
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
	return nil
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
	post.Intent.Claims = append([]string(nil), post.Intent.Claims...)
	post.Intent.RespondsToClaims = append([]string(nil), post.Intent.RespondsToClaims...)
	post.Intent.RenderContext = ""
	return post
}
