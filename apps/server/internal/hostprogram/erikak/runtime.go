package erikak

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type board struct {
	ID     string
	Name   string
	Hidden bool
}

var boards = []board{
	{ID: "1", Name: "雑談・ローカル"},
	{ID: "2", Name: "PC-98 / MODEM"},
	{ID: "3", Name: "SOFTWARE / DATA"},
	// A station-specific hidden board. Erika installations were often customized,
	// and numeric destinations did not have to be listed on the visible menu.
	{ID: "9", Name: "夜更かし部屋", Hidden: true},
}

type Runtime struct {
	Host     world.Host
	Store    world.Store
	state    string
	handle   string
	boardID  string
	threadID int64
	subject  string
}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{Host: host, Store: store, state: "login", handle: "GUEST"}
}

func (r *Runtime) Welcome() string {
	return fmt.Sprintf("\x1b[2J\x1b[H========================================\r\n  %-36s\r\n========================================\r\n  絵理香K版 / %d回線 / MAX %dbps\r\n  %s\r\n----------------------------------------\r\n\r\nHANDLE NAME (RETURN=GUEST) > ", trimRunes(r.Host.Name, 36), r.Host.Lines, r.Host.MaxBaud, r.Host.Region)
}

func (r *Runtime) HandleLine(line string) (output string, disconnect bool) {
	line = strings.TrimSpace(line)

	switch r.state {
	case "login":
		if line != "" {
			r.handle = trimRunes(line, 16)
		}
		r.state = "main"
		return fmt.Sprintf("\r\n%s さん、いらっしゃいませ。\r\n", r.handle) + r.renderMainMenu(), false

	case "new_subject":
		if line == "" {
			r.state = "board"
			return "\r\n中止しました。\r\n" + r.renderBoard(), false
		}
		r.subject = trimRunes(line, 48)
		r.state = "new_body"
		return "本文を入力してください (1行 / RETURNのみで中止)\r\n> ", false

	case "new_body":
		if line == "" {
			r.state = "board"
			return "\r\n中止しました。\r\n" + r.renderBoard(), false
		}
		p := r.Store.AddPost(r.Host.ID, world.Post{
			BoardID:   r.boardID,
			Author:    r.handle,
			Subject:   r.subject,
			Body:      line,
			CreatedAt: time.Now(),
		})
		r.state = "thread"
		r.threadID = p.ID
		return fmt.Sprintf("\r\n記事 %d を登録しました。\r\n", p.ID) + r.renderThread(p.ID), false

	case "append_body":
		if line == "" {
			r.state = "thread"
			return "\r\nアペを中止しました。\r\n" + r.renderThread(r.threadID), false
		}
		root, ok := r.rootPost(r.threadID)
		if !ok {
			r.state = "board"
			return "\r\n記事が見つかりません。\r\n" + r.renderBoard(), false
		}
		r.Store.AddPost(r.Host.ID, world.Post{
			BoardID:   root.BoardID,
			ParentID:  root.ID,
			Author:    r.handle,
			Subject:   "Re: " + root.Subject,
			Body:      line,
			CreatedAt: time.Now(),
		})
		r.state = "thread"
		return "\r\nアペを追加しました。\r\n" + r.renderThread(r.threadID), false
	}

	switch r.state {
	case "main":
		return r.handleMain(line)
	case "board":
		return r.handleBoard(line)
	case "thread":
		return r.handleThread(line)
	default:
		r.state = "main"
		return r.renderMainMenu(), false
	}
}

func (r *Runtime) handleMain(line string) (string, bool) {
	upper := strings.ToUpper(line)
	if b, ok := findBoard(line); ok {
		r.boardID = b.ID
		r.state = "board"
		return r.renderBoard(), false
	}

	switch upper {
	case "", "H", "HELP", "?":
		return r.renderMainMenu(), false
	case "F", "FILE":
		return "\r\n---- FILE TRANSFER ----\r\nNMODEM : AVAILABLE\r\n(転送エンジンはprototypeでは未実装です)\r\n-----------------------\r\n\r\nSELECT > ", false
	case "U", "USER", "USERS":
		return fmt.Sprintf("\r\n---- ONLINE USERS ----\r\n01 SYSOP\r\n02 MARI\r\n03 %s\r\n----------------------\r\n\r\nSELECT > ", r.handle), false
	case "Q", "QUIT", "BYE", "G", "GOODBYE":
		return "\r\nご利用ありがとうございました。\r\nまたのお越しをお待ちしています。\r\n", true
	default:
		return "? INPUT ERROR\r\n\r\nSELECT > ", false
	}
}

func (r *Runtime) handleBoard(line string) (string, bool) {
	upper := strings.ToUpper(line)
	switch upper {
	case "", "L", "LIST":
		return r.renderBoard(), false
	case "N", "NEW", "W", "WRITE":
		r.state = "new_subject"
		return "新規記事の題名を入力してください。\r\n> ", false
	case "M", "MAIN":
		r.state = "main"
		return r.renderMainMenu(), false
	case "Q", "QUIT":
		return "\r\nご利用ありがとうございました。\r\n", true
	}

	id, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return "記事番号 / N(New) / M(Main) > ", false
	}
	root, ok := r.rootPost(id)
	if !ok || root.BoardID != r.boardID {
		return "その記事はありません。\r\n記事番号 / N(New) / M(Main) > ", false
	}
	r.threadID = id
	r.state = "thread"
	return r.renderThread(id), false
}

func (r *Runtime) handleThread(line string) (string, bool) {
	switch strings.ToUpper(line) {
	case "", "R", "READ":
		return r.renderThread(r.threadID), false
	case "A", "APE", "APPEND":
		r.state = "append_body"
		return "アペを入力してください (1行 / RETURNのみで中止)\r\n> ", false
	case "B", "BACK":
		r.state = "board"
		return r.renderBoard(), false
	case "M", "MAIN":
		r.state = "main"
		return r.renderMainMenu(), false
	case "Q", "QUIT":
		return "\r\nご利用ありがとうございました。\r\n", true
	default:
		return "A(アペ) / B(戻る) / M(Main) > ", false
	}
}

func (r *Runtime) renderMainMenu() string {
	var b strings.Builder
	b.WriteString("\r\n+--------------------------------------+\r\n")
	b.WriteString("|              MAIN MENU               |\r\n")
	b.WriteString("+--------------------------------------+\r\n")
	for _, item := range boards {
		if item.Hidden {
			continue
		}
		fmt.Fprintf(&b, " %s. %s\r\n", item.ID, item.Name)
	}
	b.WriteString("\r\n F. FILE TRANSFER   U. USERS   Q. LOG OFF\r\n")
	b.WriteString("----------------------------------------\r\n")
	b.WriteString("SELECT > ")
	return b.String()
}

func (r *Runtime) renderBoard() string {
	item, ok := findBoard(r.boardID)
	if !ok {
		r.state = "main"
		return r.renderMainMenu()
	}

	posts := r.Store.ListPosts(r.Host.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[%s] %s\r\n", item.ID, item.Name)
	b.WriteString("----------------------------------------\r\n")
	b.WriteString(" No.   HANDLE       SUBJECT                 APE\r\n")
	b.WriteString("----------------------------------------\r\n")
	found := false
	for _, p := range posts {
		if p.BoardID != item.ID || p.ParentID != 0 {
			continue
		}
		found = true
		fmt.Fprintf(&b, " %4d  %-12s %-23s %3d\r\n", p.ID, trimRunes(p.Author, 12), trimRunes(p.Subject, 23), r.appendCount(p.ID))
	}
	if !found {
		b.WriteString(" (記事はありません)\r\n")
	}
	b.WriteString("----------------------------------------\r\n")
	b.WriteString("記事番号 / N(New) / M(Main) > ")
	return b.String()
}

func (r *Runtime) renderThread(id int64) string {
	root, ok := r.rootPost(id)
	if !ok {
		return "\r\n記事が見つかりません。\r\nA(アペ) / B(戻る) / M(Main) > "
	}

	posts := r.Store.ListPosts(r.Host.ID)
	var b strings.Builder
	b.WriteString("\r\n========================================\r\n")
	fmt.Fprintf(&b, "[%d] %s\r\n", root.ID, root.Subject)
	fmt.Fprintf(&b, "FROM: %s\r\n", root.Author)
	b.WriteString("----------------------------------------\r\n")
	b.WriteString(normalizeNewlines(root.Body))
	b.WriteString("\r\n")

	appendNo := 0
	for _, p := range posts {
		if p.ParentID != root.ID {
			continue
		}
		appendNo++
		fmt.Fprintf(&b, "\r\n--- アペ %d : %s ---\r\n", appendNo, p.Author)
		b.WriteString(normalizeNewlines(p.Body))
		b.WriteString("\r\n")
	}
	b.WriteString("========================================\r\n")
	b.WriteString("A(アペ) / B(戻る) / M(Main) > ")
	return b.String()
}

func (r *Runtime) appendCount(rootID int64) int {
	count := 0
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.ParentID == rootID {
			count++
		}
	}
	return count
}

func (r *Runtime) rootPost(id int64) (world.Post, bool) {
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.ID == id && p.ParentID == 0 {
			return p, true
		}
	}
	return world.Post{}, false
}

func findBoard(id string) (board, bool) {
	for _, item := range boards {
		if item.ID == strings.TrimSpace(id) {
			return item, true
		}
	}
	return board{}, false
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func trimRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
