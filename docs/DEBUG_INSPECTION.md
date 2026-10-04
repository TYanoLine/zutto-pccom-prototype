# Generated article inspection

The restricted diagnostic endpoint
`GET /api/debug/bbs/sample?phone=0920000196&board=70/1`
shows the current PC-98 board's generated headers. It requires the server's
`DEBUG_RESET_TOKEN` as the `X-Zutto-Debug-Token` request header.
Use Render operational logs and normal terminal reads to distinguish active
generation, failed generation, and successfully committed titles.

This is inspection of ordinary World-owned posts, not a separate generation
engine. The old general-purpose `/api/debug/world` export was removed.

## Automatic generated-content logs

The production observation path logs **saved** world-engine posts for hosts
whose station flags opt in to
Render as one-line JSON prefixed with `BBS generated content:`. This restores
quality inspection without bringing back Lab, public export APIs, or another
generation path. `header_committed` contains board, post ID, author, visible
subject, Situation kind/summary, and canonical Situation facts. `body_committed`
contains the same board/post ID, visible subject, and **complete article body**
after the lazy body has successfully been saved. Search that prefix and join
records by `host`, `board`, and `post_id`; reply/append records may correctly
have an empty host-native subject.

`DEBUG_LOG_GENERATED_CONTENT=1` is enabled by default. Set it to `0` to disable
content logging.
This logger ignores all human posts and all other stations, including their
normal operational timing/error telemetry. It does not log draft Situations,
unsaved bodies, model prompts, API credentials or private persona profiles.
The generated text is still world content: limit access to Render logs and
turn off this setting before enabling real user content.

A header record means the post exists in the canonical store. It is not a
claim that all ten slots succeeded. Body records appear only as articles are
opened (and not on repeated reads of already materialized bodies). Historical
Article Detail logging remains a separate opt-in diagnostic; Situation-first
root headers normally have their detail completion bit set upstream.


## Title-led body trial

When `GENERATION_FREEFORM_BODY=1` (default), the article worker receives the real board.Name, the saved subject, the author and a concise accepted Situation summary. Replies also retain relevant parent text, reply purpose and required referents. The underlying persisted facts are unchanged.

For hosts with `generation.freeform_body`, this mode skips extra Article Detail and historical-evidence research before prose. The trace therefore shows initial Situation and title calls and each body call/retry, but no invented placeholder for the skipped stages. All other stations retain their existing process. Set `GENERATION_FREEFORM_BODY=0` and redeploy to restore the earlier process for subsequent article reads.

## Live generation inspector (temporary evaluation mode)

While connected to **HAKATA CANAL NET**, the modern browser application shows
a small **生成ログ** button (desktop top bar / mobile status bar). Open it to
inspect the current trace immediately; no key-entry dialog is necessary.
Capture and polling start automatically for an opted-in evaluation host.

`GET /api/debug/bbs/generation-trace` is a **public, unauthenticated**, read-only,
no-store endpoint while `DEBUG_GENERATION_TRACE` is enabled (default `1`, set
`0` to disable and return HTTP 403). No other host's generation is captured.
The former HAKATA-prefixed environment names (such as `DEBUG_HAKATA_LLM_TRACE`)
are no longer read; a deployment that still sets one of them gets the default.
It never starts generation or exposes a new Lab API. The existing
`DEBUG_RESET_TOKEN` still protects destructive BBS resets and article sample
inspection; it does not control this trace endpoint.

The panel polls every 2.5 seconds while an opted-in host is connected and displays each
in-progress or recent board-header/body operation, then each *actual* Azure
OpenAI model call within it:

- **Situation**: complete prompt with World-selected slots and activity focus,
  and the provider's raw output text or API error. Retries appear separately.
- **件名**: complete prompt using accepted Situation and its raw title output.
- **Article Detail / Article Detail 再検索**: only when older/reply state
  needs additional facts; search retries appear as separate steps.
- **記事本文**: complete article-worker prompt and its raw body JSON output,
  including each retry.

The run-level status marks upstream validation failures as **failed**, even
when an earlier model call successfully returned text. A successful model
response is only `応答受信`, **not** a claim that World accepted its content.
An unavailable output text appears with its provider error instead of a
fabricated result. The already-existing `BBS generated content:` operational
log records what actually reached the canonical store and remains the reference
for committed subject/body comparison.

Only the latest 12 runs and 30 model calls per run are held in a process-local
bounded buffer. Individual prompt/output fields are capped at 24,000/36,000
Unicode code points and marked when truncated. Reloading the server clears
all traces. This display is a *modern development inspector*, not a historical
host-program screen. **WARNING:** anyone who knows or discovers the public
server URL can retrieve the trace while enabled. Existing posts (including
human replies/quotes), persona facts, and thread context may appear in model
prompts. This mode is only appropriate for controlled quality evaluation. Set
`DEBUG_GENERATION_TRACE=0` before enabling real user access. Do not mistake the
absence of the browser link outside an opted-in host for
server-side access control.
