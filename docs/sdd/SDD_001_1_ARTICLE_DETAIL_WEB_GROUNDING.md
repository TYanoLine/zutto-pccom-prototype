# SDD-001.1: Web-grounded Article Detail concretization

Status: experimental implementation
Updated: 2026-09-28

## Goal

Keep accepted BBS subjects unchanged while allowing Article Detail generation to become concrete enough that the resulting article/append clearly refers to a plausible 1996-era situation.

The Article Detail planner may use OpenAI Responses API built-in `web_search` when external historical facts are needed.

## Relationship to SDD-001

SDD-001 remains the canonical-before-prose contract.

This extension changes only how Article Detail obtains historically safe external facts. It does not change title generation, post topology, persona-history persistence, or social-memory semantics.

## Behavior

```text
accepted subject + selected actor + persona facts/history + thread context
    -> Article Detail planner
         -> use fictional article-local facts directly when no external history is needed
         -> optionally use web_search when a real work/product/person/place/spec/event is needed
         -> repair an incompatible/unsupported idea into a historically valid expression
    -> canonical Article Detail
    -> prose worker
```

The model receives `web_search` with `tool_choice=auto`. Search is therefore available but not mandatory for purely fictional article-local detail.

## Historical grounding policy

- A fictional person's one-off experience, timing, reaction, comparison, or decision may be created without Web search if it does not assert external historical facts.
- When the detail names or relies on a real work, product, person, company, place, event, release status, specification, story element, price, or other external historical fact, the planner should search rather than rely only on model memory.
- If an initially considered concrete candidate does not fit the post timestamp or cannot be supported, do not fail the article or fall back to vague prose by default.
- Search success is not itself evidence that every remembered property of the target is correct. Before returning an external-history detail, the exact proposition in that detail must be supported by the search result used for grounding.
- Do not merge similarly named enemies/items, platform/version differences, ports, sequels, or separate works. If only the target's existence is supported, keep game/story/specification claims out of the canonical detail. Preserve the selected article intent and repair the candidate into:
  1. a supported historically valid target/fact,
  2. a supported narrower statement, or
  3. a generalized expression only when a safe concrete alternative cannot be established.
- Web search results, URLs, citations, and the fact that a search occurred are operational evidence only. They are never diegetic BBS content.
- Only the final structured Article Detail is committed as world state.

## Referent detail

Article Detail adds the `referent` kind for a concrete real-world target of the post/reply.

Example:

```text
subject:
  エンディングを見た人へ

possible grounded details:
  referent: 今回話している作品はゲーム「...」である
  reaction_context: X68.Vはその作品を最後まで遊び、最後のある場面の受け取り方が気になった
```

The final prose worker may use a canonical `referent`; it still may not introduce additional unsupported external facts. Verification dates, source URLs, and phrases such as "existence confirmed" remain diagnostic metadata and must not leak into the diegetic detail.

## Reply behavior

A reply should inherit the current thread context. If the root already established a canonical concrete referent, the reply normally uses that context rather than independently choosing another work/product.

Web search remains available when the reply needs an additional external fact that is not already canonical.

## OpenAI integration

The Article Detail structured Responses API call uses:

- strict JSON Schema output;
- `tools: [{"type":"web_search"}]`;
- `tool_choice: "auto"`;
- medium reasoning effort after the first live sample showed a searched but unsupported game-specific claim;
- a 60 second Article Detail deadline.

The application records the number of `web_search_call` output items and returned source URLs as non-world diagnostic metadata. When `DEBUG_LOG_BBS_ARTICLE_DETAILS=1`, it also logs the final validated Article Detail payload immediately before persistence so a grounding error can be distinguished from a later prose-worker invention.

## Forced-search retry experiment

The first pass continues to use `tool_choice=auto`.

During the current GAME-board experiment, if all of the following are true:

- the article is a semantic root (`discourse_mode=thread_start`);
- the first Article Detail pass made zero Web searches; and
- that root still has no `referent` detail,

the planner performs one bounded retry with `tool_choice=required`.

The retry must search at least once and is instructed to select one historically valid real work as the root's concrete referent while preserving the already-selected article intent. Unsupported work-specific boss names, stage names, mechanics, plot facts, numbers, and version details remain forbidden unless directly supported by the search evidence.

Replies do not trigger this retry merely because they lack a referent; they are expected to inherit the root/thread context.

This is intentionally scoped to GAME-board evaluation first. The trigger should be generalized only after live samples show that the rule improves concrete naturalness without forcing irrelevant proper nouns into ordinary roots.

## Failure behavior

A provider/tool/schema failure follows the existing bounded Article Detail retry path. Failed detail materialization does not commit partial details.

The experiment must not silently fall back to an ungrounded concrete real-world claim.

## Evaluation

For newly materialized samples, inspect:

- whether abstract titles become concrete in the body;
- whether real named referents are plausible at the post timestamp;
- whether the model actually invokes Web search when external history is needed;
- whether obviously fictional/local details avoid unnecessary searches;
- whether replies preserve the root referent and add a distinct contribution;
- latency and token/search-tool cost;
- any cases where search evidence was available but the final detail remained generic.

This is an experiment. Search policy and reasoning effort may be adjusted after observing real samples.
