package world

import (
	"errors"
	"sync"
	"time"
)

type Store interface {
	HostByPhone(phone string) (Host, error)
	ListPosts(hostID string) []Post
	AddPost(hostID string, p Post) Post
}

// BoardPostStore is an optional capability used by host runtimes after they have
// already established that the historical host-program board exists. Keeping it
// optional preserves simple stores while allowing a repository layer to lazily
// materialize content for known-present boards without teaching the host runtime
// anything about AI or research.
type BoardPostStore interface {
	ListBoardPosts(host Host, boardID, boardTopic string) []Post
}

// The following writer capabilities are intentionally optional. WorldRepository
// uses them to persist materialized state without making every Store implementation
// support the development materialization demo.
type HostWriter interface{ SaveHost(Host) }
type BoardStore interface {
	ListBoards(hostID string) []Board
	SaveBoards(hostID string, boards []Board)
}
type PostUpdater interface{ UpdatePost(hostID string, p Post) (Post, bool) }

// PersonaStore keeps the global-persona / host-membership split explicit even in
// the in-memory PoC. A persona can later be attached to more than one host without
// cloning their identity or behavior profile.
type PersonaStore interface {
	ListHostPersonas(hostID string) []Persona
	SavePersona(Persona)
	PersonaByID(id string) (Persona, bool)
	AddMembership(hostID, personaID string)
}

type MemoryStore struct {
	mu          sync.RWMutex
	hosts       map[string]Host
	boards      map[string][]Board
	posts       map[string][]Post
	personas    map[string]Persona
	memberships map[string][]string
	next        int64
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		hosts:       map[string]Host{},
		boards:      map[string][]Board{},
		posts:       map[string][]Post{},
		personas:    map[string]Persona{},
		memberships: map[string][]string{},
		next:        1000,
	}

	h := Host{ID: "moonlight-yokohama", Phone: "0451234567", Name: "YOKOHAMA MOONLIGHT NETWORK", Region: "神奈川県横浜市", Software: "KTBBS compatible / customized", SoftwareID: "generic", Lines: 4, Popularity: .70, MaxBaud: 14400, Members: 187, ANSI: true, GuestAllowed: true, TelehoFriendly: true}
	s.hosts[h.Phone] = h
	s.posts[h.ID] = []Post{
		{ID: 1, BoardID: "main", Author: "SYSOP", Subject: "HDD増設しました", Body: "先週、HDDを340MBに増設しました。\r\nファイルボードも少し整理しています。", CreatedAt: time.Date(1996, 8, 25, 21, 14, 0, 0, time.Local)},
		{ID: 2, BoardID: "main", Author: "NEKO", Subject: "土曜のオフ", Body: "集合は18時に関内駅でいいんでしたっけ？(^^;", CreatedAt: time.Date(1996, 8, 26, 0, 42, 0, 0, time.Local)},
		{ID: 3, BoardID: "main", Author: "TAKA", Subject: "Win95どうです？", Body: "うちはまだ3.1です。98で使うには重い気もしますが…。", CreatedAt: time.Date(1996, 8, 26, 1, 7, 0, 0, time.Local)},
	}

	erika := Host{ID: "hakata-canal-net", Phone: "0920000196", Name: "HAKATA CANAL NET", Region: "福岡県福岡市", Software: "絵理香K版", SoftwareID: "erika-k", Lines: 3, Popularity: .58, MaxBaud: 14400, Members: 326, ANSI: false, GuestAllowed: true, TelehoFriendly: true}
	s.hosts[erika.Phone] = erika
	s.posts[erika.ID] = []Post{
		{ID: 101, BoardID: "1", Author: "SYSOP", Subject: "今週末のメンテナンス", Body: "土曜の午前3時ごろに30分ほど止めます。\r\nHDDの整理とログの退避をします。", CreatedAt: time.Date(1996, 8, 24, 22, 10, 0, 0, time.Local)},
		{ID: 102, BoardID: "1", ParentID: 101, Author: "MARI", Subject: "Re: 今週末のメンテナンス", Body: "了解ですー。夜更かし組はその前に落ちます(^^;", CreatedAt: time.Date(1996, 8, 24, 23, 2, 0, 0, time.Local)},
		{ID: 103, BoardID: "1", ParentID: 101, Author: "KAZU", Subject: "Re: 今週末のメンテナンス", Body: "バックアップご苦労さまです。", CreatedAt: time.Date(1996, 8, 25, 0, 18, 0, 0, time.Local)},
		{ID: 120, BoardID: "4", Author: "MARI", Subject: "まだまだ暑いですね～", Body: "昼の天神は暑かったです(^^;\r\n夜になると少し楽かな。", CreatedAt: time.Date(1996, 8, 25, 19, 34, 0, 0, time.Local)},
		{ID: 121, BoardID: "4", ParentID: 120, Author: "YUKI", Subject: "Re: まだまだ暑いですね～", Body: "こっちはクーラー全開です(笑)", CreatedAt: time.Date(1996, 8, 25, 20, 1, 0, 0, time.Local)},
		{ID: 110, BoardID: "10/2", Author: "YUKI", Subject: "天神でオフしません？", Body: "9月の最初の土曜あたり、天神でどうでしょう。\r\n人数集まりそうなら店を探します。", CreatedAt: time.Date(1996, 8, 25, 20, 45, 0, 0, time.Local)},
		{ID: 111, BoardID: "10/2", ParentID: 110, Author: "MARI", Subject: "Re: 天神でオフしません？", Body: "参加希望です(^_^)/", CreatedAt: time.Date(1996, 8, 25, 21, 3, 0, 0, time.Local)},
		{ID: 130, BoardID: "20/1", Author: "KAZU", Subject: "ポケモン赤と緑", Body: "弟がずっとやってます。\r\n通信ケーブルまで買わされました(^^;", CreatedAt: time.Date(1996, 8, 25, 17, 48, 0, 0, time.Local)},
		{ID: 201, BoardID: "60/1", Author: "TAKU", Subject: "PC-9821で28.8K", Body: "V.34モデムを入れてみました。\r\n回線によっては26400くらいに落ちますね。", CreatedAt: time.Date(1996, 8, 25, 18, 27, 0, 0, time.Local)},
		{ID: 202, BoardID: "60/1", ParentID: 201, Author: "SYSOP", Subject: "Re: PC-9821で28.8K", Body: "うちの3回線目も夜は24000まで落ちることがあります。", CreatedAt: time.Date(1996, 8, 25, 19, 12, 0, 0, time.Local)},
		{ID: 210, BoardID: "60/1", Author: "NORI", Subject: "WTERMの設定", Body: "自動巡回のマクロを作り直してます。\r\nうまくいったらアップします。", CreatedAt: time.Date(1996, 8, 26, 0, 20, 0, 0, time.Local)},
		{ID: 301, BoardID: "60/3", Author: "SYSOP", Subject: "NMODEMテスト用ファイル", Body: "NMODEMの転送テストをする人は声をかけてください。\r\n夜中なら空いていることが多いです。", CreatedAt: time.Date(1996, 8, 23, 23, 50, 0, 0, time.Local)},
		{ID: 401, BoardID: "70/1", Author: "NORI", Subject: "9821Xaのメモリ", Body: "32MBまで増やしたらWin95がかなり楽になりました。", CreatedAt: time.Date(1996, 8, 25, 22, 8, 0, 0, time.Local)},
		{ID: 501, BoardID: "80/2", Author: "KAZU", Subject: "MSX turbo Rまだ現役", Body: "うちはFS-A1GTがまだ机の横にいます(^^)", CreatedAt: time.Date(1996, 8, 24, 21, 16, 0, 0, time.Local)},
		{ID: 901, BoardID: "99", Author: "MIDNIGHT", Subject: "ここ見つけた人いる？", Body: "ボードマップには出てないけど BJ 99 で入れるみたい(笑)", CreatedAt: time.Date(1996, 8, 26, 2, 11, 0, 0, time.Local)},
	}

	s.hosts["0450000001"] = Host{ID: "quiet-test", Phone: "0450000001", Name: "QUIET TEST BBS", Region: "神奈川県", Software: "mmm compatible", SoftwareID: "generic", Lines: 8, Popularity: .05, MaxBaud: 28800, Members: 22, ANSI: false, GuestAllowed: true}
	s.hosts["0459999999"] = Host{ID: "busy-test", Phone: "0459999999", Name: "POPULAR TEST BBS", Region: "神奈川県", Software: "BIG-Model compatible", SoftwareID: "generic", Lines: 1, Popularity: 1, MaxBaud: 14400, Members: 912, ANSI: true, GuestAllowed: true}

	// Development-only seed: intentionally incomplete. WorldRepository fills the
	// missing profile when this number is first dialed, then stores the result.
	s.hosts["0450000196"] = Host{ID: "materialize-demo", Phone: "0450000196", Region: "神奈川県", SoftwareID: "materialization-demo"}
	return s
}

func (s *MemoryStore) HostByPhone(phone string) (Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hosts[phone]
	if !ok {
		return Host{}, errors.New("host not found")
	}
	return h, nil
}
func (s *MemoryStore) SaveHost(h Host) { s.mu.Lock(); defer s.mu.Unlock(); s.hosts[h.Phone] = h }
func (s *MemoryStore) ListPosts(hostID string) []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.posts[hostID]
	out := make([]Post, len(p))
	copy(out, p)
	return out
}
func (s *MemoryStore) ListBoardPosts(host Host, boardID, _ string) []Post {
	all := s.ListPosts(host.ID)
	out := make([]Post, 0)
	for _, p := range all {
		if p.BoardID == boardID {
			out = append(out, p)
		}
	}
	return out
}
func (s *MemoryStore) AddPost(hostID string, p Post) Post {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	p.ID = s.next
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.posts[hostID] = append(s.posts[hostID], p)
	return p
}
func (s *MemoryStore) UpdatePost(hostID string, p Post) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.posts[hostID] {
		if s.posts[hostID][i].ID == p.ID {
			s.posts[hostID][i] = p
			return p, true
		}
	}
	return Post{}, false
}
func (s *MemoryStore) ListBoards(hostID string) []Board {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := s.boards[hostID]
	out := make([]Board, len(v))
	copy(out, v)
	return out
}
func (s *MemoryStore) SaveBoards(hostID string, boards []Board) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Board, len(boards))
	copy(out, boards)
	s.boards[hostID] = out
}
func (s *MemoryStore) SavePersona(p Persona) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.personas[p.ID] = p
}
func (s *MemoryStore) PersonaByID(id string) (Persona, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.personas[id]
	return p, ok
}
func (s *MemoryStore) AddMembership(hostID, personaID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.memberships[hostID] {
		if existing == personaID {
			return
		}
	}
	s.memberships[hostID] = append(s.memberships[hostID], personaID)
}
func (s *MemoryStore) ListHostPersonas(hostID string) []Persona {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.memberships[hostID]
	out := make([]Persona, 0, len(ids))
	for _, id := range ids {
		if p, ok := s.personas[id]; ok {
			out = append(out, p)
		}
	}
	return out
}
