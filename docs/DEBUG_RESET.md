# Generator-evaluation reset

A host whose preset sets `debug.reset_articles_on_connect: true` clears its
generated articles on every new successful CONNECT during quality evaluation.
This is an explicit per-host debug exception; it is not the persistent-world
default, and a host without the flag is never reset. Today only the fictional
HAKATA station (0920000196) opts in. Reset is refused while header or body
generation is still running, and the CONNECT then fails with NO CARRIER rather
than showing a mixed old/new sample.

There is no reset at server startup. With `debug.snapshot` the world state,
articles included, is restored after a restart; clear it with the manual reset
below (or by connecting, if the host has the flag above).

Operator diagnostics, secured by a non-empty server-side `DEBUG_RESET_TOKEN` and
available only for hosts whose preset sets `debug.http_endpoints: true`:
`POST /api/debug/bbs/reset?phone=0920000196`
with `X-Zutto-Debug-Token`. An unknown number and a host without the flag get
the same answer. The former general world/host reset
endpoints were deleted alongside the retired PoCs. Never expose this token
in browser environment variables, links, logs or screenshots.

See `apps/server/internal/hostcatalog/README.md` for every debug flag.
