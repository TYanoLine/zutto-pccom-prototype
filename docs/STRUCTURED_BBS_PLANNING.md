# Structured BBS semantic planning

The development BBS semantic planner uses the Responses API Structured Outputs path (`text.format.type = json_schema`, `strict = true`) rather than relying on prompt-only JSON formatting.

This is an operational output contract, not a content template. The JSON schema constrains only the transport shape (`events`, free-form semantic strings, and free-form persona fact key/value pairs). It does not contain a topic catalog, subject bank, information-slot list, response-act enum, or conversation progression template.

World state ownership remains unchanged: actor, timestamp, and reply topology are selected before the planner runs; returned semantics are validated by the application; existing Persona facts win on conflicts; no proposed facts are committed until the full atomic planning sequence succeeds.

## Subject-line calibration

Root subjects are generated as the text that the selected actor would actually type into the historical BBS subject field, rather than as a modern headline or article-summary task.

The production structured planner includes a compact calibration derived from preserved Japanese PC-communication subject-line corpora. The evidence shows that subject fields can be terse, fragmentary, person-directed, context-dependent, declarative, announcement-like, playful, or interrogative. Questions are therefore not the default form, and subjects do not need to summarize the body or make sense to an outsider without board context.

This calibration is deliberately **not** a subject template bank or percentage distribution. The planner does not rotate through title categories, copy historical strings, or assign a fixed numeric subject-style vector to every persona. It uses the actor, event semantics, prior board history, and existing persona behavior, while checking a batch for accidental convergence on the same rhetorical construction.

Research basis and corpus caveats are recorded in `docs/research/BBS_SUBJECT_CORPUS.md`. Raw third-party corpus data remains outside Git.
