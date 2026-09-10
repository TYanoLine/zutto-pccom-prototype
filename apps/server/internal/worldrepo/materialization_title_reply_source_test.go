package worldrepo

import (
	"strings"
	"testing"

	"zutto-pccom/apps/server/internal/world"
)

func TestTitleFirstReplyNamespacesSourceFacts(t *testing.T) {
	source := world.Post{ID: 42, Intent: world.PostIntent{
		SituationKind:    "title_first",
		SituationSummary: "この人物がロマンシング サ・ガ3を始め、そのことを話題にする。",
		SituationFacts: []string{
			"title_first_subject=ロマンシング サ・ガ3を始めました",
			"world_adopted_summary=この人物がロマンシング サ・ガ3を始め、そのことを話題にする。",
			"article_detail=timing:前日の夜にプレイを始めた",
			"article_detail=sequence:説明書をざっと見てから起動した",
			"article_detail_contract=root contract",
			"subject_contract=root subject contract",
		},
	}}

	got := developmentSituationFromSource(source, false)
	if got.kind != "title_first" {
		t.Fatalf("kind = %q", got.kind)
	}
	if !strings.Contains(got.summary, "Reply to canonical source post 0042") || !strings.Contains(got.summary, "Source event summary") {
		t.Fatalf("reply summary does not identify source ownership: %q", got.summary)
	}
	joined := strings.Join(got.facts, "\n")
	if !strings.Contains(joined, "source_article_detail=timing:前日の夜にプレイを始めた") {
		t.Fatalf("source detail was not namespaced: %s", joined)
	}
	if strings.Contains(joined, "\narticle_detail=") || strings.HasPrefix(joined, "article_detail=") {
		t.Fatalf("root article detail leaked as reply-author fact: %s", joined)
	}
	if strings.Contains(joined, "article_detail_contract=root contract") || strings.Contains(joined, "subject_contract=root subject contract") {
		t.Fatalf("root-only contracts leaked into reply: %s", joined)
	}
	if !strings.Contains(joined, "reply_source_contract=") {
		t.Fatalf("reply source ownership contract missing: %s", joined)
	}
}
