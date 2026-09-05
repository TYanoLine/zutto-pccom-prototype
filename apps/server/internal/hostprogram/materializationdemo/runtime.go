package materializationdemo

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type materializingStore interface {
	world.Store
	HostWasMaterialized(hostID string) bool
	PopulationWasMaterialized(hostID string) bool
	MaterializationPersonas(host world.Host) ([]world.Persona, bool)
	MaterializationBoards(host world.Host) ([]world.Board, bool)
	MaterializationPersonaArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool)
	MaterializationArticleWithDebug(host world.Host, board world.Board, postID int64) (world.Post, bool, bool, string)
	MaterializationPlanningDiagnostic(hostID, boardID string) string
	MaterializationUsageTotalText() string
	ResetMaterializationConversation(host world.Host) (postsCleared int, personaFactsCleared int, ok bool)
}

type Runtime struct {
	Host  world.Host
	Store world.Store
	state string
	board world.Board
}

type bulkBodyTarget struct {
	board  world.Board
	postID int64
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
		"[P] 住人一覧  [B] 掲示板一覧  [ALLBODY] 全本文一括生成  [H] ヘルプ  [G] 切断\r\n"+
		"[RESET] 投稿履歴＋会話で遅延具体化したPersona事実を消して再比較\r\n\r\nDEV> ", created, population, r.Host.Name, r.Host.Region, r.Host.Software, r.Host.Lines, r.Host.MaxBaud, r.Host.Members)
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
		return "\r\nP PERSON  コア住人一覧（初回ホスト観測で固定）\r\nB BOARD   掲示板一覧を要求（未生成なら訪問→ROM/書込→因果Envelopeを生成・保存）\r\nALLBODY   全板のEnvelopeを生成後、全記事をランダムなアクセス順で開いて本文を一括生成\r\n          ※返信を先に開いた場合も、そのスレッドの先行記事は因果順を守って先に本文化されます\r\nRESET     投稿履歴と遅延Persona事実だけ消去（Persona骨格・局・板は保持）\r\nG BYE     切断\r\n\r\nDEV> ", false
	case "P", "PERSON", "PERSONA":
		return r.renderPersonas(), false
	case "B", "BOARD":
		r.state = "boards"
		return r.renderBoards(), false
	case "ALLBODY", "BULK", "RENDERALL":
		return r.bulkRenderBodies(), false
	case "RESET":
		return r.resetConversation(), false
	case "G", "BYE", "GOODBYE":
		return "\r\nNO CARRIER\r\n", true
	default:
		return "? COMMAND ERROR\r\nDEV> ", false
	}
}

func (r *Runtime) bulkRenderBodies() string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\n[DEV] BULK BODY STORE UNAVAILABLE\r\nDEV> "
	}

	boards, _ := s.MaterializationBoards(r.Host)
	targets := make([]bulkBodyTarget, 0, 48)
	initialBodies := 0
	emptyBoards := make([]string, 0, len(boards))
	for _, board := range boards {
		posts, _ := s.MaterializationPersonaArticleHeaders(r.Host, board)
		if len(posts) == 0 {
			emptyBoards = append(emptyBoards, board.Name)
			continue
		}
		for _, post := range posts {
			targets = append(targets, bulkBodyTarget{board: board, postID: post.ID})
			if strings.TrimSpace(post.Body) != "" {
				initialBodies++
			}
		}
	}
	if len(targets) == 0 {
		return "\r\n[DEV] BULK BODY : NO ARTICLE ENVELOPES\r\n[DEV] Boards may be empty by simulation or semantic planning may have failed.\r\nDEV> "
	}

	seed := time.Now().UnixNano()
	order := shuffledBulkBodyTargets(targets, seed)
	failures := make([]string, 0)
	for _, target := range order {
		post, found, _, diagnostic := s.MaterializationArticleWithDebug(r.Host, target.board, target.postID)
		if !found {
			failures = append(failures, fmt.Sprintf("%s:%04d not-found", target.board.ID, target.postID))
			continue
		}
		if strings.TrimSpace(post.Body) == "" {
			failure := fmt.Sprintf("%s:%04d empty-body", target.board.ID, target.postID)
			if diagnostic = strings.TrimSpace(diagnostic); diagnostic != "" {
				failure += " (" + diagnostic + ")"
			}
			failures = append(failures, failure)
		}
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[bulkBodyTargetKey(target)] = struct{}{}
	}
	complete := 0
	for _, post := range s.ListPosts(r.Host.ID) {
		key := post.BoardID + ":" + strconv.FormatInt(post.ID, 10)
		if _, wanted := targetSet[key]; wanted && strings.TrimSpace(post.Body) != "" {
			complete++
		}
	}
	generated := complete - initialBodies
	if generated < 0 {
		generated = 0
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[DEV] BULK BODY      : boards=%d envelopes=%d initial_bodies=%d generated=%d complete=%d failures=%d\r\n", len(boards), len(targets), initialBodies, generated, complete, len(failures))
	fmt.Fprintf(&b, "[DEV] ACCESS SEED    : %d\r\n", seed)
	b.WriteString("[DEV] REQUEST ORDER  : randomized across boards/articles\r\n")
	b.WriteString(formatBulkBodyOrder(order))
	b.WriteString("[DEV] THREAD ORDER   : predecessor bodies may be generated first when a random request lands on a later reply\r\n")
	if len(emptyBoards) > 0 {
		fmt.Fprintf(&b, "[DEV] EMPTY BOARDS   : %s\r\n", strings.Join(emptyBoards, " / "))
	}
	if len(failures) > 0 {
		b.WriteString("[DEV] FAILURES       :\r\n")
		for _, failure := range failures {
			fmt.Fprintf(&b, "  %s\r\n", failure)
		}
	}
	if total := s.MaterializationUsageTotalText(); total != "" {
		fmt.Fprintf(&b, "[DEV] TOKEN TOTAL    : %s\r\n", total)
	}
	b.WriteString("\r\nDEV> ")
	return b.String()
}

func shuffledBulkBodyTargets(targets []bulkBodyTarget, seed int64) []bulkBodyTarget {
	out := append([]bulkBodyTarget(nil), targets...)
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func bulkBodyTargetKey(target bulkBodyTarget) string {
	return target.board.ID + ":" + strconv.FormatInt(target.postID, 10)
}

func formatBulkBodyOrder(order []bulkBodyTarget) string {
	var b strings.Builder
	for i, target := range order {
		if i%8 == 0 {
			b.WriteString("                      ")
		}
		fmt.Fprintf(&b, "%s:%04d", target.board.ID, target.postID)
		if i == len(order)-1 || i%8 == 7 {
			b.WriteString("\r\n")
		} else {
			b.WriteString(" ")
		}
	}
	return b.String()
}

func (r *Runtime) resetConversation() string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\n[DEV] RESET STORE UNAVAILABLE\r\nDEV> "
	}
	posts, facts, ok := s.ResetMaterializationConversation(r.Host)
	if !ok {
		return "\r\n[DEV] RESET STORE UNAVAILABLE\r\nDEV> "
	}
	r.board = world.Board{}
	return fmt.Sprintf("\r\n[DEV] CONVERSATION RESET : posts=%d / persona_facts=%d\r\n[DEV] KEPT               : host + boards + core persona skeletons\r\n次に B で掲示板へ入ると因果Envelopeを再生成します。\r\n\r\nDEV> ", posts, facts)
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
	posts, created := s.MaterializationPersonaArticleHeaders(r.Host, r.board)
	status := "STORED REUSE"
	if created {
		status = "SPARSE CAUSAL ENVELOPES MATERIALIZED + STORED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n[%s]\r\n", r.board.Name)
	if showMaterialization || created {
		fmt.Fprintf(&b, "[DEV] ARTICLE INDEX : %s / %d ENVELOPES\r\n", status, len(posts))
		if diagnostic := s.MaterializationPlanningDiagnostic(r.Host.ID, r.board.ID); diagnostic != "" {
			fmt.Fprintf(&b, "[DEV] CAUSAL PLAN   : %s\r\n", diagnostic)
		}
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
		status = "BODY RENDERED FROM PERSONA + CAUSAL INTENT + BBS CONTEXT, STORED"
	}
	if strings.TrimSpace(p.Body) == "" {
		status = "BODY GENERATION FAILED / ENVELOPE KEPT"
	}
	tokenLine := "[DEV] TOKENS        : n/a\r\n"
	if usage != "" {
		tokenLine = "[DEV] TOKENS        : " + usage + "\r\n"
	}
	if total := s.MaterializationUsageTotalText(); total != "" {
		tokenLine += "[DEV] TOKEN TOTAL   : " + total + "\r\n"
	}
	claimLine := "[DEV] CLAIMS        : (none)\r\n"
	if len(p.Intent.Claims) > 0 {
		claimLine = "[DEV] CLAIMS        : " + strings.Join(p.Intent.Claims, " / ") + "\r\n"
	}
	semanticLine := ""
	if p.Intent.AnchorKey != "" {
		semanticLine += "[DEV] ANCHOR        : " + p.Intent.AnchorKey + "\r\n"
	}
	if p.Intent.CauseKind != "" {
		semanticLine += "[DEV] CAUSE         : " + p.Intent.CauseKind + "\r\n"
	}
	if p.Intent.SourcePostID != 0 {
		semanticLine += fmt.Sprintf("[DEV] SOURCE MSG    : %04d\r\n", p.Intent.SourcePostID)
	}
	if p.Intent.Goal != "" {
		semanticLine += "[DEV] GOAL          : " + p.Intent.Goal + "\r\n"
	}
	if p.Intent.Stance != "" {
		semanticLine += "[DEV] STANCE        : " + p.Intent.Stance + "\r\n"
	}
	if p.Intent.RespondsToPostID != 0 {
		semanticLine += fmt.Sprintf("[DEV] TARGET MSG    : %04d\r\n", p.Intent.RespondsToPostID)
	}
	if len(p.Intent.RespondsToClaims) > 0 {
		semanticLine += "[DEV] RESPONDS TO   : " + strings.Join(p.Intent.RespondsToClaims, " / ") + "\r\n"
	}
	r.state = "article"
	return fmt.Sprintf("\r\n[DEV] ARTICLE BODY : %s\r\n[DEV] ACTOR         : %s (%s)\r\n[DEV] ENVELOPE      : action=%s / topic=%s\r\n[DEV] MOTIVATION    : %s\r\n%s%s%s\r\nMSG No.%04d  %s\r\nFROM: %s\r\n------------------------------------------------------------\r\n%s\r\n------------------------------------------------------------\r\nRETURNで記事一覧 > ", status, p.Author, p.AuthorPersonaID, p.Intent.Action, p.Intent.Topic, p.Intent.Motivation, claimLine, semanticLine, tokenLine, p.ID, p.Subject, p.Author, p.Body), false
}
