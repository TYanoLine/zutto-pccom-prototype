package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGeneratePersonaProfilesKeepsPresentationBoundary(t *testing.T) {
	response := `{
		"model":"gpt-profile-test",
		"output":[{"content":[{"type":"output_text","text":"{\"profiles\":[{\"id\":\"P00001\",\"distinctive_hook\":\"普段は聞き役だが、質問を受けると急に説明好きになる\",\"core_traits\":[\"慎重\",\"面倒見がよい\",\"長話は苦手\"],\"social_dynamics\":[\"新人には丁寧に接する\",\"意見が割れると一歩引く\"],\"participation_habits\":[\"質問には返事をしやすい\",\"雑談には毎回は入らない\"],\"everyday_context\":[\"夜に落ち着いてから接続することが多い\"],\"voice_notes\":[\"短めの文を重ねる\",\"断定より経験談として書く\"],\"profile\":\"普段は聞き役に回りがちだが、質問を受けると急に説明が細かくなる。新人には丁寧で、わからないことを責めない。意見が割れると深追いせず一歩引く。夜に落ち着いてから接続し、短めの文を重ねる。\"}]}"}]}],
		"usage":{"input_tokens":321,"output_tokens":88,"total_tokens":409}
	}`
	var prompt string
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		APIKey: "test-key",
		Model:  "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			prompt, _ = payload["input"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(response)),
			}, nil
		})},
	}}
	draft, err := provider.GeneratePersonaProfiles(context.Background(), "1996-08-26", []PersonaProfileSeed{{
		ID: "P00001", Handle: "NORI", Age: 27, Occupation: "会社員", ActivityClass: "active",
		TopInterests: []string{"communications", "games"}, StyleTags: []string{"短文", "引用少なめ"},
		ConnectWindow: "23:00-01:00", Quirk: "質問には答えるが雑談には毎回は入らない。",
	}}, []string{"説明が長くなると自分で脱線する"})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Profiles) != 1 || draft.Profiles[0].ID != "P00001" || draft.Profiles[0].Profile == "" || draft.Profiles[0].DistinctiveHook == "" {
		t.Fatalf("unexpected profiles: %+v", draft.Profiles)
	}
	if draft.Usage.Model != "gpt-profile-test" || draft.Usage.TotalTokens != 409 {
		t.Fatalf("unexpected usage: %+v", draft.Usage)
	}
	for _, want := range []string{"WORLD DATE: 1996-08-26", "PRIMARY GOAL — INDIVIDUALITY", "Similar skeletons MUST still become different people", "EXISTING DISTINCTIVE HOOKS TO AVOID", "Never expose internal machine category labels", "NEVER with the handle/id", "exact input occupation string"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}


func TestLocalizePersonaInterestsHidesMachineKeys(t *testing.T) {
	got := localizePersonaInterests([]string{"communications", "games", "local"})
	joined := strings.Join(got, "/")
	if strings.Contains(joined, "communications") || strings.Contains(joined, "games") || strings.Contains(joined, "local") {
		t.Fatalf("machine keys leaked: %q", joined)
	}
	for _, want := range []string{"パソコン通信", "ゲーム", "地域・身近な話題"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("localized interests missing %q: %q", want, joined)
		}
	}
}


func TestStripPersonaHandleOpening(t *testing.T) {
	got := stripPersonaHandleOpening("NORIは、質問されると説明が細かくなる。", "NORI")
	if got != "質問されると説明が細かくなる。" {
		t.Fatalf("unexpected stripped profile: %q", got)
	}
}

func TestValidatePersonaDraftAgainstSeedRejectsOccupationDrift(t *testing.T) {
	seed := PersonaProfileSeed{ID: "P00001", Occupation: "販売・サービス業"}
	draft := PersonaProfileDraft{
		ID: "P00001",
		DistinctiveHook: "返事は早いが雑談では引き際が早い",
		CoreTraits: []string{"実務的", "少しお調子者", "面倒見がよい"},
		SocialDynamics: []string{"新顔にも気軽に話す", "口論は避ける"},
		ParticipationHabits: []string{"質問にはすぐ返す", "雑談は長引かせない"},
		EverydayContext: []string{"週末に長めに接続する"},
		VoiceNotes: []string{"短い挨拶から入る", "くだけた口調"},
		Profile: "人と接する仕事の勢いを掲示板にも持ち込む会社員だ。",
	}
	if err := validatePersonaDraftAgainstSeed(seed, draft); err == nil {
		t.Fatal("expected occupation drift error")
	}
}

func TestValidatePersonaDraftAgainstSeedRejectsMachineKeyLeak(t *testing.T) {
	seed := PersonaProfileSeed{ID: "P00001", Occupation: "会社員"}
	draft := PersonaProfileDraft{
		ID: "P00001",
		DistinctiveHook: "gamesの話だけ急に長くなる",
		CoreTraits: []string{"慎重", "好奇心旺盛", "飽きっぽい"},
		SocialDynamics: []string{"新顔には丁寧", "常連には軽口"},
		ParticipationHabits: []string{"質問に返す", "雑談は読むだけの日もある"},
		EverydayContext: []string{"夜に接続する"},
		VoiceNotes: []string{"短文", "断定を避ける"},
		Profile: "質問には丁寧に返す。",
	}
	if err := validatePersonaDraftAgainstSeed(seed, draft); err == nil {
		t.Fatal("expected machine key leak error")
	}
}


func TestValidatePersonaDraftAgainstSeedAllowsSecondaryJobContext(t *testing.T) {
	seed := PersonaProfileSeed{ID: "P00068", Occupation: "大学生"}
	draft := PersonaProfileDraft{
		ID: "P00068",
		DistinctiveHook: "授業の後は慎重なのにアルバイト帰りだけ急に饒舌になる",
		CoreTraits: []string{"慎重", "好奇心旺盛", "少し気疲れしやすい"},
		SocialDynamics: []string{"新顔には距離を取る", "常連には軽口"},
		ParticipationHabits: []string{"質問に返す", "雑談は読むだけの日もある"},
		EverydayContext: []string{"授業とアルバイトのある日は遅く接続する"},
		VoiceNotes: []string{"短文", "断定を避ける"},
		Profile: "授業の後は静かに読むことが多いが、アルバイト帰りの夜だけは話題へ入りやすい。",
	}
	if err := validatePersonaDraftAgainstSeed(seed, draft); err != nil {
		t.Fatalf("secondary job context should not contradict student occupation: %v", err)
	}
}
