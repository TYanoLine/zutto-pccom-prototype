# TurboBBS information/menu assets

Status: direct inspection of supplied `BBSINFO`-style text assets. These files are valuable evidence for the caller-visible UI, but they are **not one pristine v1.05 default set**.

See:

- `turbo-bbs.md` — historical lineage and v1.05 manual
- `turbo-bbs-source-notes.md` — source-level runtime reconstruction

## Evidence inventory

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `source.txt` | 1,920 | `8c9382980cac70d9ff6d6ce8e03506c81931645e1f18f3a27b221c5b371364f4` |
| `bbshelp.txt` | 4,352 | `cf3e65c620ad59b5cd19033547d58ac253e7e97990e4cf7394759d3d9583daa0` |
| `mainmenu.txt` | 512 | `026225590956351464eac80dbdeed0c4131fa12a604594f997f36338dbb6e26e` |
| `readmenu.txt` | 256 | `65e1507d468570ebcdf58a32952386b83d95143610f33362713188f3fe9d6a6a` |
| `editmenu.txt` | 384 | `c003d1f1a0b3ce1a2ebcc389418ff99beac2cd48d2f48fd313c0ca8826216e2b` |
| `filemenu.txt` | 896 | `6d83b38ef93d75eaafc01099a2bf43ccd31670f3aedd090c02597793a8d445d0` |
| `filehlp.txt` | 3,456 | `e77c25977e8ca0f94f1d2f1007b87db7fe06a3702906dedfbf33e4608924c803` |
| `welcome.txt` | 512 | `9ebb6e672b1684fa9fe60c0ab96a215bf046e0fbac42122fd668135966054da5` |
| `bulletin.txt` | 384 | `2ac0c94d0c0c9fc7b4d7d3eb87f0251ff378db02d93494012c1eb0cb3953991f` |
| `applying.txt` | 512 | `98e6254717aa10f83d344ffa22535f1811e71df7aa1d4210717e7a3b7285aa86` |
| `meeting.txt` | 256 | `460ab268f2effcf83b7a63f8c6cd719184b2a3b49e9b574ac7c5685be032c945` |
| `bbslist.txt` | 256 | `7b6060ffc026fcb7ba72441fe4ac62ffd17028eae678821c6813645da8fadbe6` |
| `sysinfo.txt` | 512 | `4b4918bb457d88c4df18f9415ac1fa251bd39ae56984f8ceefaef23fa36f4f80` |

Historical source files themselves are not committed here; only hashes and research findings are recorded.

## Provenance and version separation

### `SOURCE.TXT` — v1.05 source-distribution instructions

`SOURCE.TXT` identifies itself as instructions for **TurboBBS source code files version 1.05**.

It says:

- source is distributed in `TURBOBBS.ARC`;
- unpack the archive, adapt files to the installation, then compile `BBS.PAS`;
- the other source files are pulled in by `BBS.PAS` and are not independently compiled;
- the operator is expected to adapt the separate `BBSINFO.ARC` information files;
- the information files are WordStar Document files because TurboBBS uses WordStar soft carriage returns for automatic output wrapping;
- the information files need not remain inside a library if the BBS.PAS filename configuration is changed;
- a "much-improved version 2.0" is described as being under development.

The last item is evidence of an announced/in-development 2.0, **not proof that 2.0 was released**.

This document is also direct evidence that station operators were expected to customize caller-visible text. Therefore surviving WELCOME/SYSINFO/BULLETIN/menu files must not automatically be called vendor-default UI.

### Caller-visible asset set — v1.09 / station-specific

The supplied `BBSHELP.TXT` explicitly titles itself **TurboBBS Version 1.09 Help File**.

The supplied `SYSINFO.TXT` describes the station as:

- IBM PCjr;
- 640K;
- V20 CPU;
- cloned Hayes 1200 modem;
- DOS 2.10;
- a "slightly modified" TurboBBS.

The supplied `WELCOME.TXT` brands the station **DING DONG Turbo BBS**.

Accordingly, treat `BBSHELP.TXT` and the accompanying BBSINFO/menu assets as evidence for a **v1.09-era customized station surface**, not as a byte-for-byte v1.05 default.

The exact relationship between this station asset set and the supplied configured v1.08 source snapshot is not established.

## WordStar soft-return storage

The raw information files contain many characters with bit 7 set. Stripping bit 7 reveals ordinary ASCII prose.

This matches both:

- `SOURCE.TXT`, which says the BBSINFO files are WordStar Document files using soft carriage returns for automatic word wrap;
- the inspected I/O source, which treats WordStar soft CR specially and masks the eighth bit while outputting no-wrap information files.

For a faithful reconstruction, preserve the semantic behavior—flowing station text wrapped to the caller's terminal width—without requiring the modern project to store literal WordStar high-bit encoding unless a byte-level archival mode is desired.

## v1.09 help: complete caller command surface

The help file exposes the caller-facing main commands:

| Command | Caller-visible meaning |
| --- | --- |
| `A` | Apply for access level 2 |
| `B` | Bulletins |
| `C` | Chat with SYSOP |
| `E` | Enter message |
| `F` | File subsystem |
| `G` | Goodbye / hang up; may leave SYSOP comments |
| `H` | Help |
| `I` | Install/configure terminal parameters |
| `K` | Kill/delete message |
| `L` | User log |
| `M` | Local meetings |
| `N` | New messages since last call |
| `O` | Other systems |
| `P` | Change password |
| `Q` | Log off without disconnecting; **changes not saved** |
| `R` | Read messages |
| `S` | Scan message headers |
| `U` | User list |
| `W` | Welcome |
| `X` | Expert mode |
| `Y` | System information |
| `?` | Help menu in expert mode |
| `#` | Caller/message-system status |

The compact `MAINMENU.TXT` shows essentially the same public command surface but omits `Q`, matching the inspected source behavior where `Q` exists yet is absent from the ordinary prompt/menu presentation.

### Read menu

`READMENU.TXT` confirms these read modes:

- `A` — all, from a starting number;
- `I` — individual message number;
- `F` — from a user;
- `T` — to a user;
- `S` — by section;
- direct message-number entry.

This matches the source-level `messagesearch` dispatcher.

### Edit menu

`EDITMENU.TXT` confirms:

- `A` abort;
- `C` continue entering;
- `D` delete line;
- `E` edit a string;
- `L` list;
- `R` replace full line;
- `P` preformatted store;
- `S` no-wrap store.

The wording "No-Wrap" here means the stored body is not hard-wrapped into the user's current screen width; it is compatible with later output-time wrapping. Do not reinterpret it as "disable all display wrapping."

## v1.09 help: control characters and chained input

The help file explicitly documents:

- `C` or Ctrl-C aborting ordinary functions/displays;
- `S` or Ctrl-S pausing output;
- XMODEM cancellation with Ctrl-X;
- fast message-reading behavior where Ctrl-C skips the current message and Ctrl-K aborts the message-reading run;
- a five-minute input timeout.

Most importantly, it documents **chained commands/input**:

- `RS1` → read messages in section 1;
- `GN` → disconnect without comments;
- semicolon terminates multi-character chained input;
- `USERNAME;PASSWORD` is given as a sign-on example.

This is user-facing confirmation that semicolon command/input chaining was an intended TurboBBS feature, not merely an accidental implementation detail.

## v1.09 help: logon synchronization

The help says the system checks for:

- carriage return;
- Ctrl-C;
- period (`.`);

at 1200 and 300 baud, and instructs callers to type one of those repeatedly until sign-on begins.

Treat this as evidence for the v1.09-era station/help set. Do not force this exact synchronization scheme onto every machine-specific TurboBBS port without matching transport source.

## v1.09 access-level semantics

The help file gives a complete caller-facing level table:

| Level | Meaning in this help set |
| ---: | --- |
| 0 | disallowed; no system access |
| 1 | entry level; cannot send messages or upload |
| 2 | can send/delete messages and upload; should Apply with name/address/phone |
| 3 | access to hidden/non-public data files |
| 4 | reserved for future use |
| 5 | SYSOP |

`APPLYING.TXT` reinforces that the application is for **access level 2** and warns that the application is not completed unless the caller exits with `G`oodbye.

### Important conflict with the supplied v1.08 source snapshot

The supplied configured v1.08 main source instead labels:

```text
newuser = 2
reg     = 3
sysop   = 5
```

and its upload/message gates are built around those constants.

Therefore the v1.09 help's 1→2→3 semantics **must not be retroactively applied to the supplied v1.08 source**. Plausible explanations include a later version change or local source/text customization, but the current evidence does not distinguish them.

This discrepancy is precisely why versioned evidence must remain separate.

## File menu/help: v1.09-era caller surface

The file menu/help documents:

| Command | Meaning |
| --- | --- |
| `A` | calculate file CRC, CRCK-compatible |
| `C` | checksum-mode XMODEM upload |
| `D` | directory |
| `G` | Goodbye |
| `H` | file help |
| `L` | .LBR member directory |
| `Q` | return to main BBS |
| `S` | XMODEM send/download |
| `T` | text type/display |
| `U` | CRC-mode XMODEM upload |
| `V` | verbatim/text-capture upload |

The menu states that `U/C/V` require level 2.

The help further documents:

- library-member access as `LIBNAME/MEMBER.EXT`;
- automatic unsqueezing for `T`ype display but not for XMODEM downloads;
- `.ARC` archives are not decomposed by the BBS;
- uploaded files start private until SYSOP release;
- a 2K text-capture buffer with XON/XOFF;
- directory cancellation/skip controls.

### `A` CRC is not in the supplied v1.08 file-source dispatcher

The inspected v1.08-compatible `filesys.pas` dispatcher has no `A` command. Its upload gate also requires `access > newuser`, which with that source's `newuser = 2` means level 3 or higher.

The v1.09 file menu/help instead advertises:

- `A` CRC calculation;
- upload at level 2.

This is additional concrete evidence that the BBSINFO/help assets represent a different version/configuration from the supplied v1.08-compatible source set.

## Station-specific flavor

The caller-visible text also preserves examples of station personality:

- `WELCOME.TXT`: DING DONG Turbo BBS branding;
- `BULLETIN.TXT`: a topical sports bulletin;
- `BBSLIST.TXT`: joking claim that no other systems are currently listed;
- `APPLYING.TXT`: humorous "$5000" application joke before requesting name/address/phone;
- `SYSINFO.TXT`: joking description of the station's purpose.

These are useful evidence that even early TurboBBS deployments could have substantial local voice while retaining the same runtime command structure. Treat the prose as **Station-specific**, not product-default copy.

## Implications for reconstruction

For `ずっとパソコン通信`:

1. Keep menu/help content outside the core TurboBBS state machine so fictional stations can vary their voice.
2. Preserve versioned command behavior; do not make one universal TurboBBS menu from conflicting 1.05/1.08/1.09 evidence.
3. Model `Q` as a distinct relog path with unsaved profile changes where the selected version supports it.
4. Preserve chained input and semicolon syntax for a v1.09-style runtime.
5. Keep WordStar-era content semantics (soft-wrap information text) separate from modern storage representation.
6. Use the DING DONG files as a real station-customization example, not as the default station fixture.

## Remaining gaps

- provenance of the DING DONG asset archive and date;
- matching v1.09 Pascal source to explain the access-level and file-command changes;
- whether CRC command `A` was a stock v1.09 addition or station-specific modification;
- matching machine/ASYNC layer for the DING DONG IBM PCjr configuration;
- surviving v1.05 BBSINFO files, if different from this later station set.
