package chatslot

import (
	"sort"

	"zutto-pccom/apps/server/internal/world"
)

// limit puts the slots in time order.
func (c computation) limit(slots []Slot, _ []world.ChatUtterance) []Slot {
	sort.SliceStable(slots, func(i, j int) bool {
		if !slots[i].At.Equal(slots[j].At) {
			return slots[i].At.Before(slots[j].At)
		}
		return slots[i].Key < slots[j].Key
	})
	return slots
}
