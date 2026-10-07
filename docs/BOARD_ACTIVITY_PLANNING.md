# Board activity planning before article materialization

## Purpose

A board must not appear to have zero history merely because no user has opened it
yet.

The world layer therefore plans cheap, prose-free activity state before any BBS
subject/body generation. Entering a board reveals/materializes part of history
that already exists in the simulated world; it does not create a previously empty
community around the observer.

This document describes the current **experimental reconstruction heuristic**.
The numeric formula and HAKATA board weights are fictional simulation policy, not
historical statistics about Japanese grassroots BBSes or Erika-K.

## Inputs

The current activity plan uses:

- station founding date (`Host.FoundedOn`);
- configured world date/time;
- present member count;
- a deterministic inferred membership-growth curve from a small founding
  membership to the present membership;
- host popularity;
- station-specific board activity weight;
- station-specific board reply tendency;
- a retained-root cap representing the amount of indexable history still kept by
  that board.

The HAKATA experiment assigns activity/reply weights to its concrete boards.
Those values describe **HAKATA CANAL NET fiction only**. They are not Erika-K
defaults. In particular, activity weight does not assert the semantic meaning of
an historically unclear board name such as `夢工房はかた`.

## Membership growth

The current prototype does not ask an LLM to invent membership history.

For a host with a known opening date and present membership, World computes a
stable growth curve:

```text
founding membership
  -> deterministic curved growth
  -> current member count at world-now
```

The curve is integrated into member-days. Board activity is then derived from
member-days × host popularity × board activity weight, with stable station/board
variation.

The resulting state records:

- initial member estimate;
- present member count;
- average member count over the station lifetime;
- growth exponent used by the reconstruction;
- cumulative root/reply counts;
- retained root/reply counts;
- retained-history start time;
- latest planned post time.

This is enough to inspect why a board has a particular amount of history without
materializing any prose.

## Replies belong to threads

`reply_rate` is the board's mean replies per root, not a fixed ratio. Each root
(by its 1-based ordinal on the board) draws its own reply count with
`world.ThreadReplyCount`: a root is unanswered with probability
`exp(-0.55 * reply_rate)`, and an answered root draws a geometric count whose
mean keeps the board average near `reply_rate`. Many threads therefore get no
reply and a few run long (bounded by `MaxThreadReplies`). "No reply is normal"
(`docs/WORLD_SIMULATION.md`) is decided here, by the world, never by the language
model. The retained reply total is the sum over the newest retained roots.

When a board is first observed, the newest retained roots (at most 10) are
materialized together with exactly the replies their own threads drew (at most
`InteractiveInitialReplyLimit` in total). Replies follow their root with a
heavy-tailed delay, never outrun the present, and the thread starter sometimes
returns. These numbers are a fictional reconstruction heuristic, not historical
statistics.

## Observation/materialization boundary

Before article materialization:

```text
Host founded date
+ world-now
+ membership curve
+ board characteristics
        |
        v
BoardActivityState
  total roots/replies
  retained roots/replies
  retained time window
  last activity timestamp
```

No title/body LLM call is needed.

For Erika-K, login preplans activity for all leaf boards. Board menus can therefore
show non-zero root counts even while the host has zero materialized `Post`
records.

When the user actually enters a never-materialized board:

```text
BoardActivityState
        |
        v
World selects exact author/time/root/reply slots
        |
        v
subject/header materialization
        |
        v
body remains lazy until BR/read observation
```

The shared BBS engine realizes the exact retained root/reply totals in the
activity state. This replaces the old HAKATA-only fixed 40-root initial batch.

## Persistence during the experiment

The development runtime computes this state deterministically from canonical
inputs. Menu-time planning is kept off the synchronous Postgres snapshot write
path so opening a board menu does not trigger many small database writes.

Once article materialization writes posts, the normal development snapshot also
contains the associated board-activity state.

During the current experiment it is acceptable for reset/code changes to produce
a different fictional activity history. Production persistence can later move
this state into normalized world tables.

## Host-program boundary

The activity planner decides **how much world activity exists**, not how a host
program displays it.

A HostProgram still owns:

- whether board menus display counts at all;
- what a count means in that host;
- retention/deletion semantics once historically researched;
- article numbering;
- reply/append display;
- unread behavior.

Do not create a fictional shared BBS menu solely because activity state is shared
below the host-program layer.
