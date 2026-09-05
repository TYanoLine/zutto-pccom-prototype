package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type debugExportPersona struct {
	ID                  string             `json:"id"`
	Handle              string             `json:"handle"`
	Age                 int                `json:"age"`
	Gender              string             `json:"gender"`
	Occupation          string             `json:"occupation"`
	ActivityPattern     string             `json:"activity_pattern"`
	ReplyTendency       float64            `json:"reply_tendency"`
	ThreadStartTendency float64            `json:"thread_start_tendency"`
	LurkerTendency      float64            `json:"lurker_tendency"`
	NewcomerOpenness    float64            `json:"newcomer_openness"`
	Argumentativeness   float64            `json:"argumentativeness"`
	WritingStyle        string             `json:"writing_style"`
	EverydayContext     []string           `json:"everyday_context,omitempty"`
	Interests           map[string]float64 `json:"interests,omitempty"`
	Opinions            map[string]float64 `json:"opinions,omitempty"`
}

type debugExportPost struct {
	ID              int64            `json:"id"`
	BoardID         string           `json:"board_id,omitempty"`
	ParentID        int64            `json:"parent_id,omitempty"`
	Author          string           `json:"author"`
	AuthorPersonaID string           `json:"author_persona_id,omitempty"`
	Subject         string           `json:"subject"`
	Intent          world.PostIntent `json:"intent,omitempty"`
	Body            string           `json:"body,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

type debugExportCounts struct {
	Boards         int `json:"boards"`
	Personas       int `json:"personas"`
	PersonaFacts   int `json:"persona_facts"`
	TotalHostPosts int `json:"total_host_posts"`
	ReturnedPosts  int `json:"returned_posts"`
}

type debugRuntimeExport struct {
	SchemaVersion int                           `json:"schema_version"`
	ReadOnly      bool                          `json:"read_only"`
	Scope         string                        `json:"scope"`
	GeneratedAt   time.Time                     `json:"generated_at"`
	Host          world.Host                    `json:"host"`
	BoardFilter   string                        `json:"board_filter,omitempty"`
	BodiesIncluded bool                         `json:"bodies_included"`
	Boards        []world.Board                 `json:"boards"`
	Personas      []debugExportPersona           `json:"personas"`
	PersonaFacts  map[string][]world.PersonaFact `json:"persona_facts"`
	Posts         []debugExportPost              `json:"posts"`
	Counts        debugExportCounts              `json:"counts"`
	Note          string                         `json:"note"`
}

// newDebugExportHandler exposes a development-only, read-only snapshot of the
// already materialized in-memory runtime state. It deliberately reads the base
// store directly: exporting must never trigger world catch-up, an LLM call, or
// any other materialization side effect.
func newDebugExportHandler(store *world.MemoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
			return
		}

		phone := strings.TrimSpace(r.URL.Query().Get("phone"))
		if phone == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "phone query parameter is required"})
			return
		}
		host, err := store.HostByPhone(phone)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "host not found"})
			return
		}

		boardFilter := strings.TrimSpace(r.URL.Query().Get("board"))
		full := r.URL.Query().Get("full") == "1" || strings.EqualFold(r.URL.Query().Get("full"), "true")
		boards := store.ListBoards(host.ID)
		personas := store.ListHostPersonas(host.ID)
		sort.SliceStable(personas, func(i, j int) bool {
			if personas[i].Handle == personas[j].Handle {
				return personas[i].ID < personas[j].ID
			}
			return personas[i].Handle < personas[j].Handle
		})

		exportPersonas := make([]debugExportPersona, 0, len(personas))
		personaFacts := make(map[string][]world.PersonaFact, len(personas))
		factCount := 0
		for _, persona := range personas {
			exportPersonas = append(exportPersonas, debugExportPersona{
				ID:                  persona.ID,
				Handle:              persona.Handle,
				Age:                 persona.Age,
				Gender:              persona.Gender,
				Occupation:          persona.Occupation,
				ActivityPattern:     persona.ActivityPattern,
				ReplyTendency:       persona.ReplyTendency,
				ThreadStartTendency: persona.ThreadStartTendency,
				LurkerTendency:      persona.LurkerTendency,
				NewcomerOpenness:    persona.NewcomerOpenness,
				Argumentativeness:   persona.Argumentativeness,
				WritingStyle:        persona.WritingStyle,
				EverydayContext:     persona.EverydayContext,
				Interests:           persona.Interests,
				Opinions:            persona.Opinions,
			})
			facts := store.ListPersonaFacts(persona.ID)
			if len(facts) > 0 {
				personaFacts[persona.ID] = facts
				factCount += len(facts)
			}
		}

		allPosts := store.ListPosts(host.ID)
		sort.SliceStable(allPosts, func(i, j int) bool {
			if allPosts[i].CreatedAt.Equal(allPosts[j].CreatedAt) {
				return allPosts[i].ID < allPosts[j].ID
			}
			return allPosts[i].CreatedAt.Before(allPosts[j].CreatedAt)
		})
		posts := make([]debugExportPost, 0, len(allPosts))
		for _, post := range allPosts {
			if boardFilter != "" && post.BoardID != boardFilter {
				continue
			}
			body := ""
			if full {
				body = post.Body
			}
			posts = append(posts, debugExportPost{
				ID:              post.ID,
				BoardID:         post.BoardID,
				ParentID:        post.ParentID,
				Author:          post.Author,
				AuthorPersonaID: post.AuthorPersonaID,
				Subject:         post.Subject,
				Intent:          post.Intent,
				Body:            body,
				CreatedAt:       post.CreatedAt,
			})
		}

		result := debugRuntimeExport{
			SchemaVersion:  1,
			ReadOnly:       true,
			Scope:          "current-server-memory",
			GeneratedAt:    time.Now().UTC(),
			Host:           host,
			BoardFilter:    boardFilter,
			BodiesIncluded: full,
			Boards:         boards,
			Personas:       exportPersonas,
			PersonaFacts:   personaFacts,
			Posts:          posts,
			Counts: debugExportCounts{
				Boards:         len(boards),
				Personas:       len(personas),
				PersonaFacts:   factCount,
				TotalHostPosts: len(allPosts),
				ReturnedPosts:  len(posts),
			},
			Note: "Development diagnostic snapshot only. This endpoint never materializes or mutates world state and contains no user/session credentials.",
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}
