# Historical knowledge research PoC

## Purpose

Historical and cultural knowledge is global service knowledge, not per-user world state. A fact researched for one world should be reusable by every world whose mapped date and context permit it.

This PoC adds a persistent operator-review workflow without making historical research part of a host-program runtime.

## Current PoC flow

1. An operator opens `/admin/research`.
2. A topic/question is submitted.
3. The server asks the Azure OpenAI Responses API to research it with the built-in `web_search` tool.
4. The model must return a usable provisional answer, confidence, and a list of information it could not verify.
5. Search citations returned by the Responses API are stored with the case.
6. The whole case is persisted in PostgreSQL and is global rather than tied to a generated user world.
7. Low-confidence/incomplete research is marked `needs_review` rather than blocking world generation.
8. The operator may chat with the research agent. Each message causes another web-backed research pass using the existing case as context.
9. The operator may replace the provisional answer with their own supplement and mark it `operator_verified`.
10. The operator may finally promote it to `canonical` or reject it.

## Status meanings

- `provisional`: automated research found no important gap and confidence is at least the PoC threshold.
- `needs_review`: important information is missing, confidence is low, or automated research failed.
- `operator_verified`: the operator supplied/approved the usable content.
- `canonical`: approved global historical knowledge suitable for normal world use.
- `rejected`: should not be used.

## Provenance rule

Do not erase uncertainty. Sources, confidence, missing information, AI research messages, and operator supplements remain separate fields/history in the research case.

An operator supplement is not retroactively represented as a web-confirmed fact.

## Security

For the PoC only, the admin research APIs reuse `DEBUG_RESET_TOKEN` via the `X-Zutto-Debug-Token` header. Production should use a distinct authenticated operator identity/authorization layer.

## Deliberate limitations

This PoC does **not** yet connect research cases to the WorldEngine/materializer automatically. It proves the research/review/persistence loop first.

Before production integration, add:

- normalized canonical fact/event/cultural-signal tables separate from research cases;
- temporal validity (`known_at`, `announced_at`, `occurred_at`, `released_at`, etc.);
- source quality/evidence classes matching `HISTORICAL_ACCURACY.md`;
- deduplication and contradiction detection;
- a read-only HistoricalKnowledge service for WorldEngine/materializers;
- policies controlling whether provisional knowledge may be used;
- operator authentication separate from debug tooling.

Host-program runtimes must remain unaware of AI/web research. They should continue to read world/domain data through normal stores/services; missing global historical knowledge is resolved below that boundary.
