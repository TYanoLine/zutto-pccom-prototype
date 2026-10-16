package world

import (
	"embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var worldDataFS embed.FS

// TopicEntry defines matching words and broader parent topic keys for one
// open-vocabulary topic key.
type TopicEntry struct {
	Words   []string `yaml:"words" json:"words,omitempty"`
	Broader []string `yaml:"broader" json:"broader,omitempty"`
}

// Lexicon holds open-vocabulary topic cues, broader-topic relations, and
// date-bounded salience boosts.
type Lexicon struct {
	Topics   map[string]TopicEntry `yaml:"topics" json:"topics,omitempty"`
	Salience []Salience            `yaml:"salience" json:"salience,omitempty"`
}

var (
	defaultLexiconOnce sync.Once
	defaultLexiconVal  *Lexicon
)

// DefaultLexicon returns the embedded 1996 default lexicon (data/lexicon-1996.yaml).
func DefaultLexicon() *Lexicon {
	defaultLexiconOnce.Do(func() {
		raw, err := worldDataFS.ReadFile("data/lexicon-1996.yaml")
		if err != nil {
			defaultLexiconVal = &Lexicon{Topics: map[string]TopicEntry{}}
			return
		}
		lex, err := LoadLexiconYAML(raw)
		if err != nil {
			defaultLexiconVal = &Lexicon{Topics: map[string]TopicEntry{}}
			return
		}
		defaultLexiconVal = lex
	})
	return defaultLexiconVal
}

// LoadLexiconYAML parses a Lexicon from YAML bytes and normalizes all topic keys.
func LoadLexiconYAML(data []byte) (*Lexicon, error) {
	var raw Lexicon
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse lexicon yaml: %w", err)
	}
	norm := &Lexicon{
		Topics:   make(map[string]TopicEntry, len(raw.Topics)),
		Salience: make([]Salience, 0, len(raw.Salience)),
	}
	for k, entry := range raw.Topics {
		nk := NormalizeTopicKey(k)
		if nk == "" {
			continue
		}
		broader := make([]string, 0, len(entry.Broader))
		for _, b := range entry.Broader {
			if nb := NormalizeTopicKey(b); nb != "" {
				broader = append(broader, nb)
			}
		}
		words := make([]string, 0, len(entry.Words))
		for _, w := range entry.Words {
			if tw := strings.TrimSpace(w); tw != "" {
				words = append(words, tw)
			}
		}
		norm.Topics[nk] = TopicEntry{
			Words:   words,
			Broader: broader,
		}
	}
	for _, s := range raw.Salience {
		nt := NormalizeTopicKey(s.Topic)
		if nt == "" || s.Boost <= 0 {
			continue
		}
		norm.Salience = append(norm.Salience, Salience{
			Topic: nt,
			From:  strings.TrimSpace(s.From),
			To:    strings.TrimSpace(s.To),
			Boost: s.Boost,
		})
	}
	return norm, nil
}

// MergeLexicons merges one or more lexicons into a new Lexicon. Later lexicons
// extend or override words and broader keys for matching topics.
func MergeLexicons(lexicons ...*Lexicon) *Lexicon {
	out := &Lexicon{
		Topics: map[string]TopicEntry{},
	}
	for _, l := range lexicons {
		if l == nil {
			continue
		}
		for k, entry := range l.Topics {
			nk := NormalizeTopicKey(k)
			if nk == "" {
				continue
			}
			existing := out.Topics[nk]
			existing.Words = mergeUniqueStrings(existing.Words, entry.Words)
			existing.Broader = mergeUniqueNormalized(existing.Broader, entry.Broader)
			out.Topics[nk] = existing
		}
		out.Salience = append(out.Salience, l.Salience...)
	}
	return out
}

func mergeUniqueStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func mergeUniqueNormalized(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		ns := NormalizeTopicKey(s)
		if ns != "" && !seen[ns] {
			seen[ns] = true
			out = append(out, ns)
		}
	}
	for _, s := range b {
		ns := NormalizeTopicKey(s)
		if ns != "" && !seen[ns] {
			seen[ns] = true
			out = append(out, ns)
		}
	}
	return out
}

// Broader returns the broader parent topic keys registered for topic.
func (l *Lexicon) Broader(topic string) []string {
	if l == nil || len(l.Topics) == 0 {
		return nil
	}
	key := NormalizeTopicKey(topic)
	entry, ok := l.Topics[key]
	if !ok || len(entry.Broader) == 0 {
		return nil
	}
	out := make([]string, 0, len(entry.Broader))
	for _, b := range entry.Broader {
		if nb := NormalizeTopicKey(b); nb != "" {
			out = append(out, nb)
		}
	}
	return out
}

// Match returns topic keys whose words appear in text (case-insensitive).
// Unknown topics that are not in the lexicon are not returned by Match,
// whereas direct topic tags are handled by ArticleTopicWeights / ExtractPostTopics.
func (l *Lexicon) Match(text string) []string {
	if l == nil || len(l.Topics) == 0 || strings.TrimSpace(text) == "" {
		return nil
	}
	lower := strings.ToLower(text)
	keys := make([]string, 0, len(l.Topics))
	for k := range l.Topics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var matched []string
	seen := map[string]bool{}
	for _, k := range keys {
		entry := l.Topics[k]
		for _, w := range entry.Words {
			w = strings.TrimSpace(w)
			if w == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(w)) {
				nk := NormalizeTopicKey(k)
				if nk != "" && !seen[nk] {
					seen[nk] = true
					matched = append(matched, nk)
				}
				break
			}
		}
	}
	return matched
}
