# TurboBBS source reconstruction notes

Status: source-level reconstruction based on a supplied TurboBBS 1.08 main file plus two different sets of support modules. This document deliberately separates compatible/configured source sets instead of treating every uploaded file as one pristine release.

See `turbo-bbs.md` for historical lineage and the v1.05 manual summary.

## Evidence inventory

Directly inspected artifacts:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `bbs105.doc` | 20,352 | `6d7b3f0a834366d9b9a74fd3fd55c67b9c2042d871edc4baa55f12aa35ea2ca9` |
| `bbs.pas` | 7,510 | `251d4139ae2b72bc82e9cfb2b0e6b1181f1f716c8d9a247c66eacaabefcbd82d` |
| `bbs2.pas` | 11,382 | `1cdf5c791d1ae73642c298af7b0d33a9d08eba259dbff02e0d84011dd7ea26b4` |
| `io.pas` | 19,575 | `5ac99128691590d84dea2e7838176331ca619769d903a62e672ff92826f6f0a5` |
| `machdep.pas` | 5,553 | `200a98e215065a628c7bf068b3b1f579cb53550cbf6e1faa60f060a881c81b06` |
| `mailsys.pas` | 17,383 | `e36cd488e8a4960824dd8f3f161aaab91d75b308e581e693a08446c93b8c198d` |
| `filesys.pas` | 21,109 | `7e0e1f6a7e901451af0720a645e4afdefa32a78ebac65b58d93a085b0d6775df` |
| `config.bbs` | 56 | `ccb8fc46bebd1629d51a83354561d683f8f4fb3d2af8dd179ef9063e0fd33112` |
| `bbs2.inc` | 11,814 | `bd961557774b6eb4808658ed3c1f3416f66cd7b5c9db2c4f56b7776fbf3e6a3f` |
| `io.inc` | 18,940 | `faf35ea8d6d1e8738ad16d13860c75d730ffcc92a2121f8363c777e723fcc2e9` |
| `machdep.inc` | 5,682 | `90dd12d779e383cc1ff67cd25aa075f413833514dc7918eea435ebac096f3fd7` |
| `mailsys.inc` | 17,080 | `a129d04602ef0b823e643e4e864858457a555f9383a4bad9f913fd6ee2a443dd` |
| `filesys.inc` | 21,189 | `14d0f7727f9f2d3249c00a3c9236690fde2988be1b880977d8e41773c6b519ec` |

Do not commit the historical source itself to this repository merely because it was available for inspection. The period distribution wording is not a clean modern license; preserve hashes and findings unless redistribution rights are separately established.

## Source-set classification

### v1.08-compatible configured set — Confirmed compatibility, configuration-specific values

The supplied `bbs.pas` declares:

```text
version = '1.08'
```

and includes `ASYNC.PAS` and `INT24.PAS` before the support files.

The supplied `*.pas` support variants strongly match that main file:

- `bbs2.pas`, `io.pas`, `mailsys.pas`, and `filesys.pas` use `int24result`;
- `io.pas` / `machdep.pas` use the asynchronous-communications API (`carrier`, `purge`, `term_ready`, `new_baud`, `cgetc`, `send`, `strsend`);
- `machdep.pas` defines `clock(...: integer)`, matching the integer clock variables in the supplied `bbs.pas`;
- `machdep.pas` handles `very_fast` as 2400 baud, matching the `rate = (slow,fast,very_fast)` declaration in the main file.

Therefore this `*.pas` group is the best source for reconstructing the supplied v1.08 main snapshot.

However, it is visibly configured/customized rather than safely assumed to be the pristine vendor/default 1.08 distribution:

- `bbs.pas` uses `filedrive = 'F:'`;
- `mailsys.pas` has AI-oriented section names such as Artificial Intelligence, Prolog, Lisp, Pascal, C and Basic;
- `mailsys.pas` sets `maxmess = 300`;
- `filesys.pas` sets `mostfiles = 999` and `drivecap = 4096`;
- `config.bbs` ends with the string `ANALYZER.BBS`.

The exact meaning of the individual `config.bbs` fields is **not confirmed** because the corresponding ASYNC/configuration format documentation was not present. Do not assert that `ANALYZER.BBS` is the station name without further evidence.

### Alternate `*.inc` set — Confirmed separate port/configuration

The supplied `*.inc` variants are not simply identical copies of the `*.pas` set:

- they use Turbo Pascal `IOresult` rather than `int24result`;
- `machdep.inc` directly addresses PC serial UART registers at COM1/COM2-style I/O addresses;
- `machdep.inc` defines `clock(...: byte)`, which does not match the integer `var` parameters used by the supplied v1.08 `bbs.pas`;
- `mailsys.inc` has generic PC-oriented sections and `maxmess = 60`;
- `filesys.inc` uses `mostfiles = 40` and `drivecap = 360`.

This is a separate MS-DOS/PC serial-port adaptation or configuration. It may preserve much of the same logical runtime, but do not mix its machine layer into the supplied v1.08 main file as though all files came from one exact build.

It also should not be called the pristine v1.05 distribution: the v1.05 manual describes a Kaypro/Rixon distribution, `maxmess = 52`, a 191K-class files disk and `B:` default file drive, while this source set has later/different values and a PC UART implementation.

## Account model and first-call lifecycle

### No separate handle field in the inspected TurboBBS source

The v1.08 `sysid` account record contains a single `user` field plus password/access/terminal state. There is no separate textual member ID and handle field in this source.

Messages store numeric `sender` and `recver` values. These are positions in `IDS.BBS`; `getname()` resolves them back to the account's `user` field.

This is important lineage evidence: KTBBS's documented separation of member ID and handle should not be projected backward into this TurboBBS snapshot.

### Sign-on flow — Confirmed in both BBS2 variants

The sign-on logic is:

1. prompt `What is your full name?`;
2. uppercase the response;
3. require more than four characters;
4. search `IDS.BBS`;
5. existing user → password check and saved terminal defaults;
6. unknown user on a local or open system → confirmation, new password and terminal setup;
7. unknown remote user on a private system → hang up after three name attempts;
8. access level 0 → immediate denial/hangup.

Existing-user password entry allows three attempts. Password input is uppercased and limited to 14 characters.

New users are assigned level 2 (`newuser`) unless the name is exactly `SYSOP`, which receives level 5.

### New users are not persisted until Goodbye

During the first session, a newly introduced user still has `usernum = 0`. `savedefaults`, called from the normal Goodbye path, allocates an `IDS.BBS` slot and writes the account.

Consequences for a faithful reconstruction:

- a new caller who drops carrier before a normal Goodbye may not become a persisted account;
- `readmine` skips the first session because it requires `usernum > 0`;
- level-2 new users can browse/read/apply but cannot post messages or upload files;
- promotion to regular access is a SYSOP action, not an automatic result of `A`pply.

The `A`pply command displays `APPLYING.TXT` and accepts up to four lines into `COMMENTS.BBS`; it does not directly change the caller's level.

## Main command loop

The v1.08-compatible BBS2 source implements the following main commands:

| Command | Behavior |
| --- | --- |
| `A` | Apply: display applying instructions, then leave up to four lines for SYSOP |
| `B` | Bulletin |
| `C` | Chat / summon SYSOP |
| `E` | Enter message |
| `F` | File subsystem |
| `G` | Goodbye / comments / save profile / hang up |
| `H` | Help |
| `I` | Terminal parameter setup |
| `K` | Delete a message if permitted |
| `L` | Call log |
| `M` | Meetings information |
| `N` | Messages newer than saved `lastmess` |
| `O` | Other BBS list |
| `P` | Change password |
| `Q` | Re-log/sign on as another user without a modem hangup |
| `R` | Read/search messages |
| `S` | Quick-scan message headers |
| `U` | User list |
| `W` | Welcome text |
| `X` | Toggle expert/menu-suppression mode |
| `Y` | System information |
| `#` | Status, clock and connect time |
| `?` | Main menu in expert mode |
| `@` | SYSOP-only subcommand prompt |
| `!` | SYSOP-only printer mirroring toggle |

Notably, `Q` is implemented but omitted from the ordinary non-expert prompt string in the inspected source.

The v1.08-compatible `bbs2.pas` has an additional SYSOP-only `R` inside the `@` prompt which removes the serial-port handler, restores INT 24h handling and halts the program. This is absent from the alternate `bbs2.inc` and from the v1.05 manual; treat it as version/configuration-specific.

## Terminal profile and input behavior

Persisted terminal/profile settings include:

- expert/menu mode;
- uppercase-only output;
- whether line feeds are sent;
- prompt bell;
- backspace character;
- clear-screen string;
- terminal width.

Width accepts 20–132 columns (or 0 to leave unchanged); default is 80.

Password and terminal changes explicitly say they are saved by `G`oodbye. The implementation confirms that the save occurs in the Goodbye path.

### Semicolon is an input delimiter

The generic `getinput` routine keeps an input buffer and treats `;` as a delimiter between queued answers/commands. Single-character reads consume one buffered character and skip a following semicolon; longer reads return text up to the next semicolon.

This is useful lineage evidence for KTBBS research: later claims that semicolon required special handling are consistent with semicolon already being syntactically meaningful in upstream TurboBBS input. Do **not** claim KTBBS retained this exact implementation until its source is checked.

### Pause/cancel during output

The output layer watches incoming input while sending text:

- Ctrl-S **or the letter S** pauses until another character;
- Ctrl-C **or the letter C** cancels the current output.

The input layer also normalizes DEL to backspace and supports a configurable backspace/control-character setup.

## Message model

The inspected message record contains:

```text
number
sender
recver
subject
date
private
section
repto
reply
recved
```

### Flat numbered messages plus sections

Messages have a global numeric message number and one section number. There is no RT-BBS-style hierarchical board model here.

The inspected `mailsys.pas` has ten configured sections; the alternate `mailsys.inc` has a different ten-section set. Section labels are configuration/source values, not universal TurboBBS defaults.

### `repto` and `reply` are dormant in this snapshot

The two fields exist in the record and are initialized to zero for a new message, but neither inspected MAILSYS variant otherwise reads or updates them.

Therefore the inspected runtime does **not** provide evidence for an active threaded reply linkage despite carrying placeholder fields.

### Public/private visibility

A private message is readable only by:

- sender;
- receiver;
- SYSOP.

A message addressed to `ALL` is always forced public. Private status is only offered when a specific registered recipient is selected.

### Received flag

When the addressed recipient actually reads the body, `recved` is set true. Headers then show the recipient with a received marker.

This flag is distinct from the account's high-message/last-message state.

### "New" is new-since-last-saved-session, not true unread

`N` begins at `lastmess + 1`.

`lastmess` comes from `IDS.BBS.lstm`, and normal Goodbye saves:

```text
lstm := nextmess - 1
```

So `N` means messages numbered after the highest message number known at the caller's previous **properly saved Goodbye**, not "messages whose body I have never read."

Separately, `readmine` runs:

```text
messagesearch(1, 0, usernum, 0)
```

which searches all messages addressed to that user and does not filter on `recved`. Previously received mail can therefore be presented again, with the received marker showing its state.

For implementation, preserve these two distinct concepts instead of collapsing both into one modern unread boolean.

### Read/search commands

The `R`ead subsystem supports:

- `A` — all;
- `I` — individual message number;
- `F` — by sender;
- `T` — by addressee;
- `S` — by section.

`S` at the main menu is a header-only quick scan with a selectable starting message; `*` maps to new messages.

### Deletion permissions

Deletion is allowed to the message sender, receiver, or SYSOP. Unrelated users cannot delete a message even when it is public.

## Message editor

Regular message entry requires `access >= reg` (level 3 in the supplied v1.08 main). Level-2 new users are told to use `A`pply.

The editor supports at most 24 lines of up to 80 characters.

Header entry:

- addressee, or ALL;
- subject, maximum 14 characters;
- section;
- private Y/N for addressed mail;
- timestamp;
- global message number.

Editor commands:

| Command | Behavior |
| --- | --- |
| `A` | Abort |
| `C` | Continue composing / append lines |
| `D` | Delete a selected line |
| `E` | Replace a substring on a selected line |
| `L` | List from a selected line onward |
| `P` | Save preformatted |
| `R` | Replace an entire selected line |
| `S` | Save normal flowing form |
| `?` | Editor menu |

Normal flowing save joins entered lines with spaces. A line beginning with `.` forces a hard line break and has the leading dot stripped. `P` preserves line breaks for every entered line.

This directly explains the v1.05 manual's description of flowing versus hard-return message-body formats.

## File subsystem

The file record contains:

```text
title
submit
date
size
accesses
ASCII
section
public
```

New uploads/installs begin with `public = false`.

### File command map

| Command | Behavior |
| --- | --- |
| `D` | Directory |
| `S` | Send/download via XMODEM |
| `T` | ASCII/text dump |
| `H` | File help |
| `L` | Library (`.LBR`) directory |
| `U` | XMODEM upload, initially requesting CRC |
| `C` | XMODEM upload, initially checksum mode |
| `V` | Raw text capture upload |
| `Q` | Return to main command loop |
| `G` | Goodbye/hangup |
| `K` | SYSOP: delete file |
| `I` | SYSOP: install existing disk file |
| `E` | SYSOP: edit file metadata/release status |
| `?` | File menu |

Upload commands `U/C/V` require `access > newuser`, so a level-2 new user cannot upload while regular level 3 can.

### DOS-style filenames

The file-name validator uppercases names and enforces an 8.3-style shape with characters limited to:

- A–Z;
- 0–9;
- dot;
- hyphen;
- underscore.

Only one dot is allowed.

### XMODEM implementation

The source implements 128-byte SOH blocks with:

- checksum mode;
- CRC-16 mode using polynomial `0x1021`;
- ACK/NAK/CAN handling;
- EOT completion;
- block-number/complement checking;
- retry logic which can switch CRC/checksum behavior after repeated errors.

The upload text says repeated Ctrl-X cancels. The code uses CAN = `0x18` (Ctrl-X).

### Text-capture upload

Text capture is a separate non-XMODEM path:

- optional echo;
- two Ctrl-C characters abort;
- two Ctrl-Z characters finish;
- XOFF/XON-style flow control is used while buffering disk writes.

### LBR and squeezed files

The I/O layer contains:

- `.LBR` directory/member lookup;
- member addressing through `library/member` syntax internally;
- an inline unsqueezer recognizing the historical SQueezed-file signature;
- automatic display of the original filename when a squeezed file is decoded.

This is executable source-level confirmation of the v1.05 manual's "squeezed and library files" feature.

### Private-file behavior changed or differs from the v1.05 manual

The v1.05 manual says level-3 users can access a private file if they know its filename.

The inspected later file source checks:

```text
public or (access > reg)
```

for direct lookup. With the supplied v1.08 main where `reg = 3`, this means access must be greater than 3 for a non-public direct lookup.

At the same time, directory listing shows a private file to its submitter or SYSOP.

This is a real discrepancy between the v1.05 documentation and the inspected later/configured code. Preserve it as a version/configuration difference (or possible bug); do not normalize the behavior to one rule.

## Modem/transport variants

### v1.08-compatible ASYNC-based variant

The compatible `machdep.pas` wraps an external asynchronous communications layer:

- 300 / 1200 / 2400 baud are represented;
- `carrier` supplies online state;
- `term_ready` controls modem readiness;
- `ATS0=1 E0` is sent for auto-answer/no echo;
- `cgetc`, `send`, `strsend`, `purge`, `new_baud` provide serial operations.

The supplied `io.pas` wait-for-call path currently selects 300 or 1200 from a hardware/status bit. A previously attempted multi-speed framing-based detection block is commented out, so do not claim that this exact build automatically negotiates 2400 merely because `very_fast` exists.

### Alternate direct-UART `machdep.inc`

The alternate machine layer directly uses PC-style UART registers:

- COM1-style base `0x3F8`;
- COM2-style base `0x2F8`;
- 8N1 setup;
- explicit divisor programming for 300/1200;
- modem-status polling;
- direct modem-control writes.

Its `cts` function checks modem-status bit `0x80`, which on an 8250-family UART is DCD rather than the CTS bit. The procedure name/comments retain "CTS" terminology inherited from the generic interface, while this PC port effectively uses DCD for carrier.

The same source writes the modem-control register to zero when "dropping RTS", thereby dropping both DTR and RTS in that port implementation.

This is a good example of why the abstract historical behavior ("carrier state via machine-dependent status line") should be separated from any one port's register-level implementation.

## Implementation guidance for ずっとパソコン通信

For a Maxwell TurboBBS runtime, implement the software state machine separately from its machine/serial adapter.

Core source-backed runtime behavior now available:

- first-call/new-user lifecycle;
- account persistence on Goodbye;
- numeric internal user references;
- access-level gates;
- main command map;
- terminal profile;
- message search, privacy, received flag and "new since prior saved call" semantics;
- message editor;
- file subsystem and XMODEM/text capture;
- `.LBR` and SQueezed display;
- SYSOP functions.

Do not yet claim a single byte-for-byte "TurboBBS 1.08 default" appearance because the surviving source snapshot is visibly configured and the external menu/help text files are still missing.

For a 1996 Japanese world, TurboBBS should remain primarily a lineage/ancestor runtime unless a historically plausible surviving installation is intentionally modeled. Do not import KTBBS/RT-BBS hierarchy, IDs/handles, multi-line behavior or later menu semantics back into this runtime.

## Remaining gaps

- matching `ASYNC.PAS` and `INT24.PAS` for the configured v1.08-compatible set;
- documentation for the `CONFIG.BBS` format;
- menu/help assets: `MAINMENU.TXT`, `READMENU.TXT`, `EDITMENU.TXT`, `FILEMENU.TXT`, `BBSHELP.TXT`, etc.;
- provenance/version history that identifies exactly where each `*.pas` and `*.inc` variant came from;
- pristine 1.05 source for a byte-level comparison with the 1.05 manual;
- KTBBS source comparison to identify retained TurboBBS records/input semantics versus later redesigns.
