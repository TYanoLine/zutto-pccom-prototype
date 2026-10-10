package llm

import (
	"hash/fnv"
	"sort"
)

// titleFormNotes shows the breadth of ways a person words a subject line. They
// are deliberately unrelated to any board's field (no games, no places) and are
// not topics to reuse. Each call receives a different small selection.
var titleFormNotes = []string{
	"庭の南天が今年は実をつけません",
	"カレーの隠し味、みなさんは？",
	"夜行列車の旅から戻りました",
	"文庫本の古本屋めぐり、おすすめは",
	"ベランダのトマト、ようやく赤く",
	"【お知らせ】読書会の日程について",
	"鈍行で行く冬の北陸、感想など",
	"梅干しの土用干し、初挑戦",
	"自転車の変速、調整のコツ",
	"お茶の淹れ方、やっぱり奥が深い",
	"朝顔の種、どなたか分けてください",
	"ラジオ深夜便、聞いている方います？",
}

const titleFormNoteCount = 3

// selectTitleFormNotes picks titleFormNoteCount notes, deterministically from
// seed. The same seed gives the same notes; other seeds give other sets.
func selectTitleFormNotes(seed string) []string {
	type scored struct {
		note string
		key  uint64
	}
	items := make([]scored, len(titleFormNotes))
	for i, note := range titleFormNotes {
		h := fnv.New64a()
		_, _ = h.Write([]byte(seed))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(note))
		items[i] = scored{note, h.Sum64()}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
	out := make([]string, 0, titleFormNoteCount)
	for _, it := range items[:titleFormNoteCount] {
		out = append(out, it.note)
	}
	return out
}
