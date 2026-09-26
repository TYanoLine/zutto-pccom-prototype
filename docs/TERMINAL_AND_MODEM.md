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

## Mobile browser presentation

The mobile shell is a modern accessibility adaptation, not a claim about a
historical communications program. It keeps the same 80×25 cell buffer, ANSI
colors, full-width continuation cells, cursor positions, and host commands.

- At widths up to 680px or on coarse-pointer devices, the terminal defaults to
  `全体表示` with 80 columns. The logical width remains 640px (8px × 80 cells)
  and is fitted uniformly to the device width. Cell aspect ratio is preserved:
  the client never stretches the 400px/25-row screen vertically. Instead the
  presentation row count is computed as
  `floor((available viewport height - bottom control region) / displayed row height)`,
  with a 25-row minimum. The emulated terminal core itself remains an 80×25
  screen. A taller backing canvas presents additional rows from existing receive
  scrollback plus the live 25-row screen, rather than resizing or truncating the
  terminal buffer. The default mobile fit view is slightly inset so text is not
  unnecessarily large while all 80 columns remain visible.
- Mobile uses the full dynamic viewport height with one compact connection
  status row at the top. The row shows the connected network name (or OFFLINE),
  current session elapsed time and current-session pseudo telephone charge. The
  network name may ellipsize, while elapsed time and charge remain visible.
  Directly below it, an optional modem-status strip defaults to `ランプ` and
  can be switched from `機能` among `ランプ`, `デジタル`, and `OFF`.
  `OFF` removes only the modem-status strip; the connection/time/charge row
  always remains. The digital strip is a flat LCD-style presentation inspired
  by surviving aiwa PV-AF288-family displays, not a reproduction of the modem
  enclosure. The old desktop title bar, modem/call status strip, help text,
  debug/settings panels and development-only PoC shortcuts remain hidden on
  coarse-pointer/small-screen presentation. The compact `機能` button also
  contains `文字拡大`, `全体表示`, history up/down and `最新` controls.
  Host output is never rewrapped or replaced with a common host menu.
- Horizontal dragging pans the enlarged screen; vertical dragging reads the
  existing receive scrollback. While history is visible, a floating `最新へ`
  control remains available over the terminal. Beginning input returns the
  terminal to live output before the new text is echoed.
- On mobile, the visible terminal area itself is covered by a native single-line
  input. The user's tap therefore lands directly on the editable control; the
  client does not synthesize focus from a canvas pointer event. The mobile input
  element is fully transparent as a composited layer (in addition to transparent
  text/caret/selection styles), so iOS-native caret or selection decorations do
  not leak through at positions unrelated to the emulated terminal cursor. The
  Canvas cursor is the only visible cursor. The input uses a real 16px font and
  the same command routing, IME composition guard and Enter handling as physical
  keyboard input. Desktop keeps the cursor-sized proxy and click-to-focus
  behavior.
- Focusing the textarea does not immediately resize/reposition the terminal.
  Once the software keyboard actually changes VisualViewport height, the client
  handles resize events and computes terminal scroll positions from the live
  cursor row as absolute targets. It does not accumulate relative corrections
  or follow VisualViewport scroll events, preventing the cursor from drifting
  upward during continued typing. Horizontal panning is likewise corrected only
  as needed to keep the live cursor cell visible.
- The center directory retains its own six navigation/call keys. The command
  field is read-only while selecting a center; Enter calls the selected center.
  Esc is an offline client-menu operation, not a fabricated shared BBS command.
- `文字拡大` remains available as an explicit mobile fallback to the classic
  80×25 / 640×400 readable presentation with horizontal panning. Desktop keeps its
  existing canvas behavior and click-to-type interaction. The mobile center
  directory is overlaid at the bottom rather than consuming terminal height,
  retains its six explicit navigation/call keys, and the floating `最新へ`
  action remains at least 44px high. Device safe areas and browser zoom remain
  enabled.

Production deployment retries should not change modem behavior or mobile layout.
A retry for Ver 0.22 must not alter the PV-AF LCD presentation or modem telemetry semantics.

Validation should cover 320/390px portrait and desktop widths, direct native
input focus on iOS/Android, tap-to-type versus drag gestures, horizontal
panning, receive scrollback, software-keyboard resize, repeated typing without
vertical drift, physical/soft Enter, IME composition, center selection,
history/latest, and reading without unintended keyboard focus. Real iOS/Android software-keyboard behavior still requires
device testing.

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

Mobile modem indicators are driven by structured modem telemetry rather than by
parsing human-readable status strings. Lamp activity is state/data-driven:
MR/TR readiness, SD/RD activity pulses, OH off-hook, CD carrier detect, AA
auto-answer (currently off because incoming calls are not implemented), and HS
for an established 9600-bps-or-faster carrier.

For the reconstructed digital display, OFH means Off Hook. Protocol capability
and negotiated protocol are stored separately: during dialing/ringing/training,
AUTO mode may present both V.42bis and MNP5 capability legends; after the
simulated negotiation settles, the display collapses to the selected protocol.
MNP4 displays MNP without the class-5 digit. The variable speed field is rendered
as explicit seven-segment digits, while V.42bis / MNP / OFH / DSR / CTS and the
speed-unit K remain fixed LCD legends. The amber illumination keeps the Ver 0.21
center brightness but darkens only toward the LCD edges, avoiding the raised
bevel appearance of the first implementation. This display-state mapping is kept
separate from call state because the complete manufacturer LCD state table has
not yet been recovered. See `docs/research/AIWA_PV_AF288_LCD.md`.

## Transport is not the call

A WebSocket is a transport attachment, not the logical BBS/call session.

A logical call session survives a brief browser/network interruption. The server retains the session and line occupancy during a reconnect grace period. A replacement WebSocket may resume that session. Explicit `ATH`, host logout, or grace expiry terminates it.

Do not regress this by tying BBS runtime lifetime directly to one WebSocket object's lifetime.

## Telephone network

Long-term BUSY behavior should reflect logical line occupancy, NPC schedules, popularity, line count, time/day, events, and host policy.

A world-generation lease or bounded generation capacity may also make a line temporarily unavailable when admitting the call would require conflicting or over-budget materialization. In that case `BUSY` is an intentional form of runtime backpressure, not a fabricated random failure.

The service may maintain an atmospheric pseudo telephone bill. It never charges
real telephone money. The client tariff table follows NTT's March 1996 dial-call
history: tax-exclusive 10-yen pulse units whose duration varies by time band and
distance.

Caller origin is persisted separately from modem settings. The current preset is
福岡 / 092. For the client simulation, destinations sharing the configured area
code use the local-rate bucket; non-local destinations use historical distance
bands when geographic metadata is available and otherwise an explicit >160 km
fallback. Future UI may change this caller location without changing BBS/world
state.

Telehodai simulation uses registered destination numbers and the historical
23:00–08:00 window. See `docs/research/NTT_DIAL_TARIFF_1996.md` for source
links, implemented pulse seconds and the current geographic abstraction.

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
