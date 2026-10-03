package world

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDevelopmentSnapshotRoundTripPreservesMaterializedState(t *testing.T) {
	store := NewMemoryStore()
	host := genericTestHost(store)
	host.Name = "PERSIST TEST"
	store.SaveHost(host)
	store.SaveBoards(host.ID, []Board{{ID: "1", Name: "雑談"}})
	persona := Persona{ID: host.ID + "-p1", Handle: "P1", EverydayContext: []string{"日常"}, Interests: map[string]float64{"local": .8}}
	store.SavePersona(persona)
	store.AddMembership(host.ID, persona.ID)
	store.SavePersonaFact(PersonaFact{PersonaID: persona.ID, Key: "commute.route", Value: "駅まで徒歩", MaterializedAt: time.Date(1996, 8, 20, 1, 2, 3, 0, time.UTC)})
	post := store.AddPost(host.ID, Post{BoardID: "1", Author: "P1", AuthorPersonaID: persona.ID, Subject: "test", Intent: PostIntent{Action: "root", AnchorKey: "local", Claims: []string{"x"}, ArticleDetailsMaterialized: true}, CreatedAt: time.Date(1996, 8, 21, 1, 2, 3, 0, time.UTC)})
	post.Body = "saved body"
	if _, ok := store.UpdatePost(host.ID, post); !ok {
		t.Fatal("update post failed")
	}

	snapshot, ok := store.DevelopmentSnapshot(host.Phone)
	if !ok {
		t.Fatal("snapshot not found")
	}
	if snapshot.NextPostID < post.ID {
		t.Fatalf("next post id=%d, post id=%d", snapshot.NextPostID, post.ID)
	}

	restored := NewMemoryStore()
	if err := restored.RestoreDevelopmentSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	gotHost, _ := restored.HostByPhone(host.Phone)
	if gotHost.Name != host.Name {
		t.Fatalf("host name=%q, want %q", gotHost.Name, host.Name)
	}
	if got := restored.ListBoards(host.ID); len(got) != 1 || got[0].Name != "雑談" {
		t.Fatalf("boards=%+v", got)
	}
	if got := restored.ListHostPersonas(host.ID); len(got) != 1 || got[0].Handle != "P1" {
		t.Fatalf("personas=%+v", got)
	}
	if got := restored.ListPersonaFacts(persona.ID); len(got) != 1 || got[0].Value != "駅まで徒歩" {
		t.Fatalf("facts=%+v", got)
	}
	if got := restored.ListPosts(host.ID); len(got) != 1 || got[0].Body != "saved body" || got[0].Intent.AnchorKey != "local" || !got[0].Intent.ArticleDetailsMaterialized {
		t.Fatalf("posts=%+v", got)
	}

	next := restored.AddPost(host.ID, Post{BoardID: "1", Author: "P1", Subject: "next"})
	if next.ID <= post.ID {
		t.Fatalf("restored next id=%d, want > %d", next.ID, post.ID)
	}
}

func TestLegacyPostIntentWithoutDetailCompletionDefaultsToIncomplete(t *testing.T) {
	var intent PostIntent
	if err := json.Unmarshal([]byte(`{"situation_facts":["article_detail=observation:existing"]}`), &intent); err != nil {
		t.Fatal(err)
	}
	if intent.ArticleDetailsMaterialized {
		t.Fatal("legacy intent without explicit completion must remain incomplete")
	}
	encoded, err := json.Marshal(PostIntent{ArticleDetailsMaterialized: true})
	if err != nil {
		t.Fatal(err)
	}
	var restored PostIntent
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.ArticleDetailsMaterialized {
		t.Fatalf("completion state did not survive JSON round-trip: %s", encoded)
	}
}

func TestDevelopmentSnapshotDetachesPostIntentSlices(t *testing.T) {
	store := NewMemoryStore()
	host := genericTestHost(store)
	post := store.AddPost(host.ID, Post{
		BoardID: "1",
		Author:  "P1",
		Subject: "slice clone",
		Intent: PostIntent{
			SituationFacts:         []string{"fact-1"},
			Claims:                 []string{"claim-1"},
			RespondsToClaims:       []string{"respond-1"},
			ProducerReferents:      []string{"ref-1"},
			ProducerActorKnowledge: []string{"know-1"},
			ProducerAudienceContext: []string{
				"aud-1",
			},
			ProducerContribution: []string{"contrib-1"},
			ProducerMustNot:      []string{"avoid-1"},
			RenderContext:        "transient",
		},
	})

	snapshot, ok := store.DevelopmentSnapshot(host.Phone)
	if !ok {
		t.Fatal("snapshot not found")
	}
	post.Intent.SituationFacts[0] = "mutated-fact"
	post.Intent.Claims[0] = "mutated-claim"
	post.Intent.RespondsToClaims[0] = "mutated-respond"
	post.Intent.ProducerReferents[0] = "mutated-ref"
	post.Intent.ProducerActorKnowledge[0] = "mutated-know"
	post.Intent.ProducerAudienceContext[0] = "mutated-aud"
	post.Intent.ProducerContribution[0] = "mutated-contrib"
	post.Intent.ProducerMustNot[0] = "mutated-avoid"
	if _, ok := store.UpdatePost(host.ID, post); !ok {
		t.Fatal("update post failed")
	}

	got := snapshot.Posts[0].Intent
	for field, want := range map[string]string{
		"situation_facts":           "fact-1",
		"claims":                    "claim-1",
		"responds_to_claims":        "respond-1",
		"producer_referents":        "ref-1",
		"producer_actor_knowledge":  "know-1",
		"producer_audience_context": "aud-1",
		"producer_contribution":     "contrib-1",
		"producer_must_not":         "avoid-1",
	} {
		var gotValue string
		switch field {
		case "situation_facts":
			gotValue = got.SituationFacts[0]
		case "claims":
			gotValue = got.Claims[0]
		case "responds_to_claims":
			gotValue = got.RespondsToClaims[0]
		case "producer_referents":
			gotValue = got.ProducerReferents[0]
		case "producer_actor_knowledge":
			gotValue = got.ProducerActorKnowledge[0]
		case "producer_audience_context":
			gotValue = got.ProducerAudienceContext[0]
		case "producer_contribution":
			gotValue = got.ProducerContribution[0]
		case "producer_must_not":
			gotValue = got.ProducerMustNot[0]
		}
		if gotValue != want {
			t.Fatalf("%s=%q, want %q", field, gotValue, want)
		}
	}
	if got.RenderContext != "" {
		t.Fatalf("render context leaked into snapshot: %q", got.RenderContext)
	}
}
