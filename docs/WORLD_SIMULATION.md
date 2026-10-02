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
post-purpose context, persisted member facts and previous observed root
subjects, then proposes the still-unknown concrete occurrence for that board.
The validated occurrence becomes canonical before the model words its title.
Those accepted details are committed. The planner validates all required slots and fails the
batch if generation cannot supply valid subjects; a generic board-name fallback
must not be committed. Once a subject is shown in a board index, body rendering
cannot replace it with the body's generated subject. Host-specific append
semantics remain intact.

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

## Moderation

Misbehavior can occur: spam, arguments, impersonation, local-rule violations, escape-code abuse, etc. SYSOP personality and local policy determine warnings, deletion, read-only restrictions, temporary suspension, or removal. The human-controlled member is subject to the same local rules.

## Time

Use a `WorldClock` abstraction. Real Japan time can drive time-of-day behavior while dates map into the 1996 world. Tests should be able to jump across important boundaries such as 22:59, 23:00, and 08:00.
