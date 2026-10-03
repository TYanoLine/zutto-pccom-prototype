# Project documentation

Core design documents live in this directory. For the current persistent-world generation layer, see:

- `WORLD_SIMULATION.md` — canonical world-state policy
- `HISTORICAL_ACCURACY.md` — evidence classification
- `HOST_DEFINITION.md` — how hosts are defined, listed, dialed and given roles (presets, the experiment host, test hosts, open work)
- `HOST_SKELETONS.md` — persisted per-host skeleton facts and evidence boundary
- `DEBUG_RESET.md` — protected prototype reset operations
- `DEBUG_INSPECTION.md` — experiment-station inspection and the generation trace

Host-program-specific research and behavior belongs in the host-program documents and runtimes rather than a shared BBS UI.

## Development verification

- Server (`apps/server`, Go 1.23+): `go vet ./... && go test ./...`. Run the whole module, not only the packages you touched: removing or changing a shared fixture breaks tests in packages that look unrelated.
- Web (`apps/web`): `npm ci`, then `npx tsc -b` and `npm test`.
- CI (`.github/workflows`) runs the server and web tests on pull requests. Do not report CI as passing unless you observed it.
