# Zutto transport protocol (prototype)

V0 uses JSON WebSocket messages for fast iteration. This is deliberately an adapter boundary, not the final wire format.

Client -> server:

```json
{"type":"dial","phone":"0920000196","attempt":1}
{"type":"line","line":"B"}
{"type":"hangup"}
```

Server -> client:

```json
{"type":"dial_result","result":"connect","baud":14400,"line":2,"host":{...}}
{"type":"dial_result","result":"busy"}
{"type":"terminal","text":"..."}
{"type":"carrier","result":"off"}
```

## Intended V1 boundary

The BBS runtime must eventually consume/produce bytes through a `Transport`, with CP932/Shift_JIS encoding at the line boundary. WebSocket control messages should remain for DIAL / carrier metadata, while terminal traffic can move to binary frames.

The same logical operations must be mappable to a future RS-232C bridge:

- `ATDT...` -> dial request
- `BUSY`, `NO CARRIER`, `CONNECT n`
- carrier state
- serial payload bytes
- DTR / RTS-CTS / XON-XOFF where practical
