package erikak

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type boardNode struct {
	Path   string
	Key    string
	Parent string
	Name   string
	Hidden bool
}

var boardTree = []boardNode{
	{Path: "1", Key: "1", Name: "事務局からのお知らせ"},
	{Path: "2", Key: "2", Name: "自己紹介・新人歓迎"},
	{Path: "3", Key: "3", Name: "Ｑ＆Ａ（質問ボード）"},
	{Path: "4", Key: "4", Name: "ふり～と～く"},
	{Path: "5", Key: "5", Name: "オフライントピックス"},
	{Path: "6", Key: "6", Name: "街角情報スポット"},
	{Path: "7", Key: "7", Name: "ＣＡＮＡＬ市場"},
	{Path: "8", Key: "8", Name: "夢工房はかた"},
	{Path: "10", Key: "10", Name: "博多・天神広場"},
	{Path: "20", Key: "20", Name: "アミューズメントフォーラム"},
	{Path: "60", Key: "60", Name: "コンピュータワールド"},
	{Path: "68", Key: "68", Name: "ＣＡＮＡＬ Ｘ村"},
	{Path: "70", Key: "70", Name: "９８ VS ＡＴ互換機"},
	{Path: "80", Key: "80", Name: "その他のコンピュータ"},
	{Path: "99", Key: "99", Name: "夜更かし部屋", Hidden: true},

	{Path: "10/1", Key: "1", Parent: "10", Name: "博多・天神ローカル"},
	{Path: "10/2", Key: "2", Parent: "10", Name: "オフ会連絡"},
	{Path: "20/1", Key: "1", Parent: "20", Name: "ＧＡＭＥ"},
	{Path: "20/2", Key: "2", Parent: "20", Name: "ＡＮＩＭＥ／ＭＡＮＧＡ"},
	{Path: "60/1", Key: "1", Parent: "60", Name: "ＰＣ－９８／ＭＯＤＥＭ"},
	{Path: "60/2", Key: "2", Parent: "60", Name: "Ｗｉｎｄｏｗｓ／ＤＯＳ"},
	{Path: "60/3", Key: "3", Parent: "60", Name: "ＳＯＦＴＷＡＲＥ／ＤＡＴＡ"},
	{Path: "68/1", Key: "1", Parent: "68", Name: "深夜雑談"},
	{Path: "70/1", Key: "1", Parent: "70", Name: "ＰＣ－９８"},
	{Path: "70/2", Key: "2", Parent: "70", Name: "ＤＯＳ／Ｖ"},
	{Path: "80/1", Key: "1", Parent: "80", Name: "ＦＭ－ＴＯＷＮＳ"},
	{Path: "80/2", Key: "2", Parent: "80", Name: "Forever with MSX"},
	{Path: "80/3", Key: "3", Parent: "80", Name: "ワープロ"},
	{Path: "80/4", Key: "4", Parent: "80", Name: "その他(PC88,FMR,etc)"},
}

var unreadBoard = map[string]bool{
	"1": true, "4": true, "10/2": true, "60/1": true, "60/3": true,
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
			Subject:   "Re: " + root.Subject,
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

func (r *Runtime) finishLogin() string {
	r.state = "main"
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
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n〖%s〗  ★☆＝未読  〖Board.OP〗SYSOP\r\n", node.Name)
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	found := false
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.BoardID != r.boardPath || p.ParentID != 0 {
			continue
		}
		found = true
		mark := "☆"
		if unreadBoard[r.boardPath] {
			mark = "★"
		}
		fmt.Fprintf(&b, "%s[%04d] %-10s %-28s APE:%d\r\n", mark, p.ID, trimRunes(p.Author, 10), trimRunes(p.Subject, 28), r.appendCount(p.ID))
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
	root, ok := r.rootPost(id)
	if !ok {
		return "\r\nMSGが見つかりません。\r\n" + r.threadPrompt()
	}
	var b strings.Builder
	b.WriteString("\r\n========================================================================\r\n")
	fmt.Fprintf(&b, "MSG:%d  FROM:%s  DATE:%s\r\n", root.ID, root.Author, root.CreatedAt.Format("96/01/02 15:04"))
	fmt.Fprintf(&b, "SUBJ:%s\r\n", root.Subject)
	b.WriteString("------------------------------------------------------------------------\r\n")
	b.WriteString(normalizeNewlines(root.Body))
	b.WriteString("\r\n")

	appendNo := 0
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.ParentID != root.ID {
			continue
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
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.BoardID == path && p.ParentID == 0 {
			count++
		}
	}
	return count
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
