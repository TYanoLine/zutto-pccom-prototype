# Verification

## Part 1

### Automated checks

| Command | Result |
|---|---|
| `go -C apps/server build ./...` | Passed |
| `go -C apps/server test ./internal/world/... ./internal/worldrepo/... -count=1` | Passed |
| `go -C apps/server test ./internal/world/... ./internal/worldrepo/... ./cmd/server/... -count=1` | Passed |
| `go -C apps/server vet ./...` | Passed |
| `go -C apps/server test ./... -count=1` | Passed |

The focused tests cover the preset-derived directory, separation of listed and
dialable hosts, repository delegation, and HTTP method/JSON/header behavior.

### Manual checks

Not run: production deployment and the Render endpoint check require the server
PR to be deployed.

### Scope

Only the server-side Part 1 changes are included. `/api/world/bootstrap` and
`/api/centers` were left unchanged; `/api/directory` was added.

## Part 2（Web）

| Command | Result | Notes |
|---|---|---|
| `npm --prefix apps/web test` | Not run | Pending implementation validation |
| `npm --prefix apps/web run build` | Not run | Pending implementation validation |
| grep（旧コードの残り） | Not run | Pending implementation validation |

The Part 1 protocol documentation already describes the Web client's use of
`/api/directory`.

## 手動確認

- 電話帳に HAKATA が表示され、発信できる: 未実施（ブラウザ確認が必要）
- `/api/world/bootstrap` が呼ばれない: 未実施（ブラウザ確認が必要）
