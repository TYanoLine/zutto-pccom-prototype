package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type expandedBBSMessage struct {
	Type   string `json:"type"`
	Result string `json:"result"`
	Text   string `json:"text"`
}

func expandedBBSWrite(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}

func expandedBBSRead(ctx context.Context, c *websocket.Conn) (expandedBBSMessage, error) {
	_, b, err := c.Read(ctx)
	if err != nil {
		return expandedBBSMessage{}, err
	}
	var m expandedBBSMessage
	err = json.Unmarshal(b, &m)
	return m, err
}

func expandedBBSLine(ctx context.Context, c *websocket.Conn, line string) (expandedBBSMessage, error) {
	if err := expandedBBSWrite(ctx, c, map[string]any{"type": "line", "line": line}); err != nil {
		return expandedBBSMessage{}, err
	}
	return expandedBBSRead(ctx, c)
}

var expandedEnvelopeCountRE = regexp.MustCompile(`(?i)(?:/|envelopes=)\s*([0-9]+)\s*ENVELOPES?`)

func envelopeCountFromIndex(text string) (int, bool) {
	m := expandedEnvelopeCountRE.FindStringSubmatch(text)
	if len(m) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

func TestLiveExpandedInteractiveBBSSample(t *testing.T) {
	if os.Getenv("ZUTTO_LIVE_SMOKE") != "1" {
		t.Skip("live smoke disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss://zutto-pccom-prototype.onrender.com/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "done")

	connected := false
	for attempt := 1; attempt <= 12; attempt++ {
		if err := expandedBBSWrite(ctx, c, map[string]any{"type": "dial", "phone": "0450000196", "attempt": attempt}); err != nil {
			t.Fatal(err)
		}
		dial, err := expandedBBSRead(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		if dial.Type == "dial_result" && dial.Result == "connect" {
			connected = true
			if _, err := expandedBBSRead(ctx, c); err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !connected {
		t.Fatal("could not connect to 0450000196")
	}

	reset, err := expandedBBSLine(ctx, c, "RESET")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reset.Text, "CONVERSATION RESET") {
		t.Fatalf("reset did not complete: %q", reset.Text)
	}
	t.Logf("RESET: %q", reset.Text)

	boards, err := expandedBBSLine(ctx, c, "B")
	if err != nil {
		t.Fatal(err)
	}
	boardNames := []string{"フリートーク", "パソコン通信・モデム", "地域の話題", "ゲーム", "音楽", "ソフトウェア"}
	for _, name := range boardNames {
		if !strings.Contains(boards.Text, name) {
			t.Fatalf("board catalog missing %q: %s", name, boards.Text)
		}
	}
	t.Logf("BOARD CATALOG: %q", boards.Text)

	total := 0
	for i, name := range boardNames {
		index, err := expandedBBSLine(ctx, c, strconv.Itoa(i+1))
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		for polls := 0; strings.Contains(index.Text, "GENERATING IN BACKGROUND"); polls++ {
			if polls >= 120 {
				t.Fatalf("board %d %s index generation did not complete within polling budget", i+1, name)
			}
			time.Sleep(time.Second)
			index, err = expandedBBSLine(ctx, c, "R")
			if err != nil {
				t.Fatal(err)
			}
		}
		count, ok := envelopeCountFromIndex(index.Text)
		if !ok {
			// A completed empty board still renders the board header/list; count it as 0.
			if strings.Contains(index.Text, "MSG No.を選択") {
				count = 0
			} else {
				t.Fatalf("board %d %s did not render an article index: %q", i+1, name, index.Text)
			}
		}
		total += count
		t.Logf("BOARD_RESULT id=%d name=%s envelopes=%d elapsed=%s\n%s", i+1, name, count, time.Since(started).Round(100*time.Millisecond), index.Text)

		if i != len(boardNames)-1 {
			catalog, err := expandedBBSLine(ctx, c, "Q")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(catalog.Text, "BOARD CATALOG") {
				t.Fatalf("board %d Q did not return to catalog: %q", i+1, catalog.Text)
			}
		}
	}

	t.Logf("EXPANDED_SAMPLE_TOTAL envelopes=%d across=%d boards", total, len(boardNames))
	if total == 0 {
		t.Fatal("expanded sample produced no envelopes")
	}
	fmt.Printf("EXPANDED_SAMPLE_TOTAL=%d\n", total)
}
