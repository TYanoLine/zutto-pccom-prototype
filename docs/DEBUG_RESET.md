# Debug World Reset

These endpoints are prototype-only destructive operations. They are disabled unless the server has a non-empty `DEBUG_RESET_TOKEN`.

## Reset the current world

`POST /api/debug/world/reset?key=<worldKey>` with header `X-Zutto-Debug-Token: <token>`.

This deletes the canonical `worlds` row. PostgreSQL cascades the deletion to hosts and future world-owned descendants. Call `/api/world/bootstrap?key=<sameWorldKey>` afterwards to generate a new world using a fresh seed and a fresh 100-name catalog.

## Reset one host

`POST /api/debug/host/reset?key=<worldKey>&host=world-014` with the same debug-token header.

This asks the naming model for one fresh name, increments the host generation, regenerates its skeleton, and replaces that directory slot. Its directory ID and phone number remain stable so the communications-software registration can keep referring to the slot during debugging.

The debug token is a server secret. Do not put it in `VITE_*`, localStorage, source code, screenshots, or exported center-directory data.
