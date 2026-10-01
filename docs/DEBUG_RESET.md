# HAKATA generator-evaluation reset

The fixed HAKATA station (0920000196) deliberately clears generated articles
on every new successful CONNECT during quality evaluation. This is an explicit
station-specific exception; it is not the persistent-world default. Reset is
refused while header or body generation is still running.

Operator diagnostics, secured by a non-empty server-side `DEBUG_RESET_TOKEN`:
`POST /api/debug/bbs/reset?phone=0920000196`
with `X-Zutto-Debug-Token`. The former general world/host reset
endpoints were deleted alongside the retired PoCs. Never expose this token
in browser environment variables, links, logs or screenshots.
