# Lazy persona facts

## Purpose

A persona must not be eagerly expanded into every possible life detail when the account is first materialized. Core persona state fixes durable behavioral structure such as age range, occupation, interests, activity tendencies and writing style. Concrete details are materialized only when a world action actually needs them.

This is the same lazy-world principle used for hosts and unobserved history: **unknown is not the same as nonexistent**.

## Development PoC flow

```text
sparse persona skeleton
 -> WorldEngine selects a topic/action
 -> reconstruct current thread semantic state
 -> select a conversational move
    (thread_start / answer_and_expand / compare_and_expand / add_new_detail)
 -> select information slots needed by that move
 -> required persona fact is missing
 -> materialize only those fact slots
 -> persist the facts as world truth
 -> commit PostIntent claims + reply/question links
 -> render prose from Persona + PostIntent
```

Once a persona fact has been materialized, later posts reuse it. A prose renderer must not silently improvise a contradictory durable fact.

The important boundary is that conversation may **cause a previously unknown part of a persona to become concrete**, but the LLM writing the visible prose does not get to decide that durable fact on its own.

## Thread semantic progression

A reply is not considered meaningful merely because it has `Re:` or shares a semantic topic. The development PoC reconstructs a small thread state from committed post envelopes:

- which semantic information dimensions have already appeared;
- whether a prior post left a concrete follow-up question unanswered;
- which post/claim/question the next reply is addressing;
- which new information dimensions the reply will contribute.

`PostIntent.ResponseAct` records the conversational job. `InformationSlots` records the dimensions being added. `RespondsToPostID`, `RespondsToClaims` and `RespondsToQuestion` identify the semantic target. `FollowUpSlot` and `FollowUpQuestion` can deliberately leave a new question for a later resident.

When a pending question can be answered by the selected persona, that answer is preferred and the necessary persona fact slot is materialized at that moment. A reply may then add another previously uncovered dimension. If the selected persona cannot answer the pending question, the engine must not pretend that it did or pile another unrelated question on top merely to keep prose moving.

This is intentionally **not** a rule that every reply must add maximum information. Silence, short agreement and repetition remain valid future world behaviors. The PoC uses stronger progression pressure because it is specifically testing the previously observed failure mode where several residents independently said little more than “I use mine that way too.” Production behavior should later calibrate progression rates from historical material rather than force every thread to be productive.

## Historical boundary

Persona facts describe fictional residents and may include fictional household/use details where they do not assert external historical specifications. For example, `TAKA uses the same PC-98 for communication and games` is fictional world state.

Claims about real hardware limits, exact product behavior, release dates, prices, protocols or other external historical facts remain subject to `HISTORICAL_ACCURACY.md` and the Historical Knowledge boundary.

## Current PoC scope

The first concrete schema is `pc98_environment`, because the development materialization host exposed thin replies around the topic `通信に使ってる98の構成`.

The PoC has a small set of topic-specific latent fact candidates for the core demo residents (usage pattern, modem style, startup configuration habits, log retention, shared-machine use, other use and pain points). **They are not all persisted up front.** The semantic move requests one or two slots, and only those slots become persona facts. This fixed candidate set is development scaffolding, not the intended production representation of every possible fact a person can have.

The production direction is to let the world engine generate/materialize a bounded fact only when the conversation or another world action requires it, subject to existing persona state, already-observed facts, historical constraints and deterministic/shared-world rules.

`PostIntent.Claims` records what the actor will actually say. The LLM verbalizes the committed semantic move; it does not decide the resident's durable setup, the question being answered, or the new information that enters the thread.

## Development reset

The materialization demo host provides a `RESET` command. It deletes:

- posts for the development host;
- topic-triggered persona facts for the development host cast;
- development token-usage totals.

It preserves the host profile, board catalog, memberships and sparse core persona skeletons. This exists only to compare PoC behavior and is not a production-world operation.
