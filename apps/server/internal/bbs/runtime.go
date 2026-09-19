package bbs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type Runtime struct {
	Host    world.Host
	Store   world.Store
	state   string
	subject string
}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{Host: host, Store: store, state: "command"}
}

func (r *Runtime) ObservationBoards() []world.Board {
	return []world.Board{{ID: "main", Name: "フリートーク"}}
}

func (r *Runtime) Welcome() string {
	ansiTitle := ""
	ansiReset := ""
	if r.Host.ANSI {
		ansiTitle = "\x1b[1;36m"
		ansiReset = "\x1b[0m"
	}
	return fmt.Sprintf("\x1b[2J\x1b[H%s****************************************\r\n* %-36s *\r\n* Since 1992 / %d lines / %dbps      *\r\n****************************************%s\r\n\r\nあなたは GUEST です。\r\n[H]elp  [B]oard  [W]rite  [U]sers  [G]oodbye\r\n\r\nCommand> ", ansiTitle, trimWidth(r.Host.Name, 36), r.Host.Lines, r.Host.MaxBaud, ansiReset)
}

func (r *Runtime) HandleLine(line string) (output string, disconnect bool) {
	line = strings.TrimSpace(line)
	switch r.state {
	case "write_subject":
		if line == "" {
			r.state = "command"
			return "中止しました。\r\n\r\nCommand> ", false
		}
		r.subject = line
		r.state = "write_body"
		return "本文を1行で入力してください（prototype）。\r\n> ", false
	case "write_body":
		if line == "" {
			r.state = "command"
			return "中止しました。\r\n\r\nCommand> ", false
		}
		r.Store.AddPost(r.Host.ID, world.Post{Author: "USER", Subject: r.subject, Body: line, CreatedAt: time.Now()})
		r.state = "command"
		return "\r\n書き込みました。\r\n※ 住人が反応するとは限りません。\r\n\r\nCommand> ", false
	}

	switch strings.ToUpper(line) {
	case "", "H", "HELP", "?":
		return "\r\nH HELP     この表示\r\nB BOARD    掲示板を読む\r\nW WRITE    書き込む\r\nU USERS    接続者表示\r\nG GOODBYE  切断\r\n\r\nCommand> ", false
	case "B", "BOARD", "R", "READ":
		return r.renderPosts() + "\r\nCommand> ", false
	case "W", "WRITE":
		r.state = "write_subject"
		return "件名を入力してください。\r\n> ", false
	case "U", "USERS":
		return "\r\n現在 3 回線使用中です。\r\n 1: NEKO\r\n 2: TAKA\r\n 3: GUEST\r\n\r\nCommand> ", false
	case "G", "GOODBYE", "BYE", "LOGOUT":
		return "\r\nご利用ありがとうございました。\r\nまたどうぞ。\r\n", true
	default:
		return "? COMMAND ERROR\r\n\r\nCommand> ", false
	}
}

func (r *Runtime) renderPosts() string {
	board := world.Board{ID: "main", Name: "フリートーク"}
	posts := r.Store.ListPosts(r.Host.ID)
	if observer, ok := r.Store.(world.HostObservationStore); ok {
		if observed, err := observer.WaitForBoardHeaders(context.Background(), r.Host, board); err == nil {
			posts = observed
		}
	}
	var b strings.Builder
	b.WriteString("\r\n----- MESSAGE BOARD -----\r\n")
	for _, p := range posts {
		if p.BoardID != "" && p.BoardID != board.ID {
			continue
		}
		if strings.TrimSpace(p.Body) == "" {
			if observer, ok := r.Store.(world.HostObservationStore); ok {
				if rendered, found, err := observer.WaitForArticleBody(context.Background(), r.Host, board, p.ID); err == nil && found {
					p = rendered
				}
			}
		}
		fmt.Fprintf(&b, "%04d %-8s %s\r\n     %s\r\n", p.ID, p.Author, p.Subject, strings.ReplaceAll(p.Body, "\r\n", "\r\n     "))
	}
	b.WriteString("-------------------------\r\n")
	return b.String()
}

func trimWidth(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
