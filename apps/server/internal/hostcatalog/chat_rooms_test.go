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

func TestErikaKChatIsParsed(t *testing.T) {
	d := mustParseErikaK(t, `
detail:
  chat_rooms:
    - { number: 0, weight: 1 }
    - { number: 2, capacity: 4, weight: 1 }
  erika_k:
    chat:
      call_menu:
        title: "TITLE"
        lines: ["----", "MENU", "----"]
        hint: "HINT"
      chat_menu: ["== ROOMS ==", "[0] ROOM 0", "[2] ROOM 2"]
      room_notices:
        2: ["notice one", "notice two"]
`)
	c := d.Chat
	if c == nil || c.CallMenu.Title != "TITLE" || len(c.CallMenu.Lines) != 3 || c.CallMenu.Hint != "HINT" ||
		len(c.ChatMenu) != 3 || len(c.RoomNotices[2]) != 2 {
		t.Fatalf("chat = %+v", c)
	}
}

func TestErikaKChatNeedsKnownRooms(t *testing.T) {
	requireErikaKError(t, `
detail:
  erika_k:
    chat:
      chat_menu: ["ROOMS"]
`, "detail.erika_k.chat", "needs detail.chat_rooms")
	requireErikaKError(t, `
detail:
  chat_rooms: [{ number: 0, weight: 1 }]
  erika_k:
    chat:
      room_notices:
        5: ["no such room"]
`, "detail.erika_k.chat", "room 5 is not in detail.chat_rooms")
	requireErikaKError(t, `
detail:
  chat_rooms: [{ number: 0, weight: 1 }]
  erika_k:
    chat:
      call_menu: { hint: "hello {handle}" }
`, "chat.call_menu.hint", "{handle} is not replaced")
}
