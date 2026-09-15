# Debug world inspection

The prototype exposes a protected inspection endpoint so a developer can bridge persisted Render state into debugging without direct database access.

`GET /api/debug/world?key=<worldKey>`

Send the same `X-Zutto-Debug-Token` configured as `DEBUG_RESET_TOKEN` on the server. The endpoint is disabled when the token is unset.

The response includes the persisted world id, seed, generation version, creation timestamp and every host's directory id/order, generation, name, phone, baud, software family, line count, founding date, popularity, member count and skeleton basis.

This is intentionally a development interface. The browser world key is not authorization and the endpoint must not be exposed without the debug token.

Example bridge command:

```sh
curl -sS -H "X-Zutto-Debug-Token: $DEBUG_RESET_TOKEN" \
  "https://<render-service>/api/debug/world?key=<32-hex-world-key>" > world-debug.json
```

The resulting JSON can be shared in the development conversation for inspection. Do not share the debug token.
