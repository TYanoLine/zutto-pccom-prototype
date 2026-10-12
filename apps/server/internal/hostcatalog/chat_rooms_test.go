package hostcatalog

import "testing"

func TestChatRoomsAreParsed(t *testing.T) {
	p, err := parseErikaKPreset(t, "erika-k", `
detail:
  chat_rooms:
    - { number: 0, capacity: 0, weight: 0.7 }
    - { number: 1, capacity: 4, weight: 0.3 }
`)
	if err != nil {
		t.Fatalf("valid chat rooms were rejected: %v", err)
	}
	rooms := p.Detail.ChatRooms
	if len(rooms) != 2 || rooms[0] != (ChatRoom{Number: 0, Capacity: 0, Weight: 0.7}) || rooms[1] != (ChatRoom{Number: 1, Capacity: 4, Weight: 0.3}) {
		t.Fatalf("rooms = %+v", rooms)
	}
}

func TestChatRoomsAreOptional(t *testing.T) {
	p, err := parseErikaKPreset(t, "turbobbs", "")
	if err != nil {
		t.Fatalf("a preset without chat rooms was rejected: %v", err)
	}
	if len(p.Detail.ChatRooms) != 0 {
		t.Fatalf("rooms = %+v, want none", p.Detail.ChatRooms)
	}
}

func TestChatRoomsAreValidated(t *testing.T) {
	cases := map[string]struct {
		rooms string
		want  string
	}{
		"negative number":   {"[{ number: -1, weight: 1 }]", "number: -1 must not be negative"},
		"duplicate number":  {"[{ number: 2, weight: 1 }, { number: 2, weight: 1 }]", "number: 2 is listed twice"},
		"negative capacity": {"[{ number: 0, capacity: -3, weight: 1 }]", "capacity: -3 must not be negative"},
		"negative weight":   {"[{ number: 0, weight: -1 }, { number: 1, weight: 2 }]", "weight: -1 must be a non-negative number"},
		"zero total weight": {"[{ number: 0, weight: 0 }, { number: 1, weight: 0 }]", "the weights must add up to more than 0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			requireErikaKError(t, "detail:\n  chat_rooms: "+tc.rooms+"\n", "detail.chat_rooms", tc.want)
		})
	}
}
