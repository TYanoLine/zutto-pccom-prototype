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

type replyRepair102Message struct {
	Type   string `json:"type"`
	Result string `json:"result"`
	Text   string `json:"text"`
}

func repair102Write(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil { return err }
	return c.Write(ctx, websocket.MessageText, b)
}
func repair102Read(ctx context.Context, c *websocket.Conn) (replyRepair102Message, error) {
	_, b, err := c.Read(ctx)
	if err != nil { return replyRepair102Message{}, err }
	var m replyRepair102Message
	err = json.Unmarshal(b, &m)
	return m, err
}
func repair102Line(ctx context.Context, c *websocket.Conn, line string) (replyRepair102Message, error) {
	if err := repair102Write(ctx, c, map[string]any{"type":"line", "line":line}); err != nil { return replyRepair102Message{}, err }
	return repair102Read(ctx, c)
}

func TestLiveReplyRepairAfter102(t *testing.T) {
	if os.Getenv("ZUTTO_LIVE_SMOKE") != "1" { t.Skip("live smoke disabled") }
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss://zutto-pccom-prototype.onrender.com/ws", nil)
	if err != nil { t.Fatal(err) }
	defer c.Close(websocket.StatusNormalClosure, "done")

	connected := false
	for attempt := 1; attempt <= 10; attempt++ {
		if err := repair102Write(ctx, c, map[string]any{"type":"dial", "phone":"0450000196", "attempt":attempt}); err != nil { t.Fatal(err) }
		dial, err := repair102Read(ctx, c); if err != nil { t.Fatal(err) }
		if dial.Type == "dial_result" && dial.Result == "connect" {
			connected = true
			if _, err := repair102Read(ctx, c); err != nil { t.Fatal(err) }
			break
		}
	}
	if !connected { t.Fatal("could not connect") }

	boards, err := repair102Line(ctx, c, "B"); if err != nil { t.Fatal(err) }
	if !strings.Contains(boards.Text, "BOARD CATALOG") { t.Fatalf("unexpected board response: %q", boards.Text) }
	index, err := repair102Line(ctx, c, "2"); if err != nil { t.Fatal(err) }
	for i := 0; i < 60 && strings.Contains(index.Text, "GENERATING IN BACKGROUND"); i++ {
		time.Sleep(time.Second)
		index, err = repair102Line(ctx, c, "R"); if err != nil { t.Fatal(err) }
	}
	t.Logf("board2=%q", index.Text)
	if strings.Contains(index.Text, "（本文生成時に決定）") { t.Fatalf("placeholder still visible: %s", index.Text) }
	for _, want := range []string{"Re: ISDNに移行した方、感想を教えてください", "Re: ログ取りに便利な通信ソフト"} {
		if !strings.Contains(index.Text, want) { t.Fatalf("missing repaired subject %q in %s", want, index.Text) }
	}
}
