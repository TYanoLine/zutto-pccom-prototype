# SDD-003: Claims, contradictions, and social memory

Status: specified, not implemented
Updated: 2026-09-27

## Goal

The world must distinguish "what is true about a person" from "what that person said". This allows coherent conversation about changed preferences, jokes, uncertainty, mistaken statements, and contradictions.

The same model applies to NPC-to-NPC, human-to-NPC, NPC-to-human, and human-to-human-visible world history where supported.

## ClaimEvent

A ClaimEvent is a canonical record that a speaker asserted something in a post. Suggested logical fields:

```text
claim_id
speaker_persona_or_member_id
post_id
semantic_key
value / stance
asserted_at
source_span_or_semantic_provenance
```

A claim does not automatically become a timeless PersonaFact.

Example:

```text
96/03: B says "納豆が好き"
96/08: B says "納豆が嫌い"
```

Both claims remain canonical. The system may later know that the preference changed, or it may leave the apparent contradiction unresolved.

## Human boundary

For a human-controlled member, the service may canonically store "the member said X" and normal BBS interaction facts.

It must not turn an utterance into an asserted private psychological truth beyond what is needed for the in-world service.

## NPC boundary

For an NPC, private/world facts may also exist. Generated speech should normally agree with those facts unless a separately canonical situation permits a joke, lie, uncertainty, social accommodation, change of mind, or mistake.

A prose-worker contradiction with no such world support is a generation error, not spontaneous character development.

## Contradictions

Contradiction is not automatically an error at the ClaimEvent layer.

Later conversation may:

- notice it;
- ignore it;
- ask whether the preference changed;
- interpret it as context-dependent;
- leave it unresolved.

The response choice follows the observing actor's persona and knowledge, not a global "contradiction detected => mention it" rule.

## Retrieval contract

A reply planner may receive a few prior relevant claims only when SDD-004 says the actor could know them.

Claims must never be injected merely because they exist globally in the database.

## Non-goals

- storing every sentence as a claim;
- natural-language inference across the entire archive at every generation;
- treating contradiction detection as moderation or an error for human users.

## Acceptance criteria

- conflicting claims can coexist;
- a claim never overwrites PersonaFact by itself;
- human claims do not become hidden private truth automatically;
- NPC speech that contradicts private canonical state requires explicit world support;
- claim retrieval is actor-knowledge filtered by SDD-004.