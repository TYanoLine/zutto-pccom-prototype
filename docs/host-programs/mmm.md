# mmm

Status: research and implementation pending.

mmm is expected to differ materially from other host packages, especially around message/board structure. Do not force it into KTBBS/Erika/BIG-Model concepts for reuse. Research its own terminology, hierarchy, article model, commands, unread behavior, login/logout, mail/chat/files and customization before implementation.

## Account ID / handle research

Confirmed / strongly supported:

- Period material for a mmm Rev.4.1 station describes entering new at the ID prompt to receive an ID.
- Surviving MASH/mmm-family user documentation distinguishes account ID from a registered handle name.
- The handle command is documented as setting/changing the handle name.
- Mail addressing can use an ID or the registered handle, so the handle is functionally significant and not merely decorative display text.
- Current Midnight Driving operation documentation says MASH commands are entered in lowercase. This is command syntax evidence only; do not infer handle case-sensitivity from it.

Still unknown:

- handle maximum/minimum length
- permitted punctuation
- case preservation
- case-folding for lookup
- exact duplicate-handle behavior
- whether these rules changed between mmm revisions and MASH derivatives

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

Sources:

- https://old.tsg.ne.jp/buho/189/tsg189
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/manual/MASHMAN/USER-utf8.txt
- https://webmid.kinet.ne.jp/mid/manual/wtsbbs/manual/MASHMAN/COMMANDS-utf8.txt
