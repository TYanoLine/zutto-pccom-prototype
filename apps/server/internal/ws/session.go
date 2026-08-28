package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"zutto-pccom/apps/server/internal/hostprogram"
	"zutto-pccom/apps/server/internal/telephone"
	"zutto-pccom/apps/server/internal/world"
)

type Handler struct {
	Network  *telephone.Network
	Store    world.Store
	Sessions *SessionManager
}

type clientMessage struct {
	Type      string `json:"type"`
	Phone     string `json:"phone,omitempty"`
	Line      string `json:"line,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type serverMessage struct {
	Type      string      `json:"type"`
	Result    string      `json:"result,omitempty"`
	Baud      int         `json:"baud,omitempty"`
	Line      int         `json:"line,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Host      *world.Host `json:"host,omitempty"`
	Text      string      `json:"text,omitempty"`
}

var fallbackSessions = NewSessionManager(DefaultReconnectGrace)

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := r.Context()

	sessions := h.Sessions
	if sessions == nil {
		sessions = fallbackSessions
	}

	var active *CallSession
	var attachment uint64
	defer func() {
		if active != nil {
			sessions.Detach(active.ID, attachment)
		}
	}()

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
			if active != nil && sessions.IsCurrent(active.ID, attachment) {
				sessions.End(active.ID)
			}
			active = nil
			attachment = 0

			phone := digitsOnly(msg.Phone)
			res := h.Network.Dial(phone, msg.Attempt)
			sm := serverMessage{Type: "dial_result", Result: string(res.Result), Baud: res.Baud, Line: res.Line}
			if res.Result == telephone.Connect {
				runtime := hostprogram.New(res.Host, h.Store)
				session, token, err := sessions.Create(res.Host, res.Baud, res.Line, runtime)
				if err != nil {
					_ = writeJSON(ctx, conn, serverMessage{Type: "error", Text: "could not create call session"})
					return
				}
				active = session
				attachment = token
				sm.Host = &active.Host
				sm.SessionID = active.ID
			}
			if err := writeJSON(ctx, conn, sm); err != nil {
				return
			}
			if active != nil && res.Result == telephone.Connect {
				// Tiny pause so CONNECT appears before host bytes, like a modem handoff.
				time.Sleep(120 * time.Millisecond)
				if err := writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: active.Runtime.Welcome()}); err != nil {
					return
				}
			}

		case "resume":
			id := strings.TrimSpace(msg.SessionID)
			if id == "" {
				_ = writeJSON(ctx, conn, serverMessage{Type: "resume_result", Result: "not_found"})
				continue
			}

			if active != nil && sessions.IsCurrent(active.ID, attachment) && active.ID != id {
				sessions.Detach(active.ID, attachment)
			}
			active = nil
			attachment = 0

			session, token, result := sessions.Resume(id)
			if result != "ok" {
				_ = writeJSON(ctx, conn, serverMessage{Type: "resume_result", Result: result, SessionID: id})
				continue
			}
			active = session
			attachment = token
			if err := writeJSON(ctx, conn, serverMessage{
				Type:      "resume_result",
				Result:    "ok",
				SessionID: active.ID,
				Baud:      active.Baud,
				Line:      active.Line,
				Host:      &active.Host,
			}); err != nil {
				return
			}

		case "line":
			if active == nil || !sessions.IsCurrent(active.ID, attachment) {
				active = nil
				attachment = 0
				_ = writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: "NO CARRIER\r\n"})
				continue
			}
			out, disconnect := active.Runtime.HandleLine(msg.Line)
			if err := writeJSON(ctx, conn, serverMessage{Type: "terminal", Text: out}); err != nil {
				return
			}
			if disconnect {
				sessions.End(active.ID)
				active = nil
				attachment = 0
				_ = writeJSON(ctx, conn, serverMessage{Type: "carrier", Result: "off"})
			}

		case "hangup":
			if active != nil && sessions.IsCurrent(active.ID, attachment) {
				sessions.End(active.ID)
			}
			active = nil
			attachment = 0
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
