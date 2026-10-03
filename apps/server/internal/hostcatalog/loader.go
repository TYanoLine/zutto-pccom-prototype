package hostcatalog

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
)

//go:embed presets/*.yaml
var embeddedPresets embed.FS

// LoadPresets loads and validates the preset hosts compiled into the binary.
func LoadPresets(o Options) ([]Preset, error) {
	return LoadPresetsFS(embeddedPresets, "presets", o)
}

// LoadPresetsFS loads every *.yaml file in dir, in file-name order. Loading is
// all-or-nothing: any invalid file, duplicate key or duplicate phone number
// fails the whole load so a broken definition stops startup instead of making a
// host silently disappear.
func LoadPresetsFS(fsys fs.FS, dir string, o Options) ([]Preset, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read preset directory %q: %w", dir, err)
	}
	var (
		presets []Preset
		errs    []error
	)
	keys := map[string]string{}
	phones := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch path.Ext(name) {
		case ".yaml":
		case ".yml":
			errs = append(errs, fmt.Errorf("%s: use the .yaml extension", name))
			continue
		default:
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		p, err := ParsePreset(name, data, o)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if prev, dup := keys[p.Key]; dup {
			errs = append(errs, fmt.Errorf("%s: key %q is already defined in %s", name, p.Key, prev))
			continue
		}
		keys[p.Key] = name
		if p.Phone != "" {
			if prev, dup := phones[p.Phone]; dup {
				errs = append(errs, fmt.Errorf("%s: phone %q is already used by %s", name, p.Phone, prev))
				continue
			}
			phones[p.Phone] = name
		}
		presets = append(presets, p)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid host presets:\n%w", errors.Join(errs...))
	}
	return presets, nil
}

// Descriptors converts fully specified presets into descriptors. It is a
// convenience for callers that do not (yet) support generated fill-in.
func Descriptors(presets []Preset) ([]HostDescriptor, error) {
	out := make([]HostDescriptor, 0, len(presets))
	var errs []error
	for _, p := range presets {
		d, err := p.Descriptor()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, d)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
