package materializationdemo

import (
	"fmt"
	"strconv"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

type materializingStore interface {
	world.Store
	HostWasMaterialized(hostID string) bool
	PopulationWasMaterialized(hostID string) bool
	MaterializationPersonas(host world.Host) ([]world.Persona, bool)
	MaterializationBoards(host world.Host) ([]world.Board, bool)
	MaterializationDenseArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool)
	MaterializationArticleWithDebug(host world.Host, board world.Board, postID int64) (world.Post, bool, bool, string)
	MaterializationUsageTotalText() string
}

type Runtime struct {
	Host  world.Host
	Store world.Store
	state string
	board world.Board
}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{Host: host, Store: store, state: "command"}
}

func (r *Runtime) Welcome() string {
	created := "STORED REUSE"
	population := "STORED REUSE"
	if s, ok := r.Store.(materializingStore); ok {
		if s.HostWasMaterialized(r.Host.ID) {
			created = "MATERIALIZED + STORED"
		}
		if s.PopulationWasMaterialized(r.Host.ID) {
			population = "CORE PERSONAS MATERIALIZED + STORED"
		}
	}
	return fmt.Sprintf("\x1b[2J\x1b[H=== DEVELOPMENT MATERIALIZATION HOST ===\r\n"+
		"[DEV] HOST PROFILE : %s\r\n"+
		"[DEV] POPULATION   : %s\r\n\r\n"+
		"NAME     %s\r\nREGION   %s\r\nSOFTWARE %s\r\nLINES    %d\r\nMAX BAUD %d\r\nMEMBERS  %d\r\n\r\n"+
		"この局は開発確認用です。会員総数は人口事実として保持し、初回アクセスではコア住人だけを実体化します。\r\n"+
		"[P] 住人一覧  [B] 掲示板一覧  [H] ヘルプ  [G] 切断\r\n\r\nDEV> ", created, population, r.Host.Name, r.Host.Region, r.Host.Software, r.Host.Lines, r.Host.MaxBaud, r.Host.Members)
}

func (r *Runtime) HandleLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	switch r.state {
	case "boards":
		return r.handleBoards(line)
	case "articles":
		return r.handleArticles(line)
	case "article":
		r.state = "articles"
		return r.renderArticles(false), false
	}
	switch strings.ToUpper(line) {
	case "", "H", "HELP", "?":
		return "\r\nP PERSON  コア住人一覧（初回ホスト観測で固定）\r\nB BOARD   掲示板一覧を要求（未生成ならPersona駆動で履歴を生成・保存）\r\nG BYE     切断\r\n\r\nDEV> ", false
	case "P", "PERSON", "PERSONA":
		return r.renderPersonas(), false
	case "B", "BOARD":
		r.state = "boards"
		return r.renderBoards(), false
	case "G", "BYE", "GOODBYE":
		return "\r\nNO CARRIER\r\n", true
	default:
		return "? COMMAND ERROR\r\nDEV> ", false
	}
}

func (r *Runtime) renderPersonas() string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\n[DEV] MATERIALIZATION STORE UNAVAILABLE\r\nDEV> "
	}
	personas, created := s.MaterializationPersonas(r.Host)
	status := "STORED REUSE"
	if created {
		status = "CORE PERSONAS MATERIALIZED + STORED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[DEV] CORE PERSONAS : %s\r\n", status)
	fmt.Fprintf(&b, "HOST MEMBERS=%d / DETAILED CORE=%d\r\n", r.Host.Members, len(personas))
	b.WriteString("--------------------------------------------------------------------------\r\n")
	for _, p := range personas {
		fmt.Fprintf(&b, " %-6s age=%2d reply=%.2f lurker=%.2f start=%.2f  %s\r\n", p.Handle, p.Age, p.ReplyTendency, p.LurkerTendency, p.ThreadStartTendency, p.ActivityPattern)
		fmt.Fprintf(&b, "        %s / %s\r\n", p.Occupation, p.WritingStyle)
	}
	b.WriteString("--------------------------------------------------------------------------\r\nDEV> ")
	return b.String()
}

func (r *Runtime) renderBoards() string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\n[DEV] MATERIALIZATION STORE UNAVAILABLE\r\nDEV> "
	}
	boards, created := s.MaterializationBoards(r.Host)
	status := "STORED REUSE"
	if created {
		status = "MATERIALIZED + STORED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[DEV] BOARD CATALOG : %s\r\n", status)
	b.WriteString("----------------------------------------\r\n")
	for i, v := range boards {
		fmt.Fprintf(&b, " %d. %s\r\n", i+1, v.Name)
	}
	b.WriteString("----------------------------------------\r\n番号を選択 / Q=戻る > ")
	return b.String()
}

func (r *Runtime) handleBoards(line string) (string, bool) {
	if strings.EqualFold(line, "Q") || line == "/" {
		r.state = "command"
		return "\r\nDEV> ", false
	}
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "STORE ERROR\r\n", false
	}
	boards, _ := s.MaterializationBoards(r.Host)
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(boards) {
		return "? BOARD NUMBER\r\n番号を選択 / Q=戻る > ", false
	}
	r.board = boards[n-1]
	r.state = "articles"
	return r.renderArticles(true), false
}

func (r *Runtime) renderArticles(showMaterialization bool) string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\nSTORE ERROR\r\n"
	}
	posts, created := s.MaterializationDenseArticleHeaders(r.Host, r.board)
	status := "STORED REUSE"
	if created {
		status = "PERSONA-DRIVEN ENVELOPES MATERIALIZED + STORED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[%s]\r\n", r.board.Name)
	if showMaterialization || created {
		fmt.Fprintf(&b, "[DEV] ARTICLE INDEX : %s / %d ENVELOPES\r\n", status, len(posts))
	}
	b.WriteString("------------------------------------------------------------------------\r\n")
	for _, p := range posts {
		body := "本文:保存済"
		if strings.TrimSpace(p.Body) == "" {
			body = "本文:未生成"
		}
		fmt.Fprintf(&b, " %04d %s %-8s %-24s [%s/%s]\r\n", p.ID, p.CreatedAt.Format("01/02 15:04"), p.Author, p.Subject, p.Intent.Action, body)
	}
	b.WriteString("------------------------------------------------------------------------\r\nMSG No.を選択 / Q=掲示板一覧 > ")
	return b.String()
}

func (r *Runtime) handleArticles(line string) (string, bool) {
	if strings.EqualFold(line, "Q") || line == "/" {
		r.state = "boards"
		return r.renderBoards(), false
	}
	id, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return "? MSG NUMBER\r\nMSG No.を選択 / Q=掲示板一覧 > ", false
	}
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "STORE ERROR\r\n", false
	}
	p, found, created, usage := s.MaterializationArticleWithDebug(r.Host, r.board, id)
	if !found {
		return "MSG NOT FOUND\r\nMSG No.を選択 / Q=掲示板一覧 > ", false
	}
	status := "STORED REUSE"
	if created {
		status = "BODY RENDERED FROM PERSONA + ENVELOPE, STORED"
	}
	if strings.TrimSpace(p.Body) == "" {
		status = "BODY GENERATION FAILED / ENVELOPE KEPT"
	}
	tokenLine := "[DEV] TOKENS        : n/a (fallback / non-OpenAI renderer)\r\n"
	if usage != "" {
		tokenLine = "[DEV] TOKENS        : " + usage + "\r\n"
	}
	if total := s.MaterializationUsageTotalText(); total != "" {
		tokenLine += "[DEV] TOKEN TOTAL   : " + total + "\r\n"
	}
	r.state = "article"
	return fmt.Sprintf("\r\n[DEV] ARTICLE BODY : %s\r\n[DEV] ACTOR         : %s (%s)\r\n[DEV] ENVELOPE      : action=%s / topic=%s\r\n[DEV] MOTIVATION    : %s\r\n%s\r\nMSG No.%04d  %s\r\nFROM: %s\r\n------------------------------------------------------------\r\n%s\r\n------------------------------------------------------------\r\nRETURNで記事一覧 > ", status, p.Author, p.AuthorPersonaID, p.Intent.Action, p.Intent.Topic, p.Intent.Motivation, tokenLine, p.ID, p.Subject, p.Author, p.Body), false
}
