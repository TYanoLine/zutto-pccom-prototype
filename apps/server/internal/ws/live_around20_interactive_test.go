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

type around20Message struct {
	Type   string `json:"type"`
	Result string `json:"result"`
	Text   string `json:"text"`
}

func around20Write(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}

func around20Read(ctx context.Context, c *websocket.Conn) (around20Message, error) {
	_, b, err := c.Read(ctx)
	if err != nil {
		return around20Message{}, err
	}
	var m around20Message
	err = json.Unmarshal(b, &m)
	return m, err
}

func around20Line(ctx context.Context, c *websocket.Conn, line string) (around20Message, error) {
	if err := around20Write(ctx, c, map[string]any{"type": "line", "line": line}); err != nil {
		return around20Message{}, err
	}
	return around20Read(ctx, c)
}

var around20EnvelopeRE = regexp.MustCompile(`/ ([0-9]+) ENVELOPES`)

func around20Count(text string) (int, bool) {
	m := around20EnvelopeRE.FindStringSubmatch(text)
	if len(m) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

func TestLiveAround20InteractiveBoards(t *testing.T) {
	if os.Getenv("ZUTTO_LIVE_SMOKE") != "1" {
		t.Skip("live smoke disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 22*time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss://zutto-pccom-prototype.onrender.com/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "done")

	connected := false
	for attempt := 1; attempt <= 10; attempt++ {
		if err := around20Write(ctx, c, map[string]any{"type": "dial", "phone": "0450000196", "attempt": attempt}); err != nil {
			t.Fatal(err)
		}
		dial, err := around20Read(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		if dial.Type == "dial_result" && dial.Result == "connect" {
			connected = true
			if _, err := around20Read(ctx, c); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !connected {
		t.Fatal("could not connect")
	}

	reset, err := around20Line(ctx, c, "RESET")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RESET: %q", reset.Text)

	boards, err := around20Line(ctx, c, "B")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"フリートーク", "パソコン通信・モデム", "地域の話題", "ゲーム", "音楽", "ソフトウェア"} {
		if !strings.Contains(boards.Text, name) {
			t.Fatalf("missing board %q in %q", name, boards.Text)
		}
	}

	total := 0
	counts := make([]int, 0, 6)
	for boardNo := 1; boardNo <= 6; boardNo++ {
		start := time.Now()
		msg, err := around20Line(ctx, c, strconv.Itoa(boardNo))
		if err != nil {
			t.Fatal(err)
		}
		for poll := 0; poll < 180 && strings.Contains(msg.Text, "GENERATING IN BACKGROUND"); poll++ {
			time.Sleep(2 * time.Second)
			msg, err = around20Line(ctx, c, "R")
			if err != nil {
				t.Fatal(err)
			}
		}
		if strings.Contains(msg.Text, "ARTICLE INDEX : READY") {
			msg, err = around20Line(ctx, c, "R")
			if err != nil {
				t.Fatal(err)
			}
		}
		count, ok := around20Count(msg.Text)
		if !ok {
			t.Fatalf("board %d did not expose envelope count: %q", boardNo, msg.Text)
		}
		counts = append(counts, count)
		total += count
		t.Logf("BOARD_RESULT id=%d envelopes=%d elapsed=%s\n%s", boardNo, count, time.Since(start).Round(100*time.Millisecond), msg.Text)
		if count == 0 {
			t.Fatalf("board %d is empty", boardNo)
		}
		back, err := around20Line(ctx, c, "Q")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(back.Text, "BOARD CATALOG") {
			t.Fatalf("board %d did not return to catalog: %q", boardNo, back.Text)
		}
	}
	fmt.Printf("AROUND20_COUNTS=%v TOTAL=%d\n", counts, total)
}
