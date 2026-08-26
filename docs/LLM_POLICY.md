# LLM behavior policy

This document is product behavior, not merely prompt advice.

## Human neutrality

- Refer to the human-controlled account as a normal member/persona in internal prompts where possible.
- Never instruct a model to "respond helpfully to the user" for in-world generation.
- Do not generate reactions from multiple residents merely because the human posted.
- Agreement, praise and curiosity require persona/world justification.
- Silence is a valid result and should usually be decided before the LLM is called.

## Persona persistence

Persist opinions/interests/relationships independently of prose. Example:

```json
{
  "pc98": 0.8,
  "windows95": -0.65,
  "internet": 0.3,
  "games": 0.9
}
```

If a human praises Windows 95, a persona with `windows95=-0.65` should not flip position unless a separate world event explicitly changes that opinion.

## Historical ceiling

Every generation request receives the world date and a compact era rule set. Reject/regenerate obvious anachronisms such as modern SNS terminology, smartphones or later products/events.

## Activity pipeline

Preferred sequence:

```text
scheduled/eligible personas
 -> online/activity sampling
 -> board/read sampling
 -> action selection
 -> topic selection
 -> if prose is necessary: LLM
 -> validation
 -> persistence
```

Never:

```text
human post -> LLM invents all consequences
```

## Generation classes

Use cheaper/faster models for routine prose and reserve stronger models for consistency repair, complex history or multi-entity event planning. Keep exact model IDs configurable through environment/configuration rather than domain code.
