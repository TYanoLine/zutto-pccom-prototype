# Fresh materialization lab observability

The isolated `materialization-lab-fresh` job captures the generated article surface and Producer brief fields after the run completes. This is development-only instrumentation used to inspect the same RESET-equivalent -> World Window Producer -> ALLBODY pipeline without mutating the persisted demo world.

Captured fields include article identity, board/thread linkage, subject/body, causal routing metadata, and the canonical Producer episode/referents/knowledge/audience/contribution/must-not instructions supplied to the Article Worker.
