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

## Desktop browser presentation

The desktop shell gives the host terminal the full available window. Its header
shows the active host at left when connected, caller-origin MA and the 1996
world date/time before the modem display, and a hamburger menu at the far right.
The menu selects lamp, digital, or hidden modem display, provides hang-up and
auto-redial controls, and offers two terminal viewport modes.
The footer keeps the connection state, negotiated speed, elapsed call time, and
pseudo-charge together in a fixed-width right-aligned region.

The emulated terminal buffer remains 80×25. The default desktop presentation
keeps 80 columns and expands the visible row count to the available terminal
height, showing existing receive scrollback above the live 25-row buffer. The
80桁 × 25行固定 option displays only the historical 25-row viewport. The
desktop preference persists in local storage. Mobile presentation continues to
calculate its own fit rows from the visible device and keyboard viewport; the
desktop setting does not override its sizing or function menu.

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
  status row at the top. The row shows `オフライン` while idle; during call
  setup it shows `発信中…`, `呼出中…`, or `接続処理中…`, and during
  carrier recovery it shows `再接続中…`. These in-progress labels pulse
  subtly and include a compact three-bar activity indicator whose bars light in
  sequence, so muted users can still tell that the call is advancing. The
  indicator is absent while idle and after a stable connection is established.
  Reduced-motion preferences keep the indicator visible but static and disable
  the label pulse without changing the text. Once the carrier
  is established, the row switches to the connected host name (or local-test
  name), alongside current session elapsed time and current-session pseudo
  telephone charge. The name may ellipsize, while elapsed time and charge remain
  visible. Use `viewport-fit=auto` so iOS Safari itself keeps the layout
  inside its safe display area. The compact status row is 24px tall with
  no extra top padding or device-specific minimum inset; the modem strip
  naturally follows it. Do not reintroduce fixed iPhone top clearance on the
  shell or status row: iOS versions and Safari chrome configurations can
  already reserve that area, producing large double spacing.
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
- The terminal remains a fixed 80-column cell grid rather than a proportional
  text layout. Half-width and full-width glyphs use separate font sizes chosen
  to fit the historical 8px / 16px cell grid naturally; they are centered in
  their cells rather than horizontally squeezed with Canvas `maxWidth`.
  Cell width follows a PC-9801 / Shift_JIS model rather than Unicode East Asian
  Width: ASCII and JIS X 0201 half-width kana are one cell; Shift_JIS double-byte
  Japanese characters and symbols are two cells. Terminal output is filtered to
  the Shift_JIS repertoire at the server boundary. Unicode-only presentation
  controls are removed, `▫`/ `▪` are mapped to the PC-98-safe `□`/ `■`,
  and unsupported Unicode characters fall back to `?`. The server-side 絵理香K
  layout helper uses the same cell convention instead of counting Unicode
  runes, so mixed Japanese/ASCII columns align with the terminal core.
  Login banners and the main-menu three-column layout are also generated from
  display-cell widths instead of hand-counted spaces: decorative/banner rows are
  exactly 80 cells and main-menu columns begin at cells 0, 22 and 44. The Canvas uses a
  device-aware backing raster capped at 2x. Fit mode uses normal resampling
  instead of pixelated CSS scaling so Japanese and Latin glyphs retain their
  natural proportions when the 640px terminal is fractionally reduced on
  high-DPI phones.
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

Production deployment retries should not change modem behavior, terminal character policy, or mobile layout.
Ver 0.22 production retries are deployment-only and must leave the accepted PV-AF LCD appearance unchanged.

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
MR/TR readiness, SD/RD activity pulses, OH off-hook, CD carrier detect, AI
(see below), and HS for an established 9600-bps-or-faster carrier.

The AI lamp is not a modem signal. It replaces the former AA (auto-answer) lamp,
which could never light because incoming calls are not implemented. While the
server reports world-data generation in progress (the `generating` prop, fed from
the generation-trace `running` state), AI flickers at random: lit phases are
longer and more frequent than dark ones. The state is only available where the
station enables the `generation_trace` debug flag; elsewhere AI stays dark.

For the reconstructed digital display, OFH means Off Hook. Protocol capability
and negotiated protocol are stored separately: during dialing/ringing/training,
AUTO mode may present both V.42bis and MNP5 capability legends; after the
simulated negotiation settles, the display collapses to the selected protocol.
MNP4 displays MNP without the class-5 digit. The variable speed field is rendered
as explicit seven-segment digits, while V.42bis / MNP / OFH and the serial-signal
labels remain fixed LCD legends. The right-side signal matrix follows the
photographed PV-AF-family order DTR/DSR, RTS/CTS, AA/DCD. DTR currently follows
the modeled terminal-ready state, DCD follows carrier detect, AI follows the
generation-in-progress flicker described above, and RTS remains intentionally
unlit until an independent RTS state exists. The circle above OFH is an outline indicator rather than a filled
dot. The speed-unit K is also a fixed legend. The amber illumination keeps the
Ver 0.21 center brightness but darkens only toward the LCD edges, avoiding the
raised bevel appearance of the first implementation. This display-state mapping is kept
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


### PC-9801 character policy

Terminal-visible host text targets the PC-9801-era Shift_JIS/JIS repertoire.
Do not use emoji, variation-selector forms, or Unicode-only enclosed/symbol
characters in BBS source strings. Prefer period-appropriate JIS glyphs such as
`□ ■ ＊ ※ ○ ◎ ◇ ◆ ★ ☆ → ―`. Host output is validated at the WebSocket
terminal boundary so generated article/body text cannot introduce unsupported
modern Unicode into the emulated terminal.

## Out-of-world deployment diagnostics

The modern client menus may show the client build SHA/ref/time and asynchronously
fetch the running backend's SHA/branch/start time from `GET /api/version`.
This is operational metadata, separate from the 1996 in-world terminal and BBS.
The request derives its origin from the *configured WebSocket URL*, so a Vercel
preview cannot accidentally label the wrong backend. A failed or unavailable
lookup remains explicitly unknown and must never block terminal input, dialing,
center-directory loading or a live call. Commit differences alone do not establish
which side is newer (e.g. when a preview is connected to production).
