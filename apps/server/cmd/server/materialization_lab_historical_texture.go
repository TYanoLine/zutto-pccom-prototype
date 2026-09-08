package main

// freshHistoricalTextureFacts is deliberately small and fresh-Lab-only.
//
// These strings are permissions/background constraints, not topic templates.
// They intentionally avoid exact release dates, prices, specifications, plot
// details, rankings, or claims about what every member owns/uses. The initial
// set is derived from the repository's existing 1996 in-world guidance; see
// docs/research/HISTORICAL_TEXTURE_POC.md before expanding it.
func freshHistoricalTextureFacts(mode string) []string {
	if mode != "1996-08-curated" {
		return nil
	}
	return []string{
		"By this world window, Windows 95 is an available contemporary OS/topic in Japan. The name may be used when a concrete software or environment distinction matters; do not invent edition, price, release-day, ownership, or upgrade facts.",
		"PC-98 and DOS/V are contemporary Japanese personal-computer environment terms. Use a concrete term only when the selected situation actually depends on that environment distinction; ordinary ownership/use is not itself news.",
		"Macintosh is a contemporary computer-platform name. It may be mentioned when a supplied situation genuinely involves comparison, compatibility, files, software, or another concrete distinction; do not invent a model.",
		"NIFTY-Serve and PC-VAN are contemporary Japanese commercial personal-computer communication service names. They are external services, not this fictional host. Do not invent fees, forum names, access numbers, membership facts, or service changes unless separately supplied.",
		"ISDN is contemporary communications vocabulary in this world window. It may be named when a concrete communication comparison or question genuinely needs the term; do not invent a carrier, plan, line installation, speed, or technical specification.",
		"PlayStation and Sega Saturn are contemporary game-platform names. They may appear naturally in game talk when the platform distinction matters; do not invent a specific game's platform, price, hardware specification, purchase, or popularity claim.",
		"Pocket Monsters / ポケモン is a contemporary game/work name by this world window. The name may be used as an ordinary current referent when relevant; do not invent version-specific, character, sales, release-date, or platform facts beyond what is supplied here.",
		"新世紀エヴァンゲリオン / Evangelion is a contemporary work name by this world window. It may be mentioned as an ordinary current referent when relevant; do not invent episode, broadcast-status, plot, merchandise, or popularity details.",
		"MIDI and FM sound are contemporary music/computer-audio terms. Use them only when the concrete situation already concerns playback, composition, sound, files, or equipment distinctions; do not invent device models or specifications.",
		"秋葉原 / Akihabara is a real contemporary Tokyo place name associated with electronics/computer shopping. Mention it only when an actor's concrete situation plausibly involves going there or comparing shops; do not silently place every member in Tokyo or invent a named store.",
	}
}
