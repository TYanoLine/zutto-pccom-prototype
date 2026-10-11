# Persona continuity / social memory SDD overview

Status: active design
Updated: 2026-09-27

## Goal

BBS residents, including the human-controlled member where applicable, should accumulate a coherent past through persisted world state rather than provider chat history.

The design is intentionally split into four specifications so each layer can be implemented and evaluated independently.

1. [SDD-001 Body concretization](./SDD_001_BODY_CONCRETIZATION.md)
   - [SDD-001.1 Web-grounded Article Detail concretization](./SDD_001_1_ARTICLE_DETAIL_WEB_GROUNDING.md)
2. [SDD-002 Persona history feedback](./SDD_002_PERSONA_HISTORY_FEEDBACK.md)
3. [SDD-003 Claims and social memory](./SDD_003_CLAIMS_AND_SOCIAL_MEMORY.md)
4. [SDD-004 Observation and relationship retrieval](./SDD_004_OBSERVATION_AND_RELATIONSHIP_RETRIEVAL.md)
5. [SDD-005 Cross-channel interaction feedback](./SDD_005_CROSS_CHANNEL_INTERACTION_FEEDBACK.md) — generalizes SDD-002–004 from board posts to every dynamic interaction (board, mail, chat, telegram, SYSOP actions), defines how derived state feeds later action selection, and introduces actor-scoped agentic memory retrieval tools.

## Dependency order

```text
SDD-001 concrete article/reply realization
  -> SDD-001.1 optional Web-grounded external-history concretization
      -> SDD-002 facts/history created by committed posts
      -> SDD-003 statements/contradictions become first-class social history
          -> SDD-004 actor-specific knowledge/retrieval stays sparse
              -> SDD-005 one interaction ledger for all channels; relationships /
                 open loops / opinion versions feed the next world-selected action
```

SDD-005 phases P1–P3 (actor identity, ledger, board-derived observation/relationship/open loops) can be implemented before SDD-002/003, because they depend only on world-selected canonical intent and human-authored text, not on extracted history or claims.

The specifications share these invariants:

- PostgreSQL/world persistence is canonical; LLM session history is never memory.
- World/action selection remains separate from prose generation.
- Persona facts/history constrain later generation but do not become automatic topic triggers. A concrete interaction addressed to or involving the actor (being replied to, asked, mailed, a commitment, an ongoing dispute) is different: World may hold it as an open loop and offer it as a candidate cause (SDD-005 §2.3).
- Unknown remains unknown. Do not eagerly fill a person's entire biography.
- Human-controlled members are ordinary world participants, not protagonists.
- Historical claims about real products/works/services remain subject to the historical evidence boundary.
- Lazy materialization must not make history depend on the order in which a human opens articles.

## Rollout discipline

Implement one SDD at a time. Keep the next SDDs documented before implementation so schema/provenance choices made earlier do not block later social-memory work.

SDD-001 is the first implementation target.