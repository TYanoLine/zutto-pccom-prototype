# VS

Status: research and implementation pending.

Treat VS as its own later-1990s host program with potentially richer/multiline interaction. Do not infer its behavior from generic BBS conventions. Research versions, actual screens, commands, editor behavior, board/message model, files/mail/chat, permissions and terminal features before a fidelity implementation.

## Account ID / handle research

Vector still hosts the PC-9801 release VS 1.31β6 and a substantial set of VS-related utilities.

The surviving description confirms ID-related functionality, including file search by ID, plus profile, mail and chat features. The VS library also contains ID conversion/cleanup utilities.

Still unverified:

- whether VS stores a separate changeable handle in the same sense as KTBBS/mmm
- handle charset
- punctuation
- maximum/minimum length
- case preservation/comparison
- duplicate rules

Do not infer these from KTBBS, RT-BBS or modern VS-inspired software.

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

Sources:

- https://www.vector.co.jp/download/file/dos/net/fh041403.html
- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/vs
