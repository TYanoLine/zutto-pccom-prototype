# Fresh materialization lab observability

The isolated `materialization-lab-fresh` job captures the generated article surface after the run completes. It is development-only instrumentation and does not mutate the persisted demo world.

The **current fresh Lab intentionally enables the conversation-view materialization PoC** for its isolated repository. It performs RESET-equivalent world-shell selection, bypasses the host-wide semantic Producer, reconstructs transient conversation context from canonical BBS records, and then runs ALLBODY. Ordinary runtime materialization still uses the existing Producer path unless explicitly changed elsewhere.

Captured fields include article identity, board/thread linkage, subject/body, causal routing metadata, source/responds-to IDs, and the legacy Producer observability fields. In the current conversation-view PoC, `producer_episode`, `producer_referents`, `producer_actor_knowledge`, `producer_audience_context`, `producer_contribution`, and `producer_must_not` are expected to be empty because no Producer brief is created. Their presence in the response shape is retained for comparison with Producer-based runs and older tooling.
