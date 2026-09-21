# 絵理香K版 runtime notes

Implementation notes for the fictional HAKATA CANAL NET sample station.

The runtime is intentionally modeled after surviving logs of 絵理香K版 installations, especially late-1990s command/menu structure, while station wording and content remain fictional. It is not claimed to reproduce a vendor-default screen byte-for-byte.

Historical traits used by the runtime:
- Main menu mixes numeric and mnemonic commands such as BM/FM/MAIL/C/MODE/BYE, A/ASET/MA/T/V/H.
- Board navigation is hierarchical and exposes a current-path prompt in the style of `(BJ\\80) BOARD [M]=MENU [?]=HELP -->`.
- RETURN moves to the parent level and `/` returns to the main menu.
- Parent messages render together with appended replies ("アペ").
- Command-mode aliases from older installations are accepted where practical: BM/BX/BR/BW/BJ, FM, MAIL, WHO, MEMB, MODE, GUIDE, BYE.
- NMODEM is shown as a supported transfer protocol, but the binary transfer engine remains out of scope for this prototype.

Primary surviving-log reference used for screen/command structure:
- mixi community archive, "くにびきネット / 覚えていますか？" (1989 and 1998 connection logs).

Secondary references are used only for broad traits such as the parent+append thread model and NMODEM support.

## Station master configuration

The reconstruction now has a host-wide master capability layer above role/ACL checks.

Precedence:

1. station master feature switch
2. account/role authorization
3. board/file ACL and state-specific rules

If a station feature is OFF, no account class can re-enable it. Disabled features are removed from the rendered menu and direct commands are rejected.

Configuration is modern JSON, not an attempt to reproduce the original Erika-K config-file syntax.

Example:

`apps/server/config/erika-k.example.json`

All known features and transfer protocols default to ON. A config file may therefore contain only the keys that need to be disabled. Unknown feature/protocol keys are rejected by the parser.

Transfer protocol IDs:

- `raw` — 無手順
- `xmodem`
- `xmodem_crc`
- `xmodem_1k`
- `ymodem`
- `ymodem_g`
- `zmodem`
- `nmodem`

The default reconstructed protocol-selection UI uses numeric keys 0-7 in the order above. The exact historical key assignment remains provisional.

Use `LoadConfigFile(path)` + `NewWithConfig(host, store, cfg)` for explicit configuration injection. The generic host-program factory intentionally does not silently auto-load files yet, because its current constructor cannot report malformed-config errors safely.
