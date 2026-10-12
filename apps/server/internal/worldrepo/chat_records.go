package worldrepo

import (
	"errors"
	"sort"

	"zutto-pccom/apps/server/internal/presence"
	"zutto-pccom/apps/server/internal/world"
)

// Records of what callers say in chat rooms and send to lines (spec 016
// FR-006): saved as world history and entered in the interaction ledger.

var errNoChatRecords = errors.New("the store keeps no chat records")

// RecordChatUtterance saves what a caller said in a room and records it in the
// ledger, addressed to the residents in the room at u.At. Their plans become
// observed: the ledger now says they were there.
func (r *Repository) RecordChatUtterance(host world.Host, u world.ChatUtterance) (world.ChatUtterance, error) {
	records, ok := r.Base.(world.ChatRecordStore)
	if !ok {
		return world.ChatUtterance{}, errNoChatRecords
	}
	u.HostID = host.ID
	var audience []world.ActorRef
	if ps, ok := r.presenceStore(); ok {
		if rooms := r.chatRooms(host); len(rooms) > 0 {
			lock := presenceLock(host.ID)
			lock.Lock()
			ctx := r.gatherCalls(ps, host, u.At)
			var present []world.CallActivity
			observed := presence.Result{Profiles: map[string]presence.Profile{}}
			sessions := map[string]world.PresenceSession{}
			for _, s := range ctx.sessions {
				sessions[world.CallActivity{Actor: s.Actor, Start: s.Start}.Key()] = s
			}
			for key, a := range r.plans(host, rooms, ctx) {
				seg, in := a.SegmentAt(u.At)
				if !in || seg.State != world.CallChat || seg.Room != u.Room || a.Actor == u.Actor {
					continue
				}
				present = append(present, a)
				if s, ok := sessions[key]; ok {
					observed.Sessions = append(observed.Sessions, s)
					if prof, ok := ctx.profiles[s.Actor.ID]; ok {
						observed.Profiles[s.Actor.ID] = prof
					}
				}
			}
			if len(present) > 0 {
				r.observePresence(ps, host, observed, u.At, present...)
			}
			lock.Unlock()
			for _, a := range present {
				audience = append(audience, a.Actor)
			}
			sort.Slice(audience, func(i, j int) bool { return audience[i].String() < audience[j].String() })
		}
	}
	saved := records.SaveChatUtterance(u)
	err := r.appendLedger(world.InteractionEvent{
		EventID:       world.ChannelChatUtterance + ":" + saved.ID,
		HostID:        host.ID,
		Channel:       world.ChannelChatUtterance,
		Actor:         saved.Actor,
		AudienceScope: world.ChatRoomScope(host.ID, saved.Room),
		AddressedTo:   audience,
		ContentRef:    "chat:" + saved.ID,
		OccurredAt:    saved.At,
		SourceKind:    sourceKindOf(saved.Actor),
	})
	return saved, err
}

// SendLineMessage saves a line message and records it in the ledger, addressed
// to its recipient.
func (r *Repository) SendLineMessage(host world.Host, m world.LineMessage) (world.LineMessage, error) {
	records, ok := r.Base.(world.ChatRecordStore)
	if !ok {
		return world.LineMessage{}, errNoChatRecords
	}
	m.HostID = host.ID
	saved := records.SaveLineMessage(m)
	err := r.appendLedger(world.InteractionEvent{
		EventID:       world.ChannelLineMessage + ":" + saved.ID,
		HostID:        host.ID,
		Channel:       world.ChannelLineMessage,
		Actor:         saved.From,
		AudienceScope: world.PrivateScope(host.ID),
		AddressedTo:   []world.ActorRef{saved.To},
		ContentRef:    "line_message:" + saved.ID,
		OccurredAt:    saved.SentAt,
		SourceKind:    sourceKindOf(saved.From),
	})
	return saved, err
}

func (r *Repository) appendLedger(event world.InteractionEvent) error {
	ledger, ok := r.Base.(world.InteractionLedgerStore)
	if !ok {
		return nil
	}
	return ledger.AppendInteractionEvent(event)
}

func sourceKindOf(actor world.ActorRef) string {
	if actor.Kind == world.ActorKindMember {
		return "human_authored"
	}
	return "npc_world_selected"
}
