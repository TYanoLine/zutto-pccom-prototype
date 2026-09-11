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

type repairSmokeMessage struct {
	Type   string `json:"type"`
	Result string `json:"result"`
	Text   string `json:"text"`
}

func repairSmokeWrite(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}

func repairSmokeRead(ctx context.Context, c *websocket.Conn) (repairSmokeMessage, error) {
	_, b, err := c.Read(ctx)
	if err != nil {
		return repairSmokeMessage{}, err
	}
	var m repairSmokeMessage
	err = json.Unmarshal(b, &m)
	return m, err
}

func repairSmokeLine(ctx context.Context, c *websocket.Conn, line string) (repairSmokeMessage, error) {
	if err := repairSmokeWrite(ctx, c, map[string]any{"type": "line", "line": line}); err != nil {
		return repairSmokeMessage{}, err
	}
	return repairSmokeRead(ctx, c)
}

func TestLiveReplySubjectRepairAfter101(t *testing.T) {
	if os.Getenv("ZUTTO_LIVE_SMOKE") != "1" {
		t.Skip("live smoke disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss://zutto-pccom-prototype.onrender.com/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "done")

	connected := false
	for attempt := 1; attempt <= 10; attempt++ {
		if err := repairSmokeWrite(ctx, c, map[string]any{"type": "dial", "phone": "0450000196", "attempt": attempt}); err != nil {
			t.Fatal(err)
		}
		dial, err := repairSmokeRead(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		if dial.Type == "dial_result" && dial.Result == "connect" {
			connected = true
			if _, err := repairSmokeRead(ctx, c); err != nil { // welcome
				t.Fatal(err)
			}
			break
		}
	}
	if !connected {
		t.Fatal("could not connect")
	}

	boards, err := repairSmokeLine(ctx, c, "B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(boards.Text, "BOARD CATALOG") {
		t.Fatalf("unexpected board response: %q", boards.Text)
	}
	index, err := repairSmokeLine(ctx, c, "2")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60 && strings.Contains(index.Text, "GENERATING IN BACKGROUND"); i++ {
		time.Sleep(time.Second)
		index, err = repairSmokeLine(ctx, c, "R")
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("board2=%q", index.Text)
	if strings.Contains(index.Text, "（本文生成時に決定）") {
		t.Fatalf("persisted placeholder reply still visible after repair: %s", index.Text)
	}
	if !strings.Contains(index.Text, "MSG No.") {
		t.Fatalf("article index did not become ready: %s", index.Text)
	}
}
