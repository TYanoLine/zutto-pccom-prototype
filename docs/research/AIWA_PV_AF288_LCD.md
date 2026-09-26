# aiwa PV-AF288-family modem status display research

Status: confirmed labels plus a deliberately isolated reconstruction layer.

## Confirmed material

The current mobile modem-status UI borrows only indicator/display semantics from
the aiwa PV-AF288 family. It does **not** reproduce the product enclosure.

Confirmed external evidence:

- Mitsubishi Electric PLC manuals list aiwa PV-AF288 modem initialization and
  explicitly describe `&M5` as V.42bis communication mode:
  https://dl.mitsubishielectric.com/dl/fa/document/manual/plc_fx/jy997d16901/jy997d16901r.pdf
- PC Watch's 1998 aiwa PV-JF56E1 article says its three LEDs indicate
  `OFH`, `DCD`, and `DTR`:
  https://pc.watch.impress.co.jp/docs/article/980223/aiwa.htm
- ASCII's 1999 aiwa PV-JF56E6 article likewise names `OFH/DCD/DTR` as modem
  operating-state indicators:
  https://ascii.jp/elem/000/000/305/305560/
- Cisco modem documentation explicitly uses `Off-hook (OFH)` as the modemcap
  abbreviation:
  https://www.cisco.com/c/en/us/td/docs/routers/access/2600/software/notes/analogat.html

Together these make `OFH = Off Hook` strong enough to use as the status-strip
meaning. In the simulator, OFH follows actual logical line seizure: dialing,
ringing/training and online states are off-hook; idle/busy/no-answer after
release are on-hook.

## LCD reconstruction

Available photos of PV-AF288/PV-AFV144-family displays show fixed legends
including `V.42bis`, `MNP`, a separate class digit, a large speed field,
`OFH`, `DSR`, and `CTS`. A complete original LCD-state table has not yet
been found.

The implementation therefore keeps two concepts separate:

1. protocols/capabilities currently enabled for negotiation;
2. the protocol selected by the simulator after negotiation.

Working reconstruction used by the UI:

- on-hook: show the configured/preferred protocol indication;
- dialing/ringing/negotiating: capability indications may appear together, so
  AUTO can show both `V.42bis` and `MNP 5`;
- connected: collapse to the simulated negotiated protocol;
- MNP4 uses `MNP` without the `5` class segment;
- MNP5 uses `MNP 5`;
- OFH is independent from the protocol indicators.

The simultaneous V.42bis/MNP5 behavior is a reconstruction based on surviving
photographs and user recollection, not a confirmed manufacturer state table.
It is intentionally implemented in the presentation mapping rather than baked
into the modem engine so a future manual can replace it without changing world
or call state.

## Lamps

The lamp mode uses conventional modem labels `MR TR SD RD OH CD AA HS`.
The simulator does not blink lamps randomly:

- MR/TR follow modem/terminal readiness;
- SD pulses on terminal-to-modem line submission;
- RD pulses when modem data is actually delivered to the terminal;
- OH follows off-hook state;
- CD follows established carrier;
- AA remains off until incoming-call/auto-answer behavior exists;
- HS follows an established 9600-bps-or-faster carrier.

These mappings are a service UI convention, not a claim that PV-AF288 itself
used this exact lamp bank.
