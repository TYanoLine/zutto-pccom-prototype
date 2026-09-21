# RT-BBS

Status: research and implementation pending.

Treat RT-BBS as an independent runtime. Gather historical documentation and connection evidence before implementing exact UI/command behavior. Record confirmed version/era, state flow, message structure, feature set and station customization separately from inferred or fictional behavior.

## Account ID / handle research

Vector still hosts both The resource版 Turbo-BBS and the The resource版 Turbo-BBS source kit 5.3βb.
The source-kit description states that the host is written in Turbo Pascal 6.0A and that the source is published.

This makes RT-BBS one of the best candidates for source-level verification of:

- member/account record layout
- handle field length
- minimum length validation
- permitted characters
- upper/lowercase normalization
- duplicate comparison
- signup/update behavior

No exact RT-BBS handle validation rule is currently considered confirmed.

Project-wide handle policy is documented in:
docs/research/ACCOUNT_ID_HANDLE_EVIDENCE.md

Sources:

- https://www.vector.co.jp/vpack/filearea/dos/net/comm/host/rtbbs
- https://www.vector.co.jp/soft/dos/net/se048002.html
