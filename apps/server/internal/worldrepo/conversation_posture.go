package worldrepo

import "strings"

type conversationPosture struct {
	knowledge string
	attention string
}

func conversationPostureForRoot(discourseMode, summary string) conversationPosture {
	knowledge := "tentative"
	switch strings.TrimSpace(discourseMode) {
	case "share_observation", "share_experience", "share_tip":
		knowledge = "firsthand"
	case "state_opinion":
		knowledge = "opinion"
	case "ask_peers":
		knowledge = "question"
	}
	attention := strings.TrimSpace(summary)
	if attention == "" {
		attention = "immediate topic"
	}
	return conversationPosture{knowledge: knowledge, attention: attention}
}

func appendConversationPostureFacts(facts []string, p conversationPosture) []string {
	if p.knowledge != "" {
		facts = append(facts, "utterance_knowledge="+p.knowledge)
	}
	if p.attention != "" {
		facts = append(facts, "utterance_attention="+p.attention)
	}
	return facts
}
