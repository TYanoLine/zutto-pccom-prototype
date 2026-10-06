package llm

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func titlePromptRequest() BBSSituationTitleRequest {
	return BBSSituationTitleRequest{
		HostName: "TEST BBS", HostRegion: "福岡県", BoardID: "4", BoardName: "GAME", BoardScope: "ゲームの話",
		WorldDate:      "1996-08-29",
		RecentSubjects: []string{"最近気づいたこと", "庭の話"},
		FormSeed:       "host|board|0",
		Articles: []BBSSituationTitleSeed{{
			EventID: "slot-1", AuthorHandle: "YUKI", CreatedAt: "1996-08-28T23:15:00+09:00",
			PersonaProfile:   "30代の会社員。文体は丁寧",
			SituationKind:    "open_topic",
			SituationSummary: "サクラ大戦を進めていて迷った",
			SituationFacts:   []string{"post_content=他の会員の進め方を聞きたい"},
		}},
	}
}

func TestSituationTitlePromptCarriesMaterials(t *testing.T) {
	prompt, err := buildSituationTitlePrompt(titlePromptRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"persona_profile":"30代の会社員。文体は丁寧"`, `"handle":"YUKI"`,
		`"summary":"サクラ大戦を進めていて迷った"`, "最近気づいたこと", "庭の話",
		"36文字以内・1行・Re:なし",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Index(prompt, `"author"`) > strings.Index(prompt, `"background"`) {
		t.Error("author must come before background")
	}
	if strings.Contains(prompt, "situation_summary") {
		t.Error("prompt still uses situation_summary")
	}
}

func TestSituationTitlePromptStatesBackgroundRoleOnce(t *testing.T) {
	prompt, _ := buildSituationTitlePrompt(titlePromptRequest())
	if n := strings.Count(prompt, "書き手が何をしたかを知る背景"); n != 1 {
		t.Fatalf("background role stated %d times, want 1", n)
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
		req := titlePromptRequest()
		req.RecentSubjects = nil
		req.FormSeed = string(rune('a' + seed))
		prompt, _ := buildSituationTitlePrompt(req)
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

func TestSituationTitlePromptAddsNoProhibitions(t *testing.T) {
	req := titlePromptRequest()
	req.RecentFormFacts = []string{"直近20件のうち、対象名から始まるものが13件"}
	prompt, _ := buildSituationTitlePrompt(req)
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
	prompt, _ := buildSituationTitlePrompt(titlePromptRequest())
	if strings.Contains(prompt, "直近の題名について測った事実") {
		t.Fatal("empty facts must add no material")
	}
}

func TestDecodeSituationTitlesValidation(t *testing.T) {
	req := titlePromptRequest()
	long := strings.Repeat("あ", 37)
	for _, bad := range []string{long, "Re: 何か", "一行目\n二行目", ""} {
		raw, _ := json.Marshal(map[string]any{"titles": map[string]string{"slot-1": bad}})
		if _, err := decodeSituationTitles(string(raw), req); err == nil {
			t.Errorf("title %q should be rejected", bad)
		}
	}
	if got, err := decodeSituationTitles(`{"titles":{"slot-1":"サクラ大戦で迷子"}}`, req); err != nil || got["slot-1"][0] != "サクラ大戦で迷子" {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestDecodeSituationTitleVariants(t *testing.T) {
	req := titlePromptRequest()
	req.MultiVariant = true
	got, err := decodeSituationTitles(`{"titles":{"slot-1":["案1","Re: 案2","案3","案4","案5"]}}`, req)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first three variants are read; the invalid one is dropped.
	if want := []string{"案1", "案3"}; !reflect.DeepEqual(got["slot-1"], want) {
		t.Fatalf("variants = %v, want %v", got["slot-1"], want)
	}
	if _, err := decodeSituationTitles(`{"titles":{"slot-1":["Re: x"]}}`, req); err == nil {
		t.Fatal("all-invalid variants must fail")
	}
	if got, err := decodeSituationTitles(`{"titles":{"slot-1":["だけ"]}}`, req); err != nil || len(got["slot-1"]) != 1 {
		t.Fatalf("single variant: %v %v", got, err)
	}
}

func TestSituationTitleMultiVariantOffKeepsSingleSchema(t *testing.T) {
	req := titlePromptRequest()
	schema, err := situationTitleSchema(req)
	if err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)["titles"].(map[string]any)["properties"].(map[string]any)
	if props["slot-1"].(map[string]any)["type"] != "string" {
		t.Fatalf("single-title schema changed: %v", props)
	}
	req.MultiVariant = true
	schema, _ = situationTitleSchema(req)
	props = schema["properties"].(map[string]any)["titles"].(map[string]any)["properties"].(map[string]any)
	if props["slot-1"].(map[string]any)["type"] != "array" {
		t.Fatalf("variant schema is not an array: %v", props)
	}
	off, _ := buildSituationTitlePrompt(titlePromptRequest())
	on, _ := buildSituationTitlePrompt(req)
	if off == on || !strings.Contains(on, "最大3案") || strings.Contains(off, "最大3案") {
		t.Fatal("multi-variant wording must appear only when enabled")
	}
}
