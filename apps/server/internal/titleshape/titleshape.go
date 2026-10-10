// Package titleshape measures the shape of BBS subject lines. Every function
// is pure: no LLM, network, clock or randomness. The measurements describe
// what was generated; they are never a reason to reject a title.
package titleshape

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Lead is the kind of word a title starts with.
type Lead string

const (
	LeadReferent Lead = "referent"
	LeadPlace    Lead = "place"
	LeadHandle   Lead = "handle"
	LeadOther    Lead = "other"
)

const (
	nearDuplicateThreshold = 0.8 // 近似重複の Jaccard しきい値
	suffixLen              = 4
	minHandleRun           = 3
	minReferentRun         = 2
	minAbbrevRun           = 3
)

const terminalMarks = "？！?!…。.．、,，～~ー"

// Normalize applies NFKC, drops spaces, punctuation and symbols (including the
// middle dot) and lower-cases, so that cosmetic differences do not hide a match.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Referents returns the objects a summary is about: words in 『…』; when there
// are none, the leading run of katakana/alphanumerics (at least two runes).
func Referents(summary string) []string {
	var out []string
	seen := map[string]bool{}
	rs := []rune(summary)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '『' {
			continue
		}
		for j := i + 1; j < len(rs); j++ {
			if rs[j] == '』' {
				if w := strings.TrimSpace(string(rs[i+1 : j])); w != "" && !seen[w] {
					seen[w] = true
					out = append(out, w)
				}
				i = j
				break
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	run := 0
	for _, r := range rs {
		if isKatakanaOrAlnum(r) {
			run++
			continue
		}
		break
	}
	if run >= minReferentRun {
		out = append(out, string(rs[:run]))
	}
	return out
}

func isKatakanaOrAlnum(r rune) bool {
	return unicode.Is(unicode.Katakana, r) || r == 'ー' || r == '・' ||
		(r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)))
}

// ClassifyLead classifies how a title starts. A referent wins over a place,
// and a place over a handle-like run.
func ClassifyLead(title string, referents, places []string) Lead {
	n := Normalize(title)
	if n == "" {
		return LeadOther
	}
	for _, w := range referents {
		if leadsWith(n, Normalize(w), title) {
			return LeadReferent
		}
	}
	for _, w := range places {
		if w = Normalize(w); w != "" && strings.HasPrefix(n, w) {
			return LeadPlace
		}
	}
	run := 0
	for _, r := range strings.TrimSpace(norm.NFKC.String(title)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			run++
			continue
		}
		break
	}
	if run >= minHandleRun {
		return LeadHandle
	}
	return LeadOther
}

// leadsWith reports whether a normalized title starts with a referent, allowing
// the shortened forms people actually write: the referent itself, one of its
// space-separated words, or an abbreviation that is a substring of it
// (マリオカート for スーパーマリオカート). The leading run of katakana/ASCII
// must be at least minAbbrevRun runes to count as an abbreviation.
func leadsWith(normTitle, normRef, rawTitle string) bool {
	if normRef == "" {
		return false
	}
	if strings.HasPrefix(normTitle, normRef) {
		return true
	}
	if lead := leadingRun(rawTitle); len([]rune(lead)) >= minAbbrevRun && strings.Contains(normRef, lead) {
		return true
	}
	return false
}

// mentions reports whether a normalized title mentions a referent: the whole
// name, one of its words (when long enough), or a leading abbreviation.
func mentions(normTitle string, refRaw, rawTitle string) bool {
	normRef := Normalize(refRaw)
	if normRef == "" {
		return false
	}
	if strings.Contains(normTitle, normRef) {
		return true
	}
	for _, tok := range strings.Fields(norm.NFKC.String(refRaw)) {
		if t := Normalize(tok); len([]rune(t)) >= minAbbrevRun && strings.Contains(normTitle, t) {
			return true
		}
	}
	return leadsWith(normTitle, normRef, rawTitle)
}

// leadingRun is the normalized leading run of katakana/ASCII letters and digits.
func leadingRun(title string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(strings.TrimSpace(title)) {
		if !isKatakanaOrAlnum(r) {
			break
		}
		if !unicode.IsSpace(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// SuffixKey is the last four runes of the title without its terminal marks.
func SuffixKey(title string) string {
	rs := []rune(strings.TrimRightFunc(strings.TrimSpace(title), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(terminalMarks, r)
	}))
	if len(rs) > suffixLen {
		rs = rs[len(rs)-suffixLen:]
	}
	return string(rs)
}

// IsDuplicate reports an exact match after Normalize.
func IsDuplicate(a, b string) bool {
	na := Normalize(a)
	return na != "" && na == Normalize(b)
}

// Similarity is the Jaccard index of the character 2-grams of the normalized
// titles. Titles shorter than two runes compare as a single gram.
func Similarity(a, b string) float64 {
	ga, gb := bigrams(Normalize(a)), bigrams(Normalize(b))
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	inter := 0
	for g := range ga {
		if gb[g] {
			inter++
		}
	}
	return float64(inter) / float64(len(ga)+len(gb)-inter)
}

func bigrams(s string) map[string]bool {
	rs := []rune(s)
	out := map[string]bool{}
	switch {
	case len(rs) == 0:
	case len(rs) == 1:
		out[s] = true
	default:
		for i := 0; i+1 < len(rs); i++ {
			out[string(rs[i:i+2])] = true
		}
	}
	return out
}

// Item is one title with the summary of the event it words.
type Item struct {
	Subject string `json:"subject"`
	Summary string `json:"summary"`
}

type SuffixCount struct {
	Suffix string `json:"suffix"`
	Count  int    `json:"count"`
}

// Report is a deterministic description of a set of titles.
type Report struct {
	N                    int              `json:"n"`
	LeadCounts           map[Lead]int     `json:"lead_counts"`
	LeadRates            map[Lead]float64 `json:"lead_rates"`
	SuffixMaxShare       float64          `json:"suffix_max_share"`
	SuffixTop            []SuffixCount    `json:"suffix_top"`
	ExactDuplicateTitles int              `json:"exact_duplicate_titles"` // titles that share a normalized form with an earlier one
	NearDuplicatePairs   int              `json:"near_duplicate_pairs"`
	DuplicateRate        float64          `json:"duplicate_rate"`
	LenMin               int              `json:"len_min"`
	LenMedian            int              `json:"len_median"`
	LenMax               int              `json:"len_max"`
	RetentionN           int              `json:"retention_n"` // items whose summary has a referent
	RetentionRate        float64          `json:"retention_rate"`
}

const suffixTopN = 5

// Measure computes the Report for items. places are words (e.g. place names)
// that count as a place lead.
func Measure(items []Item, places []string) Report {
	r := Report{N: len(items), LeadCounts: map[Lead]int{}, LeadRates: map[Lead]float64{}}
	if len(items) == 0 {
		return r
	}
	suffixes := map[string]int{}
	seen := map[string]bool{}
	lens := make([]int, 0, len(items))
	retained := 0
	for _, it := range items {
		refs := Referents(it.Summary)
		r.LeadCounts[ClassifyLead(it.Subject, refs, places)]++
		suffixes[SuffixKey(it.Subject)]++
		lens = append(lens, len([]rune(it.Subject)))
		n := Normalize(it.Subject)
		if seen[n] {
			r.ExactDuplicateTitles++
		}
		seen[n] = true
		if len(refs) > 0 {
			r.RetentionN++
			for _, w := range refs {
				if mentions(n, w, it.Subject) {
					retained++
					break
				}
			}
		}
	}
	for l, c := range r.LeadCounts {
		r.LeadRates[l] = float64(c) / float64(len(items))
	}
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if Similarity(items[i].Subject, items[j].Subject) >= nearDuplicateThreshold {
				r.NearDuplicatePairs++
			}
		}
	}
	r.DuplicateRate = float64(r.ExactDuplicateTitles) / float64(len(items))
	for s, c := range suffixes {
		r.SuffixTop = append(r.SuffixTop, SuffixCount{s, c})
	}
	sort.Slice(r.SuffixTop, func(i, j int) bool {
		if r.SuffixTop[i].Count != r.SuffixTop[j].Count {
			return r.SuffixTop[i].Count > r.SuffixTop[j].Count
		}
		return r.SuffixTop[i].Suffix < r.SuffixTop[j].Suffix
	})
	r.SuffixMaxShare = float64(r.SuffixTop[0].Count) / float64(len(items))
	if len(r.SuffixTop) > suffixTopN {
		r.SuffixTop = r.SuffixTop[:suffixTopN]
	}
	sort.Ints(lens)
	r.LenMin, r.LenMedian, r.LenMax = lens[0], lens[len(lens)/2], lens[len(lens)-1]
	if r.RetentionN > 0 {
		r.RetentionRate = float64(retained) / float64(r.RetentionN)
	}
	return r
}
