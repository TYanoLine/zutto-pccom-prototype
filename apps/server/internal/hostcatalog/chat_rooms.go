package hostcatalog

import (
	"errors"
	"fmt"
	"math"
)

// ChatRoom is one chat room of a station, as the station definition lists it
// under detail.chat_rooms. It is a world resource shared by every host
// program: which residents are in a room is decided from these numbers. How a
// room looks on screen belongs to the host program's own section. See spec 016.
type ChatRoom struct {
	Number int `yaml:"number"`
	// Capacity is the most people the room holds; 0 means unlimited.
	Capacity int `yaml:"capacity"`
	// Weight is how often a resident picks this room relative to the others.
	// It is a fictional tuning value, not a historical figure.
	Weight float64 `yaml:"weight"`
}

// validateChatRooms checks room numbers, capacities and weights. An empty list
// is valid: the station has no chat rooms and residents never chat there.
func validateChatRooms(rooms []ChatRoom) error {
	if len(rooms) == 0 {
		return nil
	}
	var errs []error
	seen := make(map[int]bool, len(rooms))
	total := 0.0
	for i, room := range rooms {
		field := fmt.Sprintf("[%d]", i)
		if room.Number < 0 {
			errs = append(errs, fmt.Errorf("%s.number: %d must not be negative", field, room.Number))
		} else if seen[room.Number] {
			errs = append(errs, fmt.Errorf("%s.number: %d is listed twice", field, room.Number))
		}
		seen[room.Number] = true
		if room.Capacity < 0 {
			errs = append(errs, fmt.Errorf("%s.capacity: %d must not be negative (0 means unlimited)", field, room.Capacity))
		}
		if math.IsNaN(room.Weight) || math.IsInf(room.Weight, 0) || room.Weight < 0 {
			errs = append(errs, fmt.Errorf("%s.weight: %v must be a non-negative number", field, room.Weight))
		} else {
			total += room.Weight
		}
	}
	if total <= 0 {
		errs = append(errs, errors.New("the weights must add up to more than 0"))
	}
	return errors.Join(errs...)
}
