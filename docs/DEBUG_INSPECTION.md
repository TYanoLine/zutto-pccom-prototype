# HAKATA article inspection

The restricted diagnostic endpoint
`GET /api/debug/bbs/sample?phone=0920000196&board=70/1`
shows the current PC-98 board's generated headers. It requires the server's
`DEBUG_RESET_TOKEN` as the `X-Zutto-Debug-Token` request header.
Use Render operational logs and normal terminal reads to distinguish active
generation, failed generation, and successfully committed titles.

This is inspection of ordinary World-owned posts, not a separate generation
engine. The old general-purpose `/api/debug/world` export was removed.

## Automatic HAKATA generated-content logs

The production observation path logs **saved** HAKATA world-engine posts to
Render as one-line JSON prefixed with `BBS generated content:`. This restores
quality inspection without bringing back Lab, public export APIs, or another
generation path. `header_committed` contains board, post ID, author, visible
subject, Situation kind/summary, and canonical Situation facts. `body_committed`
contains the same board/post ID, visible subject, and **complete article body**
after the lazy body has successfully been saved. Search that prefix and join
records by `host`, `board`, and `post_id`; reply/append records may correctly
have an empty host-native subject.

`DEBUG_LOG_HAKATA_GENERATED=1` is enabled by default for the current fictional
HAKATA quality-evaluation station. Set it to `0` to disable content logging.
This logger ignores all human posts and all other stations, including their
normal operational timing/error telemetry. It does not log draft Situations,
unsaved bodies, model prompts, API credentials or private persona profiles.
The generated text is still world content: limit access to Render logs and
turn off this setting before reusing HAKATA for real user content.

A header record means the post exists in the canonical store. It is not a
claim that all ten slots succeeded. Body records appear only as articles are
opened (and not on repeated reads of already materialized bodies). Historical
Article Detail logging remains a separate opt-in diagnostic; Situation-first
root headers normally have their detail completion bit set upstream.


## HAKATA live generation inspector (operator-only)

While connected to **HAKATA CANAL NET**, the modern browser application shows
a small **生成ログ** button (desktop top bar / mobile status bar). Open it and
enter the server's existing `DEBUG_RESET_TOKEN` manually. Keep that secret out
of Vite environment variables, URLs, localStorage, screenshots and reports.
The React component retains it **only in memory** until authentication is
cleared or the page is reloaded.

`GET /api/debug/bbs/generation-trace` is a read-only, no-store endpoint using
the `X-Zutto-Debug-Token` header. It returns HTTP 403 unless the configured
debug secret matches. Capture is enabled for this fictional HAKATA evaluation
station when the secret exists and `DEBUG_HAKATA_LLM_TRACE` is enabled
(default `1`, explicitly disable with `0`). No other host's generation
is traced. It never starts generation or exposes a new Lab API.

The panel polls every 2.5 seconds after authentication and displays each
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
host-program screen. Existing posts (especially replies/quotes) or persona
facts may appear in model prompts. Restrict operator access and disable the
trace before exposing HAKATA to real user content.
