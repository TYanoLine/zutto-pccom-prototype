package world

// GuestLoginID is the shared login ID of guests. Residents cannot tell guests
// apart, so a guest is never someone they know (spec 019).
const GuestLoginID = "GUEST"

// KnownAs is a person as residents know them: a resident as itself, a human by
// the login ID shown on the record. A guest, or a human without a login ID, is
// the zero ActorRef: nobody residents can know.
func KnownAs(actor ActorRef, login string) ActorRef {
	switch actor.Kind {
	case ActorKindPersona, ActorKindLogin:
		return actor
	case ActorKindMember:
		if login == "" || login == GuestLoginID {
			return ActorRef{}
		}
		return ActorRef{Kind: ActorKindLogin, ID: login}
	}
	return ActorRef{}
}

// PostLogin is the login ID a human post was written under. Posts written
// before spec 019 hold it in AuthorMemberID or Author only.
func PostLogin(p Post) string {
	if p.AuthorMemberID != "" {
		return p.AuthorMemberID
	}
	return p.Author
}

// PostActor names a post's author for the system: a resident by persona ID, a
// human by human ID. A post written before spec 019 has no human ID and keeps
// the login ID as the member ID, as the interaction ledger always recorded it.
func PostActor(p Post) ActorRef {
	switch {
	case p.AuthorPersonaID != "":
		return ActorRef{Kind: ActorKindPersona, ID: p.AuthorPersonaID}
	case p.AuthorHumanID != "":
		return ActorRef{Kind: ActorKindMember, ID: p.AuthorHumanID}
	}
	if login := PostLogin(p); login != "" {
		return ActorRef{Kind: ActorKindMember, ID: login}
	}
	return ActorRef{}
}

// PostKnownAs names a post's author as residents know them.
func PostKnownAs(p Post) ActorRef {
	if p.AuthorPersonaID != "" {
		return ActorRef{Kind: ActorKindPersona, ID: p.AuthorPersonaID}
	}
	return KnownAs(ActorRef{Kind: ActorKindMember, ID: "-"}, PostLogin(p))
}

// knownLogin names a human actor of a ledger event as residents know them.
// An event recorded without a login ID names its human by login ID already.
func knownLogin(actor ActorRef, login string) ActorRef {
	if actor.Kind == ActorKindMember && login == "" {
		login = actor.ID
	}
	return KnownAs(actor, login)
}

// KnownActor is the event's actor as residents know them.
func (e InteractionEvent) KnownActor() ActorRef { return knownLogin(e.Actor, e.ActorLogin) }

// KnownAddressed is the i-th addressee as residents know them.
func (e InteractionEvent) KnownAddressed(i int) ActorRef {
	if i < 0 || i >= len(e.AddressedTo) {
		return ActorRef{}
	}
	login := ""
	if i < len(e.AddressedLogins) {
		login = e.AddressedLogins[i]
	}
	return knownLogin(e.AddressedTo[i], login)
}
