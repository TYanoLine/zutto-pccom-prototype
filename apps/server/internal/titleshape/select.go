package titleshape

import (
	"fmt"
	"hash/fnv"
	"sort"
)

// 調整値: 形の距離の重み（先頭 3 文字の一致、末尾 4 文字の一致、2-gram 類似度）。
const (
	weightLead   = 0.4
	weightSuffix = 0.4
	weightGram   = 0.2
	leadLen      = 3
)

// 調整値: 直近の形の事実を文にするしきい値。偏りがこれ以下なら何も渡さない。
const (
	factMinCount = 3
	factMinShare = 0.15
	factMinN     = 8
	factLeadLen  = 2
)

func hash64(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// ShuffleSeeded returns a copy of items in an order fixed by seed (and
// independent of the input order for equal item sets).
func ShuffleSeeded(seed string, items []string) []string {
	out := append([]string(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		hi, hj := hash64(seed, out[i]), hash64(seed, out[j])
		if hi != hj {
			return hi < hj
		}
		return out[i] < out[j]
	})
	return out
}

func leadKey(title string) string { return leadPrefix(title, leadLen) }

func leadPrefix(title string, n int) string {
	rs := []rune(Normalize(title))
	if len(rs) > n {
		rs = rs[:n]
	}
	return string(rs)
}

// Distance is how different two titles are in shape, 0 (same shape) to 1.
func Distance(a, b string) float64 {
	same := func(x, y string) float64 {
		if x != "" && x == y {
			return 1
		}
		return 0
	}
	sim := weightLead*same(leadKey(a), leadKey(b)) +
		weightSuffix*same(SuffixKey(a), SuffixKey(b)) +
		weightGram*Similarity(a, b)
	return 1 - sim
}

// PickVariant returns the index of the variant whose smallest Distance to the
// covered titles is largest. Ties are broken by hash(seed, index). It chooses
// among the model's own variants; it assigns no shape. It returns -1 for no
// variants.
func PickVariant(seed string, variants []string, covered []string) int {
	best, bestDist, bestTie := -1, -1.0, uint64(0)
	for i, v := range variants {
		d := 1.0
		for _, c := range covered {
			if x := Distance(v, c); x < d {
				d = x
			}
		}
		tie := hash64(seed, fmt.Sprint(i))
		if best == -1 || d > bestDist || (d == bestDist && tie < bestTie) {
			best, bestDist, bestTie = i, d, tie
		}
	}
	return best
}

// FormFacts states, as plain facts, how recent titles repeat: a shared opening
// or a shared ending. It returns nothing when no repetition passes the
// thresholds, so quiet boards get no extra material. The sentences describe;
// they do not instruct.
func FormFacts(titles []string) []string {
	n := len(titles)
	if n < factMinN {
		return nil
	}
	leads, tails := map[string]int{}, map[string]int{}
	leadShown, tailShown := map[string]string{}, map[string]string{}
	for _, t := range titles {
		lk, sk := leadPrefix(t, factLeadLen), SuffixKey(t)
		leads[lk]++
		tails[sk]++
		if _, ok := leadShown[lk]; !ok {
			leadShown[lk] = lk
		}
		tailShown[sk] = sk
	}
	var facts []string
	if k, c := top(leads); c >= factMinCount && float64(c)/float64(n) >= factMinShare {
		facts = append(facts, fmt.Sprintf("直近%d件のうち、「%s」で始まる題名が%d件ある。", n, leadShown[k], c))
	}
	if k, c := top(tails); c >= factMinCount && float64(c)/float64(n) >= factMinShare {
		facts = append(facts, fmt.Sprintf("直近%d件のうち、「%s」で終わる題名が%d件ある。", n, tailShown[k], c))
	}
	return facts
}

func top(m map[string]int) (string, int) {
	bestK, bestC := "", 0
	for k, c := range m {
		if c > bestC || (c == bestC && k < bestK) {
			bestK, bestC = k, c
		}
	}
	return bestK, bestC
}
