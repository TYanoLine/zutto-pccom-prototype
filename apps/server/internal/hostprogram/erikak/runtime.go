package erikak

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type boardNode struct {
	Path            string
	Key             string
	Parent          string
	Name            string
	Hidden          bool
	SemanticScope   string
	ActivityWeight  float64
	ReplyRate       float64
	RetainedRootCap int
}

var boardTree = []boardNode{
	// Activity values below are HAKATA station fiction used by the world
	// simulation. They are not claimed Erika-K defaults.
	{Path: "1", Key: "1", Name: "事務局からのお知らせ", SemanticScope: "SYSOP・運営側からの局内告知、利用案内、保守連絡。一般会員の趣味相談や雑談を置かない。", ActivityWeight: .10, ReplyRate: .20, RetainedRootCap: 24},
	{Path: "2", Key: "2", Name: "自己紹介・新人歓迎", SemanticScope: "新規会員の自己紹介、常連からの歓迎、局内での呼び名や簡単な近況。特定趣味の専門相談板にはしない。", ActivityWeight: .34, ReplyRate: 1.30, RetainedRootCap: 36},
	{Path: "3", Key: "3", Name: "Ｑ＆Ａ（質問ボード）", SemanticScope: "会員が分野を限定せず日常の疑問や相談を持ち寄る一般質問板。地域生活、仕事・学校、買い物、交通、食事、趣味、局の使い方などが混在する。PC・ゲームの専門質問は専用板が別にあるため、この板全体をPC/ゲーム中心にしない。", ActivityWeight: .58, ReplyRate: 2.10, RetainedRootCap: 48},
	{Path: "4", Key: "4", Name: "ふり～と～く", SemanticScope: "会員の日常雑談。仕事・学校・家族・食事・天気・街・趣味・最近あった小さな出来事など何でもあり。専門板の話題だけに偏らない。", ActivityWeight: 1.25, ReplyRate: 2.00, RetainedRootCap: 60},
	{Path: "5", Key: "5", Name: "オフライントピックス", SemanticScope: "局外で会うこと、オフ会、待ち合わせ、参加確認、持ち物、終了後の連絡など実際に会う活動。", ActivityWeight: .48, ReplyRate: 1.70, RetainedRootCap: 42},
	{Path: "6", Key: "6", Name: "街角情報スポット", SemanticScope: "福岡・博多・天神周辺の店、交通、街の変化、地域の用事や生活情報。PC/ゲームの話は地域情報として必要な場合だけ。", ActivityWeight: .72, ReplyRate: 1.30, RetainedRootCap: 48},
	{Path: "7", Key: "7", Name: "ＣＡＮＡＬ市場", SemanticScope: "会員同士の譲ります・譲ってください・交換・探し物などの売買交換連絡。", ActivityWeight: .30, ReplyRate: .75, RetainedRootCap: 30},
	// "夢工房はかた" の意味は史料未確定。ここでは意味を推測せず、
	// 局固有の活動量だけを設定する。
	{Path: "8", Key: "8", Name: "夢工房はかた", SemanticScope: "史料上の板の意味は未確認。板名からゲーム制作・創作工房などの意味を推測して話題を決めない。局固有設定が確定するまで狭い専門内容を自動付与しない。", ActivityWeight: .42, ReplyRate: 1.10, RetainedRootCap: 36},
	{Path: "10", Key: "10", Name: "博多・天神広場"},
	{Path: "20", Key: "20", Name: "アミューズメントフォーラム"},
	{Path: "60", Key: "60", Name: "コンピュータワールド"},
	{Path: "68", Key: "68", Name: "ＣＡＮＡＬ Ｘ村"},
	{Path: "70", Key: "70", Name: "９８ VS ＡＴ互換機"},
	{Path: "80", Key: "80", Name: "その他のコンピュータ"},
	{Path: "99", Key: "99", Name: "夜更かし部屋", Hidden: true, ActivityWeight: .34, ReplyRate: 2.30, RetainedRootCap: 36},

	{Path: "10/1", Key: "1", Parent: "10", Name: "博多・天神ローカル", SemanticScope: "博多・天神を中心とした地域の日常、店、交通、待ち合わせ、街の変化、地元での小さな出来事。", ActivityWeight: 1.00, ReplyRate: 1.45, RetainedRootCap: 54},
	{Path: "10/2", Key: "2", Parent: "10", Name: "オフ会連絡", SemanticScope: "オフ会の日程、集合場所、参加可否、当日の連絡、終了後の忘れ物など。", ActivityWeight: .48, ReplyRate: 1.75, RetainedRootCap: 36},
	{Path: "20/1", Key: "1", Parent: "20", Name: "ＧＡＭＥ", SemanticScope: "家庭用・PC等のゲームについての感想、攻略上の詰まり、対戦、貸し借り、購入相談など。ゲーム以外のPC一般話題を持ち込まない。", ActivityWeight: .92, ReplyRate: 1.65, RetainedRootCap: 52},
	{Path: "20/2", Key: "2", Parent: "20", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ", SemanticScope: "アニメ、漫画、関連する雑談や感想。ゲームやPC一般は主題にしない。", ActivityWeight: .64, ReplyRate: 1.55, RetainedRootCap: 44},
	{Path: "60/1", Key: "1", Parent: "60", Name: "ＰＣ－９８／ＭＯＤＥＭ", SemanticScope: "PC-98系やモデム、通信環境についての具体的な相談・情報交換。", ActivityWeight: .84, ReplyRate: 1.95, RetainedRootCap: 50},
	{Path: "60/2", Key: "2", Parent: "60", Name: "Ｗｉｎｄｏｗｓ／ＤＯＳ", SemanticScope: "WindowsやDOSの操作、設定、ソフト利用上の相談や情報交換。", ActivityWeight: .74, ReplyRate: 1.85, RetainedRootCap: 48},
	{Path: "60/3", Key: "3", Parent: "60", Name: "ＳＯＦＴＷＡＲＥ／ＤＡＴＡ", SemanticScope: "ソフトウェア、データ、ファイル、ツール利用の情報交換。ハードやゲームそのものへ逸れすぎない。", ActivityWeight: .70, ReplyRate: 1.70, RetainedRootCap: 46},
	{Path: "68/1", Key: "1", Parent: "68", Name: "深夜雑談", SemanticScope: "深夜に接続している会員のゆるい雑談。日常、眠気、仕事・学校、食事、テレビ、音楽、趣味など幅広く、PC/ゲーム専用ではない。", ActivityWeight: .62, ReplyRate: 2.20, RetainedRootCap: 44},
	{Path: "70/1", Key: "1", Parent: "70", Name: "ＰＣ－９８", SemanticScope: "PC-98系機種の利用、設定、周辺機器、ソフト利用など。", ActivityWeight: .67, ReplyRate: 1.85, RetainedRootCap: 46},
	{Path: "70/2", Key: "2", Parent: "70", Name: "ＤＯＳ／Ｖ", SemanticScope: "DOS/V・AT互換機側の利用、設定、周辺機器、ソフト利用など。", ActivityWeight: .48, ReplyRate: 1.55, RetainedRootCap: 38},
	{Path: "80/1", Key: "1", Parent: "80", Name: "ＦＭ－ＴＯＷＮＳ", SemanticScope: "FM TOWNS利用者の機種・ソフト・周辺機器等の情報交換。", ActivityWeight: .31, ReplyRate: 1.35, RetainedRootCap: 30},
	{Path: "80/2", Key: "2", Parent: "80", Name: "Forever with MSX", SemanticScope: "MSX利用者の機種・ソフト・周辺機器等の情報交換。", ActivityWeight: .28, ReplyRate: 1.40, RetainedRootCap: 28},
	{Path: "80/3", Key: "3", Parent: "80", Name: "ワープロ", SemanticScope: "ワープロ専用機や文書作成・印刷等の利用情報。", ActivityWeight: .30, ReplyRate: 1.20, RetainedRootCap: 30},
	{Path: "80/4", Key: "4", Parent: "80", Name: "その他(PC88,FMR,etc)", SemanticScope: "PC-88、FMR等、他の専用板に当てはまらないコンピュータ機種の情報交換。", ActivityWeight: .26, ReplyRate: 1.25, RetainedRootCap: 26},
}

var unreadBoard = map[string]bool{
	"1": true, "4": true, "10/2": true, "60/1": true, "60/3": true,
}

// BoardByPath exposes the canonical Erika-K board catalog to shared debug/
// observation tooling without duplicating the host program's private board tree.
func BoardByPath(path string) (world.Board, bool) {
	node, ok := findNode(strings.TrimSpace(path))
	if !ok || node.Hidden {
		return world.Board{}, false
	}
	return worldBoard(node), true
}

func worldBoard(node boardNode) world.Board {
	return world.Board{
		ID:              node.Path,
		Name:            node.Name,
		SemanticScope:   node.SemanticScope,
		ActivityWeight:  node.ActivityWeight,
		ReplyRate:       node.ReplyRate,
		RetainedRootCap: node.RetainedRootCap,
	}
}

type Runtime struct {
	Host      world.Host
	Store     world.Store
	state     string
	handle    string
	boardPath string
	threadID  int64
	subject   string
}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{Host: host, Store: store, state: "login_id", handle: "GUEST"}
}

func (r *Runtime) ObservationBoards() []world.Board {
	// CONNECT itself does not fan out materialization. Login and navigation start
	// narrowly-scoped predictive jobs; a board read blocks on its own job if needed.
	return nil
}

func (r *Runtime) beginBoardPrefetch(boards []world.Board) {
	if len(boards) == 0 {
		return
	}
	if prefetcher, ok := r.Store.(world.HostPrefetchStore); ok {
		prefetcher.BeginHostPrefetch(r.Host, boards)
		return
	}
	if observer, ok := r.Store.(world.HostObservationStore); ok {
		observer.BeginHostObservation(r.Host, boards)
	}
}

func (r *Runtime) prefetchLoginBoard() {
	// Keep speculative work intentionally tiny. Free-talk is a plausible first
	// destination, but choosing any other board simply waits on that board later.
	if node, ok := findNode("4"); ok {
		r.beginBoardPrefetch([]world.Board{worldBoard(node)})
	}
}

func (r *Runtime) prefetchFirstForumChild(path string) {
	for _, child := range visibleChildren(path) {
		if r.isForum(child.Path) {
			continue
		}
		r.beginBoardPrefetch([]world.Board{worldBoard(child)})
		return
	}
}

func (r *Runtime) cachedBoardPosts(path string) []world.Post {
	if _, ok := findNode(path); !ok {
		return nil
	}
	out := make([]world.Post, 0)
	for _, post := range r.Store.ListPosts(r.Host.ID) {
		if post.BoardID == path {
			out = append(out, post)
		}
	}
	return out
}

// observedBoardPosts returns only already committed canonical state. The board
// index itself applies a blocking WaitForBoardHeaders barrier when that state is
// not ready; article prose has its own later WaitForArticleBody barrier.
func (r *Runtime) observedBoardPosts(path string) []world.Post {
	return r.cachedBoardPosts(path)
}

func (r *Runtime) Welcome() string {
	return fmt.Sprintf("\x1b[2J\x1b[Hжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжж\r\n"+
		"                 %s\r\n"+
		"        %s / %d回線 / MAX %dbps\r\n"+
		"жжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжжж\r\n"+
		"◎ 会員以外の方は ID:GUEST でご利用下さい\r\n\r\nYOUR ID:", trimRunes(r.Host.Name, 34), r.Host.Region, r.Host.Lines, r.Host.MaxBaud)
}

func (r *Runtime) HandleLine(line string) (output string, disconnect bool) {
	line = strings.TrimSpace(line)

	switch r.state {
	case "login_id":
		if line == "" {
			line = "GUEST"
		}
		r.handle = trimRunes(line, 16)
		if strings.EqualFold(r.handle, "GUEST") {
			r.handle = "GUEST"
			return r.finishLogin(), false
		}
		r.state = "login_password"
		return "PASSWORD:", false

	case "login_password":
		// Password validation belongs to the future membership/account layer. For
		// now the prompt and transition are reproduced without storing a secret.
		return r.finishLogin(), false

	case "new_subject":
		if line == "" {
			r.state = "board"
			return "\r\n書き込みを中止しました。\r\n" + r.renderBoardIndex(), false
		}
		r.subject = trimRunes(line, 48)
		r.state = "new_body"
		return "本文を入力してください。\r\n終了は1行入力後 RETURN（prototype）\r\n> ", false

	case "new_body":
		if line == "" {
			r.state = "board"
			return "\r\n書き込みを中止しました。\r\n" + r.renderBoardIndex(), false
		}
		p := r.Store.AddPost(r.Host.ID, world.Post{
			BoardID:   r.boardPath,
			Author:    r.handle,
			Subject:   r.subject,
			Body:      line,
			CreatedAt: time.Now(),
		})
		r.threadID = p.ID
		r.state = "thread"
		return fmt.Sprintf("\r\nMSG No.%d を登録しました。\r\n", p.ID) + r.renderThread(p.ID), false

	case "append_body":
		if line == "" {
			r.state = "thread"
			return "\r\nアペを中止しました。\r\n" + r.renderThread(r.threadID), false
		}
		root, ok := r.rootPost(r.threadID)
		if !ok {
			r.state = "board"
			return "\r\nMSGが見つかりません。\r\n" + r.renderBoardIndex(), false
		}
		r.Store.AddPost(r.Host.ID, world.Post{
			BoardID:   root.BoardID,
			ParentID:  root.ID,
			Author:    r.handle,
			// Erika-K's append is represented by the parent/root relationship;
			// the append itself has no independently displayed subject.
			Subject:   "",
			Body:      line,
			CreatedAt: time.Now(),
		})
		r.state = "thread"
		return "\r\nアペンドしました。\r\n" + r.renderThread(r.threadID), false
	}

	switch r.state {
	case "main":
		return r.handleMain(line)
	case "board":
		return r.handleBoard(line)
	case "thread":
		return r.handleThread(line)
	case "file":
		return r.handleSimpleMenu(line, "file")
	case "mail":
		return r.handleSimpleMenu(line, "mail")
	case "chat":
		return r.handleSimpleMenu(line, "chat")
	case "junk":
		return r.handleSimpleMenu(line, "junk")
	case "mode":
		return r.handleSimpleMenu(line, "mode")
	default:
		r.state = "main"
		return r.renderMainMenu(), false
	}
}

func (r *Runtime) planBoardActivity() {
	planner, ok := r.Store.(world.BoardActivityStore)
	if !ok {
		return
	}
	for _, node := range boardTree {
		if r.isForum(node.Path) {
			continue
		}
		_, _ = planner.BoardActivity(r.Host, worldBoard(node))
	}
}

func (r *Runtime) finishLogin() string {
	r.state = "main"
	r.planBoardActivity()
	r.prefetchLoginBoard()
	last := "--/--/-- --:--"
	if r.handle != "GUEST" {
		last = "96/08/25 23:41"
	}
	return fmt.Sprintf("\r\n前回アクセス %s\r\n\r\n", last) +
		"######################## WELCOME TO HAKATA CANAL NET ########################\r\n" +
		"■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■\r\n" +
		"■  博多から、夜更かしネットワーカーのみなさんへ。                  ■\r\n" +
		"■  23:00以降は混み合います。長時間の席取りはほどほどに(^^;      ■\r\n" +
		"■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■\r\n" +
		"############################################################ ERIKA-K ####\r\n" +
		fmt.Sprintf("\r\n深夜のアクセスご苦労様！ %sさん、いらっしゃいませ。\r\n", r.handle) +
		r.renderMainMenu()
}

func (r *Runtime) handleMain(line string) (string, bool) {
	upper := strings.ToUpper(line)
	switch upper {
	case "", "GUIDE":
		return r.renderMainMenu(), false
	case "1", "BM":
		r.state = "board"
		r.boardPath = ""
		return r.renderBoardMenu(), false
	case "2", "FM":
		r.state = "file"
		return r.renderFileMenu(), false
	case "3", "MAIL", "MX":
		r.state = "mail"
		return r.renderMailMenu(), false
	case "4", "C", "CHAT":
		r.state = "chat"
		return r.renderChatMenu(), false
	case "5", "JUNK":
		r.state = "junk"
		return r.renderJunkMenu(), false
	case "6", "MODE":
		r.state = "mode"
		return r.renderModeMenu(), false
	case "7":
		return "\r\nSYSOP宛メール\r\n現在prototypeのため閲覧のみです。\r\n\r\nMAIN MENU --> ", false
	case "9", "BYE", "QUIT", "GOODBYE":
		return "\r\nご利用ありがとうございました。\r\nまた HAKATA CANAL NET でお会いしましょう。\r\n", true
	case "0":
		return "\r\n〖入会登録〗\r\nGUEST登録受付は現在準備中です。\r\n\r\nMAIN MENU --> ", false
	case "A":
		return r.renderAutoRun(), false
	case "ASET":
		return "\r\n〖自動運転登録〗 ASET\r\nBM/T -> MAIL -> FM/NEW の順で登録されています。\r\n（編集機能はprototypeでは未実装）\r\n\r\nMAIN MENU --> ", false
	case "MA":
		return r.renderBoardMap() + "\r\nMAIN MENU --> ", false
	case "T":
		return r.renderUnreadSummary() + "\r\nMAIN MENU --> ", false
	case "V":
		return fmt.Sprintf("\r\n〖アクセス記録〗\r\n96/08/26 00:20 NORI\r\n96/08/26 00:42 MARI\r\n96/08/26 01:07 SYSOP\r\nNOW             %s\r\n\r\nMAIN MENU --> ", r.handle), false
	case "H", "HELP", "?", "8":
		return r.renderCommandHelp() + "\r\nMAIN MENU --> ", false
	case "WHO":
		return fmt.Sprintf("\r\n〖使用状態表示〗\r\nLINE 1  SYSOP     14400\r\nLINE 2  MARI       9600\r\nLINE 3  %-10s ONLINE\r\n\r\nMAIN MENU --> ", r.handle), false
	case "MEMB":
		return "\r\n〖メンバーリスト〗\r\nSYSOP  MARI  KAZU  YUKI  TAKU  NORI  MIDNIGHT ...\r\n\r\nMAIN MENU --> ", false
	default:
		if strings.HasPrefix(upper, "BJ") {
			r.state = "board"
			if path, ok := parseBJ(line); ok && r.setBoardPath(path) {
				return r.renderBoardMenu(), false
			}
		}
		return "? COMMAND ERROR\r\nMAIN MENU [?]=HELP --> ", false
	}
}

func (r *Runtime) handleBoard(line string) (string, bool) {
	upper := strings.ToUpper(line)

	if line == "/" {
		r.state = "main"
		r.boardPath = ""
		return r.renderMainMenu(), false
	}
	if upper == "BYE" {
		return "\r\nご利用ありがとうございました。\r\n", true
	}
	if upper == "BM" {
		r.boardPath = ""
		return r.renderBoardMenu(), false
	}
	if upper == "H" || upper == "HELP" || line == "?" {
		return r.renderBoardHelp(), false
	}
	if strings.HasPrefix(upper, "BJ") {
		path, ok := parseBJ(line)
		if !ok || !r.setBoardPath(path) {
			return "? BOARD PATH ERROR\r\n" + r.boardPrompt(), false
		}
		return r.renderBoardMenu(), false
	}

	if r.boardPath == "" || r.isForum(r.boardPath) {
		if line == "" {
			if r.boardPath == "" {
				r.state = "main"
				return r.renderMainMenu(), false
			}
			r.boardPath = parentPath(r.boardPath)
			return r.renderBoardMenu(), false
		}
		if upper == "M" {
			return r.renderBoardMenu(), false
		}
		if upper == "T" || upper == "00" || upper == "0" {
			return r.renderUnreadSummary() + "\r\n" + r.boardPrompt(), false
		}
		if target, ok := r.childSelection(line); ok {
			r.boardPath = target
			return r.renderBoardMenu(), false
		}
		return "? BOARD No. ERROR\r\n" + r.boardPrompt(), false
	}

	// Leaf-board command mode.
	if line == "" {
		r.boardPath = parentPath(r.boardPath)
		return r.renderBoardMenu(), false
	}
	switch upper {
	case "M", "BX", "BXS":
		return r.renderBoardIndex(), false
	case "T", "00", "0":
		return r.renderBoardIndex(), false
	case "BW", "BWX", "W", "NEW":
		r.state = "new_subject"
		return "TITLE --> ", false
	}
	if strings.HasPrefix(upper, "BR ") {
		line = strings.TrimSpace(line[3:])
	}
	if id, err := strconv.ParseInt(line, 10, 64); err == nil {
		root, ok := r.rootPost(id)
		if !ok || root.BoardID != r.boardPath {
			return "そのMSGはありません。\r\n" + r.boardPrompt(), false
		}
		r.threadID = id
		r.state = "thread"
		return r.renderThread(id), false
	}
	return "? COMMAND ERROR\r\n" + r.boardPrompt(), false
}

func (r *Runtime) handleThread(line string) (string, bool) {
	upper := strings.ToUpper(line)
	if line == "/" {
		r.state = "main"
		r.boardPath = ""
		return r.renderMainMenu(), false
	}
	switch upper {
	case "", "BX", "BXS":
		r.state = "board"
		return r.renderBoardIndex(), false
	case "BR", "R":
		return r.renderThread(r.threadID), false
	case "A", "APE", "APPEND":
		r.state = "append_body"
		return "APE --> ", false
	case "BW", "BWX":
		r.state = "new_subject"
		return "TITLE --> ", false
	case "M":
		r.state = "board"
		return r.renderBoardIndex(), false
	case "H", "HELP", "?":
		return r.renderBoardHelp(), false
	case "BYE":
		return "\r\nご利用ありがとうございました。\r\n", true
	default:
		return "? COMMAND ERROR\r\n" + r.threadPrompt(), false
	}
}

func (r *Runtime) handleSimpleMenu(line, menu string) (string, bool) {
	upper := strings.ToUpper(line)
	if line == "/" || upper == "GUIDE" || upper == "M" {
		r.state = "main"
		return r.renderMainMenu(), false
	}
	if upper == "BYE" {
		return "\r\nご利用ありがとうございました。\r\n", true
	}
	if upper == "H" || upper == "HELP" || line == "?" {
		return r.renderCommandHelp() + "\r\n/ = MAIN MENU\r\n> ", false
	}
	switch menu {
	case "file":
		if upper == "FM" || line == "" {
			return r.renderFileMenu(), false
		}
		if upper == "FX" || upper == "FXS" {
			return "\r\n〖FILE INDEX〗\r\n  001 WTERM_MAC.LZH   48KB  WTERM巡回マクロ\r\n  002 MODEMFAQ.TXT    12KB  モデムFAQ\r\n  003 CANALMAP.LZH    31KB  局内ボードマップ\r\n\r\n(FM) FILE [M]=MENU [?]=HELP --> ", false
		}
		if upper == "NMODEM" || upper == "FR" {
			return "\r\nNMODEM READY\r\n※ バイナリ転送エンジンはprototypeでは未実装です。\r\n\r\n(FM) FILE [M]=MENU [?]=HELP --> ", false
		}
	case "mail":
		if upper == "MAIL" || upper == "MX" || line == "" {
			return r.renderMailMenu(), false
		}
		if upper == "MR" {
			return "\r\nMAIL No.1 FROM:SYSOP\r\nSUBJ: はじめまして\r\n局の使い方で判らないことがあれば7番からどうぞ。\r\n\r\n(MAIL) MAIL [M]=MENU [?]=HELP --> ", false
		}
	case "chat":
		if upper == "C" || upper == "CHAT" || line == "" {
			return r.renderChatMenu(), false
		}
		if upper == "WHO" {
			return fmt.Sprintf("\r\nLINE1 SYSOP  LINE2 MARI  LINE3 %s\r\n\r\n(C) CHAT [M]=MENU [?]=HELP --> ", r.handle), false
		}
		if strings.HasPrefix(upper, "CALL") {
			return "\r\n*** 電報を送信しました ***\r\n\r\n(C) CHAT [M]=MENU [?]=HELP --> ", false
		}
	case "junk":
		return r.renderJunkMenu(), false
	case "mode":
		return r.renderModeMenu(), false
	}
	return "? COMMAND ERROR\r\n/ = MAIN MENU  [?]=HELP --> ", false
}

func (r *Runtime) renderMainMenu() string {
	return "\r\n-HＡＫＡＴＡ ＣＡＮＡＬ ＮＥＴ-  〖Ｍain Ｍenu〗  絵理香Ｋ版\r\n" +
		"――――――――――――――――――――――――――――――――――――――\r\n" +
		"[1] ボード(BM)       [2] ファイル(FM)      [3] メール(MAIL)\r\n" +
		"[4] 電報･チャット(C)  [5] ジャンク(JUNK)    [6] 各種設定(MODE)\r\n" +
		"[7] SYSOP宛メール     [9] 接続終了(BYE)     [0] 入会登録\r\n" +
		"[A] 自動運転         [ASET] 自動運転登録    [MA] ボードマップ\r\n" +
		"[T] 未読検索         [V] アクセス記録      [H] その他のコマンド\r\n" +
		"――――――――――――――――――――――――――――――――――――――\r\n" +
		"MAIN MENU [?]=HELP --> "
}

func (r *Runtime) renderBoardMenu() string {
	if r.boardPath != "" && !r.isForum(r.boardPath) {
		return r.renderBoardIndex()
	}
	if r.boardPath == "" {
		return r.renderRootBoardMenu()
	}

	node, _ := findNode(r.boardPath)
	r.prefetchFirstForumChild(r.boardPath)
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n      〖%s〗        ★☆＝未読   〖Forum.OP〗SYSOP\r\n", node.Name)
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	children := visibleChildren(r.boardPath)
	for i := 0; i < len(children); i += 2 {
		left := r.formatBoardEntry(children[i])
		right := ""
		if i+1 < len(children) {
			right = r.formatBoardEntry(children[i+1])
		}
		fmt.Fprintf(&b, "%s%s\r\n", padRunes(left, 39), right)
	}
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	b.WriteString("[T/00/0]新MSG [ﾘﾀｰﾝ] 前の階へ [/] ﾒｲﾝﾒﾆｭｰへ [H] ｺﾏﾝﾄﾞ一覧 [?] 説明\r\n")
	b.WriteString(r.boardPrompt())
	return b.String()
}

func (r *Runtime) renderRootBoardMenu() string {
	var b strings.Builder
	b.WriteString("\r\n〖ボード／フォーラムメニュー〗 BM   ★☆＝未読   GUEST:一部利用可\r\n")
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	items := visibleChildren("")
	for i := 0; i < len(items); i += 2 {
		left := r.formatBoardEntry(items[i])
		right := ""
		if i+1 < len(items) {
			right = r.formatBoardEntry(items[i+1])
		}
		fmt.Fprintf(&b, "%s%s\r\n", padRunes(left, 39), right)
	}
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	b.WriteString("[T/00/0]新MSG [ﾘﾀｰﾝ] 前の階へ [/] ﾒｲﾝﾒﾆｭｰへ [H] ｺﾏﾝﾄﾞ一覧 [?] 説明\r\n")
	b.WriteString(r.boardPrompt())
	return b.String()
}

func (r *Runtime) renderBoardIndex() string {
	node, ok := findNode(r.boardPath)
	if !ok || r.isForum(r.boardPath) {
		return r.renderBoardMenu()
	}
	board := worldBoard(node)
	posts := r.observedBoardPosts(r.boardPath)
	if observer, ok := r.Store.(world.HostObservationStore); ok {
		// Predictive work may already be running from login/forum navigation. If
		// this exact board is not ready, join/start only its shared job and wait;
		// never expose an empty placeholder that requires the user to refresh.
		//
		// Title/header materialization can depend on external model/research
		// services. Repository observation deliberately forgets a failed lease so
		// a later read can retry. Do that retry here once for an explicit Erika-K
		// board read instead of immediately leaking a transient backend failure as
		// a host-program error. Persistent failures remain visible after attempt 2.
		for attempt := 0; attempt < 2; attempt++ {
			observer.BeginHostObservation(r.Host, []world.Board{board})
			ready, err := observer.WaitForBoardHeaders(context.Background(), r.Host, board)
			if err == nil {
				posts = ready
				break
			}
			if attempt == 1 {
				return "\r\n? BOARD READ ERROR\r\n" + r.boardPrompt()
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n〖%s〗  ★☆＝未読  〖Board.OP〗SYSOP\r\n", node.Name)
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	found := false
	for _, p := range posts {
		if p.ParentID != 0 {
			continue
		}
		found = true
		mark := "☆"
		if unreadBoard[r.boardPath] {
			mark = "★"
		}
		fmt.Fprintf(&b, "%s[%04d] %-10s %-28s APE:%d\r\n", mark, p.ID, trimRunes(p.Author, 10), trimRunes(p.Subject, 28), r.appendCountFrom(posts, p.ID))
	}
	if !found {
		b.WriteString("              --- MSG はありません ---\r\n")
	}
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	b.WriteString("[BX]一覧 [BR n]読む [BW]書く [ﾘﾀｰﾝ]前の階へ [/]MAIN [H]HELP\r\n")
	b.WriteString(r.boardPrompt())
	return b.String()
}

func (r *Runtime) renderThread(id int64) string {
	posts := r.observedBoardPosts(r.boardPath)
	var root world.Post
	found := false
	for _, post := range posts {
		if post.ID == id && post.ParentID == 0 {
			root = post
			found = true
			break
		}
	}
	if !found {
		return "\r\nMSGが見つかりません。\r\n" + r.threadPrompt()
	}
	boardNode, _ := findNode(r.boardPath)
	board := worldBoard(boardNode)
	if strings.TrimSpace(root.Body) == "" {
		if observer, ok := r.Store.(world.HostObservationStore); ok {
			if rendered, ok, err := observer.WaitForArticleBody(context.Background(), r.Host, board, root.ID); err == nil && ok {
				root = rendered
			}
		}
	}

	var b strings.Builder
	b.WriteString("\r\n========================================================================\r\n")
	fmt.Fprintf(&b, "MSG:%d  FROM:%s  DATE:%s\r\n", root.ID, root.Author, root.CreatedAt.Format("96/01/02 15:04"))
	fmt.Fprintf(&b, "SUBJ:%s\r\n", root.Subject)
	b.WriteString("------------------------------------------------------------------------\r\n")
	b.WriteString(normalizeNewlines(root.Body))
	b.WriteString("\r\n")

	appendNo := 0
	for _, p := range posts {
		if p.ParentID != root.ID {
			continue
		}
		if strings.TrimSpace(p.Body) == "" {
			if observer, ok := r.Store.(world.HostObservationStore); ok {
				if rendered, ok, err := observer.WaitForArticleBody(context.Background(), r.Host, board, p.ID); err == nil && ok {
					p = rendered
				}
			}
		}
		appendNo++
		fmt.Fprintf(&b, "\r\n--------------------------- アペ %d ---------------------------\r\n", appendNo)
		fmt.Fprintf(&b, "FROM:%s  DATE:%s\r\n", p.Author, p.CreatedAt.Format("96/01/02 15:04"))
		b.WriteString(normalizeNewlines(p.Body))
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "\r\n------------------------- APE:%d -------------------------------\r\n", appendNo)
	b.WriteString("[A]アペ [RETURN]一覧 [/]MAIN [H]HELP\r\n")
	b.WriteString(r.threadPrompt())
	return b.String()
}

func (r *Runtime) renderCommandHelp() string {
	return "\r\n《コマンド・モード》\r\n" +
		"GUIDE と入力すると、メニュー方式に戻ります。\r\n" +
		"HELP と入力すると コマンド の簡単な説明が出てきます。\r\n" +
		"==== 次のコマンドが使用出来ます ====\r\n" +
		"1. ボード\r\n" +
		"   メニュー [BM] インデックス [BX | BXS] 読む [BR]\r\n" +
		"   書く [BW | BWX] 階層移動 [BJ]\r\n" +
		"2. ファイル\r\n" +
		"   メニュー [FM] インデックス [FX | FXS] 読む [FR]\r\n" +
		"   書く [FW | FWX] 階層移動 [FJ]\r\n" +
		"3. メール  インデックス [MX] 読む [MR] 書く [MW]\r\n" +
		"4. チャット 入る [CHAT] 呼ぶ [CALL]\r\n" +
		"5. 使用状態表示 [WHO]   6. メンバーリスト [MEMB]\r\n" +
		"7. パスワード変更 [PASS] 8. 設定変更 [MODE]\r\n" +
		"9. メニューモード [GUIDE] 10. 終了 [BYE]\r\n"
}

func (r *Runtime) renderBoardHelp() string {
	return "\r\n〖BOARD COMMAND〗\r\nBM       ボード／フォーラムメニュー\r\nBJ n     階層移動   例: BJ 60 / BJ\\80\\2\r\nBX/BXS   MSGインデックス\r\nBR n     MSGを読む\r\nBW/BWX   新規MSGを書く\r\nA        現在のMSGへアペ（HAKATA局ショートカット）\r\nRETURN   前の階へ\r\n/        MAIN MENU\r\n\r\n" + r.boardPrompt()
}

func (r *Runtime) renderFileMenu() string {
	return "\r\n〖Ｆile Ｍenu〗 FM\r\n" +
		"――――――――――――――――――――――――――――――――――――――\r\n" +
		"[FX] INDEX   [FR] READ/DOWNLOAD   [FW] UPLOAD\r\n" +
		"転送プロトコル: XMODEM / NMODEM\r\n" +
		"[RETURN] このメニュー   [/] MAIN MENU   [H] HELP\r\n" +
		"(FM) FILE [M]=MENU [?]=HELP --> "
}

func (r *Runtime) renderMailMenu() string {
	return "\r\n〖Ｍail Ｍenu〗 MAIL\r\n" +
		"――――――――――――――――――――――――――――――――――――――\r\n" +
		"★ 1  SYSOP     はじめまして\r\n" +
		"☆ 2  MARI      オフの件です(^^)\r\n" +
		"[MX] INDEX [MR] READ [MW] WRITE [MKILL] DELETE\r\n" +
		"[/] MAIN MENU\r\n" +
		"(MAIL) MAIL [M]=MENU [?]=HELP --> "
}

func (r *Runtime) renderChatMenu() string {
	return "\r\n〖電報・チャット〗 C\r\n" +
		"CHAT        チャットへ入る\r\n" +
		"CALL <ID>   電報を送る\r\n" +
		"WHO         現在の使用状態\r\n" +
		"[/] MAIN MENU\r\n" +
		"(C) CHAT [M]=MENU [?]=HELP --> "
}

func (r *Runtime) renderJunkMenu() string {
	return "\r\n〖ＪＵＮＫ〗\r\n" +
		"[1] 今日の運勢（prototype）\r\n" +
		"[2] SYSOPのひとりごと\r\n" +
		"[3] アクセスランキング\r\n" +
		"[/] MAIN MENU\r\n" +
		"JUNK --> "
}

func (r *Runtime) renderModeMenu() string {
	return "\r\n〖各種設定〗 MODE\r\n" +
		"KANJI CODE : SHIFT-JIS\r\n" +
		"PAGE LENGTH: 24\r\n" +
		"ECHO       : ON\r\n" +
		"AUTO READ  : OFF\r\n" +
		"[/] MAIN MENU\r\n" +
		"MODE --> "
}

func (r *Runtime) renderAutoRun() string {
	return "\r\n〖自動運転〗 A\r\n" +
		"未読ボード → 新着メール → 新着ファイル を巡回します。\r\n" +
		"prototypeでは巡回結果のみ表示します。\r\n\r\n" +
		"BM: 5 unread threads / MAIL: 1 unread / FILE: 1 new\r\n"
}

func (r *Runtime) renderBoardMap() string {
	var b strings.Builder
	b.WriteString("\r\n〖ボードマップ〗 MA\r\n")
	for _, node := range boardTree {
		if node.Hidden {
			continue
		}
		indent := strings.Count(node.Path, "/")
		fmt.Fprintf(&b, "%s%-6s %s\r\n", strings.Repeat("  ", indent), strings.ReplaceAll(node.Path, "/", "\\"), node.Name)
	}
	return b.String()
}

func (r *Runtime) renderUnreadSummary() string {
	var b strings.Builder
	b.WriteString("\r\n〖未読検索〗 T\r\n")
	for _, node := range boardTree {
		if !unreadBoard[node.Path] || node.Hidden || r.isForum(node.Path) {
			continue
		}
		fmt.Fprintf(&b, "★ %-6s %s  %d MSG\r\n", strings.ReplaceAll(node.Path, "/", "\\"), node.Name, r.rootCount(node.Path))
	}
	return b.String()
}

func (r *Runtime) formatBoardEntry(node boardNode) string {
	mark := " "
	if unreadBoard[node.Path] {
		mark = "★"
	}
	entry := fmt.Sprintf("%s[%s] %s", mark, node.Key, node.Name)
	if !r.isForum(node.Path) {
		entry += fmt.Sprintf(" %d", r.rootCount(node.Path))
	}
	return entry
}

func (r *Runtime) boardPrompt() string {
	path := strings.ReplaceAll(r.boardPath, "/", "\\")
	if path != "" {
		path = "\\" + path
	}
	return fmt.Sprintf("(BJ%s) BOARD [M]=MENU [?]=HELP --> ", path)
}

func (r *Runtime) threadPrompt() string {
	path := strings.ReplaceAll(r.boardPath, "/", "\\")
	return fmt.Sprintf("(BR\\%s\\%d) BOARD [M]=MENU [?]=HELP --> ", path, r.threadID)
}

func (r *Runtime) setBoardPath(path string) bool {
	path = normalizePath(path)
	if path == "" {
		r.boardPath = ""
		return true
	}
	if _, ok := findNode(path); !ok {
		return false
	}
	r.boardPath = path
	return true
}

func (r *Runtime) childSelection(key string) (string, bool) {
	for _, node := range boardTree {
		if node.Parent == r.boardPath && node.Key == strings.TrimSpace(key) {
			return node.Path, true
		}
	}
	return "", false
}

func (r *Runtime) isForum(path string) bool {
	for _, node := range boardTree {
		if node.Parent == path {
			return true
		}
	}
	return false
}

func (r *Runtime) rootCount(path string) int {
	count := 0
	for _, p := range r.observedBoardPosts(path) {
		if p.ParentID == 0 {
			count++
		}
	}
	if node, ok := findNode(path); ok && !r.isForum(path) {
		if planner, ok := r.Store.(world.BoardActivityStore); ok {
			if state, found := planner.BoardActivity(r.Host, worldBoard(node)); found && state.RetainedRoots > count {
				return state.RetainedRoots
			}
		}
	}
	return count
}

func (r *Runtime) appendCount(rootID int64) int {
	return r.appendCountFrom(r.Store.ListPosts(r.Host.ID), rootID)
}

func (r *Runtime) appendCountFrom(posts []world.Post, rootID int64) int {
	count := 0
	for _, p := range posts {
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

func visibleChildren(parent string) []boardNode {
	out := make([]boardNode, 0)
	for _, node := range boardTree {
		if node.Parent == parent && !node.Hidden {
			out = append(out, node)
		}
	}
	return out
}

func findNode(path string) (boardNode, bool) {
	path = normalizePath(path)
	for _, node := range boardTree {
		if node.Path == path {
			return node, true
		}
	}
	return boardNode{}, false
}

func parseBJ(input string) (string, bool) {
	s := strings.TrimSpace(input)
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "BJ") {
		return "", false
	}
	s = strings.TrimSpace(s[2:])
	s = strings.TrimLeft(s, "\\/ ")
	return normalizePath(s), true
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.Trim(path, "/")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return path
}

func parentPath(path string) string {
	path = normalizePath(path)
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return ""
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

func padRunes(s string, width int) string {
	if n := len([]rune(s)); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
