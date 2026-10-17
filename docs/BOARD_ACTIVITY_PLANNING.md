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

### Separation of concerns: reply count vs. reactor selection (Spec 008, 021)

Board activity separates reply quantity from reactor identity and rhythm:

1. **How many replies**: Thread reply count is decided by `world.ReplyCountForWithStandard` based on board `reply_rate`, the qualitative liveliness stage `reply_appeal` (0..4), and currently active population count:
   - **Appeal levels (0..4)**: Evaluated by `world.ReplyAppealJudge` in 1 batch call per generation batch using topic situation, base rate, world timestamp, active population, and recent thread lengths. Multipliers: `[0: 0.2, 1: 0.6, 2: 1.0, 3: 1.7, 4: 2.6]`.
   - **Conversion formula**:
     $$\text{time\_factor} = \text{clamp}(\text{active} / \text{standard\_active}, 0.3, 1.5)$$
     $$\text{mean\_replies} = \text{reply\_rate} \times \text{AppealMultiplier}[\text{appeal}] \times \text{time\_factor}$$
     $$\text{silent} = \exp(-0.55 \times \text{mean\_replies})$$
     If uniform RNG $< \text{silent}$, replies = 0; otherwise drawn from a geometric distribution with mean $\text{answered\_mean} = \text{mean\_replies} / (1 - \text{silent})$. The result is bounded by $[0, \min(\text{MaxThreadReplies}, \text{active} - 1)]$.
   - **Persistence**: Saved in `world.ReactionPlan` (`Count`, `Appeal`, `Judged: true`, `Reason`). Once judged, the count is immutable and never re-evaluated on subsequent observations or engine restarts.
   - **Fallback**: If the judge is unavailable, times out, or fails, the engine falls back to `appeal = 2` (baseline multiplier 1.0) and records the fallback in logs.
2. **Who replies and when**: `world.PlanReactions` (for interactive human posts) and `world.PickReactor` (for periodic/catch-up board threads) evaluate candidate personas according to:
   - topic match between post topics and persona interests;
   - 6-layer lifestyle and diurnal rhythm delays;
   - persona traits (`ReplyTendency`, `LurkerTendency`, activity pattern);
   - newcomer-welcoming boosts and relationship edges.
3. **What is written**: Article header materialization assigns the planned actor, timestamp, and subject immediately; body prose is lazily generated by `WaitForArticleBody` when read by an observer.

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
