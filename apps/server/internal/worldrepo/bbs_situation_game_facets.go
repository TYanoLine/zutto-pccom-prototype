package worldrepo

// These directions are genuine World inputs for the production GAME board.
// Diagnostic scenario text and artificial no-title restrictions were removed.
type developmentModeSituationFacet struct {
	developmentSituationFacet
	modes map[string]bool
}
func developmentModeSet(values ...string) map[string]bool {
	out:=make(map[string]bool,len(values))
	for _,value:=range values { out[value]=true }
	return out
}
func developmentRichGameSituationFacets() []developmentModeSituationFacet {

	obsExp := developmentModeSet("share_observation", "share_experience")
	obsExpOpinion := developmentModeSet("share_observation", "share_experience", "state_opinion")
	expTip := developmentModeSet("share_experience", "share_tip")
	expAsk := developmentModeSet("share_experience", "ask_peers")
	allButAsk := developmentModeSet("share_observation", "share_experience", "state_opinion", "share_tip")
	opinionAsk := developmentModeSet("state_opinion", "ask_peers")
	tipAsk := developmentModeSet("share_tip", "ask_peers")
	all := developmentModeSet("share_observation", "share_experience", "state_opinion", "share_tip", "ask_peers")

	
return []developmentModeSituationFacet{
{developmentSituationFacet: developmentSituationFacet{kind:"games_progress_setback", focus:"a recent setback followed by another attempt"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_stuck_point", focus:"a specific obstacle with visible conditions and already-tried actions"}, modes:expAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_route_choice", focus:"two clearly described next routes or actions"}, modes:opinionAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_score_retry", focus:"a repeated attempt to improve a score, time, or other visible result"}, modes:allButAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_save_decision", focus:"where to preserve progress before another risky attempt"}, modes:opinionAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_naming_choice", focus:"choosing an in-game name or label"}, modes:all},
{developmentSituationFacet: developmentSituationFacet{kind:"games_manual_lookup", focus:"checking the packaged instructions after forgetting or overlooking one ordinary control or rule"}, modes:allButAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_note_taking", focus:"writing down a small piece of play information for later"}, modes:expTip},
{developmentSituationFacet: developmentSituationFacet{kind:"games_password_recording", focus:"copying or checking a game-provided password or code used to resume"}, modes:allButAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_save_slot_housekeeping", focus:"making room among a small number of stored game records"}, modes:opinionAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_controller_handoff", focus:"passing control to another person during local play"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_multiplayer_rule", focus:"a simple local rule for taking turns or deciding when players switch"}, modes:opinionAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_local_play_setup", focus:"getting an ordinary local multiplayer session started"}, modes:obsExp},
{developmentSituationFacet: developmentSituationFacet{kind:"games_lending_return", focus:"lending or returning one game to someone the actor knows"}, modes:allButAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_storage_organization", focus:"organizing a small personal collection of games or their accompanying materials"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_shared_tv_time", focus:"fitting play around ordinary shared use of the household television or display"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_sound_volume", focus:"adjusting game sound for the time of day or other people nearby"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_choose_what_to_play", focus:"choosing one game from several already on hand for a short session"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_restart_or_continue", focus:"deciding whether to continue an existing run or start over"}, modes:opinionAsk},
{developmentSituationFacet: developmentSituationFacet{kind:"games_practice_focus", focus:"practicing one repeatable action instead of trying to clear the whole section"}, modes:expTip},
{developmentSituationFacet: developmentSituationFacet{kind:"games_watching_another_player", focus:"noticing a different approach while watching someone else play briefly"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_session_stop_point", focus:"choosing a sensible point to end a play session"}, modes:obsExpOpinion},
{developmentSituationFacet: developmentSituationFacet{kind:"games_simple_comparison", focus:"comparing two ordinary ways of approaching the same in-game task"}, modes:tipAsk},
}
}
func developmentModeFacetAllowed(f developmentModeSituationFacet, mode string) bool {
	if len(f.modes)==0 { return true }
	return f.modes[mode]
}
