# ずっとパソコン通信 — product baseline

## Premise

A web service that feels like using a PC-98 communications terminal in 1996 to dial an effectively infinite collection of Japanese grass-roots BBS hosts. Hosts, residents, posts, history, software customizations and relationships become persistent facts after they are first generated.

The world is not a chatbot wrapped in a terminal. It is a persistent network simulation that the human happens to participate in.

## Fixed baseline

- World setting: Japan, 1996.
- Client environment for the first release: PC-98 family.
- Client UI: web-based fictional communications application inspired by period Japanese terminal software, not a pixel-for-pixel clone of one commercial/shareware program.
- Historical host software families include TurboBBS, KTBBS, BIG-Model, 絵理香K版, mmm, RT-BBS and VS, with more possible as research permits.
- Historical host packages are separate runtimes/state machines. Shared lower layers are allowed, but software-specific UI/commands/semantics must not be flattened into one generic profile runtime.
- Internal canonical text: UTF-8. Serial/terminal boundary later uses CP932/Shift_JIS bytes.
- LLM: Azure OpenAI Responses API family behind an internal provider interface when AI is enabled.
- Canonical world state lives in PostgreSQL, never in an LLM conversation/session.

## Host discovery

Unknown phone numbers may instantiate a host lazily. Generation uses a stable seed derived from the phone number plus regional/world rules. Once generated, facts are committed to the database and do not silently change.

A user should never receive a complete global directory. Hosts are discovered through local phone books, recommendations, posts, SYSOP relationships, magazines, word-of-mouth and direct dialing.

## Regional simulation

The user's starting location is part of onboarding. Initially choose at least prefecture + area code, not a precise home address.

Region influences distributions, not hard rules:

- BBS density and host size
- likely host software lineage
- age of equipment / upgrade speed
- line count and supported modem speeds
- local topics and industries
- demographic distributions
- links to nearby hosts / joint offline meetings
- local vs long-distance pseudo tolls

Neighboring BBSs can share SYSOP relationships, users, rumors, software traditions and events.

## Telephone and modem simulation

Dialing is a first-class interaction:

- AT command mode
- DTMF and modem handshake audio
- BUSY / NO CARRIER / NO ANSWER / CONNECT
- negotiated line speed (e.g. 2400, 9600, 14400, 28800)
- auto-redial and `A/`
- line occupancy driven by host popularity, line count, time of day, audience profile and events
- virtual users may occupy actual logical lines
- real generation/coordination pressure may reduce admission capacity or produce `BUSY` rather than leaking modern backend errors
- simulated bps may pace terminal output independently of actual Internet/backend throughput

The user is never charged real money.

A pseudo phone bill should accumulate for atmosphere. Telehodai-style behavior is part of the world: two registered destination numbers and the 23:00–08:00 fixed-rate window. Exact historical tariffs should be researched and data-driven before production; prototype counters are only an approximation.

## Visual terminal behavior

Base logical display: 640×400, 80×25 text cells.

Hosts may be:

- plain monochrome
- lightly colored
- ANSI/ESC color heavy
- cursor-positioned / full-screen menu oriented

Terminal should support a period-appropriate subset of escape sequences and PC-98-like text presentation. Do not assume all hosts use ANSI art.

## Population and demographics

Demographics should reflect plausible 1996 Japanese PC communications usage, then vary by host theme, region, SYSOP network and opening year.

Each persona has separate private facts and public profile. Gender/age/occupation need not always be publicly visible.

Personas need at least:

- activity pattern
- interests
- opinions
- reply tendency
- thread-start tendency
- lurker tendency
- newcomer openness
- argumentativeness
- relationships
- current goals / ongoing conversations

A large fraction of members should lurk or post rarely.

## Non-player-centric simulation (critical)

The human-controlled member is one member, not the protagonist of the universe.

Never implement the loop as:

`human post -> ask LLM for everybody's reactions`.

Instead the world engine decides statistically:

1. who is active at a given time;
2. what boards they visit;
3. which messages they read;
4. whether they have any motivation to reply;
5. what independent actions/conversations they already have;
6. only then, if a text-producing action was selected, ask an LLM to verbalize it.

Reading without replying is normal. No response at all is normal. A response the next day is normal. NPC-to-NPC conversation should continue without involving the human.

Opinions are persistent facts. Do not let a human statement silently rewrite a persona's opinion simply because the LLM tends to agree with users.

## Observation-driven world advancement

The world should *appear* to continue while nobody is watching, but production simulation is primarily triggered by observation rather than continuous global generation.

Human login/activity may cause stale world scopes to catch up, but only the scopes actually exposed by the current action should be materialized in detail. Logging in must not eagerly generate every known host or resident.

Examples of observation triggers include dialing/entering a host, opening a board/thread, reading mail, or requesting another view whose facts are stale or not yet materialized.

For long inactive periods, catch up using a bounded set of coarse durable events first, then generate detailed posts/events only where the current observation requires them. Once generated and committed, those events become shared history for all later observers rather than a per-user alternate world.

Concurrent observation of the same stale scope must be serialized with a narrow generation/update lease, simulation version, or equivalent mechanism. If one observer commits the next history version first, the other reuses that committed result.

## Community openness

Hosts should generally feel smaller and less globally open than modern social media.

Possible host policies:

- guest read-only area
- automatic enrollment
- SYSOP approval
- member levels
- hidden/restricted boards
- referral-only host
- read-only punishment
- temporary posting suspension
- ban / account removal within that host

Do not use a single global reputation score. Closely connected hosts may exchange rumors or information, but reputation is primarily local.

## Misbehavior and moderation

Some personas can spam, troll, argue, impersonate, misuse escape codes or break local rules. These are community events, not random noise. SYSOP personality and rules determine warnings, deletion, restrictions and bans. Human-controlled members are subject to the same local rules.

## World-time strategy

The cultural/knowledge ceiling is 1996. Later products, slang and events must not leak into NPC knowledge.

The real Japan clock may drive time-of-day behavior and Telehodai windows, while the calendar is mapped into the 1996 world. Keep `WorldClock` abstract so tests can jump between 22:59, 23:00 and 08:00.

Inactive hosts are not simulated continuously. Catch them up lazily using coarse events and materialize detailed posts only when needed. Normal production catch-up is observation-driven and limited to relevant scopes; continuous background LLM simulation of the entire world is not a requirement.

## Future real-machine target

The same BBS runtime should eventually be reachable from a real PC-98 communications program over RS-232C through a bridge device/application that emulates enough Hayes-style modem behavior to support dialing and carrier control.

## Documentation policy

This repository is the source of truth for product and implementation decisions. Historical reconstruction rules live in `docs/HISTORICAL_ACCURACY.md`; world behavior in `docs/WORLD_SIMULATION.md`; terminal/transport invariants in `docs/TERMINAL_AND_MODEM.md`; per-program research in `docs/host-programs/`. AI/coding agents should start with `AGENTS.md`.
