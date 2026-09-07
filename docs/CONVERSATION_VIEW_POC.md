# Conversation-view materialization PoC

The isolated `fresh` materialization lab can run a development experiment that bypasses the host-wide semantic Producer.

## Goal

Test whether the naturalness of chat-style BBS roleplay can be recovered without making model chat history authoritative world state.

The database remains canonical for:

- actor identity
- timestamps
- board placement
- root/reply topology
- explicit source post
- world-selected routing domain/cause kind
- root discourse mode
- already-rendered BBS prose

The experimental header pass stores only those world-selected shells. It does **not** persist Producer `episode / referents / actor_knowledge / contribution / goal` briefs.

Immediately before prose rendering, the repository rebuilds a transient conversation view from canonical data:

- the exact thread so far
- the explicit source post selected by the world layer
- a few recent canonical posts by the same actor
- small related-post retrieval already used by the existing renderer
- the current shell's world-layer cause boundary

The article worker then chooses natural subject/body wording. Root subjects are committed from the worker result; reply subjects are canonicalized to `Re: <root subject>` after the root has been rendered. Existing chronological dependency rendering guarantees that a reply sees earlier thread prose first.

## Scope

`EnableDevelopmentConversationViewPoC()` is opt-in per repository instance. The server currently enables it only for the isolated fresh RESET-equivalent -> ALLBODY lab. Ordinary runtime materialization continues to use the existing Producer path.

This is intentionally an experiment, not a production architecture decision. If it materially improves naturalness, the next step is to replace the broad cause boundary with a typed, world-owned `WorldPostSituation` rather than re-expanding the semantic Producer.
