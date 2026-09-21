# TurboBBS / Turbo BBS (Robert H. Maxwell)

Status: primary-source baseline established from the v1.05 System Operator Notes and a v1.08 BBS.PAS main source snapshot; include-file-level command/state reconstruction remains pending.

This document uses **TurboBBS** for Robert H. Maxwell's 1985 program. It is the upstream historical context for Japanese descendants such as KTBBS and, through the KPUC lineage, RT-BBS. Do not treat those descendants as UI skins or as evidence that every later feature existed in the 1985 original.

## Identification and disambiguation

### Confirmed

- Author: Robert H. Maxwell.
- Surviving BBS Software Directory records version 1.05 in 1985.
- Maxwell's own retrospective says he started the program after receiving Borland Turbo Pascal 2.0.
- It was designed from the beginning to run under both CP/M and MS-DOS, with platform-specific communications handling.
- TurboBBS provided its own modem/communications routines rather than depending on the common CP/M BYE layer. Maxwell says this was an intentional reliability/security choice: a BBS crash should not expose an operating-system command prompt.
- Maxwell says he announced the running BBS on Borland's CompuServe SIG and, after a request from Borland SYSOP Bela Lubkin, cleaned up and publicly released the version 1.0 Pascal source in 1985.
- Known deployments/ports mentioned by Maxwell include his Kaypro and Osborne-1 systems, an MS-DOS Compaq Portable installation in Vancouver, a Nokia MicroMikko II port in Finland, and a TI Professional PC port for a Michigan user group.
- Maxwell moved from Vancouver to the Toronto area in late 1986 and describes that as effectively ending his own TurboBBS project, although the code continued to circulate and be modified elsewhere.

### Do not confuse with

The BBS Software Directory lists these as separate packages:

- **TBBS** by Phil Becker/eSoft — a different CP/M/MS-DOS BBS family.
- **Turbo Pascal Bulletin Board System / TPBBS** by James Whorton and Eddie H. Curlin — also a different program.
- Later Japanese **KTBBS** and **The resource版 Turbo-BBS / RT-BBS** — descendants/arrangements in the Turbo-BBS lineage, not the unchanged 1985 Maxwell program.

The names are close enough that searches for "TBBS", "Turbo BBS", or "Turbo Pascal BBS" can easily mix unrelated software.

## Surviving distribution evidence

### Confirmed archival evidence

A surviving software-library catalog entry titled **Turbo BBS (& source)** lists a 1985 package containing at least:

- `BBS.PAS` — bulletin-board source
- `BBS100.DOC` — documentation
- `BBSHELP.TXT` — online help
- `BBSLIST.TXT` — BBS list
- `CLOCK.INC` — include file
- `COMMENTS.BBS` — comments/messages to SYSOP
- `FILES.BBS` — file list

This is useful provenance and shows that source, documentation, help, SYSOP-comment data and file-list data circulated together. It is **not yet** sufficient to reconstruct exact login prompts, command letters, message numbering, unread semantics, password rules, or file-transfer behavior; those must come from the actual source/manual.

## Direct primary-source inspection: v1.05 manual and v1.08 main source

Two surviving primary-source artifacts were inspected directly. They are **not the same version** and must not be silently merged into one specification:

- `Turbo BBS Version 1.05 - System Operator Notes` — 20,352 bytes, SHA-256 `6d7b3f0a834366d9b9a74fd3fd55c67b9c2042d871edc4baa55f12aa35ea2ca9`
- `BBS.PAS` — 7,510 bytes, SHA-256 `251d4139ae2b72bc82e9cfb2b0e6b1181f1f716c8d9a247c66eacaabefcbd82d`; the source itself declares `version = '1.08'`

The 1.05 manual is the stronger source for 1.05 runtime behavior. The 1.08 `BBS.PAS` is the stronger source for the later main-program constants, record layout and top-level control flow. Exact behavior implemented in include files remains open unless the 1.05 manual also documents it.

### Version 1.05 — confirmed features and data model

The System Operator Notes explicitly document:

- self-contained Pascal message and file systems;
- an operating-system-independent design goal;
- WordStar-editable information files;
- variable terminal width with word wrapping;
- persisted user profile information;
- real-time-clock timestamps;
- public and private messages;
- public/private BBS configuration;
- variable access levels;
- file/message sections;
- XMODEM CRC and checksum support;
- sign-on/sign-off call logging;
- comments to SYSOP;
- squeezed/library-file support;
- direct serial-port hardware control.

Persistent files documented by the manual:

- `MESSAGES.BBS` — one metadata record per message containing sender and receiver user numbers, subject, timestamp, message number, section number and a receiver-read flag. The table is buffered in RAM while a caller is online and written back at sign-off.
- `FILES.BBS` — filename, size, contributor, access count, section, and public/private state.
- `LOG.BBS` — caller plus sign-on/sign-off times when clock support is enabled.
- `COMMENTS.BBS` — comments to SYSOP with caller and timestamp. `G`oodbye permits up to 15 lines; `A`pply permits up to 4.
- `IDS.BBS` — registered username, password, access level, terminal parameters, last-call date and high-message number.

Message bodies are separate `MESSxxxx.TXT` files. The manual describes both hard-CR formatting and a flowing format in which the output layer inserts line breaks for the current terminal width.

The distribution default is `maxmess = 52`; each message-table record is documented as using 43 bytes of RAM. The distribution file table is limited to 40 entries, with each directory record documented as using 39 bytes.

### Version 1.05 — confirmed information/menu files

The manual expects online information in `B:BBSINFO.LBR`, including:

- `WELCOME.TXT` — pre-sign-on welcome/news; also command `W`
- `BBSHELP.TXT` — command/help text; command `H`
- `BBSLIST.TXT` — other systems; command `O`
- `SYSINFO.TXT` — system information; command `Y`
- `MEETING.TXT`
- `APPLYING.TXT` — access-upgrade information used by `A`pply
- `MAINMENU.TXT`
- `EDITMENU.TXT`
- `READMENU.TXT`
- `FILEHLP.TXT`
- `FILEMENU.TXT`
- `BULLETIN.TXT`

These external text/menu files are an important customization boundary: exact wording and menu presentation can vary without changing the Pascal runtime.

### Version 1.05 — confirmed modem/serial behavior

The distribution `MACHDEP.INC` was written for a Rixon 212A Intelligent Modem.

The manual says the machine-dependent layer must provide direct serial control, including carrier/status lines and buffer state. For XMODEM, the serial format must be 8 data bits, no parity, one stop bit.

The distribution expects CTS to serve as carrier detect. It explicitly allows adapting the `cts` routine to use DSR or DCD instead. Hangup is expected to lower DTR for 400 ms; local access also lowers DTR to inhibit modem auto-answer.

This confirms that TurboBBS deliberately modeled modem state through hardware signals rather than depending only on parsing modem result strings.

### Version 1.05 — confirmed authentication/access behavior

- Usernames and passwords are mapped to uppercase.
- The username `SYSOP` is automatically assigned access level 5 on its first sign-on. The manual explicitly warns that the first person to use this name obtains master access.
- Local sign-on is entered by pressing ESC at `Waiting for call...`.
- Setting `openBBS = false` makes the system private: a remote caller who does not supply a registered username within three attempts is disconnected. New users must first be created by signing on locally.

The manual does not define a complete semantic table for every numeric access level, but it does directly establish:

- level 5 = SYSOP/master functions;
- level 3 can retrieve private files when the exact filename is known.

### Version 1.05 — confirmed SYSOP command behavior

Two main-program commands are intentionally omitted from the normal menus:

- `!` — toggles printer mirroring for a level-5 user.
- `@` — enters a SYSOP-only `?` prompt.

Inside the `@` SYSOP prompt:

- `C` — display the user-comments file, then optionally kill it;
- `L` — edit a user's access level;
- `!` — same printer toggle.

Existing commands also gain SYSOP extensions:

- `L` (call Log) offers `Kill (Y/N)?`;
- `U` (User list) displays user access levels;
- `R` (message Read) offers deletion after each message;
- SYSOP can see all messages.

File-system SYSOP commands documented by the manual:

- `I` — install a file already present on the files disk into the BBS directory;
- `E` — edit file metadata (name, contributor, section) and release the file;
- `K` — remove the file from the directory and erase it.

Newly uploaded/installed files begin private. They are not listed to ordinary users until released. Level-3 users may access a private file if they already know its filename.

### Version 1.08 `BBS.PAS` — confirmed main-source facts

The inspected main source retains Robert H. Maxwell's 1985 copyright/header and identifies the original Vancouver system as a Kaypro 2-84 with Rixon 212A modem at 300/1200 baud.

The file declares:

```text
version = '1.08'
clockin = true
sectsin = true
openBBS = true
```

Its named access constants are:

```text
twit    = 0
newuser = 2   { Let new users download and leave messages }
reg     = 3
sysop   = 5
```

No semantic label for levels 1 or 4 is present in this main file.

The source defines:

- `name = string[14]`
- `person = string[27]`
- `line = string[80]`
- `long = string[150]`

The `sysid` record used by `IDS.BBS` is:

```text
user : person
exfl : byte
lsto : name
lstm : integer
pass : name
acc  : byte
clr  : name
bsp  : char
lnf  : char
upc  : boolean
wid  : byte
```

Therefore, in this 1.08 snapshot the stored password field is at most 14 characters and the stored user field is at most 27 characters. Do not apply those exact lengths backward to 1.05 without its corresponding source.

The 1.08 main file includes:

- `ASYNC.PAS`
- `INT24.PAS`
- `MACHDEP.INC`
- `IO.INC`
- `MAILSYS.INC`
- `FILESYS.INC`
- `BBS2.INC`

The top-level call flow is directly visible:

```text
setup
defaults
awaitcall
  -> timestamp call
  -> output "TurboBBS version 1.08"
  -> WELCOME
  -> BULLETIN
  -> signon
  -> initmess
  -> readmine
  -> command
  -> endcall / close / unload / defaults
awaitcall
```

This confirms the ordering of welcome/bulletin, sign-on, message initialization and the main command loop. The exact behavior of `signon`, `readmine`, `command`, message entry, XMODEM and other subsystems cannot be reconstructed from this main file alone because their implementations live in the include files.

The inspected 1.08 snapshot has `filedrive = 'F:'`, whereas the 1.05 manual describes `B:` as the distribution files drive. Treat this as a version/configuration difference, not proof that either value was universal.

### Flat sections, not RT-BBS-style hierarchy

The 1.05 manual describes optional message/file **sections**, with section names defined in `MAILSYS.INC`; when sections are disabled, all content is placed in section 1, default `General`.

This is evidence for a comparatively simple section model in original TurboBBS 1.05. It is not evidence for the later multi-level hierarchical-board model documented in RT-BBS 5.x. Do not project RT-BBS hierarchy backward into the Maxwell runtime.

### Licensing/distribution wording

The 1.05 notes say the software, documentation and support files are "released to the public domain" while also prohibiting commercial redistribution/resale without the author's prior permission. The 1.08 source header similarly says the code is released to the public domain but that the author "reserves all rights" and restricts charges.

Those statements are historically important but internally inconsistent as modern licensing language. For this project, record the wording as provenance and **do not infer a clean modern open-source/public-domain license** from it without separate legal analysis.

### Remaining primary-source gaps

To reconstruct the original runtime exactly, obtain the corresponding include files and menu/help assets, especially:

- `BBS2.INC`
- `IO.INC`
- `MACHDEP.INC`
- `MAILSYS.INC`
- `FILESYS.INC`
- `MAINMENU.TXT`, `READMENU.TXT`, `EDITMENU.TXT`, `FILEMENU.TXT`, `BBSHELP.TXT`

These are required to establish exact sign-on prompts, new-user flow, normal command letters, message-selection/unread rules, editor subcommands, file-transfer prompts and serial/modem state transitions.

## Japanese lineage

### KTBBS — Confirmed

Vector's surviving **KTBBS ユーザーズマニュアル** explicitly expands the name as **KPUC版 Turbo BBS** and says it was initially developed by 久喜PCユーザーズクラブ (Kuki PC Users Club) for its own station, then released generally after repeated version upgrades and used by many BBSs.

A later KTBBS user retrospective likewise describes KTBBS as a modification of the freeware Turbo-BBS and notes that individual stations often modified KTBBS further.

For project purposes, therefore:

```text
Robert H. Maxwell TurboBBS (1985)
        |
        +--> KPUC版 Turbo BBS / KTBBS
                 |
                 +--> many station-specific KTBBS modifications
                 |
                 +--> RT-BBS lineage (see evidence below)
```

The exact source-level fork points and version-to-version ancestry still need to be established from surviving source headers/version histories.

### RT-BBS — strong archival provenance, exact fork point pending

A surviving archive listing for **The resource edition Turbo-BBS ver5.3a** describes it as an MS-DOS source release and includes both an Eskage/ComServe copyright line dated 1993-01-02 and a **KUKI PC-UsersClub/Taka** copyright credit.

Together with KTBBS's documented expansion as KPUC版 Turbo BBS, this is strong evidence that RT-BBS belongs to the KPUC/Turbo-BBS code lineage rather than merely sharing a similar name.

However, until the archived RT-BBS source headers and KTBBS source are compared directly, keep the following as unresolved:

- which KTBBS/Turbo-BBS version RT-BBS forked from;
- which subsystems were retained versus rewritten;
- whether data-file formats stayed compatible;
- which command letters/prompts were inherited;
- when multi-line, hierarchical-board, gateway, file-transfer and enhanced SYSOP features entered the lineage.

See `rt-bbs.md` for the later RT-BBS feature set.

## Station-specific case: ミンキームーンネットワーク

This is useful evidence for what a heavily customized **RT-BBS-family station** looked like in Japan, but it must not be mistaken for stock RT-BBS behavior.

### Confirmed — 1993 station advertisement

The station's preserved 1993 advertisement states:

- Host Program: **ミンキームーンネットワーク版RTBBS 姫ちゃんヴァージョン**
- "ホストプログラムは、RTBBSを改造しています。"
- host machine: NEC PC-9801 RA2 with Cx486DLC
- multiple modem channels, including 14,400 bps and 9,600 bps lines at the time of the advertisement
- Shift_JIS, 8 data bits, 1 stop bit, no parity, flow control enabled
- guest ID `GUEST`
- access levels 1–4 with different session-time limits

This is unusually good evidence that RT-BBS was not only deployed on PC-98 hardware but also treated as source-modifiable station software.

### Confirmed — preserved 1995 station log, but station-specific

The 30th-anniversary article reproduces a 1995 log showing the customized station's actual interaction style:

- ID/password login
- a themed main prompt such as `Ch.1 = MAIN = ... >>`
- `N` for "未読連続一気読み"
- date-based unread scanning and numeric board IDs
- `W` displaying current channels/users, IDs, handles, current activity, baud/protocol and memo
- `D` used for SYSOP calling; the retrospective says this caused a BEEP at the host
- chat-room use
- logout sequence `QY`

These are valuable **Station-specific** observations. Do not implement them as default RT-BBS commands/prompts until the RT-BBS source/manual confirms which parts are stock and which were the station's "姫ちゃんヴァージョン" modifications.

## Implications for ずっとパソコン通信

1. **TurboBBS itself is mainly an ancestor/context package for the 1996 world.** Maxwell's own development effectively stopped in the late 1980s; the Japanese descendants are more directly relevant to a 1996 Japanese simulation.
2. **KTBBS and RT-BBS must remain separate runtimes.** Shared ancestry is not a reason to collapse them into one generic menu system.
3. **Preserve descendant provenance without copying features backward.** RT-BBS 5.x features cannot be assumed to exist in TurboBBS 1.05.
4. **Treat station customizations as first-class evidence.** ミンキームーン demonstrates that a recognizable RT-BBS installation could be heavily themed and behaviorally customized.
5. **Primary-source inspection now constrains the runtime substantially.** The v1.05 manual fixes the storage model, several commands, access/SYSOP behavior and modem assumptions; the v1.08 main source fixes later constants, record layout and top-level flow. Do not invent the remaining include-file behavior.

## Open research tasks

- Obtain the remaining matching include files (`BBS2.INC`, `IO.INC`, `MACHDEP.INC`, `MAILSYS.INC`, `FILESYS.INC`) and the menu/help text assets.
- Determine the exact normal-user sign-on/new-user prompts from the matching source version.
- Determine the complete normal-user command letters and subcommands from source/menu assets.
- Determine the exact `readmine` selection/unread algorithm and how `IDS.BBS` high-message state interacts with per-message receiver-read state.
- Determine exact message reply/forward/edit semantics.
- Determine exact XMODEM/file-transfer prompts and error handling.
- Establish whether the inspected v1.08 main file is pristine upstream 1.08 or a configured/site-modified copy (notably `filedrive = 'F:'`).
- Compare TurboBBS source structure with KTBBS 6.21A and RT-BBS 5.3 source headers to establish a defensible code genealogy.
- Preserve the original distribution/licensing wording without treating it as a modern standardized license.

## Sources

### Direct-author / archival

- BBS Software Directory, TurboBBS entry (includes Robert H. Maxwell's retrospective):  
  https://mirrors.archeobits.com/bbs/software.bbsdocumentary.com/expanded.html
- ProgrammaTheek volume 3005, `Turbo BBS (& source)` catalog entry:  
  https://cdnc.heyzine.com/files/uploaded/d65ccb2f1aa4b740ec948ba6ccfef2b5c2091c0b.pdf
- Vector, KTBBS ユーザーズマニュアル:  
  https://www.vector.co.jp/soft/dos/net/se009531.html
- War Ensemble BBS archive listing, `rtb53bbs.zip`:  
  https://bbs.warensemble.com/?dir=bbsrelatbbsdos&page=002-files.xjs
- Vector, The resource版 Turbo-BBS ソースキット:  
  https://www.vector.co.jp/soft/dos/net/se048002.html
- Vector, RTBBS category:  
  https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/rtbbs

### Station evidence

- ミンキームーンネットワーク, 1993年当時のBBS宣伝テキスト:  
  https://minkymoon.jp/senden-cg/
- ミンキームーンネットワーク, 開局30周年記事 / 1995ログ:  
  https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/

### Secondary / retrospective

- Masami's Storiette, KTBBS explanation:  
  https://lavenderblue.jp/chair/candic/candicff.html
- 教科書には載らないニッポンのインターネットの歴史 (KTBBS chronology; use as secondary evidence only):  
  https://terrazi.hateblo.jp/entry/20020930/p2
