# Historical host-program index

Each historical BBS package is modeled as its own runtime/state machine. These documents collect evidence and reconstruction decisions; they are not merely feature-profile definitions.

Planned/active families:

- TurboBBS / Turbo BBS (Robert H. Maxwell) — `turbo-bbs.md`; source reconstruction — `turbo-bbs-source-notes.md`; caller-visible/menu assets — `turbo-bbs-ui-assets.md`
- KTBBS / KT-BBS — `ktbbs.md`
- BIG-Model — `big-model.md`
- 絵理香K版 — `erika-k.md`
- mmm — `mmm.md`
- RT-BBS — `rt-bbs.md`
- VS — `vs.md`

For each package, record:

1. versions/era being targeted;
2. confirmed login→logout flow;
3. commands and menus;
4. board/article/reply structure;
5. mail/chat/file-transfer behavior;
6. access-control semantics;
7. terminal/ANSI behavior;
8. confirmed customization points;
9. source list;
10. explicit `Confirmed`, `Likely`, `Station-specific`, and `Fictional reconstruction` notes.

Do not infer one program's behavior from another merely because both were Japanese grass-roots BBS software. Shared source ancestry (for example TurboBBS → KTBBS/RT-BBS) is provenance, not proof that commands or UI behavior remained unchanged.


## Reply/article representation rule

When researching a host package, record reply behavior as more than a boolean
"supports replies". At minimum distinguish:

- whether a response is stored/displayed as a child/append or a flat message;
- whether the response has its own subject field;
- whether a prefix such as `Re:` is generated automatically, copied, user-entered,
  or not present;
- whether replies appear independently in indexes;
- how response traversal/counting is exposed to the caller.

The shared world relation `RespondsToPostID` is not evidence for any of those
UI/storage details. Do not copy one host program's reply convention into another.
