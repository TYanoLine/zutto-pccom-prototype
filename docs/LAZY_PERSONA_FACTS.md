# Lazy persona facts

## Purpose

A persona must not be eagerly expanded into every possible life detail when the account is first materialized. Core persona state fixes durable behavioral structure such as age range, occupation, interests, activity tendencies and writing style. Concrete details are materialized only when a world action actually needs them.

This is the same lazy-world principle used for hosts and unobserved history: **unknown is not the same as nonexistent**.

## Development PoC flow

```text
sparse persona skeleton
 -> WorldEngine selects a topic/action
 -> required persona fact is missing
 -> materialize only that fact scope
 -> persist the fact as world truth
 -> select PostIntent claims from persisted facts
 -> for replies, select the parent claims being answered
 -> render prose from Persona + PostIntent
```

Once a persona fact has been materialized, later posts reuse it. A prose renderer must not silently improvise a contradictory durable fact.

## Historical boundary

Persona facts describe fictional residents and may include fictional household/use details where they do not assert external historical specifications. For example, `TAKA uses the same PC-98 for communication and games` is fictional world state.

Claims about real hardware limits, exact product behavior, release dates, prices, protocols or other external historical facts remain subject to `HISTORICAL_ACCURACY.md` and the Historical Knowledge boundary.

## Current PoC scope

The first concrete schema is `pc98_environment`, because the development materialization host exposed thin replies around the topic `通信に使ってる98の構成`.

The PoC lazily materializes a few facts such as shared/dedicated usage, communication-vs-game use, external modem usage, and confidence about DOS startup configuration. These facts are intentionally about the fictional resident rather than exact historical model specifications.

`PostIntent.Claims` records what the actor will actually say. `PostIntent.RespondsToClaims` records which already-committed parent claim a reply is responding to. The LLM verbalizes those facts; it does not decide them.

## Development reset

The materialization demo host provides a `RESET` command. It deletes:

- posts for the development host;
- topic-triggered persona facts for the development host cast;
- development token-usage totals.

It preserves the host profile, board catalog, memberships and sparse core persona skeletons. This exists only to compare PoC behavior and is not a production-world operation.
