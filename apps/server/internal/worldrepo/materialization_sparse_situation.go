package worldrepo

import (
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

// developmentSparseSituation is a cheap, canonical micro-situation selected only
// after the world has already decided that a write event exists. It deliberately
// stays much smaller than a simulated life: enough is fixed to keep separate
// roots distinct and answerable, while prose and incidental wording remain lazy.
type developmentSparseSituation struct {
	kind    string
	summary string
	facts   []string
}

type developmentSituationFacet struct {
	kind        string
	focus       string
	boundary    string
	occurrences []string
}

type developmentSituationCandidate struct {
	facet   developmentSituationFacet
	weight  float64
	blocked bool
}

func developmentSituationForShell(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post, source *world.Post) developmentSparseSituation {
	if source != nil {
		if shell.action == "reply" {
			return developmentSituationFromSource(*source, false)
		}
		if shell.causeKind == "continuation_progress" {
			return developmentSituationFromSource(*source, true)
		}
	}
	return developmentSelectRootSituation(host, board, shell, prior)
}

func developmentSituationFromSource(source world.Post, continuation bool) developmentSparseSituation {
	kind := strings.TrimSpace(source.Intent.SituationKind)
	if kind == "" {
		kind = "source_thread_context"
	}
	summary := strings.TrimSpace(source.Intent.SituationSummary)
	if summary == "" {
		summary = fmt.Sprintf("The canonical source post %04d is the situation boundary for this contribution.", source.ID)
	}
	facts := append([]string(nil), source.Intent.SituationFacts...)
	if continuation {
		summary = "A materially new development occurred inside the earlier canonical situation. " + summary
		facts = append(facts, "continuation=Add a genuinely new development; do not restate the earlier root.")
	} else {
		facts = append(facts, "reply_binding=Respond to the explicit canonical source; do not replace it with another topic or event.")
	}
	return developmentSparseSituation{kind: kind, summary: summary, facts: facts}
}

func developmentSelectRootSituation(host world.Host, board world.Board, shell developmentTimelineShell, prior []world.Post) developmentSparseSituation {
	facets := developmentSituationFacets(shell.anchorKey)
	if len(facets) == 0 {
		facets = developmentSituationFacets("generic")
	}
	candidates := make([]developmentSituationCandidate, 0, len(facets))
	hasUnblocked := false
	for _, facet := range facets {
		weight, blocked := developmentSituationNovelty(facet.kind, board.ID, shell.persona.ID, shell.createdAt, prior)
		if !blocked {
			hasUnblocked = true
		}
		candidates = append(candidates, developmentSituationCandidate{facet: facet, weight: weight, blocked: blocked})
	}

	total := 0.0
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		total += candidate.weight
	}
	if total <= 0 {
		total = float64(len(candidates))
		for i := range candidates {
			candidates[i].weight = 1
			candidates[i].blocked = false
		}
		hasUnblocked = true
	}

	roll := demoStableUnit(host.ID, board.ID, shell.persona.ID, shell.createdAt.Format(time.RFC3339), fmt.Sprintf("sparse-situation-v1-%d", shell.index)) * total
	selected := candidates[len(candidates)-1].facet
	for _, candidate := range candidates {
		if hasUnblocked && candidate.blocked {
			continue
		}
		if roll < candidate.weight {
			selected = candidate.facet
			break
		}
		roll -= candidate.weight
	}
	return developmentComposeSituation(host, board, shell, selected)
}

// developmentSituationNovelty is intentionally local and cheap. It does not try
// to reason over the whole world. It only prevents the same micro-situation from
// dominating a board window or the same actor's recent roots. If every facet is
// exhausted, the selector falls back to weighted reuse rather than growing a
// brittle rule catalog.
func developmentSituationNovelty(kind, boardID, personaID string, at time.Time, prior []world.Post) (float64, bool) {
	weight := 1.0
	blocked := false
	for i := len(prior) - 1; i >= 0; i-- {
		post := prior[i]
		if post.ParentID != 0 || post.BoardID != boardID || post.Intent.SituationKind != kind {
			continue
		}
		age := at.Sub(post.CreatedAt)
		if age < 0 {
			continue
		}
		switch {
		case age < 36*time.Hour:
			blocked = true
			weight *= .04
		case age < 4*24*time.Hour:
			weight *= .18
		case age < 10*24*time.Hour:
			weight *= .42
		default:
			weight *= .72
		}
		if post.AuthorPersonaID == personaID {
			if age < 10*24*time.Hour {
				blocked = true
			}
			weight *= .10
		}
	}
	if weight < .001 {
		weight = .001
	}
	return weight, blocked
}

func developmentComposeSituation(host world.Host, board world.Board, shell developmentTimelineShell, facet developmentSituationFacet) developmentSparseSituation {
	occurrence := "A small concrete occurrence happened inside this focus."
	if len(facet.occurrences) > 0 {
		index := int(demoStableUnit(host.ID, board.ID, shell.persona.ID, shell.createdAt.Format(time.RFC3339), facet.kind, "occurrence-v1") * float64(len(facet.occurrences)))
		if index >= len(facet.occurrences) {
			index = len(facet.occurrences) - 1
		}
		occurrence = facet.occurrences[index]
	}
	mode := developmentSituationModeSummary(shell.discourseMode, facet.focus)
	summary := strings.TrimSpace(mode + " Canonical occurrence: " + occurrence + " Scope boundary: " + facet.boundary)
	facts := []string{
		"focus=" + facet.focus,
		"occurrence=" + occurrence,
		"scope_boundary=" + facet.boundary,
		"root_independence=This is an independent root situation. Do not import event facts from another root or from the actor's recent posts unless SourcePostID explicitly links them.",
	}
	if shell.discourseMode == "ask_peers" {
		facts = append(facts, "answerability=State enough concrete observable detail for another member to answer without knowing an unnamed title, place, product, device, or hidden choice.")
	}
	return developmentSparseSituation{kind: facet.kind, summary: summary, facts: facts}
}

func developmentSituationModeSummary(mode, focus string) string {
	switch mode {
	case "share_observation":
		return "The actor recently noticed one small concrete condition involving " + focus + "."
	case "share_experience":
		return "The actor recently had one small concrete experience involving " + focus + " and can report what happened."
	case "state_opinion":
		return "The actor has a modest opinion about " + focus + " grounded in ordinary recent experience, not a manifesto or engagement prompt."
	case "share_tip":
		return "The actor found one small practical habit involving " + focus + " that was useful enough to share."
	case "ask_peers":
		return "The actor has one concrete, answerable uncertainty involving " + focus + " and wants contemporaries' experience."
	default:
		return "The actor has one small present-tense reason to mention " + focus + "."
	}
}

// Facets are compositional routing vocabulary, not article templates. They define
// a handful of independent everyday axes per broad domain. Concrete article prose
// remains LLM-rendered, and future production can replace/extend these facets with
// persisted world data without changing the conversation-view contract.
func developmentSituationFacets(anchor string) []developmentSituationFacet {
	switch strings.ToLower(strings.TrimSpace(anchor)) {
	case "local":
		return []developmentSituationFacet{
			{kind: "local_notice_change", focus: "a neighborhood notice or posted instruction", boundary: "Keep it mundane and local; use no real place name and do not imply an emergency or official event without evidence.", occurrences: []string{"A neighborhood notice the actor relies on was replaced with a version showing a different practical day or time.", "Two versions of an ordinary neighborhood notice were visible at once, creating a small practical ambiguity."}},
			{kind: "local_route_condition", focus: "a familiar walking route or access path", boundary: "Describe only what was observable; do not assert construction, an accident, or another cause unless the world already knows it.", occurrences: []string{"A familiar walking route was harder to pass than usual, so the actor adjusted their path.", "People were temporarily flowing around an obstruction on a usual route, but the actor did not know the cause."}},
			{kind: "local_shop_routine", focus: "an ordinary nearby shop or service used in a routine", boundary: "The shop/service is fictional and unnamed; keep the change small and practical rather than newsworthy.", occurrences: []string{"The actor arrived during their usual routine and found a different opening or closing schedule posted.", "A small queue or availability change at an ordinary nearby shop/service altered the actor's routine."}},
			{kind: "local_sound_observation", focus: "a neighborhood sound noticed from home or while walking", boundary: "Only timing, repetition, rough direction, or audibility may be observed; do not invent the source as fact.", occurrences: []string{"A short sound repeated several times during the evening, with gaps between occurrences.", "The actor heard an unfamiliar sound while walking and could not tell exactly where it came from."}},
			{kind: "local_meeting_coordination", focus: "an ordinary local meeting or pickup arrangement", boundary: "Keep the location generic and fictional; the issue should be a small timing or meeting-point coordination problem.", occurrences: []string{"A routine meeting or pickup needed a small time adjustment, and the actor had to make sure everyone had the same understanding.", "A familiar meeting point was usable but slightly ambiguous, so the actor needed to clarify where to wait."}},
			{kind: "local_lost_found_notice", focus: "a mundane lost/found or misplaced-item notice", boundary: "Keep stakes low; no crime, emergency, or identifiable real person/place may be invented.", occurrences: []string{"The actor noticed a small lost/found notice for an everyday item in the neighborhood.", "An everyday item had been left somewhere locally and a simple notice was posted so the owner could recognize it."}},
		}
	case "communications":
		return []developmentSituationFacet{
			{kind: "communications_first_screen_wait", focus: "the wait between a successful connection and the first host screen becoming usable", boundary: "Report only observed timing/variation; do not invent a technical cause, outage, or equipment failure.", occurrences: []string{"The connection succeeded, but the first host screen took noticeably longer than usual to appear on one recent session.", "The first screen appeared quickly on some sessions and after a short pause on others."}},
			{kind: "communications_redial_timing", focus: "reconnecting after a disconnect or a busy result", boundary: "Keep the sequence observable and modest; do not claim that waiting causes success unless repeated evidence exists.", occurrences: []string{"The actor retried soon after a disconnect and noticed a different result after leaving a short pause before another attempt.", "Several reconnect attempts differed in how quickly they reached a usable session, without a known cause."}},
			{kind: "communications_post_confirmation", focus: "the visible confirmation after sending a board post", boundary: "Do not invent data loss; distinguish a delayed display from an actually failed post.", occurrences: []string{"After sending a post, the confirmation display took a short while to appear even though the article was present afterward.", "The actor briefly wondered whether a post had been accepted because the visible response was slower than expected."}},
			{kind: "communications_offline_drafting", focus: "preparing text before connecting", boundary: "This is an ordinary usage habit, not a new product feature or technical breakthrough.", occurrences: []string{"The actor wrote the main points down before connecting and spent less time deciding what to type online.", "Preparing a short draft in advance made a recent posting session feel less rushed."}},
			{kind: "communications_line_coordination", focus: "sharing an ordinary household telephone line between data and voice use", boundary: "Keep it to scheduling/availability; do not invent billing, family conflict, or special telephone services.", occurrences: []string{"The actor delayed a connection because the household line was expected to be needed for an ordinary voice call.", "A planned voice call changed when the actor chose to start or end a BBS session."}},
			{kind: "communications_session_end", focus: "ending a BBS session and checking that the last action completed", boundary: "Keep it to observable terminal/BBS behavior; do not invent host-specific commands unless already canonical.", occurrences: []string{"Before disconnecting, the actor checked once that the last posted item was visible.", "The actor waited for the final visible response before ending a recent session instead of disconnecting immediately."}},
		}
	case "modem":
		return []developmentSituationFacet{
			{kind: "modem_call_progress", focus: "observable call progress from dialing toward an established connection", boundary: "Use only observable ringing/silence/connection progression; do not assert internal network causes, standards, or a specific modem model.", occurrences: []string{"The time between dialing and the remote side beginning its connection sequence varied noticeably between attempts.", "One call had a longer quiet interval during call progress but still eventually reached an established connection."}},
			{kind: "modem_busy_pattern", focus: "encountering a busy result during ordinary dialing", boundary: "Do not infer congestion source or line count from one busy result.", occurrences: []string{"A recent attempt received a busy result, while a later attempt connected normally.", "Busy results appeared intermittently rather than on every attempt in the same general time period."}},
			{kind: "modem_reconnect_variation", focus: "variation between successive connection attempts", boundary: "Describe the observed sequence only; no hardware defect or line fault may be asserted without evidence.", occurrences: []string{"Two successive attempts differed in how long establishment took, despite the actor not intentionally changing settings.", "A later reconnect behaved differently from the immediately previous attempt, but both eventually established a session."}},
			{kind: "modem_connection_sound", focus: "audible changes during connection establishment", boundary: "Use generic audible observations only; do not name a protocol, speed, failure mode, or model from sound alone.", occurrences: []string{"The audible connection sequence seemed shorter on one attempt than another, while both completed.", "A brief pause occurred between audible phases of connection establishment on a recent call."}},
			{kind: "modem_disconnect_observation", focus: "what the actor observes immediately before or after a disconnect", boundary: "Do not claim the cause of a disconnect; keep the observation at the terminal/line level.", occurrences: []string{"A recent session ended cleanly but a subsequent attempt behaved differently during establishment.", "The actor noticed a small difference in the line/terminal response immediately after ending a session."}},
	}
	case "games":
		return []developmentSituationFacet{
			{kind: "games_naming_choice", focus: "choosing an in-game name or label", boundary: "No real game title is needed; make the choice understandable without hidden context.", occurrences: []string{"The actor paused at a naming step because they wanted something memorable without using their real name.", "The actor chose a temporary name quickly and then started second-guessing it once play continued."}},
			{kind: "games_progress_setback", focus: "a recent progress setback followed by another attempt", boundary: "No real title, character, or machine name may be invented; keep the gameplay situation self-contained.", occurrences: []string{"The actor lost progress near the end of a difficult stretch, retried, and then got farther on the next attempt.", "A small mistake ruined a promising attempt, but a retry went unexpectedly smoothly."}},
			{kind: "games_route_choice", focus: "a clearly described in-game route or action choice", boundary: "If asking peers, describe both options in ordinary words so the question is answerable without a title.", occurrences: []string{"The actor reached a point where two clearly different routes/actions were available and was unsure which to try first.", "Two possible next actions both seemed plausible, and the actor wanted to avoid repeating the less useful order."}},
			{kind: "games_save_decision", focus: "deciding when to save before a risky or uncertain section", boundary: "Keep it generic and period-normal; do not invent a product-specific save system.", occurrences: []string{"The actor considered saving before trying an uncertain section because the previous attempt cost noticeable progress.", "A recent retry made the actor more careful about when to save before experimenting."}},
			{kind: "games_score_retry", focus: "trying again to improve a score or result", boundary: "Keep the metric generic unless the post itself explains it; no real title is required.", occurrences: []string{"The actor improved a recent result after one more attempt and was surprised how much difference a small change made.", "Several retries produced nearly the same result until one attempt finally improved it."}},
			{kind: "games_stuck_point", focus: "a specific stuck point that can be described without a title", boundary: "If asking for help, include the visible situation and attempted actions; never ask about an unnamed 'this part' or 'which one'.", occurrences: []string{"The actor was stuck at a self-contained obstacle and had already tried the two most obvious actions.", "A repeated section was not progressing, so the actor wanted to compare what others tried at that exact situation."}},
		}
	case "bbs":
		return []developmentSituationFacet{
			{kind: "bbs_short_posting", focus: "whether short, low-stakes posts feel welcome on the board", boundary: "Keep it about this BBS's social atmosphere; do not turn it into a universal rule or engagement tactic.", occurrences: []string{"A few brief posts made the board feel easier to join without needing a long topic every time.", "The actor noticed that a short contribution could still keep a thread or board feeling active."}},
			{kind: "bbs_subject_clarity", focus: "choosing a subject that makes an ordinary post easier to understand", boundary: "Do not assume a host feature beyond plain article subjects.", occurrences: []string{"The actor rewrote a vague subject before posting because the first version did not say what the article was about.", "A recent article was easier to recognize later because its subject described the point plainly."}},
			{kind: "bbs_reply_timing", focus: "replying after reading rather than immediately", boundary: "Keep it a personal habit; do not invent unread-state features or notification systems.", occurrences: []string{"The actor read a post first, thought about it offline, and replied on a later connection.", "Waiting until the next connection helped the actor send a shorter, clearer reply."}},
			{kind: "bbs_post_confirmation", focus: "checking that a just-sent article appeared as expected", boundary: "Do not invent message loss or duplicate behavior unless directly observed in this situation.", occurrences: []string{"The actor checked the article list once after sending because the visible response had felt slow.", "A quick list check confirmed that a recently sent article was present before the actor disconnected."}},
			{kind: "bbs_reading_routine", focus: "a personal routine for reading several new posts", boundary: "Avoid host-specific unread commands unless already canonical; keep it at the human habit level.", occurrences: []string{"The actor found it easier to read a few active threads first and return to the rest later.", "On a recent visit, the actor skimmed subjects first before deciding which articles to open carefully."}},
			{kind: "bbs_newcomer_tone", focus: "how regulars respond to someone posting for the first time", boundary: "Keep it to ordinary member behavior, not a formal moderation policy unless SYSOP context already establishes one.", occurrences: []string{"A brief friendly reply to a first-time poster made the exchange feel less formal.", "The actor noticed that a simple acknowledgement could make a new participant more comfortable posting again."}},
		}
	case "software":
		return []developmentSituationFacet{
			{kind: "software_text_entry", focus: "an ordinary text-entry habit in software", boundary: "No real product name or undocumented feature may be invented.", occurrences: []string{"A small change in the actor's text-entry routine reduced retyping during a recent task.", "The actor prepared a few lines before opening the task that needed them and found the work easier."}},
			{kind: "software_setting_change", focus: "one reversible software setting change", boundary: "Describe only the user's observed result; do not claim hidden technical causes or product-specific options.", occurrences: []string{"The actor changed one ordinary setting, compared the result, and then decided whether to keep it.", "A small setting adjustment made one routine task feel different enough to notice."}},
			{kind: "software_file_routine", focus: "a personal way of organizing or finding files", boundary: "Keep it generic and local to the actor; no product feature should be assumed.", occurrences: []string{"The actor changed how they named or grouped a few files and found one later with less searching.", "A recent cleanup made an often-used file easier to locate without changing any software."}},
			{kind: "software_startup_routine", focus: "the order in which the actor starts ordinary software tasks", boundary: "Keep it as a user routine, not an operating-system claim.", occurrences: []string{"Starting one preparatory task first made the rest of a recent session feel smoother.", "The actor changed the order of two routine startup steps and noticed the difference in convenience."}},
			{kind: "software_screen_layout", focus: "arranging information on screen for an ordinary task", boundary: "Do not invent a specific GUI feature; describe only a generic layout preference or observation.", occurrences: []string{"The actor rearranged what they kept visible while working and found it easier to compare information.", "A simpler screen arrangement reduced how often the actor had to switch attention during a recent task."}},
		}
	case "music":
		return []developmentSituationFacet{
			{kind: "music_repeat_listen", focus: "listening to the same piece repeatedly", boundary: "No real artist, title, format, or release may be invented.", occurrences: []string{"The actor replayed the same piece several times because one part kept standing out.", "A piece that seemed ordinary at first became more interesting after another listen."}},
			{kind: "music_listening_order", focus: "the order in which the actor listens to several pieces", boundary: "Keep the media/source generic and fictional.", occurrences: []string{"Changing the listening order made the actor notice a different piece than usual.", "The actor put a familiar favorite later than usual and ended up paying more attention to what came before it."}},
			{kind: "music_volume_timing", focus: "when and how loudly the actor listens at home", boundary: "Keep it a mundane household choice, not a technical audio claim.", occurrences: []string{"The actor listened later than usual and kept the volume lower, which changed what details stood out.", "A quieter listening session made the actor pay more attention to a different part of the sound."}},
			{kind: "music_share_recommendation", focus: "sharing a generic listening recommendation with another member", boundary: "No real work/artist may be named while historical references are disabled.", occurrences: []string{"The actor wanted to describe what kind of piece had been enjoyable without relying on a title.", "A recent listen gave the actor a reason to ask what others choose for a similar mood or situation."}},
		}
	case "chat":
		return []developmentSituationFacet{
			{kind: "chat_small_mistake", focus: "a small everyday mistake or near-miss", boundary: "Keep stakes low and avoid invented real institutions, products, or public events.", occurrences: []string{"The actor caught a small mistake just before it became inconvenient and found the timing funny afterward.", "An ordinary routine went slightly wrong, then was fixed without consequence."}},
			{kind: "chat_schedule_shift", focus: "a small change in an ordinary personal schedule", boundary: "No public event or named workplace/school may be invented.", occurrences: []string{"A routine plan moved by a little and changed how the actor used the evening.", "The actor finished one ordinary obligation earlier or later than expected and adjusted the rest of the day."}},
			{kind: "chat_household_routine", focus: "an ordinary household routine", boundary: "Keep it mundane and non-identifying; no family drama is needed.", occurrences: []string{"A small household task took longer than expected but was finally out of the way.", "Changing the order of two ordinary chores made the evening feel unexpectedly easier."}},
			{kind: "chat_hobby_update", focus: "a tiny update from an ordinary hobby", boundary: "Do not introduce a real product/title unless independently allowed by historical evidence.", occurrences: []string{"The actor made a little progress on a hobby after several unremarkable attempts.", "A small change in approach made an ordinary hobby session more satisfying than expected."}},
			{kind: "chat_commute_moment", focus: "a mundane moment while going out or returning home", boundary: "Keep geography generic and fictional; no real route, station, incident, or news event may be asserted.", occurrences: []string{"The usual trip took a little longer because the actor had to adjust their ordinary route.", "A small delay on the way home changed what the actor did first after arriving."}},
		}
	default:
		return []developmentSituationFacet{
			{kind: "generic_observation", focus: "one small everyday observation inside the selected routing domain", boundary: "Keep it self-contained, mundane, and answerable from the post itself; do not invent real-world proper nouns.", occurrences: []string{"The actor noticed one concrete difference from their ordinary expectation during a recent routine.", "A small recent occurrence gave the actor one specific thing worth mentioning."}},
			{kind: "generic_practical_choice", focus: "one ordinary practical choice inside the selected routing domain", boundary: "State the options or result clearly enough that the post does not depend on hidden context.", occurrences: []string{"The actor chose between two ordinary approaches and noticed a practical difference.", "A recent small decision worked out differently from what the actor expected."}},
		}
	}
}
