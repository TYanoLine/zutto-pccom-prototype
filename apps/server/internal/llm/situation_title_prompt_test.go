package llm

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func rootDraftPromptRequest() BBSRootDraftRequest {
	return BBSRootDraftRequest{
		HostName: "TEST BBS", HostRegion: "福岡県", BoardID: "4", BoardName: "GAME", BoardScope: "ゲームの話",
		WorldDate:      "1996-08-29",
		RecentSubjects: []string{"最近気づいたこと", "庭の話"},
		FormSeed:       "host|board|0",
		Events: []BBSWorldWindowEvent{{
			EventID:        "slot-1",
			AuthorHandle:   "YUKI",
			CreatedAt:      "1996-08-28T23:15:00+09:00",
			Action:         "thread_start",
			PersonaProfile: "30代の会社員。文体は丁寧",
		}},
	}
}

func TestRootDraftPromptCarriesMaterials(t *testing.T) {
	prompt, err := buildRootDraftPrompt(rootDraftPromptRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"persona_profile":"30代の会社員。文体は丁寧"`, `"author_handle":"YUKI"`,
		"最近気づいたこと", "庭の話",
		"36文字以内・1行・Re:なし", "選ぶものでも真似るものでもなく",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestSituationTitlePromptNeverContainsBaselineTitles(t *testing.T) {
	f, err := os.Open("../../../../specs/009-title-voice/baseline/root-titles-2026-10-02_06.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var titles []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var row struct {
			Subject string `json:"subject"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		titles = append(titles, row.Subject)
	}
	if len(titles) != 179 {
		t.Fatalf("baseline has %d titles", len(titles))
	}
	// Every candidate example, not only the ones a given seed selects.
	all := strings.Join(titleFormNotes, "\n")
	for _, title := range titles {
		if strings.Contains(all, title) {
			t.Errorf("form notes contain baseline title %q", title)
		}
	}
	for seed := 0; seed < 50; seed++ {
		req := rootDraftPromptRequest()
		req.RecentSubjects = nil
		req.FormSeed = string(rune('a' + seed))
		prompt, _ := buildRootDraftPrompt(req)
		for _, title := range titles {
			if strings.Contains(prompt, title) {
				t.Fatalf("prompt contains baseline title %q", title)
			}
		}
	}
}

func TestTitleFormNotesSelection(t *testing.T) {
	if n := len(titleFormNotes); n < 8 || n > 12 {
		t.Fatalf("%d form notes, want 8..12", n)
	}
	a := selectTitleFormNotes("seed-a")
	if len(a) != 3 || !reflect.DeepEqual(a, selectTitleFormNotes("seed-a")) {
		t.Fatalf("selection is not 3 deterministic notes: %v", a)
	}
	differs := false
	for i := 0; i < 20 && !differs; i++ {
		differs = !reflect.DeepEqual(a, selectTitleFormNotes("seed-"+string(rune('b'+i))))
	}
	if !differs {
		t.Fatal("selection never varies with seed")
	}
}

func TestSituationTitlePromptAsksForNameSomewhere(t *testing.T) {
	req := rootDraftPromptRequest()
	single, err := buildRootDraftPrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	req.MultiVariant = true
	multi, err := buildRootDraftPrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	want := "題名のどこかに入れてください"
	if !strings.Contains(single, want) || !strings.Contains(multi, want) {
		t.Errorf("prompt must ask for the target name somewhere in the title")
	}
	if strings.Contains(single, "少なくとも2案") || !strings.Contains(multi, "少なくとも2案") {
		t.Errorf("the 2-of-3 rule belongs to the multi-variant output only")
	}
}

func TestSituationTitlePromptAddsNoProhibitions(t *testing.T) {
	req := rootDraftPromptRequest()
	req.RecentFormFacts = []string{"直近20件のうち、対象名から始まるものが13件"}
	prompt, _ := buildRootDraftPrompt(req)
	for _, bad := range []string{"しないこと", "禁止", "してはいけ", "使わないで", "避けて"} {
		if strings.Contains(prompt, bad) {
			t.Errorf("prompt contains prohibition wording %q", bad)
		}
	}
	if !strings.Contains(prompt, "対象名から始まるものが13件") {
		t.Error("recent form fact missing")
	}
}

func TestSituationTitleFactsAbsentWhenEmpty(t *testing.T) {
	prompt, _ := buildRootDraftPrompt(rootDraftPromptRequest())
	if strings.Contains(prompt, "直近の題名について測った事実") {
		t.Fatal("empty facts must add no material")
	}
}
