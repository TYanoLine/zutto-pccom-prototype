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

type MemoryStore struct {
	mu    sync.RWMutex
	hosts map[string]Host
	posts map[string][]Post
	next  int64
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		hosts: map[string]Host{},
		posts: map[string][]Post{},
		next:  1000,
	}

	h := Host{
		ID:             "moonlight-yokohama",
		Phone:          "0451234567",
		Name:           "YOKOHAMA MOONLIGHT NETWORK",
		Region:         "神奈川県横浜市",
		Software:       "KTBBS compatible / customized",
		Lines:          4,
		Popularity:     0.70,
		MaxBaud:        14400,
		Members:        187,
		ANSI:           true,
		GuestAllowed:   true,
		TelehoFriendly: true,
	}
	s.hosts[h.Phone] = h
	s.posts[h.ID] = []Post{
		{ID: 1, Author: "SYSOP", Subject: "HDD増設しました", Body: "先週、HDDを340MBに増設しました。\r\nファイルボードも少し整理しています。", CreatedAt: time.Date(1996, 8, 25, 21, 14, 0, 0, time.Local)},
		{ID: 2, Author: "NEKO", Subject: "土曜のオフ", Body: "集合は18時に関内駅でいいんでしたっけ？(^^;", CreatedAt: time.Date(1996, 8, 26, 0, 42, 0, 0, time.Local)},
		{ID: 3, Author: "TAKA", Subject: "Win95どうです？", Body: "うちはまだ3.1です。98で使うには重い気もしますが…。", CreatedAt: time.Date(1996, 8, 26, 1, 7, 0, 0, time.Local)},
	}

	// Useful deterministic endpoints for prototype testing.
	s.hosts["0450000001"] = Host{ID: "quiet-test", Phone: "0450000001", Name: "QUIET TEST BBS", Region: "神奈川県", Software: "mmm compatible", Lines: 8, Popularity: 0.05, MaxBaud: 28800, Members: 22, ANSI: false, GuestAllowed: true}
	s.hosts["0459999999"] = Host{ID: "busy-test", Phone: "0459999999", Name: "POPULAR TEST BBS", Region: "神奈川県", Software: "BIG-Model compatible", Lines: 1, Popularity: 1.0, MaxBaud: 14400, Members: 912, ANSI: true, GuestAllowed: true}
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

func (s *MemoryStore) ListPosts(hostID string) []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.posts[hostID]
	out := make([]Post, len(p))
	copy(out, p)
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
