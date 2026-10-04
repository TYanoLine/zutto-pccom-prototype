package hostcatalog

import (
	"errors"
	"fmt"
	"strings"
)

// Population describes the resident members of a host. The slices are read-only.
type Population struct {
	Seed           int64
	IDPrefix       string
	CoreHandles    []string
	HandleBases    []string
	HandlePrefixes []string
}

type presetPopulation struct {
	Seed           *int64   `yaml:"seed"`
	IDPrefix       string   `yaml:"id_prefix"`
	CoreHandles    []string `yaml:"core_handles"`
	HandleBases    []string `yaml:"handle_bases"`
	HandlePrefixes []string `yaml:"handle_prefixes"`
}

func HandleSlug(handle string) string {
	out := make([]byte, 0, len(handle))
	for i := 0; i < len(handle); i++ {
		c := handle[i]
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		}
	}
	return string(out)
}

func parsePopulation(f *presetPopulation) (*Population, error) {
	if f == nil {
		return nil, nil
	}
	var errs []error
	add := func(field string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("population.%s: %w", field, err))
		}
	}

	pop := &Population{IDPrefix: f.IDPrefix}
	if f.Seed == nil {
		add("seed", errors.New("is required"))
	} else {
		pop.Seed = *f.Seed
	}
	if strings.TrimSpace(f.IDPrefix) == "" {
		add("id_prefix", errors.New("is required"))
	} else {
		add("id_prefix", checkKey(f.IDPrefix))
	}

	seenHandles := map[string]bool{}
	seenSlugs := map[string]string{}
	for i, raw := range f.CoreHandles {
		field := fmt.Sprintf("core_handles[%d]", i)
		h := strings.TrimSpace(raw)
		if h == "" {
			add(field, errors.New("must not be empty"))
			continue
		}
		lower := strings.ToLower(h)
		if seenHandles[lower] {
			add(field, fmt.Errorf("%q is listed twice", h))
			continue
		}
		seenHandles[lower] = true
		slug := HandleSlug(h)
		if slug == "" {
			add(field, fmt.Errorf("%q has no letters or digits to build a persona ID from", h))
			continue
		}
		if prev, dup := seenSlugs[slug]; dup {
			add(field, fmt.Errorf("%q would get the same persona ID as %q", h, prev))
			continue
		}
		seenSlugs[slug] = h
		pop.CoreHandles = append(pop.CoreHandles, h)
	}
	pop.HandleBases = cleanWords(f.HandleBases, "handle_bases", add)
	pop.HandlePrefixes = cleanWords(f.HandlePrefixes, "handle_prefixes", add)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return pop, nil
}

func cleanWords(in []string, field string, add func(string, error)) []string {
	var out []string
	for i, raw := range in {
		w := strings.TrimSpace(raw)
		if w == "" {
			add(fmt.Sprintf("%s[%d]", field, i), errors.New("must not be empty"))
			continue
		}
		out = append(out, w)
	}
	return out
}

func Populations(presets []Preset) (map[string]Population, error) {
	out := map[string]Population{}
	var errs []error
	for _, p := range presets {
		if p.Population == nil {
			continue
		}
		pop := *p.Population
		if p.Members == nil {
			errs = append(errs, fmt.Errorf("%s: population: host.members is required", p.Source))
			continue
		}
		members := *p.Members
		if len(pop.CoreHandles) > members {
			errs = append(errs, fmt.Errorf("%s: population: core_handles has %d entries but host.members is %d", p.Source, len(pop.CoreHandles), members))
		}
		if members > len(pop.CoreHandles) && len(pop.HandleBases) == 0 {
			errs = append(errs, fmt.Errorf("%s: population: handle_bases is required because host.members (%d) exceeds core_handles (%d)", p.Source, members, len(pop.CoreHandles)))
		}
		out[p.Key] = pop
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
