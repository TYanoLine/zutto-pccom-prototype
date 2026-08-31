# Terminal, modem, telephone, and transport

## Client target

Initial client is a fictional 1996 PC-98 communications application. It may evoke period software but must not be a pixel-for-pixel copy of a particular commercial/shareware application unless explicitly intended and legally appropriate.

Baseline display target:

- 640×400
- 80×25 text cells
- PC-98/Japanese bitmap-font feel
- 16-color-era presentation
- full-width / half-width handling
- box drawing
- cursor blink and terminal beep
- host-dependent ANSI/ESC usage

Long-term terminal implementation should favor a controlled Canvas 2D cell buffer over DOM-heavy rendering so byte-oriented terminal behavior remains deterministic.

Canonical stored text is UTF-8. The future terminal/serial wire boundary should use CP932/Shift_JIS-compatible bytes where historically appropriate.

## Modem interaction

Hayes-style interaction is first-class UX:

- `AT`
- `OK`
- `ATDT...`
- `A/`
- `ATH`
- `BUSY`
- `NO CARRIER`
- `NO DIALTONE` / no-answer variants where appropriate
- `CONNECT 2400/9600/14400/28800` and later historically supported rates

Handshake audio should be based on modem protocol behavior rather than arbitrary retro beeps. Existing synthesis research lives in `docs/MODEM_HANDSHAKE_SYNTHESIS.md`.

## Transport is not the call

A WebSocket is a transport attachment, not the logical BBS/call session.

A logical call session survives a brief browser/network interruption. The server retains the session and line occupancy during a reconnect grace period. A replacement WebSocket may resume that session. Explicit `ATH`, host logout, or grace expiry terminates it.

Do not regress this by tying BBS runtime lifetime directly to one WebSocket object's lifetime.

## Telephone network

Long-term BUSY behavior should reflect logical line occupancy, NPC schedules, popularity, line count, time/day, events, and host policy.

A world-generation lease or bounded generation capacity may also make a line temporarily unavailable when admitting the call would require conflicting or over-budget materialization. In that case `BUSY` is an intentional form of runtime backpressure, not a fabricated random failure.

The service may maintain an atmospheric pseudo telephone bill. It never charges real telephone money. Telehodai-style simulation uses registered destination numbers and a 23:00–08:00 window; exact historical tariffs must be researched before being presented as accurate.

## Diegetic backpressure

Operational pressure should be translated into period-appropriate telephone/BBS behavior where possible rather than exposed as modern cloud/API errors.

Possible runtime causes include:

- all logical host lines occupied;
- another request holding the generation/update lease for the required world scope;
- generation queue saturation;
- temporary LLM/provider rate limits or latency;
- configured rolling generation/token/cost budget pressure;
- database or host-runtime pressure.

Permitted user-visible outcomes include:

- `BUSY` before carrier when no line/capacity should be admitted;
- fewer simultaneously available logical lines;
- a lower negotiated connection speed for a newly established call when the host/modem configuration plausibly supports that outcome;
- slower host-side output pacing after connection;
- period-appropriate waiting/status text from the host while a committed result is being prepared;
- normal auto-redial behavior in the client.

Do not silently change an already established `CONNECT 14400` session into `2400` without a historically plausible retrain/fallback mechanism. If a call is already connected, prefer host-side output pacing or an explicit wait state rather than pretending the modem renegotiated when it did not.

Likewise, do not delay backend work merely to manufacture slowness. Complete persistence/generation as efficiently as possible; the terminal renderer may pace already available output according to the simulated line rate. Real generation delay may be hidden naturally behind dialing, handshake, host banners, menu rendering, or short wait states.

Backpressure should be state-driven and bounded. Do not randomly return `BUSY` solely to save money. If cost protection participates in admission control, it must be represented by an explicit policy/budget state together with queue/line capacity.

## Line-speed presentation

The simulated modem speed and the backend's actual network throughput are separate concepts.

After a result is available, the terminal layer may pace transmitted bytes according to the negotiated simulated bps so that 2400, 9600, 14400, and 28800 connections feel materially different. This pacing is presentation/transport simulation, not permission to hold expensive backend resources open unnecessarily.

When generation itself is slow, avoid double-counting delay: generation wait plus simulated byte pacing should still produce a believable session rather than an artificially punitive one.

## Real hardware target

The same logical host/session layer should eventually support a real PC-98 over RS-232C through a bridge that emulates sufficient Hayes modem behavior. WebSocket and Serial are transports into the same BBS/world model, not separate worlds.
