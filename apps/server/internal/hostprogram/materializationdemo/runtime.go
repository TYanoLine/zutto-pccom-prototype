package materializationdemo

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
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

	bulkMu  sync.Mutex
	bulkJob *bulkBodyJob

	indexMu   sync.Mutex
	indexJobs map[string]*articleIndexJob
}

type articleIndexJob struct {
	boardID    string
	boardName  string
	state      string
	startedAt  time.Time
	finishedAt time.Time
	envelopes  int
	created    bool
	diagnostic string
}

type bulkBodyTarget struct {
	board  world.Board
	postID int64
}

type bulkBodyJob struct {
	state           string
	startedAt       time.Time
	finishedAt      time.Time
	cancelRequested bool
	boardsTotal     int
	boardsDone      int
	envelopes       int
	initialBodies   int
	requestsDone    int
	complete        int
	current         string
	seed            int64
	order           []bulkBodyTarget
	emptyBoards     []string
	failures        []string
}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{Host: host, Store: store, state: "command"}
}

func (r *Runtime) ObservationBoards() []world.Board {
	if s, ok := r.Store.(materializingStore); ok {
		boards, _ := s.MaterializationBoards(r.Host)
		return append([]world.Board(nil), boards...)
	}
	return nil
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
		runtimeBuildLine()+
		"[DEV] HOST PROFILE : %s\r\n"+
		"[DEV] POPULATION   : %s\r\n\r\n"+
		"NAME     %s\r\nREGION   %s\r\nSOFTWARE %s\r\nLINES    %d\r\nMAX BAUD %d\r\nMEMBERS  %d\r\n\r\n"+
		"この局は開発確認用です。会員総数は人口事実として保持し、初回アクセスではコア住人だけを実体化します。\r\n"+
		"[P] 住人一覧  [B] 掲示板一覧  [ALLBODY] 全本文一括生成  [STATUS] 進捗  [CANCEL] 中断\r\n"+
		"[RESET] 投稿履歴＋会話で遅延具体化したPersona事実を消して再比較  [H] ヘルプ  [G] 切断\r\n\r\nDEV> ", created, population, r.Host.Name, r.Host.Region, r.Host.Software, r.Host.Lines, r.Host.MaxBaud, r.Host.Members)
}

func (r *Runtime) HandleLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	upper := strings.ToUpper(line)

	// Progress/cancellation stay globally reachable even while browsing boards.
	switch upper {
	case "STATUS", "PROGRESS", "ALLSTATUS":
		return r.bulkRenderStatus(), false
	case "CANCEL", "STOP", "ALLCANCEL":
		return r.cancelBulkRenderBodies(), false
	}

	switch r.state {
	case "boards":
		return r.handleBoards(line)
	case "articles":
		return r.handleArticles(line)
	case "article":
		r.state = "articles"
		return r.renderArticles(false), false
	}
	switch upper {
	case "", "H", "HELP", "?":
		return "\r\nP PERSON  コア住人一覧（初回ホスト観測で固定）\r\nB BOARD   掲示板一覧を要求（未生成なら訪問→ROM/書込→因果Envelopeを生成・保存）\r\nALLBODY   全板Envelope生成＋本文生成をバックグラウンド開始。ランダムな記事アクセス順で処理\r\nSTATUS    ALLBODYの進捗を表示（PROGRESS/ALLSTATUSも可）\r\nCANCEL    ALLBODYの中断を要求（STOP/ALLCANCELも可）\r\n          ※現在実行中の1回の生成/計画呼び出しは直ちには止まらず、その終了後に中断します\r\n          ※返信を先に開いた場合も、そのスレッドの先行記事は因果順を守って先に本文化されます\r\nRESET     投稿履歴と遅延Persona事実だけ消去（Persona骨格・局・板は保持）。ALLBODY実行中は不可\r\nG BYE     切断（実行中ALLBODYには中断要求を出します）\r\n\r\nDEV> ", false
	case "P", "PERSON", "PERSONA":
		return r.renderPersonas(), false
	case "B", "BOARD":
		r.state = "boards"
		return r.renderBoards(), false
	case "ALLBODY", "BULK", "RENDERALL":
		return r.startBulkRenderBodies(), false
	case "RESET":
		return r.resetConversation(), false
	case "G", "BYE", "GOODBYE":
		r.requestBulkCancel()
		return "\r\nNO CARRIER\r\n", true
	default:
		return "? COMMAND ERROR\r\nDEV> ", false
	}
}

func (r *Runtime) startBulkRenderBodies() string {
	if _, ok := r.Store.(materializingStore); !ok {
		return "\r\n[DEV] BULK BODY STORE UNAVAILABLE\r\nDEV> "
	}

	r.bulkMu.Lock()
	if r.bulkJob != nil && bulkJobRunning(r.bulkJob.state) {
		job := cloneBulkBodyJob(r.bulkJob)
		r.bulkMu.Unlock()
		return "\r\n[DEV] BULK BODY ALREADY RUNNING\r\n" + formatBulkBodyJobStatus(job) + "\r\nDEV> "
	}
	job := &bulkBodyJob{state: "STARTING", startedAt: time.Now()}
	r.bulkJob = job
	r.bulkMu.Unlock()

	go r.runBulkRenderBodies(job)
	return "\r\n[DEV] BULK BODY STARTED\r\n[DEV] Processing now runs in background so the terminal stays usable.\r\n[DEV] STATUS=進捗表示 / CANCEL=中断要求\r\n\r\nDEV> "
}

func (r *Runtime) runBulkRenderBodies(job *bulkBodyJob) {
	s, ok := r.Store.(materializingStore)
	if !ok {
		r.finishBulkJob(job, "FAILED", "bulk body store unavailable")
		return
	}

	boards, _ := s.MaterializationBoards(r.Host)
	r.updateBulkJob(job, func(j *bulkBodyJob) {
		j.state = "ENVELOPES"
		j.boardsTotal = len(boards)
		j.current = "board envelope planning"
	})

	// Ask every board to observe/materialize its envelope state first. A host-wide
	// producer is allowed to create posts for multiple boards during any one of
	// these calls, including a later retry after an earlier board returned empty.
	// Therefore the per-call return slice is not a stable inventory for ALLBODY.
	for i, board := range boards {
		if r.bulkCancellationRequested(job) {
			r.finishBulkJob(job, "CANCELLED", "")
			return
		}
		r.updateBulkJob(job, func(j *bulkBodyJob) {
			j.current = fmt.Sprintf("board %d/%d %s envelope planning", i+1, len(boards), board.Name)
		})
		_, _ = s.MaterializationPersonaArticleHeaders(r.Host, board)
		r.updateBulkJob(job, func(j *bulkBodyJob) {
			j.boardsDone = i + 1
		})
	}

	if r.bulkCancellationRequested(job) {
		r.finishBulkJob(job, "CANCELLED", "")
		return
	}

	// Re-read canonical state only after all envelope planning calls have returned.
	// This closes the late host-wide materialization hole where board 1/2 could be
	// reported empty, board 3 could finally succeed and create all three boards,
	// yet ALLBODY would render only board 3 because it never revisited earlier
	// return values.
	targets, initialBodies, emptyBoards := collectCanonicalBulkBodyTargets(s, r.Host.ID, boards)
	r.updateBulkJob(job, func(j *bulkBodyJob) {
		j.envelopes = len(targets)
		j.initialBodies = initialBodies
		j.complete = initialBodies
		j.emptyBoards = append([]string(nil), emptyBoards...)
	})
	if len(targets) == 0 {
		r.finishBulkJob(job, "COMPLETED", "no article envelopes")
		return
	}

	seed := time.Now().UnixNano()
	order := shuffledBulkBodyTargets(targets, seed)
	r.updateBulkJob(job, func(j *bulkBodyJob) {
		j.state = "BODIES"
		j.seed = seed
		j.order = append([]bulkBodyTarget(nil), order...)
		j.current = "waiting for first randomized article access"
	})

	failures := make([]string, 0)
	for i, target := range order {
		if r.bulkCancellationRequested(job) {
			r.finishBulkJob(job, "CANCELLED", "")
			return
		}
		r.updateBulkJob(job, func(j *bulkBodyJob) {
			j.current = fmt.Sprintf("request %d/%d %s:%04d", i+1, len(order), target.board.ID, target.postID)
		})
		post, found, _, diagnostic := s.MaterializationArticleWithDebug(r.Host, target.board, target.postID)
		if !found {
			failures = append(failures, fmt.Sprintf("%s:%04d not-found", target.board.ID, target.postID))
		} else if strings.TrimSpace(post.Body) == "" {
			failure := fmt.Sprintf("%s:%04d empty-body", target.board.ID, target.postID)
			if diagnostic = strings.TrimSpace(diagnostic); diagnostic != "" {
				failure += " (" + diagnostic + ")"
			}
			failures = append(failures, failure)
		}
		complete := countBulkCompleteBodies(s, r.Host.ID, targets)
		r.updateBulkJob(job, func(j *bulkBodyJob) {
			j.requestsDone = i + 1
			j.complete = complete
			j.failures = append([]string(nil), failures...)
		})
	}

	r.finishBulkJob(job, "COMPLETED", "")
}

func (r *Runtime) bulkRenderStatus() string {
	r.bulkMu.Lock()
	if r.bulkJob == nil {
		r.bulkMu.Unlock()
		return "\r\n[DEV] BULK STATUS : IDLE / no ALLBODY job has been started in this session\r\nDEV> "
	}
	job := cloneBulkBodyJob(r.bulkJob)
	r.bulkMu.Unlock()
	return "\r\n" + formatBulkBodyJobStatus(job) + "\r\nDEV> "
}

func (r *Runtime) cancelBulkRenderBodies() string {
	r.bulkMu.Lock()
	if r.bulkJob == nil {
		r.bulkMu.Unlock()
		return "\r\n[DEV] BULK CANCEL : no active ALLBODY job\r\nDEV> "
	}
	if !bulkJobRunning(r.bulkJob.state) {
		job := cloneBulkBodyJob(r.bulkJob)
		r.bulkMu.Unlock()
		return "\r\n[DEV] BULK CANCEL : job is not running\r\n" + formatBulkBodyJobStatus(job) + "\r\nDEV> "
	}
	r.bulkJob.cancelRequested = true
	r.bulkJob.state = "CANCELLING"
	current := r.bulkJob.current
	r.bulkMu.Unlock()
	if strings.TrimSpace(current) == "" {
		current = "between steps"
	}
	return fmt.Sprintf("\r\n[DEV] BULK CANCEL REQUESTED\r\n[DEV] CURRENT : %s\r\n[DEV] The current provider/planning call cannot be preempted by this debug wrapper; processing stops before the next board/article.\r\n[DEV] STATUSで停止完了を確認できます。\r\n\r\nDEV> ", current)
}

func (r *Runtime) requestBulkCancel() {
	r.bulkMu.Lock()
	defer r.bulkMu.Unlock()
	if r.bulkJob != nil && bulkJobRunning(r.bulkJob.state) {
		r.bulkJob.cancelRequested = true
		r.bulkJob.state = "CANCELLING"
	}
}

func (r *Runtime) bulkJobIsRunning() bool {
	r.bulkMu.Lock()
	defer r.bulkMu.Unlock()
	return r.bulkJob != nil && bulkJobRunning(r.bulkJob.state)
}

func (r *Runtime) bulkCancellationRequested(job *bulkBodyJob) bool {
	r.bulkMu.Lock()
	defer r.bulkMu.Unlock()
	return r.bulkJob != job || job.cancelRequested
}

func (r *Runtime) updateBulkJob(job *bulkBodyJob, update func(*bulkBodyJob)) {
	r.bulkMu.Lock()
	defer r.bulkMu.Unlock()
	if r.bulkJob != job {
		return
	}
	update(job)
}

func (r *Runtime) finishBulkJob(job *bulkBodyJob, state, detail string) {
	r.bulkMu.Lock()
	defer r.bulkMu.Unlock()
	if r.bulkJob != job {
		return
	}
	job.state = state
	job.finishedAt = time.Now()
	job.current = strings.TrimSpace(detail)
}

func bulkJobRunning(state string) bool {
	switch state {
	case "STARTING", "ENVELOPES", "BODIES", "CANCELLING":
		return true
	default:
		return false
	}
}

func cloneBulkBodyJob(job *bulkBodyJob) bulkBodyJob {
	clone := *job
	clone.order = append([]bulkBodyTarget(nil), job.order...)
	clone.emptyBoards = append([]string(nil), job.emptyBoards...)
	clone.failures = append([]string(nil), job.failures...)
	return clone
}

func formatBulkBodyJobStatus(job bulkBodyJob) string {
	elapsedEnd := time.Now()
	if !job.finishedAt.IsZero() {
		elapsedEnd = job.finishedAt
	}
	elapsed := elapsedEnd.Sub(job.startedAt).Round(time.Second)
	if elapsed < 0 {
		elapsed = 0
	}
	generated := job.complete - job.initialBodies
	if generated < 0 {
		generated = 0
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[DEV] BULK STATUS    : %s / elapsed=%s\r\n", job.state, elapsed)
	fmt.Fprintf(&b, "[DEV] BOARDS         : %d/%d / envelopes=%d\r\n", job.boardsDone, job.boardsTotal, job.envelopes)
	if len(job.order) > 0 {
		fmt.Fprintf(&b, "[DEV] REQUESTS       : %d/%d / bodies=%d/%d / generated=%d / failures=%d\r\n", job.requestsDone, len(job.order), job.complete, len(job.order), generated, len(job.failures))
	} else {
		fmt.Fprintf(&b, "[DEV] REQUESTS       : not started / bodies=%d / initial=%d\r\n", job.complete, job.initialBodies)
	}
	if job.current != "" {
		fmt.Fprintf(&b, "[DEV] CURRENT        : %s\r\n", job.current)
	}
	if job.seed != 0 {
		fmt.Fprintf(&b, "[DEV] ACCESS SEED    : %d\r\n", job.seed)
	}
	if len(job.emptyBoards) > 0 {
		fmt.Fprintf(&b, "[DEV] EMPTY BOARDS   : %s\r\n", strings.Join(job.emptyBoards, " / "))
	}
	if len(job.failures) > 0 {
		fmt.Fprintf(&b, "[DEV] LAST FAILURE   : %s\r\n", job.failures[len(job.failures)-1])
	}
	if job.cancelRequested && bulkJobRunning(job.state) {
		b.WriteString("[DEV] CANCEL         : requested; waiting for current call to return\r\n")
	}
	if len(job.order) > 0 && !bulkJobRunning(job.state) {
		b.WriteString("[DEV] REQUEST ORDER  : randomized across boards/articles\r\n")
		b.WriteString(formatBulkBodyOrder(job.order))
		b.WriteString("[DEV] THREAD ORDER   : predecessor bodies may have been generated first when a random request landed on a later reply\r\n")
	}
	return b.String()
}

func collectCanonicalBulkBodyTargets(s materializingStore, hostID string, boards []world.Board) ([]bulkBodyTarget, int, []string) {
	boardByID := make(map[string]world.Board, len(boards))
	counts := make(map[string]int, len(boards))
	for _, board := range boards {
		boardByID[board.ID] = board
	}

	targets := make([]bulkBodyTarget, 0, 48)
	initialBodies := 0
	for _, post := range s.ListPosts(hostID) {
		board, known := boardByID[post.BoardID]
		if !known {
			continue
		}
		targets = append(targets, bulkBodyTarget{board: board, postID: post.ID})
		counts[board.ID]++
		if strings.TrimSpace(post.Body) != "" {
			initialBodies++
		}
	}

	emptyBoards := make([]string, 0, len(boards))
	for _, board := range boards {
		if counts[board.ID] == 0 {
			emptyBoards = append(emptyBoards, board.Name)
		}
	}
	return targets, initialBodies, emptyBoards
}

func countBulkCompleteBodies(s materializingStore, hostID string, targets []bulkBodyTarget) int {
	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[bulkBodyTargetKey(target)] = struct{}{}
	}
	complete := 0
	for _, post := range s.ListPosts(hostID) {
		key := post.BoardID + ":" + strconv.FormatInt(post.ID, 10)
		if _, wanted := targetSet[key]; wanted && strings.TrimSpace(post.Body) != "" {
			complete++
		}
	}
	return complete
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
	if r.articleIndexGenerationRunning() {
		return "\r\n[DEV] RESET BLOCKED : article index generation is still running. Wait for READY, then RESET.\r\nDEV> "
	}
	if r.bulkJobIsRunning() {
		return "\r\n[DEV] RESET BLOCKED : ALLBODY is still running. Use CANCEL, wait for STATUS=CANCELLED, then RESET.\r\nDEV> "
	}
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\n[DEV] RESET STORE UNAVAILABLE\r\nDEV> "
	}
	posts, facts, ok := s.ResetMaterializationConversation(r.Host)
	if !ok {
		return "\r\n[DEV] RESET STORE UNAVAILABLE\r\nDEV> "
	}
	r.board = world.Board{}
	r.indexMu.Lock()
	r.indexJobs = nil
	r.indexMu.Unlock()
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

func runtimeBoardPosts(posts []world.Post, boardID string) []world.Post {
	out := make([]world.Post, 0)
	for _, post := range posts {
		if post.BoardID == boardID {
			out = append(out, post)
		}
	}
	return out
}

func (r *Runtime) articleIndexSnapshot(boardID string) (articleIndexJob, bool) {
	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	job := r.indexJobs[boardID]
	if job == nil {
		return articleIndexJob{}, false
	}
	return *job, true
}

func (r *Runtime) articleIndexGenerationRunning() bool {
	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	for _, job := range r.indexJobs {
		if job != nil && job.state == "RUNNING" {
			return true
		}
	}
	return false
}

func (r *Runtime) startArticleIndexGeneration() string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\nSTORE ERROR\r\n"
	}
	if len(runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)) > 0 {
		return r.renderArticles(true)
	}

	r.indexMu.Lock()
	if r.indexJobs == nil {
		r.indexJobs = map[string]*articleIndexJob{}
	}
	if existing := r.indexJobs[r.board.ID]; existing != nil {
		snapshot := *existing
		r.indexMu.Unlock()
		return formatArticleIndexJob(snapshot)
	}
	job := &articleIndexJob{boardID: r.board.ID, boardName: r.board.Name, state: "RUNNING", startedAt: time.Now()}
	r.indexJobs[r.board.ID] = job
	initial := *job
	r.indexMu.Unlock()

	board := r.board
	go func() {
		posts, created := s.MaterializationPersonaArticleHeaders(r.Host, board)
		diagnostic := s.MaterializationPlanningDiagnostic(r.Host.ID, board.ID)
		r.indexMu.Lock()
		defer r.indexMu.Unlock()
		current := r.indexJobs[board.ID]
		if current != job {
			return
		}
		job.state = "COMPLETED"
		job.finishedAt = time.Now()
		job.envelopes = len(posts)
		job.created = created
		job.diagnostic = diagnostic
	}()

	return formatArticleIndexJob(initial)
}

func formatArticleIndexJob(job articleIndexJob) string {
	elapsedEnd := time.Now()
	if !job.finishedAt.IsZero() {
		elapsedEnd = job.finishedAt
	}
	elapsed := elapsedEnd.Sub(job.startedAt).Round(100 * time.Millisecond)
	if elapsed < 0 {
		elapsed = 0
	}
	if job.state == "RUNNING" {
		return fmt.Sprintf("\r\n[DEV] ARTICLE INDEX : GENERATING IN BACKGROUND / %s / elapsed=%s\r\n[DEV] 端末は待たされません。RETURN または R で再読込、Qで掲示板一覧へ戻れます。\r\n\r\nR=再読込 / Q=掲示板一覧 > ", job.boardName, elapsed)
	}
	line := fmt.Sprintf("\r\n[DEV] ARTICLE INDEX : READY / %s / envelopes=%d / elapsed=%s\r\n", job.boardName, job.envelopes, elapsed)
	if strings.TrimSpace(job.diagnostic) != "" {
		line += "[DEV] CAUSAL PLAN   : " + job.diagnostic + "\r\n"
	}
	return line + "RETURN または R で記事一覧を表示 / Q=掲示板一覧 > "
}

func (r *Runtime) renderArticles(showMaterialization bool) string {
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "\r\nSTORE ERROR\r\n"
	}
	posts := runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)
	created := false
	if observer, ok := r.Store.(world.HostObservationStore); ok {
		observed, err := observer.WaitForBoardHeaders(context.Background(), r.Host, r.board)
		if err != nil {
			return "\r\n[DEV] ARTICLE INDEX ERROR : " + err.Error() + "\r\nQ=掲示板一覧 > "
		}
		posts = observed
	} else if len(posts) == 0 {
		// Compatibility path for isolated tests/stores that do not implement the
		// production observation barrier.
		if job, exists := r.articleIndexSnapshot(r.board.ID); exists {
			if job.state == "RUNNING" {
				return formatArticleIndexJob(job)
			}
			created = job.created
			posts = runtimeBoardPosts(r.Store.ListPosts(r.Host.ID), r.board.ID)
		} else {
			return r.startArticleIndexGeneration()
		}
	} else if job, exists := r.articleIndexSnapshot(r.board.ID); exists {
		created = job.created
	}
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
	if strings.TrimSpace(line) == "" || strings.EqualFold(line, "R") || strings.EqualFold(line, "REFRESH") {
		return r.renderArticles(false), false
	}
	if job, exists := r.articleIndexSnapshot(r.board.ID); exists && job.state == "RUNNING" {
		return formatArticleIndexJob(job), false
	}
	id, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return "? MSG NUMBER\r\nMSG No.を選択 / Q=掲示板一覧 > ", false
	}
	s, ok := r.Store.(materializingStore)
	if !ok {
		return "STORE ERROR\r\n", false
	}
	var p world.Post
	var found bool
	created := false
	usage := ""
	if observer, ok := r.Store.(world.HostObservationStore); ok {
		for _, before := range r.Store.ListPosts(r.Host.ID) {
			if before.ID == id && before.BoardID == r.board.ID {
				created = strings.TrimSpace(before.Body) == ""
				break
			}
		}
		var waitErr error
		p, found, waitErr = observer.WaitForArticleBody(context.Background(), r.Host, r.board, id)
		if waitErr != nil {
			return "MSG READ ERROR\r\nMSG No.を選択 / Q=掲示板一覧 > ", false
		}
		if total := s.MaterializationUsageTotalText(); total != "" {
			usage = total
		}
	} else {
		p, found, created, usage = s.MaterializationArticleWithDebug(r.Host, r.board, id)
	}
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
