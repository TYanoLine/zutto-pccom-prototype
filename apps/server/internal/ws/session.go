package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"zutto-pccom/apps/server/internal/bbs"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
)

type Handler struct {
	Network *telephone.Network
	Store   world.Store
}

type clientMessage struct {
	Type    string `json:"type"`
	Phone   string `json:"phone,omitempty"`
	Line    string `json:"line,omitempty"`
	Attempt int    `json:"attempt,omitempty"`
}

type serverMessage struct {
	Type   string      `json:"type"`
	Result string      `json:"result,omitempty"`
	Baud   int         `json:"baud,omitempty"`
	Line   int         `json:"line,omitempty"`
	Host   *world.Host `json:"host,omitempty"`
	Text   string      `json:"text,omitempty"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := r.Context()
	var runtime *bbs.Runtime

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = writeJSON(ctx, conn, serverMessage{Type: "error", Text: "bad request"})
			continue
		}
		switch msg.Type {
		case "dial":
			phone := digitsOnly(msg.Phone)
			res := h.Network.Dial(phone, msg.Attempt)
			sm := serverMessage{Type: "dial_result", Result: string(res.Result), Baud: res.Baud, Line: res.Line}
			if res.Result == telephone.Connect {
				sm.Host = &res.Host
				runtime = bbs.New(res.Host, h.Store)
			}
			if err := writeJSON(ctx, conn, sm); err != nil {
				return
			}
			if runtime != nil && res.Result == telephone.Connect {
				// Tiny pause so CONNECT appears before host bytes, like a modem handoff.
				time.Sleep(120 * time.Millisecond)
				if err := writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: runtime.Welcome()}); err != nil {
					return
				}
			}
		case "line":
			if runtime == nil {
				_ = writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: "NO CARRIER\r\n"})
				continue
			}
			out, disconnect := runtime.HandleLine(msg.Line)
			if err := writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: out}); err != nil {
				return
			}
			if disconnect {
				runtime = nil
				_ = writeJSON(ctx, conn, serverMessage{Type: "carrier", Result: "off"})
			}
		case "hangup":
			runtime = nil
			_ = writeJSON(ctx, conn, serverMessage{Type: "carrier", Result: "off"})
		default:
			_ = writeJSON(ctx, conn, serverMessage{Type: "error", Text: fmt.Sprintf("unknown type %q", msg.Type)})
		}
	}
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, b)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
