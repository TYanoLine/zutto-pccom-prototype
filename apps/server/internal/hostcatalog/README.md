# hostcatalog

The canonical description of a BBS host, plus the preset hosts defined as YAML.

**Status.** `world.NewMemoryStore` builds its hosts from these presets through
`world.HostFromDescriptor`, which also carries the preset `role` into
`world.Host.Role`; the hosts are no longer hard-coded there. Not driven by these
files yet: the directory the web client shows (`CenterDirectory.ts`), the dial
behaviors in `telephone`, and what the Erika-K runtime says (welcome text,
boards, SYSOP), which is still HAKATA's.

`hostcatalog` must not import `world` (the store imports it), so the
descriptor -> `world.Host` conversion lives in `world`.

## Three layers

| Layer | Type | When it is fixed | Mutable? |
|---|---|---|---|
| Skeleton | `HostDescriptor` (this package) | when the host directory is created | no (only `ResetHost` bumps `Generation`) |
| Detail | welcome text, menus, boards, program-specific data | first access to the host | no, once `ready` |
| State | posts, members, board activity (`world`, `worldrepo`) | continuously | yes |

The descriptor holds no derived values. The busy rate is computed from
popularity, lines and time of day (`telephone.busyProbability`); time-varying
membership belongs to the board activity model.

## Listed vs. dialable

`Listed` only controls whether a host appears in the host directory. It never
affects whether a number can be dialed. Hosts with role `debug` or `test` must
have `listed: false`; the validator enforces it. Generated hosts are listed by
default today; the intent is a generation setting whose value is stored on each
host when it is created, so changing the default never affects existing worlds.
Presets must state `listed` explicitly so a debug host cannot be published by
omission.

## Preset files

One file per host in `presets/`, embedded into the binary. The file name must be
`<key>.yaml`.

```yaml
schema: 1                  # required, must be 1
key: hakata-canal-net      # required, stable, lower-case words joined by "-"
revision: 1                # required, raise when the content changes
listed: true               # required, no default
role: experiment           # optional: debug | test | experiment | event

host:
  name: HAKATA CANAL NET   # required
  program: erika-k         # required, a known host-program ID
  phone: "0920000196"      # quote it; digits only, 10-11 digits, leading 0
  software_label: 絵理香K版 # optional display name, falls back to program
  region: { prefecture: 福岡県, city: 福岡市 }
  lines: 3
  max_baud: 14400          # 2400 | 9600 | 14400 | 28800
  founded_on: 1994-11-03
  popularity: 0.58         # 0..1
  members: 326
  traits: { ansi: false, guest_allowed: true, teleho_friendly: true }

dial:                      # optional
  mode: tone               # tone (default) | pulse
  behavior: normal         # normal (default) | always_connect | busy_first_n
  # busy_first_n: 5        # attempts numbered below N are BUSY (dials are numbered
  #                        # from 1, so 5 gives four BUSY dials). Required with, and
  #                        # only valid for, busy_first_n

detail:                    # optional; only `welcome` is accepted for now
  welcome: |
    ...
```

Decoding is strict: unknown keys, a second YAML document, a missing `listed`, or
any out-of-range value fail the load, and loading is all-or-nothing, so a broken
definition stops startup instead of making a host quietly disappear. All
problems in a file are reported together.

Fields under `host` other than `name` and `program` may be omitted. A preset
with omitted fields parses (`Preset.Missing()` lists them) but cannot become a
`HostDescriptor` yet: `Preset.Descriptor()` returns `*IncompleteError`. Filling
them deterministically from the world seed is the generator's job and is not
implemented.

## Changing presets safely

- A published `key` and `phone` must not change: players already know the number
  and posts are keyed by the host.
- `role: experiment` marks the evaluation station. What used to be keyed on
  HAKATA's phone number or ID now follows the role (`world.Host.IsExperiment`):
  the debug auto-reset on CONNECT, the debug reset/sample endpoints, durable
  snapshots and the startup baseline clear, the generation trace and
  generated-content log, the title-led prose experiment, and the resident
  population. At most one preset may have it, because the population generator
  uses fixed persona IDs; loading panics otherwise. Without any experiment host
  those features are simply off.
- Raise `revision` whenever the content changes. Existing worlds keep the content
  they were created with (only `listed` is meant to follow the file).
- Reserve numbers by passing `Options.ReservedPhones`; presets and (later)
  generated hosts are checked against it. This is also where future special
  numbers (110, 117 ...) plug in.

## Not done here

`HostProgram` registry (`knownPrograms` / `RuntimeSoftwareID` are stopgaps),
the web client's built-in center list, database migration (`origin`, `listed`,
`host_key`, `host_details`), generated region/traits, per-program detail
schemas, and moving the hard-coded dial fixtures in `telephone` to
`dial.behavior`.
