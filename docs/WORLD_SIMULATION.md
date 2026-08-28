# Persistent world simulation

## Core model

The world is a persistent network of hosts, people, lines, posts, relationships, and events. The human is a participant in that world, not the trigger for all activity.

The database is canonical truth. Generation fills unknown facts; observation/materialization turns those facts into persistent history.

## Observation and lazy generation

An unknown phone number may resolve to no host or lazily generate a host according to stable world rules. Generated facts should be reproducible where possible and persisted once materialized.

Unobserved details do not need continuous fine-grained simulation. Catch up inactive hosts using coarse events, schedules, and statistical activity; materialize detailed posts/events when they become relevant.

Never silently rewrite an already observed host, persona, relationship, post, or historical event simply because a later LLM call would prefer another answer.

## Actor model

Personas should have persistent traits and state such as:

- interests and dislikes
- opinions
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

No reply is normal. A delayed reply is normal. NPC-to-NPC discussion is normal. A human post being ignored is normal.

## Local communities

Reputation and moderation are primarily host-local. There is no single global social score.

Neighboring or socially connected hosts may share users, SYSOP relationships, rumors, offline meetings, referrals, and software traditions. Information can propagate through these edges without making the network globally searchable.

## Region

Region biases distributions; it does not dictate personalities or stereotypes. It may affect host density, likely line count, equipment upgrade pace, local topics, area-code/toll relationships, demographic mix, and cross-host topology.

## Lines and presence

Logical telephone lines are world resources. Human sessions and virtual users can occupy them. BUSY should eventually emerge from real logical occupancy plus host policy/scheduling rather than being merely a cosmetic random outcome.

Time-of-day, popularity, member schedules, special events, and Telehodai windows can influence occupancy.

## Moderation

Misbehavior can occur: spam, arguments, impersonation, local-rule violations, escape-code abuse, etc. SYSOP personality and local policy determine warnings, deletion, read-only restrictions, temporary suspension, or removal. The human-controlled member is subject to the same local rules.

## Time

Use a `WorldClock` abstraction. Real Japan time can drive time-of-day behavior while dates map into the 1996 world. Tests should be able to jump across important boundaries such as 22:59, 23:00, and 08:00.
