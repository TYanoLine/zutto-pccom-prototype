package worldrepo
import "strings"

type developmentSparseSituation struct {
	kind string
	summary string
	facts []string
}
type developmentSituationFacet struct {
	kind string
	focus string
}

func developmentSituationFacets(anchor string) []developmentSituationFacet {
 switch strings.ToLower(strings.TrimSpace(anchor)) {
 case "local":
 return []developmentSituationFacet{
{kind: "local_notice_change", focus: "a neighborhood notice or posted instruction"},
{kind: "local_route_condition", focus: "a familiar walking route or access path"},
{kind: "local_shop_routine", focus: "an ordinary nearby shop or service used in a routine"},
{kind: "local_sound_observation", focus: "a neighborhood sound noticed from home or while walking"},
{kind: "local_meeting_coordination", focus: "an ordinary local meeting or pickup arrangement"},
{kind: "local_lost_found_notice", focus: "a mundane lost/found or misplaced-item notice"},
 }
 case "communications":
 return []developmentSituationFacet{
{kind: "communications_first_screen_wait", focus: "the wait between a successful connection and the first host screen becoming usable"},
{kind: "communications_redial_timing", focus: "reconnecting after a disconnect or a busy result"},
{kind: "communications_post_confirmation", focus: "the visible confirmation after sending a board post"},
{kind: "communications_offline_drafting", focus: "preparing text before connecting"},
{kind: "communications_line_coordination", focus: "sharing an ordinary household telephone line between data and voice use"},
{kind: "communications_session_end", focus: "ending a BBS session and checking that the last action completed"},
 }
 case "modem":
 return []developmentSituationFacet{
{kind: "modem_call_progress", focus: "observable call progress from dialing toward an established connection"},
{kind: "modem_busy_pattern", focus: "encountering a busy result during ordinary dialing"},
{kind: "modem_reconnect_variation", focus: "variation between successive connection attempts"},
{kind: "modem_connection_sound", focus: "audible changes during connection establishment"},
{kind: "modem_disconnect_observation", focus: "what the actor observes immediately before or after a disconnect"},
 }
 case "games":
 return []developmentSituationFacet{
{kind: "games_naming_choice", focus: "choosing an in-game name or label"},
{kind: "games_progress_setback", focus: "a recent progress setback followed by another attempt"},
{kind: "games_route_choice", focus: "a clearly described in-game route or action choice"},
{kind: "games_save_decision", focus: "deciding when to save before a risky or uncertain section"},
{kind: "games_score_retry", focus: "trying again to improve a score or result"},
{kind: "games_stuck_point", focus: "a specific stuck point that can be described without a title"},
 }
 case "bbs":
 return []developmentSituationFacet{
{kind: "bbs_short_posting", focus: "whether short, low-stakes posts feel welcome on the board"},
{kind: "bbs_subject_clarity", focus: "choosing a subject that makes an ordinary post easier to understand"},
{kind: "bbs_reply_timing", focus: "replying after reading rather than immediately"},
{kind: "bbs_post_confirmation", focus: "checking that a just-sent article appeared as expected"},
{kind: "bbs_reading_routine", focus: "a personal routine for reading several new posts"},
{kind: "bbs_newcomer_tone", focus: "how regulars respond to someone posting for the first time"},
 }
 case "software":
 return []developmentSituationFacet{
{kind: "software_text_entry", focus: "an ordinary text-entry habit in software"},
{kind: "software_setting_change", focus: "one reversible software setting change"},
{kind: "software_file_routine", focus: "a personal way of organizing or finding files"},
{kind: "software_startup_routine", focus: "the order in which the actor starts ordinary software tasks"},
{kind: "software_screen_layout", focus: "arranging information on screen for an ordinary task"},
 }
 case "music":
 return []developmentSituationFacet{
{kind: "music_repeat_listen", focus: "listening to the same piece repeatedly"},
{kind: "music_listening_order", focus: "the order in which the actor listens to several pieces"},
{kind: "music_volume_timing", focus: "when and how loudly the actor listens at home"},
{kind: "music_share_recommendation", focus: "sharing a generic listening recommendation with another member"},
 }
 case "chat":
 return []developmentSituationFacet{
{kind: "chat_small_mistake", focus: "a small everyday mistake or near-miss"},
{kind: "chat_schedule_shift", focus: "a small change in an ordinary personal schedule"},
{kind: "chat_household_routine", focus: "an ordinary household routine"},
{kind: "chat_hobby_update", focus: "a tiny update from an ordinary hobby"},
{kind: "chat_commute_moment", focus: "a mundane moment while going out or returning home"},
 }
 default:
 return []developmentSituationFacet{
{kind: "generic_observation", focus: "one small everyday observation inside the selected routing domain"},
{kind: "generic_practical_choice", focus: "one ordinary practical choice inside the selected routing domain"},
 }
 }
}
