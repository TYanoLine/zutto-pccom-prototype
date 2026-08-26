package world

import "time"

type Host struct {
	ID             string  `json:"id"`
	Phone          string  `json:"phone"`
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Software       string  `json:"software"`
	Lines          int     `json:"lines"`
	Popularity     float64 `json:"popularity"`
	MaxBaud        int     `json:"max_baud"`
	Members        int     `json:"members"`
	ANSI           bool    `json:"ansi"`
	GuestAllowed   bool    `json:"guest_allowed"`
	TelehoFriendly bool    `json:"teleho_friendly"`
}

type Post struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Persona struct {
	Handle           string
	Age              int
	Gender           string
	Occupation       string
	ReplyTendency    float64
	LurkerTendency   float64
	NewcomerOpenness float64
	Interests        map[string]float64
	Opinions         map[string]float64
}
