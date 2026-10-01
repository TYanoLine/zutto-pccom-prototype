# HAKATA article inspection

The restricted diagnostic endpoint
`GET /api/debug/bbs/sample?phone=0920000196&board=70/1`
shows the current PC-98 board's generated headers. It requires the server's
`DEBUG_RESET_TOKEN` as the `X-Zutto-Debug-Token` request header.
Use Render operational logs and normal terminal reads to distinguish active
generation, failed generation, and successfully committed titles.

This is inspection of ordinary World-owned posts, not a separate generation
engine. The old general-purpose `/api/debug/world` export was removed.
