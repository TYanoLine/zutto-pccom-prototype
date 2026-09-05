package llm

// DiegeticWorldFrame is the shared interpretation contract for in-world prose
// and semantic planning. Historical ceilings prevent future knowledge; this
// frame additionally prevents a modern observer's retrospective meaning from
// leaking into what contemporary residents find notable or worth naming.
const DiegeticWorldFrame = `DIEGETIC PRESENT / ERA NORMALITY:
- The world date is the actor's literal present. Do not write as a modern author reenacting, remembering, preserving, rediscovering, or explaining that period.
- Never infer that an object, service, habit, machine family, operating system, communication method, game, music format, local custom, or other ordinary contemporary thing is old, retro, nostalgic, surprising, "still usable", or newly rediscovered merely because a later observer would see it that way.
- Broad internal labels and interest keys are routing/context metadata, not user-facing topics, headlines, or vocabulary. Do not echo an internal category just because it appears in the prompt.
- Ordinary baseline conditions stay implicit. Mention a machine, model, service, place, habit, medium, or setup only when the concrete difference, malfunction, comparison, change, decision, social interaction, question, or other supplied event makes that detail relevant.
- A baseline fact such as "uses X", "likes X", "lives near X", "belongs to this BBS", or "normally does X" is not by itself a reason to post. Do not transform it into "tried X", "could still use X", "used X for the first time", "came back to X after a long time", or another invented novelty without canonical support.
- Do not invent hiatuses, rediscoveries, upgrades, purchases, compatibility surprises, membership growth, new arrivals, maintenance, popularity changes, or other world changes merely to make a broad domain seem post-worthy.
- Contemporary residents normally do not explain their own present-day culture to one another. Prefer the unmarked inside view: what changed, failed, differed, happened, was decided, or was said now.
- If a technical family/classification is ordinary background, it normally stays unnamed. A specific product/model/setup should appear only when the exact distinction matters to the event and is supported by canonical or supplied historical facts.
- The same rule applies beyond computers: games, music, local life, BBS participation, operating systems, communication tools, and everyday culture are not automatically "period flavor" to call attention to.
- When the supplied world state does not justify a retrospective or novelty framing, leave that framing out rather than inventing a reason for it.
`
