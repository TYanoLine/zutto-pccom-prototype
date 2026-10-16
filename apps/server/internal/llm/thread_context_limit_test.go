package llm

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatThreadContextKeepsRootAndRecentRepliesOnly(t *testing.T) {
	thread := []ThreadPostContext{{Author: "ROOT", Subject: "件名", Body: "親記事本文", IsRoot: true}}
	for i := 1; i <= 8; i++ {
		thread = append(thread, ThreadPostContext{Author: fmt.Sprintf("R%d", i), Body: fmt.Sprintf("返信%d", i)})
	}
	got := formatThreadContext(thread)
	for _, want := range []string{"発言者: ROOT (親記事/スレッド起点)", "返信6", "返信7", "返信8", "間の返信 5 件は省略"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"返信1\n", "返信5\n"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("older reply %q should be omitted:\n%s", unwanted, got)
		}
	}
}

func TestFormatThreadContextShortThreadUnchanged(t *testing.T) {
	got := formatThreadContext([]ThreadPostContext{
		{Author: "ROOT", Body: "本文", IsRoot: true},
		{Author: "A", Body: "返信A"},
	})
	if strings.Contains(got, "省略") {
		t.Fatalf("short thread must not be elided:\n%s", got)
	}
}

func TestBodyPromptsDoNotPrescribeSharedSpeechSample(t *testing.T) {
	for _, freeform := range []bool{true, false} {
		prompt := BuildBoardPostPrompt(BoardPostRequest{FreeformFromSubject: freeform, BoardTopic: "雑談", WorldDate: "1996-04-19", AuthorHandle: "X"})
		if strings.Contains(prompt, "〜ですねー") {
			t.Fatalf("freeform=%v: prompt still prescribes a shared sample ending:\n%s", freeform, prompt)
		}
	}
}
