# Structured BBS semantic planning

The development BBS semantic planner uses the Responses API Structured Outputs path (`text.format.type = json_schema`, `strict = true`) rather than relying on prompt-only JSON formatting.

This is an operational output contract, not a content template. The JSON schema constrains only the transport shape (`events`, free-form semantic strings, and free-form persona fact key/value pairs). It does not contain a topic catalog, subject bank, information-slot list, response-act enum, or conversation progression template.

World state ownership remains unchanged: actor, timestamp, and reply topology are selected before the planner runs; returned semantics are validated by the application; existing Persona facts win on conflicts; no proposed facts are committed until the full atomic planning sequence succeeds.
