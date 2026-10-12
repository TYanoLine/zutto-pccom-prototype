# Persistent world simulation

## Core model

The world is a persistent network of hosts, people, lines, posts, relationships, and events. The human is a participant in that world, not the trigger for all activity.

The database is canonical truth. Generation fills unknown facts; observation/materialization turns those facts into persistent history.

## Observation and lazy generation

An unknown phone number may resolve to no host or lazily generate a host according to stable world rules. Generated facts should be reproducible where possible and persisted once materialized.

Unobserved details do not need continuous fine-grained simulation. Catch up inactive hosts using coarse events, schedules, and statistical activity; materialize detailed posts/events when they become relevant.

Never silently rewrite an already observed host, persona, relationship, post, or historical event simply because a later LLM call would prefer another answer.

### Observation-driven catch-up

For the current host-world implementation, **successful CONNECT is the host observation boundary**. Directory/catalog display, phone-number lookup, host metadata creation, and unsuccessful dial attempts do not observe the host and must not create its article history. This deliberately keeps "the host exists" separate from "somebody has entered and observed that host."

Successful CONNECT establishes the observation boundary but does not generate detailed articles. The current Erika-K runtime has **speculative background prefetch disabled entirely**: login and forum navigation only use prose-free board state. An explicit leaf-board read triggers up to 10 root headers on first observation; articles retain their simulated historical timestamps.

Do not eagerly update other hosts merely because one host was observed. A directory may contain hundreds or thousands of hosts while only connected hosts pay the expensive catch-up cost.

A typical flow is:

```text
observation request
 -> determine required world scopes
 -> load persisted facts + last_simulated_at
 -> acquire generation/update lease for each stale scope
 -> generate coarse catch-up events for elapsed time
 -> materialize only details required by the observation
 -> validate
 -> DB COMMIT
 -> release lease
 -> render the already-committed result
```

For long inactive periods, do not replay every hour/day. Compress elapsed time into a bounded number of important state transitions, summaries, schedules, and statistical outcomes. Expand recent or directly observed details only as necessary.

Example: if a host has not been observed for three months, first determine durable facts such as membership changes, important disputes, SYSOP actions, new boards, closures, or major relationships. Only generate individual recent posts needed for the user's current view.

The service should therefore *appear* as though the world continued while nobody watched, without paying to continuously materialize unobserved detail.

### Blocking read barrier

The current Erika-K flow does not pre-generate articles after CONNECT. A demanded board read may use an internal shared worker while the command waits, but the host program must not expose "generation in progress" as an in-world fact. It renders only after required headers have been committed.

The current split is:

```text
successful CONNECT
 -> establish observation boundary without article generation

navigation into a forum
 -> display prose-free board metadata without article generation

board/article index request
 -> this board's headers ready? yes: render immediately
 -> no: wait only for this board's shared observation job, then render

article/thread read
 -> body ready? yes: render immediately
 -> no: start/join the shared thread body job, wait, then render
```

Concurrent users join the same board/thread job rather than launching private generation. A ready board must never wait on unrelated work. No speculative header generation is initiated by the current Erika-K runtime; a read of an empty board starts only its demanded shared job, capped at 10 root headers initially. Internal workers coordinate callers but do not constitute speculative prefetch. Article prose remains lazy after headers are observed.

`ALLBODY`, progress polling, and explicit generation status remain development diagnostics only; ordinary host runtimes should not require the caller to refresh a menu to discover that generation finished.

### Shared history, not per-user worlds

Lazy generation is shared. The first observation that materializes a previously unknown event commits it as world history. Later observers see the same persisted result.

Do not generate a private alternate past for each viewer. Personal visibility, permissions, unread state, and private mail can differ per member, but the underlying host/person/event facts remain shared unless the product explicitly models secrecy or conflicting testimony.

### Concurrent observation and generation leases

Two users may observe the same stale scope at nearly the same time. They must not independently generate incompatible futures.

Use a per-scope generation/update lease, transactional lock, compare-and-swap version, or equivalent serialization mechanism. A conceptual scope record may include:

```text
last_simulated_at
simulation_version
generation_lease_owner
generation_lease_expires_at
```

Only one writer may commit catch-up for the same scope/version. Competing requests must re-read the committed result rather than persist a second branch.

The lock granularity should be as narrow as practical: host, board, thread, persona, or other explicit scope. Do not serialize the entire world behind one global lock.

## Actor model

Personas should have persistent traits and state such as:

- interests and dislikes
- opinions
- ordinary baseline/environment context
- activity schedule
- preferred hosts/boards
- reply probability
- thread-start probability
- lurker tendency
- newcomer openness
- argumentativeness
- relationships
- current goals / ongoing conversations
- public profile vs private facts

`relationships` and `current goals / ongoing conversations` are **derived state over the interaction ledger**, not static persona fields filled at creation. See "Interaction feedback across channels" below and `docs/sdd/SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md`. The current `world.Persona` model has neither yet.

A large proportion of accounts should read rarely, lurk, or be inactive. Online population must not equal active posters.

Treat membership scale and activity scale as different layers:

```text
registered members
 -> plausible recent visitors
 -> current time-window activity candidates
 -> readers / ROM
 -> writers
 -> actual root/reply actions
```

Do not model a 300-member local BBS as a fixed cast of a dozen recurring people.
Conversely, do not feed all 300 members into every planning call. Keep sparse
identity/activity skeletons for the membership population, then deterministically
select a bounded activity window from persisted traits, time and board context.
The current HAKATA evaluation uses an approximately 18% time-window candidate
pool with a floor/cap for small/large hosts; this number is an explicit
simulation heuristic, **not a claimed historical active-user statistic**.

### Baseline, interest, and current salience are different

Do not collapse these three concepts into one field.

**Baseline/environment** is what is already normal in this person's life: usual computer/terminal environment, ordinary BBS membership, habitual commute, normal school/work state, ordinary communication methods, etc. It constrains interpretation and consistency but normally stays unspoken.

**Interest/opinion** describes what a person tends to care about or discuss. It can influence board visitation and routing but does not prove a current event occurred.

**Current salience/event** is the concrete difference/problem/decision/interaction/question/change that may actually justify an action now.

Therefore:

```text
uses X every day       -> baseline, not a post
likes X                -> interest, not proof of a new episode
X failed/changed now   -> possible event/action cause
```

Internal classification labels are especially dangerous when confused with resident vocabulary. A machine family, service category, hobby class, or other taxonomy can be useful to the engine without being something a contemporary actor would name in an ordinary post. Exact model/setup/category wording should surface only when the concrete event requires that distinction and the world has support for it.

This rule is intentionally general: computers, operating systems, BBS usage, games, music, local life, school/work, communication tools and other period-normal culture all follow it.

## Action selection

Do not implement `GenerateReply(humanPost)` as the core loop.

Conceptually:

```go
type Action struct {
    ActorPersonaID string
    Kind           ActionKind
    BoardID        string
    ThreadID       *string
    Motivation     string
    Topic          string
}
```

The world engine determines whether an actor is active, where they go, what they read, and whether they choose an action. Only after an action requiring prose exists should a renderer/LLM produce wording.

A routing domain/interest may constrain where a root action belongs, but **ordinary membership in that domain is not itself an event**. The world/semantic boundary must not mean “communications was selected, therefore invent something surprising about using communications.” If the only available basis is an ordinary baseline condition, ROM/no-op is preferable to manufacturing novelty.

No reply is normal. A delayed reply is normal. NPC-to-NPC discussion is normal. A human post being ignored is normal.

### Probabilistic decision advisors

The World Engine may consult a fast probabilistic model for **behavioral priors** such as whether an already-plausible board visit is likely to become a write opportunity or remain ROM/no-op. That model is an advisor, not a world-authority boundary.

An advisor must not create an event, select a concrete topic, invent a purchase/problem/change, or commit a world fact. The engine combines the advisory probability with persisted persona traits, board affinity, deterministic sampling, topology/cause gates, and other world constraints. Provider failure must degrade to the local deterministic model rather than blocking simulation.

For the Jev behavior advisor, one bounded host/board window is submitted as structured state and the provider returns three independent priors for each persona × board pair: visit/read propensity, write-vs-ROM propensity conditional on a visit, and reply-vs-root propensity conditional on a write opportunity plus an already-valid reply target. Host-wide generation batches all current boards/personas into one Jev request instead of issuing one provider call per board.

The local simulation remains the majority prior. Jev contributes a bounded minority weight (currently 30% for visit activity and 40% for write/reply topology), after which deterministic quota/ranking and causal gates make the actual action decisions. Jev never creates a root cause, thread, reply target, topic, event, purchase, problem, or other world fact. A valid root cause or reply target is still required before a post exists.

Advisory responses are transient operational inputs, not canonical world state. Jev probabilities are quantized to 0.05 steps before entering deterministic sampling so small provider jitter does not routinely alter retry outcomes. Once a resulting action is materialized, the database remains canonical. A future general production world engine should persist or otherwise version advisor snapshots across retry-sensitive simulation leases if advisory decisions extend beyond this bounded development materialization path.

### World-selected roots and canonical subject stability

The shared production pipeline uses only Situation-first planning: World fixes
actor, board, time, cause, discourse mode and reply topology. Every production
board now uses open-topic Situation proposals: no activity-facet selector
chooses topics. The Situation model receives the selected board/member/date/
post-purpose context, the station-owned board purpose, persisted member facts
and previous observed root subjects, then proposes the still-unknown concrete
occurrence for that board.
The validated occurrence becomes canonical before the model words its title.
Those accepted details are committed. Board-level staff-author and fixed
conversational-act constraints belong to World-selected event shells, not
LLM prose; this may narrow *who* can originate a post or *whether* it is a
question without choosing a preset subject. The planner validates all required slots and fails the
batch if generation cannot supply valid subjects; a generic board-name fallback
must not be committed. Once a subject is shown in a board index, body rendering
cannot replace it with the body's generated subject. Host-specific append
semantics remain intact.

## Interaction feedback across channels

Every dynamic interaction — board roots and replies/appends, mail, chat utterances, telegrams, SYSOP actions — is recorded once in an append-only **interaction ledger** with an actor (persona or human member), a visibility scope, addressees, an explicit response link and world time. Board posts are not special; mail and chat are not side channels outside the world.

World derives sparse state from that ledger deterministically and idempotently:

- **observations** — who could know which event (scoped: private mail and chat sessions are visible only to participants);
- **relationship edges** — directional, bounded affinity/trust/tension/familiarity, decayed at read time;
- **open loops** — concrete pending matters owned by an actor: a question addressed to them, an unanswered mail, their own question that got answered, a commitment, an ongoing dispute, a chat topic not yet carried to a board;
- **opinion versions** — opinions change only through an evidenced, bounded `opinion_shift` event, never because an LLM agreed with someone;
- **topic salience** — a probability modifier, never a cause by itself.

These feed the *next* action selection: responder weights (spec 008's relationship term), the choice between an open-topic root cause and an `interaction_followup` cause tied to a source event, mail reply/ignore decisions at the recipient's next simulated login, and per-turn chat behaviour. The same rules apply to human and NPC interactions; ignoring is normal.

This does not contradict "Baseline, interest, and current salience are different": background facts, interests and accumulated history still never trigger posts. An interaction addressed to or involving the actor is a concrete *interaction* salience and may become a candidate cause.

Unobservable channels (NPC-to-NPC mail or chat) are simulated only as coarse prose-free events when their consequences are needed; their prose is never materialized.

Prose context is an **actor-knowledge projection**: a few items filtered by time (before the action), observation and scope compatibility (no private content in a public post unless World selected disclosure). See `docs/sdd/SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md` for the data model, phased implementation plan and acceptance criteria.

## Diegetic present

World simulation and rendering use the world date as the characters' literal present. A later historical interpretation must not retroactively change what actors find ordinary, old, surprising, nostalgic or explanation-worthy.

The simulation must not create fake causal events just to support a later observer's stereotyped idea of the era. In particular, a renderer must not invent hiatuses, rediscoveries, upgrades, purchases, compatibility doubts, new-member growth, maintenance or similar transitions unless world state actually selected/committed them.

`docs/HISTORICAL_ACCURACY.md` defines the historical/diegetic policy in more detail; `docs/LLM_POLICY.md` defines the generation contract.

## Generation budgets and backpressure

World generation is allowed to have explicit operational budgets. The service does not need to materialize arbitrary amounts of history immediately merely because a user connected.

Useful inputs include:

- generation queue depth
- active generation leases
- LLM/provider latency or rate limits
- per-request and rolling token/cost budgets
- host/session concurrency
- database pressure

When the required catch-up would exceed the current budget, prefer one or more of these strategies before generating unnecessary detail:

1. compress a longer elapsed period into fewer coarse events;
2. materialize only what the current screen/action requires;
3. defer nonessential background detail until a later observation;
4. use a cheaper generation class when policy permits;
5. apply diegetic backpressure through the telephone/host experience.

Diegetic backpressure includes a genuinely occupied or generation-locked line returning `BUSY`, accepting fewer simultaneous calls, or presenting slower host output / a lower supported connection tier where historically plausible. These outcomes should be driven by real runtime state and policy, not arbitrary punishment or a hidden random throttle.

The terminal/modem-facing rules for this behavior live in `docs/TERMINAL_AND_MODEM.md`.

## Local communities

Reputation and moderation are primarily host-local. There is no single global social score.

Neighboring or socially connected hosts may share users, SYSOP relationships, rumors, offline meetings, referrals, and software traditions. Information can propagate through these edges without making the network globally searchable.

## Region

Region biases distributions; it does not dictate personalities or stereotypes. It may affect host density, likely line count, equipment upgrade pace, local topics, area-code/toll relationships, demographic mix, and cross-host topology.

## Lines and presence

Logical telephone lines are world resources. Human sessions and virtual users can occupy them. BUSY should eventually emerge from real logical occupancy plus host policy/scheduling rather than being merely a cosmetic random outcome.

Generation/update leases and bounded processing capacity may also temporarily make a logical line unavailable when admitting another caller would require conflicting or over-budget world materialization. Treat this as explicit backpressure tied to real state.

Time-of-day, popularity, member schedules, special events, and Telehodai windows can influence occupancy.

Since spec 015 this is implemented for hosts whose store supports presence
(`internal/presence`, `worldrepo/presence.go`). Who is on a host's lines at a
world time is computed deterministically, without an LLM, from 10-minute call
windows over the last four hours. Each resident's chance to call is a product of
independent layers: `ActivityPattern`, a lifestyle daily rhythm (weekday or
holiday), Telehodai registration for that host (with a rush right after 23:00
and hang-ups before 08:00), a phone line shared with the family, a night-owl
factor, the calendar (`internal/worldcalendar`), and the station's optional
`operating_hours`. Every coefficient is a fictional tuning value, not a
historical statistic. Calls are placed on the lowest free line; a resident who
finds every line taken simply got BUSY.

The computation itself saves nothing. When a caller observes it (a dial's
BUSY/CONNECT or a WHO screen), the resident sessions and their connection
profile facts (`presence.*` persona facts) are stored and never rewritten; later
computations keep them. Human calls hold a line from CONNECT until hang-up or
reconnect-grace expiry, and are kept as history when they end. Stores without
presence keep the earlier probabilistic BUSY model.

### Within a call: chat rooms and line messages (spec 016)

This policy is host-independent. Each host program realizes it with its own
screens; Erika-K's telegram and chat screens are the first realization (see
`docs/host-programs/erika-k.md`). No layer assumes a number of lines: the line
count, the rooms and their capacities come from the station definition.

- **What a resident does within a call** is a plan of segments in a shared
  vocabulary, `world.CallState`: `login` (the first two minutes), `idle`,
  `paging` (at the screen for sending line messages, just before a chat) and
  `chat` (in a room). Host programs map it to their own labels (Erika-K:
  `paging` → `CALL`, `chat` → `CHAT`). `internal/callplan` computes the plans
  deterministically, without an LLM: a base intention from the resident's
  lifestyle and sociability (`1 − LurkerTendency` and `ReplyTendency`), tripled
  in Telehodai hours, adjusted by how crowded the room looks (counted from the
  other calls' base intentions only, so the computation never recurses), and
  never above a room's capacity. Every coefficient is a fictional tuning value.
- **Humans never enter the computation.** A resident's plan is the same whether
  or not a human is on the host or in the room.
- **Observation fixes a plan.** The first time any part of a call's plan is
  shown (a WHO row, a join or leave line), the whole plan is stored with the
  presence session and never rewritten (`worldrepo/callplan.go`).
- **Chat rooms** are a shared item of the station definition
  (`detail.chat_rooms`: number, capacity with 0 for unlimited, and a choice
  weight). A station without rooms has no chat. Residents enter rooms only
  while the server can also give them words (`SetResidentChat`, on when a
  language model is configured); otherwise their calls have no `chat` segments.
- **Asynchronous output.** A host program may implement
  `hostprogram.AsyncOutput`; the WebSocket layer polls it once a second with
  the world time and sends what it returns. The program decides when an
  interruption is appropriate (Erika-K: only inside a chat room).
- **Records.** What a human says in a room is stored as a `world.ChatUtterance`
  and entered in the interaction ledger (`chat_utterance`, audience
  `chat_room:<host>:<room>`, addressed to the residents in the room at that
  moment). A message a human sends to a line is a `world.LineMessage` with a
  delivery time the host program decides (`line_message`, audience
  `private:<host>`, addressed to the recipient). Both survive restarts with the
  host snapshot.

- **Residents speaking (spec 017).** `internal/chatslot` decides, without an
  LLM, each chance a resident has to say a line (a slot): `greet` on entering,
  `welcome` someone who entered, `reply` to a line, `talk` on their own,
  `farewell` before leaving, `sendoff` someone who says they are leaving, and
  `news`, bringing up a recent board event (a post, or a reply to a post) of an
  acquaintance in the room. At most one resident welcomes an entry, replies to
  a line or brings up an event; a line that says its speaker is leaving gets
  up to two send-offs instead of a reply. Silence is always possible. The
  chances come from the persona, spec 013's relationship and whether a handle
  was named; chains of replies, lines per minute and per hour are limited.
  Every value is a fictional tuning value, set high for now while the chat is
  being debugged.
- **Lines exist only while observed.** Slots are computed and worded only for
  times a caller is in the room (`ChatHost.EnterChatRoom` / `LeaveChatRoom`;
  a stay with no poll for a minute counts as ended). Unheard conversation is
  not a world fact. Polling a room (`ChatEvents`) returns the recorded lines
  with the joins and leaves and starts writing the lines due within twenty
  seconds; an optional `llm.ChatLineWriter` words a selected slot, and a
  failed or rejected line is silence, never canned text. A recorded line is a
  `world.ChatUtterance` with its slot and the line it answers, entered in the
  ledger as `npc_world_selected`, and is never rewritten or placed in a window
  already shown.

Implemented: residents' states on the lines, joins and leaves in chat rooms,
the human's own utterances and line messages as world records, and residents
speaking in rooms.

Planned, not implemented yet: replies to line messages and messages from
residents (spec 018), chat updating relationships, and residents posting and
replying while they are on line. Who is active keeps changing: members log off when their call
ends, which simply frees the line, and others connect in later windows. Chat
partners and the authors of such activity are chosen from the presence computed
above, never invented because the human is there; the World decides whether
each one speaks, posts, or does nothing, and an LLM only words an action that
was selected (see SDD-005 section 3.4).

## Moderation

Misbehavior can occur: spam, arguments, impersonation, local-rule violations, escape-code abuse, etc. SYSOP personality and local policy determine warnings, deletion, read-only restrictions, temporary suspension, or removal. The human-controlled member is subject to the same local rules.

## Time

Use a `WorldClock` abstraction. Real Japan time can drive time-of-day behavior while dates map into the 1996 world. Tests should be able to jump across important boundaries such as 22:59, 23:00, and 08:00.

Since spec 015 the default clock follows the real calendar. Each world has a
base year that corresponds to the real year in which the world was started; the
world time is the real Japanese time minus that many years. Month, day and time
of day match the real calendar, restarts do not move the world date, and the
world crosses New Year together with the real calendar (real 2027-01-01 is
1997-01-01). The base year is a property of the world, chosen when the world is
created (worlds are planned per user or group; the allowed range is not decided
yet). It is not a deployment setting. Until per-world creation exists, the single
world uses base year 1996 started in 2026 (`worldclock.CurrentWorldBaseYear` on
the server, `CURRENT_WORLD_BASE_YEAR` in the web client). An explicit
`WORLD_DATE` / `VITE_WORLD_DATE` keeps the older fixed-origin development clock.

Known limitation: a real 29 February that maps onto a common world year is
normalized to 1 March, so that world day occurs twice (first in real 2028).
