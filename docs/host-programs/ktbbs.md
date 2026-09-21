# KTBBS / KT-BBS

Status: research and implementation pending.

Treat KTBBS as an independent host program, not a UI profile. Before implementing a fidelity pass, collect surviving manuals, modules/source where available, distribution documentation, screenshots and connection logs. Document confirmed login→logout flow, board/article model, command semantics, unread handling, mail/chat/files, access levels, prompts and customization mechanisms here.

Do not use behavior from the current generic prototype as historical evidence.

## Account ID / handle research

Current confirmed findings:

- KTBBS separates member ID and handle.
- A surviving Canvas Network explanation states that KTBBS basically displayed ID and handle together, so a member remained identifiable by ID even after changing handles.
- Canvas records show fixed IDs such as CAN0061 and CAN0078 paired with handles.
- Handle changes over time are explicitly documented for member CAN0020.
- The Canvas SYSOP used the handle SADA.Y, confirming at least one period KTBBS-family deployment with a period in the handle.
- Canvas used a heavily modified KTBBS derivative (LF版KT), so station-specific behavior must not automatically be treated as stock KTBBS behavior.

Vector still hosts the KTBBS 6.21A source, user manual, sysop manual, database unit and binaries. Exact handle validation should therefore be established from the source rather than guessed.

Open questions to resolve from KTBBS 6.21A source:

- stored handle field length
- minimum accepted length
- accepted/rejected characters
- whether input preserves case
- whether search/duplicate comparison folds case
- whether stock KTBBS itself checks duplicate handles
- whether handle changes are available to ordinary users or station-specific extensions

A separate KTBBS extension, GIS & GIG 1.00, is described as providing an input function compatible with half-width semicolon. Treat this as evidence that semicolon was special in the standard input path, not as proof of a handle-specific prohibition.

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

Sources:

- https://lavenderblue.jp/chair/candic/candic07.html
- https://lavenderblue.jp/chair/candic/candic10r.html
- https://lavenderblue.jp/chair/candic/candicff.html
- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/ktbbs/by_date.html
- https://www.vector.co.jp/soft/dos/net/se009531.html
- https://www.vector.co.jp/soft/dos/net/se009515.html
