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
