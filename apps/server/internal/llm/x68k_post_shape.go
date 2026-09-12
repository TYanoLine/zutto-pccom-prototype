package llm

import "strings"

func NormalizeConversationKnowledge(mode, discourse string) string {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "firsthand", "tentative", "hearsay", "opinion", "question":
		return mode
	}
	switch strings.TrimSpace(discourse) {
	case "ask_peers":
		return "question"
	case "state_opinion":
		return "opinion"
	case "share_observation", "share_experience", "share_tip":
		return "firsthand"
	default:
		return "tentative"
	}
}
