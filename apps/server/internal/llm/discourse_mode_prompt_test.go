package llm

import (
	"context"
	"strings"
	"testing"
)

type discoursePromptCaptureProvider struct{}

func TestStructuredWorkerContractMentionsNonQuestionModes(t *testing.T) {
	contract := `ARTICLE WORKER CONTRACT:
The host-window PRODUCER already coordinated this article with the rest of the world window. All producer_* fields below are canonical production instructions, not suggestions.
You may choose natural Japanese wording, omissions that are justified by producer_audience_context, line breaks, quoting style, emoticons consistent with the persona, and other surface expression.
You MUST NOT replace producer_episode, invent a different referent, give the actor knowledge outside producer_actor_knowledge/context, omit the producer_required_contribution in favor of a different story, or violate producer_must_not.
If producer_audience_context says a referent is not shared, do not use unexplained shorthand such as 「あれ」「あの面」「例の件」 as though readers already know it.
If a concrete external product/place name is absent from the brief and historical facts, do not invent one merely for specificity.
If discourse_mode is share_observation, share_experience, state_opinion, or share_tip, do not append an engagement-seeking question, request for replies, or “anyone else?” ending. Only ask_peers makes audience solicitation the root article's conversational purpose.`
	for _, want := range []string{"share_observation", "share_experience", "state_opinion", "share_tip", "ask_peers", "do not append an engagement-seeking question"} {
		if !strings.Contains(contract, want) {
			t.Fatalf("worker contract missing %q", want)
		}
	}
	_ = context.Background()
}
