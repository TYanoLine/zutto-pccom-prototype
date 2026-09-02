package world

import "time"

type Host struct {
	ID             string  `json:"id"`
	Phone          string  `json:"phone"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Software       string  `json:"software"`
	SoftwareID     string  `json:"software_id,omitempty"`
	Lines          int     `json:"lines"`
	Popularity     float64 `json:"popularity"`
	MaxBaud        int     `json:"max_baud"`
	Members        int     `json:"members"`
	ANSI           bool    `json:"ansi"`
	GuestAllowed   bool    `json:"guest_allowed"`
	TelehoFriendly bool    `json:"teleho_friendly"`
}

type Board struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PostIntent struct {
	Action     string   `json:"action,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Motivation string   `json:"motivation,omitempty"`
	Stance     string   `json:"stance,omitempty"`
	Claims     []string `json:"claims,omitempty"`
}

type Post struct {
	ID              int64      `json:"id"`
	BoardID         string     `json:"board_id,omitempty"`
	ParentID        int64      `json:"parent_id,omitempty"`
	Author          string     `json:"author"`
	AuthorPersonaID string     `json:"author_persona_id,omitempty"`
	Subject         string     `json:"subject"`
	Intent          PostIntent `json:"intent,omitempty"`
	Body            string     `json:"body"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Persona struct {
	ID                  string
	Handle              string
	Age                 int
	Gender              string
	Occupation          string
	ActivityPattern     string
	ReplyTendency       float64
	ThreadStartTendency float64
	LurkerTendency      float64
	NewcomerOpenness    float64
	Argumentativeness   float64
	WritingStyle        string
	Interests           map[string]float64
	Opinions            map[string]float64
}
