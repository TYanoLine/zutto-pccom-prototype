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
