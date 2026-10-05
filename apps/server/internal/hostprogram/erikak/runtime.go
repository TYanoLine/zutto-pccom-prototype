package erikak

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

type Runtime struct {
	Host      world.Host
	Store     world.Store
	Config    Config
	state     string
	handle    string
	boardPath string
	threadID  int64
	subject   string
	detail    hostcatalog.ErikaKDetail
	catalog   boardCatalog
}

func New(host world.Host, store world.Store) *Runtime {
	return NewWithConfig(host, store, DefaultConfig())
}

func NewWithConfig(host world.Host, store world.Store, cfg Config) *Runtime {
	d := detailFor(host, store)
	return &Runtime{Host: host, Store: store, Config: cfg, state: "login_id", handle: "GUEST", detail: d, catalog: boardCatalogFromDetail(&d)}
}

func (r *Runtime) ObservationBoards() []world.Board {
	// CONNECT, login and forum navigation do not materialize article headers.
	// An explicit board-index read starts only that board's shared job.
	return nil
}

func (r *Runtime) cachedBoardPosts(path string) []world.Post {
	if _, ok := r.findNode(path); !ok {
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
	return fmt.Sprintf("\x1b[2J\x1b[H%s\r\n"+
		"                 %s\r\n"+
		"        %s / %d回線 / MAX %dbps\r\n"+
		"%s\r\n"+
		"◎ 会員以外の方は ID:GUEST でご利用下さい\r\n\r\nYOUR ID:",
		doubleCellRule("＊", 80), trimRunes(r.Host.Name, 34), r.Host.Region, r.Host.Lines, r.Host.MaxBaud, doubleCellRule("＊", 80))
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

	case "append_target":
		if line == "" {
			r.state = "board"
			return "\r\nアペを中止しました。\r\n" + r.renderBoardIndex(), false
		}
		id, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
		if err != nil {
			return "番号を数字で入力してください。\r\nアペンド対象MSG番号 --> ", false
		}
		root, ok := r.rootPost(id)
		if !ok || root.BoardID != r.boardPath {
			return "そのMSGはありません。\r\nアペンド対象MSG番号 --> ", false
		}
		r.threadID = id
		r.state = "append_body"
		return fmt.Sprintf("MSG No.%d へアペンドします。\r\nAPE --> ", id), false

	case "append_body":
		if line == "" {
			r.state = "board"
			return "\r\nアペを中止しました。\r\n" + r.renderBoardIndex(), false
		}
		root, ok := r.rootPost(r.threadID)
		if !ok {
			r.state = "board"
			return "\r\nMSGが見つかりません。\r\n" + r.renderBoardIndex(), false
		}
		r.Store.AddPost(r.Host.ID, world.Post{
			BoardID:  root.BoardID,
			ParentID: root.ID,
			Author:   r.handle,
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
		return r.handleFile(line)
	case "file_protocol":
		return r.handleFileProtocol(line)
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
	for _, node := range r.catalog.Boards {
		if r.isForum(node.Path) {
			continue
		}
		_, _ = planner.BoardActivity(r.Host, worldBoard(node))
	}
}

func (r *Runtime) finishLogin() string {
	r.state = "main"
	r.planBoardActivity()
	// Header generation is on-demand only; login never prefetches a board.
	last := "--/--/-- --:--"
	if r.handle != "GUEST" {
		last = "96/08/25 23:41"
	}
	out := fmt.Sprintf("\r\n前回アクセス %s\r\n\r\n", last)
	for _, line := range r.catalog.Texts.LoginBanner {
		out += strings.ReplaceAll(line, "{handle}", r.handle) + "\r\n"
	}
	if r.catalog.Texts.LoginGreeting != "" {
		out += "\r\n" + strings.ReplaceAll(r.catalog.Texts.LoginGreeting, "{handle}", r.handle) + "\r\n"
	}
	return out + r.renderMainMenu()
}

func (r *Runtime) handleMain(line string) (string, bool) {
	upper := strings.ToUpper(line)
	switch upper {
	case "", "GUIDE":
		return r.renderMainMenu(), false
	case "1", "BM":
		if !r.canUseFeature(FeatureBoard) {
			return r.featureUnavailable()
		}
		r.state = "board"
		r.boardPath = ""
		return r.renderBoardMenu(), false
	case "2", "FM":
		if !r.canUseFeature(FeatureFile) {
			return r.featureUnavailable()
		}
		r.state = "file"
		return r.renderFileMenu(), false
	case "3", "MAIL", "MX":
		if !r.canUseFeature(FeatureMail) {
			return r.featureUnavailable()
		}
		r.state = "mail"
		return r.renderMailMenu(), false
	case "4", "C", "CHAT":
		if !r.canUseFeature(FeatureTelegramChat) {
			return r.featureUnavailable()
		}
		r.state = "chat"
		return r.renderChatMenu(), false
	case "5", "JUNK":
		if !r.canUseFeature(FeatureJunk) {
			return r.featureUnavailable()
		}
		r.state = "junk"
		return r.renderJunkMenu(), false
	case "6", "MODE":
		if !r.canUseFeature(FeatureSettings) {
			return r.featureUnavailable()
		}
		r.state = "mode"
		return r.renderModeMenu(), false
	case "7":
		if !r.canUseFeature(FeatureSysopMail) {
			return r.featureUnavailable()
		}
		return "\r\nSYSOP宛メール\r\n現在prototypeのため閲覧のみです。\r\n\r\nMAIN MENU --> ", false
	case "9", "BYE", "QUIT", "GOODBYE":
		out := "\r\nご利用ありがとうございました。\r\n"
		if r.catalog.Texts.Goodbye != "" {
			out += r.catalog.Texts.Goodbye + "\r\n"
		}
		return out, true
	case "0":
		if !r.canUseFeature(FeatureEnrollment) {
			return r.featureUnavailable()
		}
		return "\r\n〖入会登録〗\r\nGUEST登録受付は現在準備中です。\r\n\r\nMAIN MENU --> ", false
	case "A":
		if !r.canUseFeature(FeatureAutoRun) {
			return r.featureUnavailable()
		}
		return r.renderAutoRun(), false
	case "ASET":
		if !r.canUseFeature(FeatureAutoRun) {
			return r.featureUnavailable()
		}
		return "\r\n〖自動運転登録〗 ASET\r\nBM/T -> MAIL -> FM/NEW の順で登録されています。\r\n（編集機能はprototypeでは未実装）\r\n\r\nMAIN MENU --> ", false
	case "BAT":
		if !r.canUseFeature(FeatureBatchDownload) {
			return r.featureUnavailable()
		}
		return "\r\n〖バッチダウン〗 BAT\r\n登録済みファイルをまとめて転送します。\r\n（バッチキュー実装はprototypeでは未実装）\r\n\r\nMAIN MENU --> ", false
	case "MA":
		if !r.canUseFeature(FeatureBoardMap) {
			return r.featureUnavailable()
		}
		return r.renderBoardMap() + "\r\nMAIN MENU --> ", false
	case "T":
		if !r.canUseFeature(FeatureUnreadSearch) {
			return r.featureUnavailable()
		}
		return r.renderUnreadSummary() + "\r\nMAIN MENU --> ", false
	case "V":
		if !r.canUseFeature(FeatureAccessLog) {
			return r.featureUnavailable()
		}
		return fmt.Sprintf("\r\n〖アクセス記録〗\r\n96/08/26 00:20 NORI\r\n96/08/26 00:42 MARI\r\n96/08/26 01:07 SYSOP\r\nNOW             %s\r\n\r\nMAIN MENU --> ", r.handle), false
	case "H", "HELP", "?", "8":
		return r.renderCommandHelp() + "\r\nMAIN MENU --> ", false
	case "WHO":
		if !r.canUseFeature(FeatureTelegramChat) {
			return r.featureUnavailable()
		}
		return fmt.Sprintf("\r\n〖使用状態表示〗\r\nLINE 1  SYSOP     14400\r\nLINE 2  MARI       9600\r\nLINE 3  %-10s ONLINE\r\n\r\nMAIN MENU --> ", r.handle), false
	case "MEMB":
		if !r.canUseFeature(FeatureMemberList) {
			return r.featureUnavailable()
		}
		return "\r\n〖メンバーリスト〗\r\nSYSOP  MARI  KAZU  YUKI  TAKU  NORI  MIDNIGHT ...\r\n\r\nMAIN MENU --> ", false
	default:
		if strings.HasPrefix(upper, "BJ") {
			if !r.canUseFeature(FeatureBoard) {
				return r.featureUnavailable()
			}
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
		if line == "" || line == "." {
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
	if line == "" || line == "." {
		r.boardPath = parentPath(r.boardPath)
		return r.renderBoardMenu(), false
	}
	switch upper {
	case "M", "BX", "BXS":
		return r.renderBoardIndex(), false
	case "T", "00", "0":
		return r.renderBoardIndex(), false
	case "BW", "BWX", "W", "NEW":
		if board, ok := r.boardByPath(r.boardPath); ok && board.RootAuthorPolicy == "sysop_only" {
			return "この掲示板への新規投稿は事務局のみです。\r\n" + r.boardPrompt(), false
		}
		r.state = "new_subject"
		return "TITLE --> ", false
	case "A", "APE", "APPEND":
		r.state = "append_target"
		return "アペンド対象MSG番号 --> ", false
	}
	if strings.HasPrefix(upper, "A ") {
		idStr := strings.TrimSpace(line[2:])
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			root, ok := r.rootPost(id)
			if ok && root.BoardID == r.boardPath {
				r.threadID = id
				r.state = "append_body"
				return fmt.Sprintf("MSG No.%d へアペンドします。\r\nAPE --> ", id), false
			}
		}
		return "そのMSGはありません。\r\nアペンド対象MSG番号 --> ", false
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
	case "", ".", "BX", "BXS":
		r.state = "board"
		return r.renderBoardIndex(), false
	case "BR", "R":
		return r.renderThread(r.threadID), false
	case "A", "APE", "APPEND":
		r.state = "append_body"
		return "APE --> ", false
	case "BW", "BWX":
		if board, ok := r.boardByPath(r.boardPath); ok && board.RootAuthorPolicy == "sysop_only" {
			return "この掲示板への新規投稿は事務局のみです。\r\n" + r.threadPrompt(), false
		}
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

func (r *Runtime) handleFile(line string) (string, bool) {
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
	if upper == "FM" || line == "" {
		return r.renderFileMenu(), false
	}
	if upper == "FX" || upper == "FXS" {
		return "\r\n〖FILE INDEX〗\r\n  001 WTERM_MAC.LZH   48KB  WTERM巡回マクロ\r\n  002 MODEMFAQ.TXT    12KB  モデムFAQ\r\n  003 CANALMAP.LZH    31KB  局内ボードマップ\r\n\r\n(FM) FILE [M]=MENU [?]=HELP --> ", false
	}
	if upper == "FR" {
		r.state = "file_protocol"
		return r.renderTransferProtocolMenu(), false
	}
	if upper == "BAT" {
		if !r.canUseFeature(FeatureBatchDownload) {
			return r.protocolUnavailable() + r.filePrompt(), false
		}
		return "\r\n〖バッチダウン〗 BAT\r\n登録済みファイルをまとめて転送します。\r\n（バッチキュー実装はprototypeでは未実装）\r\n\r\n" + r.filePrompt(), false
	}
	if p, ok := findTransferProtocol(line); ok {
		if !r.Config.ProtocolEnabled(p.ID) {
			return r.protocolUnavailable() + r.filePrompt(), false
		}
		return r.beginTransfer(p), false
	}
	return "? COMMAND ERROR\r\n" + r.filePrompt(), false
}

func (r *Runtime) handleFileProtocol(line string) (string, bool) {
	if line == "" || line == "/" || strings.EqualFold(line, "M") {
		r.state = "file"
		return r.renderFileMenu(), false
	}
	p, ok := findTransferProtocol(line)
	if !ok {
		return "? PROTOCOL ERROR\r\n" + r.renderTransferProtocolMenu(), false
	}
	if !r.Config.ProtocolEnabled(p.ID) {
		return r.protocolUnavailable() + r.renderTransferProtocolMenu(), false
	}
	r.state = "file"
	return r.beginTransfer(p), false
}

func (r *Runtime) beginTransfer(p TransferProtocol) string {
	return fmt.Sprintf("\r\n%s READY\r\n※ 転送エンジンはprototypeでは未実装です。\r\n\r\n%s", p.Label, r.filePrompt())
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
	const separator = "――――――――――――――――――――――――――――――――――――――――"
	item := func(feature, label string) string {
		if !r.canUseFeature(feature) {
			return ""
		}
		return label
	}
	title := ""
	if r.catalog.Texts.MainMenuTitle != "" {
		title = r.catalog.Texts.MainMenuTitle + "\r\n"
	}
	return "\r\n" + title +
		separator + "\r\n" +
		menuColumns(item(FeatureBoard, "[1] ボード(BM)"), item(FeatureFile, "[2] ファイル(FM)"), item(FeatureMail, "[3] メール(MAIL)")) + "\r\n" +
		menuColumns(item(FeatureTelegramChat, "[4] 電報･チャット(C)"), item(FeatureJunk, "[5] ジャンク(JUNK)"), item(FeatureSettings, "[6] 各種設定(MODE)")) + "\r\n" +
		menuColumns(item(FeatureSysopMail, "[7] SYSOP宛メール"), "[9] 接続終了(BYE)", item(FeatureEnrollment, "[0] 入会登録")) + "\r\n" +
		menuColumns(item(FeatureAutoRun, "[A] 自動運転"), item(FeatureAutoRun, "[ASET] 自動運転登録"), item(FeatureBoardMap, "[MA] ボードマップ")) + "\r\n" +
		menuColumns(item(FeatureBatchDownload, "[BAT] バッチダウン"), "", "") + "\r\n" +
		menuColumns(item(FeatureUnreadSearch, "[T] 未読検索"), item(FeatureAccessLog, "[V] アクセス記録"), "[H] その他のコマンド") + "\r\n" +
		separator + "\r\n" +
		"MAIN MENU [?]=HELP --> "
}

func (r *Runtime) renderBoardMenu() string {
	if r.boardPath != "" && !r.isForum(r.boardPath) {
		return r.renderBoardIndex()
	}
	if r.boardPath == "" {
		return r.renderRootBoardMenu()
	}

	node, _ := r.findNode(r.boardPath)
	// Forum navigation reads prose-free board metadata only.
	var b strings.Builder
	fmt.Fprintf(&b, "\r\n      〖%s〗        ★☆＝未読   〖Forum.OP〗SYSOP\r\n", node.Name)
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	children := r.visibleChildren(r.boardPath)
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
	items := r.visibleChildren("")
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
	node, ok := r.findNode(r.boardPath)
	if !ok || r.isForum(r.boardPath) {
		return r.renderBoardMenu()
	}
	board := worldBoard(node)
	posts := r.observedBoardPosts(r.boardPath)
	if observer, ok := r.Store.(world.HostObservationStore); ok && len(posts) == 0 {
		// A leaf board that already has visible canonical headers is ready.
		// Do not start a fresh catch-up merely by returning from an article:
		// that would waste provider capacity on unrelated pending board reads.
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
	mark := "☆"
	if node.Unread {
		mark = "★"
	}
	keyInt, err := strconv.Atoi(node.Key)
	if err != nil {
		keyInt = 1
	}
	fmt.Fprintf(&b, "\r\n%sBD# %02d %s\r\n", mark, keyInt, node.Name)
	b.WriteString("# 最新10インデックス表示\r\n")
	b.WriteString("___No. __date__ time_ _author_  ap/ref___________i n d e x_______________\r\n")

	var roots []world.Post
	for _, p := range posts {
		if p.ParentID == 0 {
			roots = append(roots, p)
		}
	}

	if len(roots) == 0 {
		b.WriteString("              --- MSG はありません ---\r\n")
	} else {
		// Show latest posts (up to 10) in reverse chronological order
		start := len(roots) - 10
		if start < 0 {
			start = 0
		}
		for i := len(roots) - 1; i >= start; i-- {
			p := roots[i]
			ap := r.appendCountFrom(posts, p.ID)
			apStr := "  "
			if ap > 0 {
				apStr = fmt.Sprintf("%2d", ap)
			}
			dateStr := p.CreatedAt.Format("06/01/02 15:04")
			author := padRunes(trimRunes(p.Author, 8), 8)
			subj := trimRunes(p.Subject, 38)
			fmt.Fprintf(&b, "%02d %4d %s %s %s %s\r\n", keyInt, p.ID, dateStr, author, apStr, subj)
		}
	}
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	b.WriteString("[BX]一覧 [BR n]読む [BW/W]書く [A]アペ [0/T]未読 [ﾘﾀｰﾝ]前の階へ [/]MAIN [H]HELP\r\n")
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
	boardNode, _ := r.findNode(r.boardPath)
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
		loadFailed := false
		if strings.TrimSpace(p.Body) == "" {
			if observer, ok := r.Store.(world.HostObservationStore); ok {
				if rendered, ok, err := observer.WaitForArticleBody(context.Background(), r.Host, board, p.ID); err == nil && ok && strings.TrimSpace(rendered.Body) != "" {
					p = rendered
				} else {
					loadFailed = true
				}
			} else {
				loadFailed = true
			}
		}
		appendNo++
		fmt.Fprintf(&b, "\r\n--------------------------- アペ %d ---------------------------\r\n", appendNo)
		fmt.Fprintf(&b, "FROM:%s  DATE:%s\r\n", p.Author, p.CreatedAt.Format("96/01/02 15:04"))
		if loadFailed || strings.TrimSpace(p.Body) == "" {
			b.WriteString("(アペンドの読み込みに失敗しました)")
		} else {
			b.WriteString(normalizeNewlines(p.Body))
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "\r\n------------------------- APE:%d -------------------------------\r\n", appendNo)
	b.WriteString("[A]アペ [RETURN]一覧 [/]MAIN [H]HELP\r\n")
	b.WriteString(r.threadPrompt())
	return b.String()
}

func (r *Runtime) renderCommandHelp() string {
	var b strings.Builder
	b.WriteString("\r\n《コマンド・モード》\r\n")
	b.WriteString("GUIDE と入力すると、メニュー方式に戻ります。\r\n")
	b.WriteString("HELP と入力すると コマンド の簡単な説明が出てきます。\r\n")
	b.WriteString("==== 次のコマンドが使用出来ます ====\r\n")
	if r.canUseFeature(FeatureBoard) {
		b.WriteString("1. ボード\r\n")
		b.WriteString("   メニュー [BM] インデックス [BX | BXS] 読む [BR]\r\n")
		b.WriteString("   書く [BW | BWX] 階層移動 [BJ]\r\n")
	}
	if r.canUseFeature(FeatureFile) {
		b.WriteString("2. ファイル\r\n")
		b.WriteString("   メニュー [FM] インデックス [FX | FXS] 読む [FR]\r\n")
		b.WriteString("   書く [FW | FWX] 階層移動 [FJ]\r\n")
	}
	if r.canUseFeature(FeatureMail) {
		b.WriteString("3. メール  インデックス [MX] 読む [MR] 書く [MW]\r\n")
	}
	if r.canUseFeature(FeatureTelegramChat) {
		b.WriteString("4. チャット 入る [CHAT] 呼ぶ [CALL]\r\n")
		b.WriteString("5. 使用状態表示 [WHO]\r\n")
	}
	if r.canUseFeature(FeatureMemberList) {
		b.WriteString("6. メンバーリスト [MEMB]\r\n")
	}
	if r.canUseFeature(FeatureSettings) {
		b.WriteString("7. パスワード変更 [PASS] 8. 設定変更 [MODE]\r\n")
	}
	b.WriteString("9. メニューモード [GUIDE] 10. 終了 [BYE]\r\n")
	return b.String()
}

func (r *Runtime) renderBoardHelp() string {
	return "\r\n〖BOARD COMMAND〗\r\nBM       ボード／フォーラムメニュー\r\nBJ n     階層移動   例: BJ 60 / BJ\\80\\2\r\nBX/BXS   MSGインデックス\r\nBR n     MSGを読む\r\nBW/W     新規MSGを書く\r\nA [n]    アペンド書き込み\r\n0/00/T   新着・未読表示\r\nRETURN/. 前の階へ\r\n/        MAIN MENU\r\n\r\n" + r.boardPrompt()
}

func (r *Runtime) renderFileMenu() string {
	var b strings.Builder
	b.WriteString("\r\n〖Ｆile Ｍenu〗 FM\r\n")
	b.WriteString("――――――――――――――――――――――――――――――――――――――\r\n")
	b.WriteString("[FX] INDEX   [FR] READ/DOWNLOAD   [FW] UPLOAD\r\n")
	if r.canUseFeature(FeatureBatchDownload) {
		b.WriteString("[BAT] BATCH DOWNLOAD\r\n")
	}
	b.WriteString("[RETURN] このメニュー   [/] MAIN MENU   [H] HELP\r\n")
	b.WriteString(r.filePrompt())
	return b.String()
}

func (r *Runtime) renderTransferProtocolMenu() string {
	var b strings.Builder
	b.WriteString("\r\n〖転送プロトコル選択〗\r\n")
	for _, p := range r.Config.EnabledTransferProtocols() {
		fmt.Fprintf(&b, "[%s] %s\r\n", p.Key, p.Label)
	}
	b.WriteString("[RETURN] CANCEL\r\n")
	b.WriteString("PROTOCOL --> ")
	return b.String()
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
	for _, node := range r.catalog.Boards {
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
	for _, node := range r.catalog.Boards {
		if !node.Unread || node.Hidden || r.isForum(node.Path) {
			continue
		}
		fmt.Fprintf(&b, "★ %-6s %s  %d MSG\r\n", strings.ReplaceAll(node.Path, "/", "\\"), node.Name, r.rootCount(node.Path))
	}
	return b.String()
}

func (r *Runtime) formatBoardEntry(node boardNode) string {
	mark := " "
	if node.Unread {
		mark = "★"
	}
	if r.isForum(node.Path) {
		if node.Alias != "" {
			return fmt.Sprintf("%s<%s><%s> %s", mark, node.Key, node.Alias, node.Name)
		}
		return fmt.Sprintf("%s<%s> %s", mark, node.Key, node.Name)
	}
	return fmt.Sprintf("%s[%s] %s %d", mark, node.Key, node.Name, r.rootCount(node.Path))
}

func (r *Runtime) accountRole() string {
	if r.handle == "" || strings.EqualFold(r.handle, "GUEST") {
		return "GUEST"
	}
	return "MEMBER"
}

// canUseFeature intentionally checks the station-wide master switch before
// account/role authorization. A disabled host capability cannot be re-enabled
// by any user role.
func (r *Runtime) canUseFeature(feature string) bool {
	if !r.Config.FeatureEnabled(feature) {
		return false
	}
	if feature == FeatureBatchDownload && !r.Config.FeatureEnabled(FeatureFile) {
		return false
	}
	switch feature {
	case FeatureEnrollment:
		return r.accountRole() == "GUEST"
	default:
		return true
	}
}

func (r *Runtime) featureUnavailable() (string, bool) {
	return "\r\nこのサービスは現在利用できません。\r\n\r\nMAIN MENU [?]=HELP --> ", false
}

func (r *Runtime) protocolUnavailable() string {
	return "\r\nこの転送方式は現在利用できません。\r\n"
}

func (r *Runtime) filePrompt() string {
	return "(FM) FILE [M]=MENU [?]=HELP --> "
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
	if _, ok := r.findNode(path); !ok {
		return false
	}
	r.boardPath = path
	return true
}

func (r *Runtime) childSelection(key string) (string, bool) {
	for _, node := range r.catalog.Boards {
		if node.Parent == r.boardPath && node.Key == strings.TrimSpace(key) {
			return node.Path, true
		}
	}
	return "", false
}

func (r *Runtime) isForum(path string) bool {
	for _, node := range r.catalog.Boards {
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
	if node, ok := r.findNode(path); ok && !r.isForum(path) {
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

func (r *Runtime) visibleChildren(parent string) []boardNode {
	out := make([]boardNode, 0)
	for _, node := range r.catalog.Boards {
		if node.Parent == parent && !node.Hidden {
			out = append(out, node)
		}
	}
	return out
}

func (r *Runtime) findNode(path string) (boardNode, bool) {
	return boardByPath(r.catalog, normalizePath(path))
}

func (r *Runtime) boardByPath(path string) (world.Board, bool) {
	node, ok := r.findNode(path)
	if !ok || node.Hidden {
		return world.Board{}, false
	}
	return worldBoard(node), true
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
	if n := displayCellWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func decorativeLine(label, fill string, width int) string {
	middle := " " + label + " "
	remaining := width - displayCellWidth(middle)
	if remaining <= 0 {
		return middle
	}
	left := remaining / 2
	right := remaining - left
	return strings.Repeat(fill, left) + middle + strings.Repeat(fill, right)
}

func doubleCellRule(symbol string, width int) string {
	cellWidth := displayCellWidth(symbol)
	if cellWidth <= 0 {
		return ""
	}
	return strings.Repeat(symbol, width/cellWidth)
}

func boxedLine(body string, width int) string {
	const prefix = "■  "
	const suffix = "■"
	innerWidth := width - displayCellWidth(prefix) - displayCellWidth(suffix)
	return prefix + padRunes(body, innerWidth) + suffix
}

func menuColumns(first, second, third string) string {
	const columnWidth = 22
	return padRunes(first, columnWidth) + padRunes(second, columnWidth) + third
}

func displayCellWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeCellWidth(r)
	}
	return width
}

func runeCellWidth(r rune) int {
	cp := int(r)
	if (cp >= 0xFE00 && cp <= 0xFE0F) || (cp >= 0xE0100 && cp <= 0xE01EF) || cp == 0x200D {
		return 0
	}
	if cp <= 0x7F || cp == 0x00A5 || cp == 0x203E || (cp >= 0xFF61 && cp <= 0xFF9F) {
		return 1
	}
	return 2
}
