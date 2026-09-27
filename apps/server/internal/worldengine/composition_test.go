package worldengine

import "testing"

func TestSelectBodyLengthBandIsStableAndUsesActionDistribution(t *testing.T) {
	got := SelectBodyLengthBand(12345, PostKindReply)
	if again := SelectBodyLengthBand(12345, PostKindReply); got != again {
		t.Fatalf("selection changed for same seed: %#v vs %#v", got, again)
	}
	if got.Min < 1 || got.Max < got.Min || got.Weight < 1 {
		t.Fatalf("invalid reply band: %#v", got)
	}
	if got := SelectBodyLengthBand(0, PostKindNewPost); got != newPostBodyLengthBands[0] {
		t.Fatalf("new-post profile did not select its first band for seed 0: %#v", got)
	}
	if got := SelectBodyLengthBand(0, PostKindReply); got != replyBodyLengthBands[0] {
		t.Fatalf("reply profile did not select its first band for seed 0: %#v", got)
	}
}
