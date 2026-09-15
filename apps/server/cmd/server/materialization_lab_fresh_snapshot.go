package main

import "zutto-pccom/apps/server/internal/world"

// snapshotForFreshWorld captures the development host even when it has
// no producer posts yet. Fresh Lab is responsible for creating a new
// isolated conversation, so unlike worker replay it must not require an
// already-materialized producer event.
func (l *materializationLab) snapshotForFreshWorld(phone string) (world.DevelopmentHostSnapshot, error) {
	host, err := l.store.HostByPhone(phone)
	if err != nil {
		return world.DevelopmentHostSnapshot{}, err
	}
	boards := append([]world.Board(nil), l.store.ListBoards(host.ID)...)
	posts := clonePostsForLab(l.store.ListPosts(host.ID))
	personas := append([]world.Persona(nil), l.store.ListHostPersonas(host.ID)...)
	facts := map[string][]world.PersonaFact{}
	memberships := make([]string, 0, len(personas))
	for _, p := range personas {
		memberships = append(memberships, p.ID)
		if pf := l.store.ListPersonaFacts(p.ID); len(pf) > 0 {
			facts[p.ID] = append([]world.PersonaFact(nil), pf...)
		}
	}
	maxID := int64(0)
	for _, p := range posts {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	return world.DevelopmentHostSnapshot{
		SchemaVersion: world.DevelopmentHostSnapshotSchemaVersion,
		Host:          host,
		Boards:        boards,
		Posts:         posts,
		Personas:      personas,
		PersonaFacts:  facts,
		Memberships:   memberships,
		NextPostID:    maxID,
	}, nil
}
