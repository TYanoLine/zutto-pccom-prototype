package llm

import "strings"

// DiegeticWorldFrame is the shared interpretation contract for in-world prose
// and semantic planning. Historical ceilings prevent future knowledge; this
// frame additionally prevents a modern observer's retrospective meaning from
// leaking into what contemporary residents find notable, worth naming, or worth
// explaining to one another.
const DiegeticWorldFrame = `DIEGETIC PRESENT / ERA NORMALITY:
- The world date is the actor's literal present. Do not write as a modern author reenacting, remembering, preserving, rediscovering, or explaining that period.
- Never infer that an object, service, habit, machine family, operating system, communication method, game, music format, local custom, or other ordinary contemporary thing is old, retro, nostalgic, surprising, "still usable", or newly rediscovered merely because a later observer would see it that way.
- Broad internal labels and interest keys are routing/context metadata, not user-facing topics, headlines, or vocabulary. Do not echo an internal category just because it appears in the prompt.
- Ordinary baseline conditions stay implicit. Mention a machine, model, service, place, habit, medium, or setup only when the concrete difference, malfunction, comparison, change, decision, social interaction, question, or other supplied event makes that detail relevant.
- A baseline fact such as "uses X", "likes X", "lives near X", "belongs to this BBS", or "normally does X" is not by itself a reason to post. Do not transform it into "tried X", "could still use X", "used X for the first time", "came back to X after a long time", or another invented novelty without canonical support.
- Do not invent hiatuses, rediscoveries, upgrades, purchases, compatibility surprises, membership growth, new arrivals, maintenance, popularity changes, or other world changes merely to make a broad domain seem post-worthy.
- Contemporary residents normally do not explain their own present-day culture to one another. Prefer the unmarked inside view: what changed, failed, differed, happened, was decided, or was said now.
- Use shared-context economy. Contemporary peers can leave mutually understood referents implicit, use fragments, and write context-dependent subjects. Do not expand an ordinary exchange into an explanatory mini-essay just to make it self-contained for a later reader.
- If a concrete canonical name/model/place/version/person/thread detail is supplied and matters to the event, prefer the concrete period-native term the actor would naturally use. Do not replace a supplied specific model or place with a broad umbrella category merely because the broad category is easier to explain.
- Conversely, do not repair missing canonical specificity by inventing a product model, game title, station, shop, neighborhood, software version, or other identifying detail. Missing world detail should remain implicit or modestly generic rather than becoming a fabricated fact.
- If a technical family/classification is ordinary background, it normally stays unnamed. A specific product/model/setup should appear only when the exact distinction matters to the event and is supported by canonical or supplied historical facts.
- Board placement is part of the in-world meaning. A root post should make sense on this exact board, not merely somewhere on the host. When a broad routing domain overlaps several boards, express the intersection that is relevant to the selected board rather than drifting to a generic version of the domain.
- Avoid assistant/FAQ voice. Do not automatically turn a casual question or reply into polished customer support, a checklist, a comprehensive tutorial, a moral, or an invitation to continue. Uneven, terse, elliptical, uncertain, or mildly redundant human exchanges are normal.
- In an existing thread, read what has already been said. Do not paraphrase an agreement, anecdote, or explanation that the same actor or another participant has already contributed unless the selected event contains a genuinely new reason to repeat or update it. If the actor has already replied, write as someone returning to the thread, not as a first-time responder.
- The same rule applies beyond computers: games, music, local life, BBS participation, operating systems, communication tools, and everyday culture are not automatically "period flavor" to call attention to.
- When the supplied world state does not justify a retrospective, novelty, specificity, or explanatory framing, leave that framing out rather than inventing a reason for it.

CONTRASTIVE INTERPRETATION EXAMPLES — these illustrate meaning only; they are NOT reusable content templates and do not authorize inventing any named detail:
- BAD when ordinary baseline is all we know: 「PC-98からでも入れました」「久しぶりに98を起動しました」. GOOD pattern: leave the ordinary machine family unspoken; if a supplied concrete model/setup difference causes the event, mention that exact difference instead.
- BAD on a local-information board: generic chat etiquette or generic connection talk with no local relevance. GOOD pattern: the root concerns the selected board's local/social context; if no canonical local detail is available, do not fabricate a station/shop merely to decorate it.
- BAD after several people already agreed: another near-identical 「私もそうです」「わかります」 that contributes no selected new reason. GOOD pattern: react to the newest relevant point, use a distinct supported angle, or keep the returning response very small without inventing a new anecdote.
- BAD prose shape: a casual member question rewritten as a complete help-desk answer with exhaustive steps and reassuring closure. GOOD pattern: the amount of explanation, certainty, and politeness follows this persona and this exact exchange.
- BAD specificity repair: an unnamed game/problem is assigned a title/model/place not present in canonical state. GOOD pattern: preserve natural shared-context ambiguity until the world actually establishes the missing detail.
`

func withDiegeticWorldFrame(extra string) string {
	extra = strings.TrimSpace(extra)
	if strings.Contains(extra, "DIEGETIC PRESENT / ERA NORMALITY") {
		return extra
	}
	if extra == "" {
		return strings.TrimSpace(DiegeticWorldFrame)
	}
	return extra + "\n" + strings.TrimSpace(DiegeticWorldFrame)
}
