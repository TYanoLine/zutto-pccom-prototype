# Materialization Lab read-only viewer

The evaluator at `/poc/materialization-lab-viewer` exists so humans can judge the actual prose and conversation produced by isolated fresh-Lab runs without copying those posts into the saved demo BBS.

## Boundary

- The viewer is read-only. Its server endpoint accepts GET only.
- Reading an archive never calls `WorldRepository`, an observation gate, RESET, the Situation Proposer, an Article Worker, or any other LLM path.
- Completed fresh jobs are archived in PostgreSQL separately from canonical world snapshots.
- Archive rows are experiment evidence and must never be interpreted as canonical world history.
- Normal runtime does not read these rows.

## UX

The page shows recent runs, boards, thread subjects and full generated bodies. Generation metadata (`anchor_key`, cause, discourse mode, Situation summary/facts and source IDs) is hidden by default so prose can first be judged as ordinary BBS conversation. A developer can opt in to the diagnostic layer for causal review.

A job can be deep-linked with `?job=<lab-job-id>` so the same generation result can be reviewed after a server deploy/restart.
