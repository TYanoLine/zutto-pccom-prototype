# SDD-001.1: Web-grounded Article Detail concretization

Status: experimental implementation
Updated: 2026-09-28

## Goal

Keep accepted BBS subjects unchanged while allowing Article Detail generation to become concrete enough that the resulting article/append clearly refers to a plausible 1996-era situation.

The Article Detail planner may use Azure OpenAI Responses API built-in `web_search` when external historical facts are needed.

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

## Azure OpenAI integration

The Article Detail structured Responses API call uses:

- strict JSON Schema output;
- `tools: [{"type":"web_search"}]`;
- `tool_choice: "auto"`;
- medium reasoning effort after the first live sample showed a searched but unsupported game-specific claim;
- a 60 second Article Detail deadline.

The application records the number of `web_search_call` output items and returned source URLs as non-world diagnostic metadata. When `DEBUG_LOG_BBS_ARTICLE_DETAILS=1`, it also logs the final validated Article Detail payload immediately before persistence so a grounding error can be distinguished from a later prose-worker invention.

## Semantic referent requirement and forced-search retry

The first pass continues to use `tool_choice=auto`.

Each Article Detail result also returns non-diegetic control metadata:

- `referent_requirement=required|optional|none`
- `referent_status=already_in_context|resolved|unresolved|not_applicable`
- `referent_grounding=external_history|world_local|inherited_context|not_applicable`

`external_history` is a real-world work/product/person/company/service/business/etc. whose period correctness must be grounded with Web evidence when used. `world_local` is a private, anonymous, or simulation-local target such as a nearby unnamed restaurant, a local shopping street, or the actor's own file; it must not be replaced with a conveniently searchable real-world entity. `inherited_context` is for replies reusing a canonical thread referent and does not require redundant research unless the reply introduces a new external historical fact.

The requirement is determined from the semantic content of the accepted subject, summary, and thread context. Board names and board categories are context only and must never be hard-coded as the trigger. This is required because boards and their names may be generated independently for each host.

`required` means the post describes or asks about a specific external instance whose identity changes the truth of the concrete experience/opinion/question. A useful counterfactual test is: if the assumed target were replaced with another work/product/place of the same broad category, would the claimed experience remain the same fact? If not, a referent is required even when the subject omits its name. Examples include one specific boss fight, episode/scene, song, magazine issue/bonus, software behavior, or product operation. Category-wide advice, recommendation requests, and open-ended lists are normally optional; ordinary personal/local discussion with no external target is none.

The request carries an explicit `is_reply` flag; discourse-mode labels are not used to infer topology.

The planner performs one bounded retry with `tool_choice=required` only for `referent_grounding=external_history` when the external referent is still unresolved, or when it is marked as already present/resolved but the first pass performed no Web search. World-local and inherited-context referents never trigger Web search merely to find a real-world substitute.

If the specific referent is already present in the subject/summary/thread context, the retry must verify and preserve that referent rather than replacing it with another famous period-appropriate target. If the referent is omitted, the retry may select a historically valid target only when it naturally makes the already-selected article intent true; satisfying the date constraint alone is not enough.

The retry must search at least once. Unsupported target-specific boss names, stage names, mechanics, plot facts, numbers, and version details remain forbidden unless directly supported by the search evidence.

Replies do not trigger a forced search merely because they rely on a referent; they normally inherit the already-canonical root/thread context. They may still use optional Web search in the first pass when they introduce a new external fact.

The semantic requirement/status fields are operational metadata only and are not persisted as fictional world facts or rendered in BBS prose.

### Required-referent commit invariant

`required` is now a hard commit invariant, not merely diagnostic metadata. A final Article Detail result may use `unresolved` only as an intermediate first-pass state that leads to recovery; it must not be persisted as materialized Article Detail.

Before canonical persistence:

- a required root must have `referent_status=resolved|already_in_context`;
- it must have non-`not_applicable` grounding and an explicit `referent` detail;
- a required reply may omit a new referent only when it is `already_in_context/inherited_context`;
- placeholder labels such as 「題名不詳」「作品名不明」「某作品」 do not count as resolving a referent;
- a required root returned as `unresolved/not_applicable` enters the bounded recovery path instead of silently becoming prose.

This closes the failure mode where the system correctly recognized that a post needed one specific work/product/issue, but then persisted only a paraphrase such as 「次号では何か手掛かりが出そう」 without identifying what the post was about.

### Root-title semantic anchoring

Title generation and title-slot fit must not rely on Article Detail to invent a missing target after adoption. On a broad board, a root title that presupposes one specific work/product/issue/episode/character/etc. but does not identify it, and is not made unambiguous by board scope or recent shared BBS context, is not a suitable root candidate. Short titles remain valid when the surrounding canonical context genuinely makes the referent unique; the rule is about missing semantic anchors, not about forcing modern descriptive headlines.

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


## Required referent rendering

A semantic root may have a validated canonical referent that is necessary to understand the post while the accepted/display subject remains intentionally terse.

When a non-reply Article Detail result has `referent_requirement=required` and a resolved `referent` detail, materialization persists an operational rendering control `article_referent_required=<referent>`.

The prose worker must ensure the visible post identifies that referent at least once across the display subject or body. If the display subject already names it, the body need not repeat it. If the display subject remains generic, the body must name the referent naturally before relying on shorthand such as "あの回", "ボス", or "このソフト".

This is a rendering requirement, not an additional fictional fact. It does not require all Article Detail facts to be enumerated, and it does not apply to replies that can rely on canonical thread context.
