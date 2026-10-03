package world

import "testing"

func TestResponseTargetIDPrefersSemanticRelationOverHostTopology(t *testing.T) {
	post := Post{
		ID:       20,
		ParentID: 0,
		Intent: PostIntent{
			DiscourseMode:    "reply",
			SourcePostID:     10,
			RespondsToPostID: 11,
		},
	}
	if got := ResponseTargetID(post); got != 11 {
		t.Fatalf("response target=%d, want RespondsToPostID 11", got)
	}
	if IsSemanticRoot(post) {
		t.Fatal("flat host reply was misclassified as a semantic root")
	}
}

func TestResponseTargetIDFallsBackForLegacyManualChild(t *testing.T) {
	post := Post{ID: 20, ParentID: 10}
	if got := ResponseTargetID(post); got != 10 {
		t.Fatalf("legacy response target=%d, want ParentID 10", got)
	}
}

func TestSemanticRootDoesNotDependOnHostParentField(t *testing.T) {
	post := Post{ID: 20, Intent: PostIntent{DiscourseMode: "thread_start"}}
	if !IsSemanticRoot(post) {
		t.Fatal("standalone post was not recognized as semantic root")
	}
}
