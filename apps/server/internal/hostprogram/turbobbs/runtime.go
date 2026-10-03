package turbobbs

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type account struct {
	Password string
	Access   int
	LastMess int64
}

type section struct {
	ID   string
	Name string
}

var sections = []section{
	{ID: "1", Name: "GENERAL"},
	{ID: "2", Name: "PC-98 / DOS"},
	{ID: "3", Name: "WINDOWS 95"},
	{ID: "4", Name: "MODEM / TELECOM"},
	{ID: "5", Name: "PROGRAMMING"},
	{ID: "6", Name: "GAMES"},
	{ID: "7", Name: "MUSIC / MIDI"},
	{ID: "8", Name: "LOCAL TALK"},
	{ID: "9", Name: "FILES / UTILITIES"},
	{ID: "10", Name: "MISC"},
}

// Runtime reconstructs the Maxwell TurboBBS interaction grammar from the
// surviving 1.05 manual and later 1.08 source. Station prose/section labels are
// fictional reconstruction; command/state behavior should stay source-backed.
//
// The shared world.Store currently has no IDS.BBS/private-mail persistence.
// Account/profile changes are therefore call-local for this first integration.
// Public messages and station posts are canonical world data.
type Runtime struct {
	Host  world.Host
	Store world.Store

	state       string
	caller      string
	access      int
	lastMess    int64
	expert      bool
	width       int
	caps        bool
	sendLF      bool
	promptBell  bool
	password    string
	loginTries  int
	newUser     bool
	pendingName string
	terminalReturn string

	accounts map[string]account


	applyLines []string

	enterTo      string
	enterSubject string
	enterSection string
	enterLines   []string

}

func New(host world.Host, store world.Store) *Runtime {
	return &Runtime{
		Host:      host,
		Store:     store,
		state:     "login_name",
		width:     80,
		sendLF:    true,
		accounts: map[string]account{
			"TARO YAMADA": {Password: "MODEM", Access: 3, LastMess: 602},
			"SYSOP":       {Password: "TURBO", Access: 5, LastMess: 0},
		},
	}
}

func (r *Runtime) ObservationBoards() []world.Board {
	out := make([]world.Board, 0, len(sections))
	for _, s := range sections {
		out = append(out, world.Board{ID: s.ID, Name: s.Name})
	}
	return out
}

func (r *Runtime) Welcome() string {
	return fmt.Sprintf(
		"\r\n+----------------------------------------------------------+\r\n"+
			"|                 %s\r\n"+
			"|             %s / %d line / MAX %dbps\r\n"+
			"+----------------------------------------------------------+\r\n"+
			"TurboBBS version 1.08\r\n"+
			"Bulletin: This old machine is still running in 1996.\r\n\r\n"+
			"What is your full name? ",
		padRight(trimRunes(r.Host.Name, 40), 40),
		r.Host.Region, r.Host.Lines, r.Host.MaxBaud,
	)
}

func (r *Runtime) HandleLine(line string) (string, bool) {
	// TurboBBS intentionally supports chained command/input syntax using ';'.
	// Feed each segment through the current state machine in sequence.
	parts := strings.Split(line, ";")
	var b strings.Builder
	for _, part := range parts {
		out, disconnect := r.handleToken(strings.TrimSpace(part))
		b.WriteString(out)
		if disconnect {
			return b.String(), true
		}
	}
	return b.String(), false
}

func (r *Runtime) handleToken(line string) (string, bool) {
	switch r.state {
	case "login_name":
		return r.handleLoginName(line)
	case "login_confirm":
		return r.handleLoginConfirm(line)
	case "login_password":
		return r.handleLoginPassword(line)
	case "new_password":
		return r.handleNewPassword(line)
	case "new_password_confirm":
		return r.handleNewPasswordConfirm(line)
	case "terminal_setup":
		return r.handleTerminalSetup(line)
	case "terminal_width":
		return r.handleTerminalWidth(line)
	case "main":
		return r.handleMain(line)
	case "read_menu":
		return r.handleReadMenu(line)
	case "read_number":
		return r.handleReadNumber(line)
	case "read_from":
		return r.handleReadFrom(line)
	case "read_to":
		return r.handleReadTo(line)
	case "read_section":
		return r.handleReadSection(line)
	case "apply":
		return r.handleApply(line)
	case "enter_to":
		return r.handleEnterTo(line)
	case "enter_subject":
		return r.handleEnterSubject(line)
	case "enter_section":
		return r.handleEnterSection(line)
	case "enter_body":
		return r.handleEnterBody(line)
	case "enter_edit":
		return r.handleEnterEdit(line)
	case "change_password":
		return r.handleChangePassword(line)
	case "change_password_confirm":
		return r.handleChangePasswordConfirm(line)
	case "goodbye_comment":
		return r.handleGoodbyeComment(line)
	case "goodbye_text":
		return r.finishGoodbye(line), true
	case "file":
		return r.handleFile(line)
	case "file_type":
		return r.handleFileType(line)
	case "sysop":
		return r.handleSysop(line)
	default:
		r.state = "main"
		return r.renderMainPrompt(true), false
	}
}

func (r *Runtime) handleLoginName(line string) (string, bool) {
	name := strings.ToUpper(strings.TrimSpace(line))
	if len([]rune(name)) <= 4 {
		return "Please enter your full name (more than 4 characters).\r\nWhat is your full name? ", false
	}
	r.pendingName = trimRunes(name, 27)
	if a, ok := r.accounts[r.pendingName]; ok {
		r.caller = r.pendingName
		r.access = a.Access
		r.lastMess = a.LastMess
		r.loginTries = 0
		r.state = "login_password"
		return "    Enter your password : ", false
	}
	r.state = "login_confirm"
	return fmt.Sprintf("%s: is this correct (Y/N)? ", r.pendingName), false
}

func (r *Runtime) handleLoginConfirm(line string) (string, bool) {
	switch strings.ToUpper(line) {
	case "Y", "YES":
		r.caller = r.pendingName
		r.newUser = true
		r.access = 2
		r.lastMess = 0
		r.state = "new_password"
		return "\r\nGetting new user password & terminal info : \r\nEnter the password you want on this system : ", false
	case "N", "NO":
		r.pendingName = ""
		r.state = "login_name"
		return "What is your full name? ", false
	default:
		return "Please answer Y or N: ", false
	}
}

func (r *Runtime) handleLoginPassword(line string) (string, bool) {
	a := r.accounts[r.caller]
	r.loginTries++
	if strings.ToUpper(line) == a.Password {
		r.password = a.Password
		return r.finishLogin(), false
	}
	if r.loginTries >= 3 {
		return "\r\nIncorrect password.\r\n", true
	}
	return "    Incorrect! - try again : ", false
}

func (r *Runtime) handleNewPassword(line string) (string, bool) {
	pw := strings.ToUpper(trimRunes(line, 14))
	if pw == "" {
		return "Password may not be empty.\r\nEnter the password you want on this system : ", false
	}
	r.password = pw
	r.state = "new_password_confirm"
	return "                 Enter it again, to be sure: ", false
}

func (r *Runtime) handleNewPasswordConfirm(line string) (string, bool) {
	if strings.ToUpper(line) != r.password {
		r.state = "new_password"
		return "\r\n         Passwords did not match!\r\nEnter the password you want on this system : ", false
	}
	r.terminalReturn = "login"
	r.state = "terminal_setup"
	return "\r\n" + r.renderTerminalSetup(), false
}

func (r *Runtime) handleTerminalSetup(line string) (string, bool) {
	switch strings.TrimSpace(line) {
	case "", "0":
		if r.terminalReturn == "main" {
			r.state = "main"
			r.terminalReturn = ""
			return "\r\nNew definitions are saved by [G]oodbye command." + r.renderMainPrompt(false), false
		}
		r.terminalReturn = ""
		return r.finishLogin(), false
	case "1":
		r.caps = !r.caps
	case "2":
		r.sendLF = !r.sendLF
	case "3":
		r.promptBell = !r.promptBell
	case "6":
		r.state = "terminal_width"
		return " Enter your terminal width (chars/line): ", false
	default:
		return "Enter number 1, 2, 3, 6, or 0 to quit: ", false
	}
	return r.renderTerminalSetup(), false
}

func (r *Runtime) handleTerminalWidth(line string) (string, bool) {
	n, err := strconv.Atoi(line)
	if err != nil || n < 20 || n > 132 {
		return " Enter width 20..132: ", false
	}
	r.width = n
	r.state = "terminal_setup"
	return r.renderTerminalSetup(), false
}

func (r *Runtime) finishLogin() string {
	if r.access == 0 {
		return fmt.Sprintf("\r\n  User %s has been denied system access!\r\n", r.caller)
	}
	r.state = "main"
	var b strings.Builder
	if !r.newUser && r.lastMess > 0 {
		fmt.Fprintf(&b, "\r\nLast message number seen: %d\r\n", r.lastMess)
	}
	fmt.Fprintf(&b, "\r\nWelcome, %s.\r\n", r.caller)
	b.WriteString(r.renderMainMenu())
	b.WriteString(r.renderMainPrompt(false))
	return b.String()
}

func (r *Runtime) handleMain(line string) (string, bool) {
	cmd := strings.ToUpper(strings.TrimSpace(line))
	switch cmd {
	case "":
		return r.renderMainPrompt(false), false
	case "A":
		r.state = "apply"
		r.applyLines = nil
		return "\r\n---- Applying for regular access ----\r\nLeave your name, address and telephone number for the Sysop.\r\nPress RETURN on an empty line when finished.\r\n1> ", false
	case "B":
		return "\r\n================= BULLETINS =================\r\nThe station is running normally.\r\n" + r.renderMainPrompt(false), false
	case "C":
		return "\r\nEntering chat mode: Ctrl-Z aborts at any time.\r\nSummoning the almighty Sysop...\a\a\a\r\nSorry... Must be down at the bar!\r\n" + r.renderMainPrompt(false), false
	case "E":
		if r.access < 3 {
			return "\r\nSorry - message entry requires regular access. Use [A]pply.\r\n" + r.renderMainPrompt(false), false
		}
		r.state = "enter_to"
		r.enterLines = nil
		return "\r\nTo (or ALL): ", false
	case "F":
		r.state = "file"
		return r.renderFileMenu(), false
	case "G":
		r.state = "goodbye_comment"
		return "\r\nAny comments to Sysop (Y/N)? ", false
	case "H":
		return r.renderHelp() + r.renderMainPrompt(false), false
	case "I":
		r.terminalReturn = "main"
		r.state = "terminal_setup"
		return "\r\n" + r.renderTerminalSetup(), false
	case "K":
		return "\r\nDelete not permitted.\r\n" + r.renderMainPrompt(false), false
	case "L":
		return r.renderUserLog() + r.renderMainPrompt(false), false
	case "M":
		return "\r\n================== MEETINGS ==================\r\nLocal user-group information is posted here when available.\r\n" + r.renderMainPrompt(false), false
	case "N":
		return r.renderPosts(func(p world.Post) bool { return p.ID > r.lastMess }, false) + r.renderMainPrompt(false), false
	case "O":
		return "\r\nOther remote access systems: ask the Sysop for the current local list.\r\n" + r.renderMainPrompt(false), false
	case "P":
		r.state = "change_password"
		return "\r\nEnter the password you want on this system : ", false
	case "Q":
		r.resetForRelog()
		return "\r\nLogoff without disconnecting - profile changes not saved.\r\nWhat is your full name? ", false
	case "R":
		r.state = "read_menu"
		return r.renderReadMenu(), false
	case "S":
		return r.renderPosts(func(world.Post) bool { return true }, true) + r.renderMainPrompt(false), false
	case "U":
		return r.renderUsers() + r.renderMainPrompt(false), false
	case "W":
		return r.renderStationWelcome() + r.renderMainPrompt(false), false
	case "X":
		r.expert = !r.expert
		return r.renderMainPrompt(false), false
	case "Y":
		return r.renderSystemInfo() + r.renderMainPrompt(false), false
	case "#":
		return r.renderStatus() + r.renderMainPrompt(false), false
	case "?":
		if r.expert {
			return r.renderMainMenu() + r.renderMainPrompt(false), false
		}
		return r.renderMainPrompt(false), false
	case "@":
		if r.access == 5 {
			r.state = "sysop"
			return "\r\n? ", false
		}
		return r.renderMainPrompt(false), false
	case "!":
		if r.access == 5 {
			return "\r\nPrinter mirror toggled.\r\n" + r.renderMainPrompt(false), false
		}
		return r.renderMainPrompt(false), false
	default:
		return "?\r\n" + r.renderMainPrompt(false), false
	}
}

func (r *Runtime) handleReadMenu(line string) (string, bool) {
	cmd := strings.ToUpper(line)
	switch cmd {
	case "A":
		r.state = "main"
		return r.renderPosts(func(world.Post) bool { return true }, false) + r.renderMainPrompt(false), false
	case "I":
		r.state = "read_number"
		return "Message number ? ", false
	case "F":
		r.state = "read_from"
		return "From user ? ", false
	case "T":
		r.state = "read_to"
		return "To user ? ", false
	case "S":
		r.state = "read_section"
		return "Section number ? ", false
	default:
		if _, err := strconv.ParseInt(cmd, 10, 64); err == nil {
			r.state = "main"
			return r.renderMessageNumber(cmd) + r.renderMainPrompt(false), false
		}
		return r.renderReadMenu(), false
	}
}

func (r *Runtime) handleReadNumber(line string) (string, bool) {
	r.state = "main"
	return r.renderMessageNumber(line) + r.renderMainPrompt(false), false
}

func (r *Runtime) handleReadFrom(line string) (string, bool) {
	name := strings.ToUpper(line)
	r.state = "main"
	return r.renderPosts(func(p world.Post) bool { return strings.ToUpper(p.Author) == name }, false) + r.renderMainPrompt(false), false
}

func (r *Runtime) handleReadTo(line string) (string, bool) {
	r.state = "main"
	if strings.EqualFold(strings.TrimSpace(line), "ALL") {
		return r.renderPosts(func(world.Post) bool { return true }, false) + r.renderMainPrompt(false), false
	}
	return "\r\nNo messages found.\r\n" + r.renderMainPrompt(false), false
}

func (r *Runtime) handleReadSection(line string) (string, bool) {
	sec := strings.TrimSpace(line)
	r.state = "main"
	if !validSection(sec) {
		return "Invalid section.\r\n" + r.renderMainPrompt(false), false
	}
	return r.renderPosts(func(p world.Post) bool { return p.BoardID == sec }, false) + r.renderMainPrompt(false), false
}

func (r *Runtime) handleApply(line string) (string, bool) {
	if line == "" {
		r.state = "main"
		return "\r\nApplication left for Sysop review.\r\n" + r.renderMainPrompt(false), false
	}
	r.applyLines = append(r.applyLines, line)
	if len(r.applyLines) >= 4 {
		r.state = "main"
		return "\r\nApplication left for Sysop review.\r\n" + r.renderMainPrompt(false), false
	}
	return fmt.Sprintf("%d> ", len(r.applyLines)+1), false
}

func (r *Runtime) handleEnterTo(line string) (string, bool) {
	to := strings.ToUpper(strings.TrimSpace(line))
	if to == "" || to == "ALL" {
		r.enterTo = "ALL"
		r.state = "enter_subject"
		return "Subject (14 chars): ", false
	}
	// Private messages require recipient/received metadata not yet present in the
	// shared world model. Keep the fictional station public-only for now.
	return "This station currently accepts public messages to ALL only.\r\nTo (or ALL): ", false
}

func (r *Runtime) handleEnterSubject(line string) (string, bool) {
	if strings.TrimSpace(line) == "" {
		return "Subject may not be empty.\r\nSubject (14 chars): ", false
	}
	r.enterSubject = trimRunes(line, 14)
	r.state = "enter_section"
	return r.renderSections() + "Section number ? ", false
}

func (r *Runtime) handleEnterSection(line string) (string, bool) {
	sec := strings.TrimSpace(line)
	if !validSection(sec) {
		return "Invalid section. Section number ? ", false
	}
	r.enterSection = sec
	r.enterLines = nil
	r.state = "enter_body"
	return "Enter message text. Empty line enters editor command mode.\r\n1> ", false
}

func (r *Runtime) handleEnterBody(line string) (string, bool) {
	if line == "" {
		r.state = "enter_edit"
		return r.renderEditMenu(), false
	}
	if len(r.enterLines) >= 24 {
		r.state = "enter_edit"
		return "\r\n24 line limit reached.\r\n" + r.renderEditMenu(), false
	}
	r.enterLines = append(r.enterLines, trimRunes(line, 80))
	return fmt.Sprintf("%d> ", len(r.enterLines)+1), false
}

func (r *Runtime) handleEnterEdit(line string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(line)) {
	case "A":
		r.clearEntry()
		r.state = "main"
		return "\r\nMessage aborted.\r\n" + r.renderMainPrompt(false), false
	case "C":
		r.state = "enter_body"
		return fmt.Sprintf("%d> ", len(r.enterLines)+1), false
	case "L":
		var b strings.Builder
		for i, text := range r.enterLines {
			fmt.Fprintf(&b, "%2d: %s\r\n", i+1, text)
		}
		b.WriteString(r.renderEditMenu())
		return b.String(), false
	case "P":
		return r.storeEntry(true), false
	case "S":
		return r.storeEntry(false), false
	case "?":
		return r.renderEditMenu(), false
	default:
		return "A,C,L,P,S or ? : ", false
	}
}

func (r *Runtime) storeEntry(preformatted bool) string {
	body := strings.Join(r.enterLines, " ")
	if preformatted {
		body = strings.Join(r.enterLines, "\r\n")
	}
	if body == "" {
		r.state = "enter_edit"
		return "Message is empty.\r\n" + r.renderEditMenu()
	}
	p := r.Store.AddPost(r.Host.ID, world.Post{
		BoardID:   r.enterSection,
		Author:    r.caller,
		Subject:   r.enterSubject,
		Body:      body,
		CreatedAt: time.Now(),
	})
	r.clearEntry()
	r.state = "main"
	return fmt.Sprintf("\r\nMessage %d stored.\r\n", p.ID) + r.renderMainPrompt(false)
}

func (r *Runtime) clearEntry() {
	r.enterTo = ""
	r.enterSubject = ""
	r.enterSection = ""
	r.enterLines = nil
}

func (r *Runtime) handleChangePassword(line string) (string, bool) {
	pw := strings.ToUpper(trimRunes(line, 14))
	if pw == "" {
		return "Password may not be empty. Enter password: ", false
	}
	r.password = pw
	r.state = "change_password_confirm"
	return "Enter it again, to be sure: ", false
}

func (r *Runtime) handleChangePasswordConfirm(line string) (string, bool) {
	if strings.ToUpper(line) != r.password {
		r.state = "change_password"
		return "Passwords did not match. Enter password: ", false
	}
	if a, ok := r.accounts[r.caller]; ok {
		a.Password = r.password
		r.accounts[r.caller] = a
	}
	r.state = "main"
	return "\r\nNew password is saved when the [G]oodbye command is executed.\r\n" + r.renderMainPrompt(false), false
}

func (r *Runtime) handleGoodbyeComment(line string) (string, bool) {
	switch strings.ToUpper(line) {
	case "Y", "YES":
		r.state = "goodbye_text"
		return "Comment: ", false
	case "N", "NO", "":
		return r.finishGoodbye(""), true
	default:
		return "Please answer Y or N: ", false
	}
}

func (r *Runtime) finishGoodbye(_ string) string {
	r.lastMess = r.maxMessageNumber()
	return fmt.Sprintf("\r\nGoodbye, %s.\r\n", r.caller)
}

func (r *Runtime) handleFile(line string) (string, bool) {
	cmd := strings.ToUpper(strings.TrimSpace(line))
	switch cmd {
	case "", "?":
		return r.renderFileMenu(), false
	case "D":
		return r.renderFileDirectory() + r.filePrompt(), false
	case "H":
		return r.renderFileHelp() + r.filePrompt(), false
	case "L":
		return "\r\nBBSINFO.LBR\r\n  WELCOME.TXT\r\n  BULLETIN.TXT\r\n  BBSHELP.TXT\r\n" + r.filePrompt(), false
	case "Q":
		r.state = "main"
		return r.renderMainPrompt(false), false
	case "G":
		r.state = "goodbye_comment"
		return "\r\nAny comments to Sysop (Y/N)? ", false
	case "T":
		r.state = "file_type"
		return "File name ? ", false
	case "S":
		return "\r\nFile not selected.\r\n" + r.filePrompt(), false
	case "U", "C", "V":
		if r.access <= 2 {
			return "\r\nUpload requires regular access.\r\n" + r.filePrompt(), false
		}
		return "\r\nTransfer cancelled.\r\n" + r.filePrompt(), false
	default:
		return "?\r\n" + r.filePrompt(), false
	}
}

func (r *Runtime) handleFileType(line string) (string, bool) {
	r.state = "file"
	switch strings.ToUpper(strings.TrimSpace(line)) {
	case "README.TXT":
		return fmt.Sprintf("\r\n%s user information.\r\nPlease leave comments for the Sysop if you find a problem.\r\n", r.Host.Name) + r.filePrompt(), false
	case "BBSINFO/BULLETIN.TXT":
		return "\r\nThe station is running normally.\r\n" + r.filePrompt(), false
	default:
		return "\r\nFile not found.\r\n" + r.filePrompt(), false
	}
}

func (r *Runtime) handleSysop(line string) (string, bool) {
	switch strings.ToUpper(line) {
	case "C":
		return "\r\nNo comments.\r\n? ", false
	case "L":
		return "\r\nAccess editor unavailable.\r\n? ", false
	case "!":
		return "\r\nPrinter mirror toggled.\r\n? ", false
	default:
		r.state = "main"
		return r.renderMainPrompt(false), false
	}
}

func (r *Runtime) resetForRelog() {
	r.state = "login_name"
	r.caller = ""
	r.access = 0
	r.lastMess = 0
	r.loginTries = 0
	r.newUser = false
	r.pendingName = ""
	r.password = ""
	r.expert = false
	r.width = 80
	r.caps = false
	r.sendLF = true
	r.promptBell = false
	r.terminalReturn = ""
	r.applyLines = nil
	r.clearEntry()
}

func (r *Runtime) renderMainMenu() string {
	return "\r\nInformation files        Message system            Functions\r\n" +
		"------------------------+----------------------+-----------------------\r\n" +
		"[B]ulletin [H]elp       [E]nter      [S]can    [A]pply    [I]nstall\r\n" +
		"[O]thersys  [U]serlist   [#]:Status   [K]ill    [C]hat     [P]assword\r\n" +
		"user[L]og   [W]elcome    [N]ew-read             [F]iles    e[X]pert\r\n" +
		"[M]eetings  s[Y]sinfo    [R]ead                 [G]oodbye\r\n"
}

func (r *Runtime) renderMainPrompt(_ bool) string {
	if r.expert {
		return "\r\nCommand: (? for menu) ? "
	}
	return "\r\nCommand: A,B,C,E,F,G,H,I,K,L,M,N,O,P,R,S,U,W,X,Y,# ? "
}

func (r *Runtime) renderHelp() string {
	return "\r\n============= TurboBBS Help =============\r\n" +
		"A Apply   B Bulletin   C Chat      E Enter message\r\n" +
		"F Files   G Goodbye    H Help      I Install terminal\r\n" +
		"K Kill    L User log   M Meetings  N New-read\r\n" +
		"O Other   P Password   Q Relog     R Read\r\n" +
		"S Scan    U User list  W Welcome   X Expert\r\n" +
		"Y Sysinfo # Status\r\n" +
		"Commands and multi-character answers may be chained with semicolons.\r\n"
}

func (r *Runtime) renderReadMenu() string {
	return "\r\n============== Read Menu ==============\r\n" +
		"[A]ll, starting from a number.    [I]ndividual, by number.\r\n" +
		"[F]rom a certain user.            [T]o a certain user.\r\n" +
		"by [S]ection number or enter message number to read.\r\nRead: "
}

func (r *Runtime) renderEditMenu() string {
	return "\r\n================ Edit menu ================\r\n" +
		"[A]bort message.               [C]ontinue entering lines.\r\n" +
		"[L]ist entered lines.\r\n" +
		"[P]reformatted store.          [S]tore flowing message.\r\nEdit: "
}

func (r *Runtime) renderTerminalSetup() string {
	return fmt.Sprintf(
		"Terminal parameters:\r\n"+
			"1 - Upper case only: %s\r\n"+
			"2 - Line feeds sent: %s\r\n"+
			"3 - Prompt bell ON : %s\r\n"+
			"6 - Terminal width : %d\r\n"+
			"Enter number of parameter to change (0 to quit): ",
		yesNo(r.caps), yesNo(r.sendLF), yesNo(r.promptBell), r.width,
	)
}

func (r *Runtime) renderPosts(match func(world.Post) bool, headersOnly bool) string {
	posts := append([]world.Post(nil), r.Store.ListPosts(r.Host.ID)...)
	sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })
	var b strings.Builder
	found := false
	for _, p := range posts {
		if !match(p) {
			continue
		}
		found = true
		fmt.Fprintf(&b, "\r\n#%d  SEC:%s  FROM:%s  %s\r\n", p.ID, p.BoardID, p.Author, p.Subject)
		if !headersOnly {
			b.WriteString(p.Body)
			if !strings.HasSuffix(p.Body, "\r\n") {
				b.WriteString("\r\n")
			}
		}
	}
	if !found {
		b.WriteString("\r\nNo messages found.\r\n")
	}
	return b.String()
}

func (r *Runtime) renderMessageNumber(raw string) string {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return "Invalid message number.\r\n"
	}
	return r.renderPosts(func(p world.Post) bool { return p.ID == id }, false)
}

func (r *Runtime) renderSections() string {
	var b strings.Builder
	b.WriteString("\r\nSections:\r\n")
	for _, s := range sections {
		fmt.Fprintf(&b, "%2s %-20s", s.ID, s.Name)
		if n, _ := strconv.Atoi(s.ID); n%2 == 0 {
			b.WriteString("\r\n")
		} else {
			b.WriteString("   ")
		}
	}
	return b.String()
}

func (r *Runtime) renderUsers() string {
	return fmt.Sprintf("\r\n--- %d users registered ---\r\n  SYSOP\r\n  TARO YAMADA\r\n  %s\r\n", r.Host.Members, r.caller)
}

func (r *Runtime) renderUserLog() string {
	return "\r\nUser Log\r\nSYSOP                    08/26 20:15 to 20:42\r\nTARO YAMADA               08/26 21:03 to 21:18\r\n"
}

func (r *Runtime) renderStationWelcome() string {
	return fmt.Sprintf("\r\nWelcome to %s.\r\nPlease enjoy the message and file sections.\r\n", r.Host.Name)
}

func (r *Runtime) renderSystemInfo() string {
	return fmt.Sprintf("\r\n============== Information about this system ==============\r\nHost: %s\r\nRegion: %s\r\nSoftware: %s\r\nLines: %d  Maximum line speed: %d bps\r\n", r.Host.Name, r.Host.Region, r.Host.Software, r.Host.Lines, r.Host.MaxBaud)
}

func (r *Runtime) renderStatus() string {
	return fmt.Sprintf("\r\nCaller: %s  Access: %d  Width: %d  Last message: %d\r\n", r.caller, r.access, r.width, r.lastMess)
}

func (r *Runtime) renderFileMenu() string {
	return "\r\nFile Menu Commands\r\n" +
		"[D]irectory  [Q]uit to main menu  [G]oodbye  [H]elp\r\n" +
		"[L]ibrary directory\r\n" +
		"[S]end (XMODEM)  [U]pload CRC  [C]hecksum upload\r\n" +
		"[V]erbatim upload  [T]ype file\r\n" +
		r.filePrompt()
}

func (r *Runtime) renderFileHelp() string {
	return "\r\nFile System Help\r\n" +
		"XMODEM checksum/CRC, text capture, .LBR member access and SQueezed\r\n" +
		"text display are supported by TurboBBS. Use [D]irectory before transfer.\r\n"
}

func (r *Runtime) renderFileDirectory() string {
	return "\r\nFilename        Size  Accesses  Section\r\n" +
		"README.TXT       2K      17       1\r\n" +
		"MODEMFAQ.TXT     8K      41       4\r\n" +
		"TURBOBBS.TXT     5K      23       9\r\n" +
		"Space remaining: 118K\r\n"
}

func (r *Runtime) filePrompt() string {
	return "\r\nFile command (? for menu): "
}

func (r *Runtime) maxMessageNumber() int64 {
	var max int64
	for _, p := range r.Store.ListPosts(r.Host.ID) {
		if p.ID > max {
			max = p.ID
		}
	}
	return max
}

func validSection(id string) bool {
	for _, s := range sections {
		if s.ID == id {
			return true
		}
	}
	return false
}

func yesNo(v bool) string {
	if v {
		return "Y"
	}
	return "N"
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func padRight(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}
