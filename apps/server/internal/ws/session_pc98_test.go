package ws

import "testing"

func TestPC98ShiftJISText(t *testing.T) {
	got := pc98ShiftJISText("ASCII ｱｲｳ　□■＊※○◎◇◆★☆→― ▫️ ① 😀")
	want := "ASCII ｱｲｳ　□■＊※○◎◇◆★☆→― □ ① ?"
	if got != want {
		t.Fatalf("pc98ShiftJISText() = %q, want %q", got, want)
	}
}
