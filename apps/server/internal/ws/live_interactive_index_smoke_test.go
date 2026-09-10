package ws_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type liveServerMessage struct {
	Type      string `json:"type"`
	Result    string `json:"result"`
	Text      string `json:"text"`
	SessionID string `json:"session_id"`
}

func liveSend(ctx context.Context, c *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, data)
}

func liveRead(ctx context.Context, c *websocket.Conn) (liveServerMessage, error) {
	_, data, err := c.Read(ctx)
	if err != nil {
		return liveServerMessage{}, err
	}
	var message liveServerMessage
	err = json.Unmarshal(data, &message)
	return message, err
}

func liveLine(ctx context.Context, c *websocket.Conn, line string) (liveServerMessage, time.Duration, error) {
	started := time.Now()
	if err := liveSend(ctx, c, map[string]any{"type": "line", "line": line}); err != nil {
		return liveServerMessage{}, 0, err
	}
	message, err := liveRead(ctx, c)
	return message, time.Since(started), err
}

func TestLiveInteractiveArticleIndexSmoke(t *testing.T) {
	if os.Getenv("ZUTTO_LIVE_SMOKE") != "1" {
		t.Skip("live smoke disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	c, _, err := websocket.Dial(ctx, "wss://zutto-pccom-prototype.onrender.com/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "done")

	connected := false
	for attempt := 1; attempt <= 10; attempt++ {
		if err := liveSend(ctx, c, map[string]any{"type": "dial", "phone": "0450000196", "attempt": attempt}); err != nil {
			t.Fatal(err)
		}
		dial, err := liveRead(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("dial attempt=%d result=%s", attempt, dial.Result)
		if dial.Type == "dial_result" && dial.Result == "connect" {
			connected = true
			welcome, err := liveRead(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			if welcome.Type != "terminal" {
				t.Fatalf("missing welcome: %+v", welcome)
			}
			break
		}
	}
	if !connected {
		t.Fatal("could not connect after 10 dial attempts")
	}

	reset, resetDuration, err := liveLine(ctx, c, "RESET")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RESET duration=%s response=%q", resetDuration, reset.Text)
	if !strings.Contains(reset.Text, "CONVERSATION RESET") {
		t.Fatalf("RESET failed: %s", reset.Text)
	}

	boards, bDuration, err := liveLine(ctx, c, "B")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("B duration=%s response=%q", bDuration, boards.Text)
	if bDuration > 2*time.Second || !strings.Contains(boards.Text, "BOARD CATALOG") {
		t.Fatalf("B was not immediate: duration=%s response=%s", bDuration, boards.Text)
	}

	initial, initialDuration, err := liveLine(ctx, c, "1")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("board 1 initial duration=%s response=%q", initialDuration, initial.Text)
	if initialDuration > 2*time.Second || !strings.Contains(initial.Text, "GENERATING IN BACKGROUND") {
		t.Fatalf("board selection blocked: duration=%s response=%s", initialDuration, initial.Text)
	}

	readyStarted := time.Now()
	for poll := 1; poll <= 90; poll++ {
		time.Sleep(2 * time.Second)
		response, duration, err := liveLine(ctx, c, "R")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("poll=%d response_duration=%s", poll, duration)
		if duration > 2*time.Second {
			t.Fatalf("refresh blocked while background generation ran: %s", duration)
		}
		if strings.Contains(response.Text, "GENERATING IN BACKGROUND") {
			continue
		}
		t.Logf("index ready after %s: %q", time.Since(readyStarted), response.Text)
		if !strings.Contains(response.Text, "MSG No.") {
			t.Fatalf("unexpected completed index response: %s", response.Text)
		}
		return
	}
	t.Fatal("article index did not become ready within 3 minutes")
}
