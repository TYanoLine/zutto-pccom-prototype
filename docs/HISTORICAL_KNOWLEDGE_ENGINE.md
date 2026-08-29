# Historical Knowledge Engine

## Purpose

The Historical Knowledge Engine supplies historically bounded reference facts to the WorldEngine without turning every in-world generation into a web-research job.

The core rule is:

**atmosphere is model-first; persistent concrete historical claims require evidence.**

The world engine remains responsible for deciding what happens. Historical research only answers questions such as what existed, what was available, what terminology was current, or what was culturally plausible at the mapped world date.

## Flow

```text
WorldEngine
  -> EvidencePolicy
       atmospheric -> model knowledge / era rules only
       plausible   -> shared KB if present; model-first fallback
       verified    -> shared KB required
                        -> if insufficient: bounded Research Sub-Agent
                        -> HistoricalFact + ResearchCase
                        -> operator review queue when uncertain
```

The Research Sub-Agent never decides that an NPC bought a modem, that a SYSOP upgraded a line, or that an event occurred in the fictional world. It only provides evidence usable by WorldEngine decisions.

## Evidence levels

### Atmospheric

Ordinary prose, mood, greetings, casual seasonal conversation, and non-critical background details. No Historical KB lookup or web search is required.

### Plausible

Specific enough that existing shared facts can improve the result, but not important enough to block the world. The engine queries the KB when appropriate. If the KB has no useful result, generation may proceed from model knowledge and era rules. Synchronous web research is intentionally not triggered.

### Verified

Persistent claims where getting the historical boundary wrong would corrupt world state. Examples include exact product availability, dates, prices, protocol capabilities, line-speed support, or other exact technical claims. A sufficiently supported shared fact is required. If absent, bounded research is triggered.

## Shared facts vs research cases

`historical_facts` is the reusable global knowledge base. It is what WorldEngine queries.

`historical_research_cases` records how a fact was investigated, unresolved gaps, confidence, source URLs, and operator conversation. It is a maintenance/audit object rather than world state.

A ResearchCase can produce a provisional or verified HistoricalFact. Operator supplementation promotes the linked fact to `operator_verified`; explicit canonical approval can promote it to `canonical`.

## Query identity and deduplication

A `KnowledgeQuery` contains a structured kind, subject, world date, region, audience, need, and required evidence level.

The shared `KnowledgeKey` deliberately excludes free-form `Need` wording so semantically identical requests with different prose converge on one historical slice. It includes kind, subject, world date, region, and audience.

`historical_research_leases` prevents multiple worlds from launching the same research job concurrently. A lease expires so crashed jobs do not block the key permanently.

## Research budget

The OpenAI Responses API research adapter is bounded by default:

- one Responses job per research acquisition;
- web search tool enabled only in the research adapter;
- `max_tool_calls = 4`;
- `max_output_tokens = 1800`;
- HTTP timeout of 60 seconds;
- medium web-search context.

These are adapter defaults and can later move to configuration. Routine in-world prose must not receive the web-search tool.

## Failure behavior

- Atmospheric generation never depends on research availability.
- Plausible generation may degrade to model-first output if knowledge retrieval is unavailable.
- Verified persistent facts must not silently become invented facts if research fails or another research job is still pending.
- Uncertain research is persisted and surfaced for operator review rather than hidden.

## WorldEngine boundary

`internal/worldengine.Engine.ResolveEvidence` is the current domain boundary. Host-program runtimes should not call OpenAI web search or HistoricalKnowledge directly. Host programs request world/domain data; WorldEngine decides evidence requirements and HistoricalKnowledge supplies historical context.

The current repository still has prototype host/world storage shortcuts. This engine establishes the production boundary for future board/article/persona materialization without flattening host-program-specific UI or state machines.
