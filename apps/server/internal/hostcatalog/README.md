# hostcatalog

The canonical description of a BBS host, plus the preset hosts defined as YAML.

**Status.** `world.NewMemoryStore` builds its hosts and resident definitions from these presets through
`world.HostFromDescriptor`, which also carries the preset `role` and the
`debug` / `generation` flags into `world.Host`; the hosts are no longer
hard-coded there. The Erika-K host program reads its station's boards and texts
from `detail.erika_k` (see below). Not driven by these files yet: the directory the
web client shows (`CenterDirectory.ts`) and the dial behaviors in `telephone`.

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

The host definition is **immutable and owned by the preset**. Nothing fills it
in or rewrites it at runtime, and a debug snapshot never restores it: only the
state layer is ever read back from storage. A later change to a host (a new
name, a new operating policy, a new SYSOP) is meant to be recorded as a separate,
dated difference on top of the definition, not as an edit to it.

## Listed vs. dialable

`Listed` only controls whether a host appears in the host directory. It never
affects whether a number can be dialed. Hosts with role `debug` or `test` must
have `listed: false`; the validator enforces it. Generated hosts are listed by
default today; the intent is a generation setting whose value is stored on each
host when it is created, so changing the default never affects existing worlds.
Presets must state `listed` explicitly so a debug host cannot be published by
omission.

`listed: true` hosts appear in the dialing directory (`GET /api/directory`,
`world.HostDirectoryStore`). A `listed: false` host is omitted from the
directory but can still be reached by dialing its number directly.

## Preset files

One file per host in `presets/`, embedded into the binary. The file name must be
`<key>.yaml`.

```yaml
schema: 1                  # required, must be 1
key: hakata-canal-net      # required, stable, lower-case words joined by "-"
revision: 4                # required, raise when the content changes
listed: true               # required, no default
role: experiment           # optional label: debug | test | experiment | event

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

population:                # optional; residents are generated only when present
  seed: 199608260920
  id_prefix: "hakata"
  core_handles: ["MARI", "YUKI", "NORI"]
  handle_bases: ["AKI", "AYA", "MODEM"]
  handle_prefixes: ["N88", "V30", "98"]

dial:                      # optional
  mode: tone               # tone (default) | pulse
  behavior: normal         # normal (default) | always_connect | busy_first_n
  # busy_first_n: 5        # attempts numbered below N are BUSY (dials are numbered
  #                        # from 1, so 5 gives four BUSY dials). Required with, and
  #                        # only valid for, busy_first_n

debug:                     # optional; every flag is opt-in and off when omitted
  reset_articles_on_connect: true  # clear generated articles on every successful CONNECT
  generation_trace: true           # keep prompts/responses for the generation-trace endpoint
  content_log: true                # log committed, world-generated content
  http_endpoints: true             # allow the token-protected debug reset/sample endpoints
  snapshot: true                   # debug snapshot of the world state (never the host definition)

generation:                # optional; experimental generation behavior, off when omitted
  freeform_body: true      # title-led article body, skipping Article Detail and evidence

detail:
  erika_k:                 # only valid when host.program is erika-k
    texts:                 # printed exactly as written; a key left out prints nothing
      login_banner: ["...", "..."]
      login_greeting: "... {handle} ..."
      main_menu_title: "..."
      goodbye: "..."
    boards:
      - path: "1"
        name: "掲示板"
        unread: true
```

Decoding is strict: unknown keys (a typo in a flag name included), a second YAML
document, a missing `listed`, or any out-of-range value fail the load, and
loading is all-or-nothing, so a broken definition stops startup instead of
making a host quietly disappear. All problems in a file are reported together.

Fields under `host` other than `name` and `program` may be omitted. A preset
with omitted fields parses (`Preset.Missing()` lists them) but cannot become a
`HostDescriptor` yet: `Preset.Descriptor()` returns `*IncompleteError`, and the
store refuses to start with such a preset. Filling them deterministically from
the world seed is the generator's job and is not implemented.

## Erika-K detail

A station's own content is data in its preset, not code in the host program.
`detail.erika_k` is valid only for `host.program: erika-k`; it is read through
`world.HostDetailStore` and is part of the immutable host definition.

**`texts`** are named parts of Erika-K's screens that the station writes itself.
The host program prints them exactly as written: it does not draw rules, build
headings, insert the station name, pad lines or convert characters, and it has no
default wording. **A key that is left out prints nothing.**

| Key | Printed |
|---|---|
| `login_banner` (list of lines) | after the "last access" line at login, one element per line, rules and headings included |
| `login_greeting` | after the banner, between blank lines |
| `main_menu_title` | the first line of the main menu |
| `goodbye` | after the standard thanks when the user ends the call |

The only substitution is `{handle}` (the handle of the user who logged in), and
only in `login_banner` and `login_greeting`; in the other texts it would be
printed literally, so it is rejected. Texts must not contain control characters.
Each line must fit the 80-cell screen (checked by `erikak.ValidateDetail`, and by
a test over every embedded preset). New screen parts, such as the menus, are
added later as new keys.

**`boards`** is the board tree in display order. A board's key and parent come
from its `path` (`"10/1"` is board 1 under forum 10), so every parent must be
listed; paths are numeric and unique. Names are required, `root_author_policy` is
empty or `sysop_only`, and the tuning values must not be negative
(`verified_referent_rate` is 0 to 1). `unread: true` marks a board in the fixed
prototype display.

Still in the host program: the standard "last access" line and the thanks line,
and the prototype screens (`V`, `WHO`, `MEMB`, mail, files, `JUNK`).

## Population

A host gets resident members only when its preset has a `population:` block; the
`role` is unrelated. YAML contains the seed, ID prefix, core handles, and handle
vocabulary, while the deterministic generation algorithm remains in `world`.
Array order affects generated values and must not be changed. `id_prefix` must
be unique across presets, and `host.members` determines the population size.
The seed, ID prefix, and handle arrays are validated; core handles must be
non-empty and unique (including their ID slugs), and a population larger than
the core handles requires at least one handle base.

## Debug and generation flags

Behavior that only an evaluation station needs is switched on per host by these
flags, never by a phone number, an ID or the role. The role is only a label.

| Flag | What it does | Also needs |
|---|---|---|
| `debug.reset_articles_on_connect` | Clears the host's generated articles on every new successful CONNECT (see `docs/DEBUG_RESET.md`). Refused while generation runs. | nothing |
| `debug.generation_trace` | Records prompts and responses of generation calls. | `DEBUG_GENERATION_TRACE` (process-wide) |
| `debug.content_log` | Logs committed generated headers and bodies. | `DEBUG_LOG_GENERATED_CONTENT` (process-wide) |
| `debug.http_endpoints` | Lets `/api/debug/bbs/reset` and `/api/debug/bbs/sample` act on the host. An unknown number and a host without the flag get the same answer. | `DEBUG_RESET_TOKEN` |
| `debug.snapshot` | Stores the host's world state (boards, posts, memberships, personas, persona facts) as one JSON snapshot in Postgres so it survives a restart. A development stopgap until the world is stored in normalized tables. It never stores or restores the host definition. | `DATABASE_URL` |
| `generation.freeform_body` | Title-led prose experiment for article bodies. | `GENERATION_FREEFORM_BODY` (process-wide) |

The three process-wide switches were renamed from `DEBUG_LOG_HAKATA_GENERATED`,
`DEBUG_HAKATA_LLM_TRACE` and `HAKATA_FREEFORM_BODY`. The former names are no
longer read: a deployment that still sets one of them gets the default (on) until
it moves to the new name.

There is no flag for clearing articles at startup: the snapshot keeps the
articles, and `debug.reset_articles_on_connect` (or the debug reset endpoint)
clears them when wanted.

## Changing presets safely

- A published `key` and `phone` must not change: players already know the number
  and posts are keyed by the host.
- Changing `population.seed`, `population.id_prefix`, or any array order changes
  existing residents and can diverge from persisted data.
- Changing the order or `path` of `detail.erika_k.boards`, or any of its `texts`,
  changes what the station's screens show. The golden screens in
  `hostprogram/erikak/testdata` record the sample station's output byte for byte.
- Raise `revision` whenever the content changes, flags included. Existing worlds
  keep the content they were created with (only `listed` is meant to follow the
  file).
- Reserve numbers by passing `Options.ReservedPhones`; presets and (later)
  generated hosts are checked against it. This is also where future special
  numbers (110, 117 ...) plug in.

## Not done here

`HostProgram` registry (`knownPrograms` / `RuntimeSoftwareID` are stopgaps),
the web client's built-in center list, database migration (`origin`, `listed`,
`host_key`, `host_details`), generated region/traits, detail schemas for the
other host programs, moving the hard-coded dial fixtures in `telephone` to
`dial.behavior`, customizing the Erika-K menus from the station definition, and
the dated host-change records mentioned above.
