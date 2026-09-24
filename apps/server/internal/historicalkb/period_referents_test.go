package historicalkb

import (
	"testing"
	"time"
)

func TestPeriodReferentsRespectEveryAvailabilityBoundary(t *testing.T) {
	for _, item := range periodReferents {
		start, err := time.Parse("2006-01-02", item.AvailableFrom)
		if err != nil || item.SourceURL == "" || item.Claim == "" { t.Fatalf("incomplete evidence: %+v", item) }
		for _, tc := range []struct { date string; want bool }{
			{start.AddDate(0, 0, -1).Format("2006-01-02"), false},
			{item.AvailableFrom, true},
		} {
			found := false
			for _, got := range PeriodReferents(tc.date) { if got.Name == item.Name { found = true } }
			if found != tc.want { t.Fatalf("%s at %s: found=%v", item.Name, tc.date, found) }
		}
	}
	for _, date := range []string{"", "1996-02-30", "1996"} {
		if len(PeriodReferents(date)) != 0 { t.Fatalf("invalid date accepted: %q", date) }
	}
}

func TestPeriodReferentsCannotMutateCatalog(t *testing.T) {
	got := PeriodReferents("1996-08-29")
	if len(got) == 0 { t.Fatal("missing period support") }
	got[0].Claim = "corrupted"
	if PeriodReferents("1996-08-29")[0].Claim == "corrupted" { t.Fatal("shared catalog mutated") }
}


func TestPeriodReferentsIncludeBroaderGameVocabularyByAugust1996(t *testing.T) {
	got := map[string]bool{}
	for _, item := range PeriodReferents("1996-08-26") {
		got[item.Name] = true
	}
	for _, name := range []string{
		"ファイナルファンタジーVI",
		"クロノ・トリガー",
		"ドラゴンクエストVI 幻の大地",
		"ときめきメモリアル",
		"幻想水滸伝",
		"スーパーマリオRPG",
		"星のカービィ スーパーデラックス",
		"バイオハザード",
	} {
		if !got[name] {
			t.Fatalf("1996-08-26 missing sourced game referent %q", name)
		}
	}
}
