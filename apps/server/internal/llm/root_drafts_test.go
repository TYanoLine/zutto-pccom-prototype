package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRootDraftsPromptAndSchemaProperties(t *testing.T) {
	events := []BBSWorldWindowEvent{
		{
			EventID:      "evt-1",
			BoardID:      "1",
			BoardName:    "パソコン・ハード",
			BoardScope:   "ハードウェアの改造や購入相談",
			AuthorHandle: "TEST_USER",
			CreatedAt:    "1996-08-26T22:00:00+09:00",
		},
	}

	req := BBSRootDraftRequest{
		HostName:        "テスト局",
		HostRegion:      "福岡",
		BoardID:         "1",
		BoardName:       "パソコン・ハード",
		BoardScope:      "ハードウェアの改造や購入相談",
		WorldDate:       "1996-08-26",
		Events:          events,
		RecentSubjects:  []string{"モデムの初期化文字列について"},
		FormSeed:        "seed-1",
		RecentFormFacts: []string{"recent titles are mostly short"},
		MultiVariant:    true,
	}

	prompt, err := buildRootDraftPrompt(req)
	if err != nil {
		t.Fatalf("buildRootDraftPrompt failed: %v", err)
	}

	// Verify essential prompt instructions
	requiredSnippets := []string{
		"対象の名前（作品名・機種名・場所など）を、先頭でなくてよいので題名のどこかに入れてください",
		"36文字以内・1行・Re:なし",
		"recent/avoid material は文脈の参考です。同じ話題や続きの話が出てもかまいません",
		"形や話題は、板の範囲の中で自然に混ざればよい。無理に散らさなくてよい",
		"別の分野の題名で、話題の手本ではありません",
		"最大3案",
		"盛り上がりの段階 (appeal: 0〜4)",
		"すべてのルートを中位（2）に偏らせず",
		"active_residents",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("prompt missing expected snippet %q", snippet)
		}
	}

	// Schema test: properties under each event must include titles and reply appeal fields
	schema := bbsRootDraftSchema(events)
	rootsProp := schema["properties"].(map[string]any)["roots"].(map[string]any)
	evt1Prop := rootsProp["properties"].(map[string]any)["evt-1"].(map[string]any)
	fields := evt1Prop["properties"].(map[string]any)
	reqFields := evt1Prop["required"].([]string)

	if _, ok := fields["titles"]; !ok {
		t.Fatal("schema missing 'titles' property")
	}
	if _, ok := fields["reply_appeal"]; !ok {
		t.Fatal("schema missing 'reply_appeal' property")
	}
	if _, ok := fields["reply_appeal_reason"]; !ok {
		t.Fatal("schema missing 'reply_appeal_reason' property")
	}

	// Verify required includes titles, then reply_appeal, then reply_appeal_reason
	titlesIdx := -1
	appealIdx := -1
	reasonIdx := -1
	for i, f := range reqFields {
		switch f {
		case "titles":
			titlesIdx = i
		case "reply_appeal":
			appealIdx = i
		case "reply_appeal_reason":
			reasonIdx = i
		}
	}
	if titlesIdx < 0 || appealIdx < 0 || reasonIdx < 0 {
		t.Fatalf("missing expected required fields in %v", reqFields)
	}
	if !(titlesIdx < appealIdx && appealIdx < reasonIdx) {
		t.Fatalf("expected order titles < reply_appeal < reply_appeal_reason, got titles=%d, appeal=%d, reason=%d in %v",
			titlesIdx, appealIdx, reasonIdx, reqFields)
	}
}

func TestRootDraftsReplyAppealFallbackAndSuccess(t *testing.T) {
	cases := []struct {
		name              string
		appealJSON        string
		expectedAppeal    int
		expectedHasAppeal bool
		expectedFallback  bool
	}{
		{
			name:              "valid appeal",
			appealJSON:        `"reply_appeal": 3, "reply_appeal_reason": "共感を呼ぶ話題",`,
			expectedAppeal:    3,
			expectedHasAppeal: true,
			expectedFallback:  false,
		},
		{
			name:              "missing appeal",
			appealJSON:        "",
			expectedAppeal:    2,
			expectedHasAppeal: false,
			expectedFallback:  true,
		},
		{
			name:              "out of bounds appeal high",
			appealJSON:        `"reply_appeal": 5, "reply_appeal_reason": "too high",`,
			expectedAppeal:    2,
			expectedHasAppeal: false,
			expectedFallback:  true,
		},
		{
			name:              "out of bounds appeal low",
			appealJSON:        `"reply_appeal": -1, "reply_appeal_reason": "too low",`,
			expectedAppeal:    2,
			expectedHasAppeal: false,
			expectedFallback:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responseJSON := fmt.Sprintf(`{
				"roots": {
					"evt-1": {
						"object_class": "PC-9821",
						"occurrence": "内蔵Cバススロットに新しいCバスボードを装着した",
						"post_content": "PC-9821Xa7へのSCSIボード増設手順を共有する",
						"novelty_key": "pc9821_scsi_board_add",
						"must_not": [],
						"titles": ["PC-9821Xa7にSCSI増設"],
						%s
						"dummy": 0
					}
				}
			}`, tc.appealJSON)
			apiResponse := `{
				"model":"gpt-test-structured",
				"output":[{"content":[{"type":"output_text","text":` + string(mustJSON(responseJSON)) + `}]}],
				"usage":{"input_tokens":120,"output_tokens":80,"total_tokens":200}
			}`

			provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
				Endpoint: "https://test.openai.azure.com",
				APIKey:   "test-key",
				Model:    "gpt-test",
				Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(apiResponse)),
					}, nil
				})},
			}}

			req := BBSRootDraftRequest{
				HostName:     "テスト局",
				BoardID:      "1",
				BoardName:    "PC-98",
				WorldDate:    "1996-08-26",
				MultiVariant: false,
				Events: []BBSWorldWindowEvent{
					{EventID: "evt-1", Action: "thread_start", BoardID: "1", BoardName: "PC-98", AuthorHandle: "USER1"},
				},
			}

			result, err := provider.GenerateBBSRootDrafts(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
			draft, ok := result.Roots["evt-1"]
			if !ok {
				t.Fatalf("missing evt-1 for %s", tc.name)
			}
			if draft.ReplyAppeal != tc.expectedAppeal {
				t.Fatalf("expected appeal %d, got %d", tc.expectedAppeal, draft.ReplyAppeal)
			}
			if draft.HasReplyAppeal != tc.expectedHasAppeal {
				t.Fatalf("expected hasReplyAppeal %v, got %v", tc.expectedHasAppeal, draft.HasReplyAppeal)
			}
			if tc.expectedFallback && !strings.HasPrefix(draft.ReplyAppealReason, "fallback:") {
				t.Fatalf("expected reason starting with fallback:, got %q", draft.ReplyAppealReason)
			}
			if !tc.expectedFallback && strings.HasPrefix(draft.ReplyAppealReason, "fallback:") {
				t.Fatalf("unexpected fallback reason: %q", draft.ReplyAppealReason)
			}
		})
	}
}

func TestRootDraftsSuccessAndMultiVariantCap(t *testing.T) {
	// Response with 4 titles (must be capped at maxTitleVariants = 3)
	responseJSON := `{
		"roots": {
			"evt-1": {
				"object_class": "PC-9821",
				"occurrence": "内蔵Cバススロットに新しいCバスボードを装着した",
				"post_content": "PC-9821Xa7へのSCSIボード増設手順を共有する",
				"novelty_key": "pc9821_scsi_board_add",
				"must_not": ["現代のPCIやUSBを混入しない"],
				"titles": [
					"PC-9821Xa7にSCSIボードを増設",
					"内蔵CバスへのSCSI増設レポート",
					"Xa7でSCSI機器を動かす手順",
					"余分な4つ目のタイトル案"
				]
			}
		}
	}`
	apiResponse := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":` + string(mustJSON(responseJSON)) + `}]}],
		"usage":{"input_tokens":120,"output_tokens":80,"total_tokens":200}
	}`

	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(apiResponse)),
			}, nil
		})},
	}}

	req := BBSRootDraftRequest{
		HostName:     "テスト局",
		BoardID:      "1",
		BoardName:    "PC-98",
		WorldDate:    "1996-08-26",
		MultiVariant: true,
		Events: []BBSWorldWindowEvent{
			{EventID: "evt-1", Action: "thread_start", BoardID: "1", BoardName: "PC-98", AuthorHandle: "USER1"},
		},
	}

	result, err := provider.GenerateBBSRootDrafts(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	draft, ok := result.Roots["evt-1"]
	if !ok {
		t.Fatal("expected draft for evt-1")
	}
	if draft.ObjectClass != "PC-9821" {
		t.Fatalf("unexpected object_class: %s", draft.ObjectClass)
	}
	if len(draft.Titles) != 3 {
		t.Fatalf("expected titles capped at 3, got %d: %v", len(draft.Titles), draft.Titles)
	}
	if draft.Titles[0] != "PC-9821Xa7にSCSIボードを増設" {
		t.Fatalf("unexpected title[0]: %s", draft.Titles[0])
	}
}

func TestRootDraftsRejectsInvalidTitles(t *testing.T) {
	// Root with only invalid titles (empty, too long, Re: prefix)
	responseJSON := `{
		"roots": {
			"evt-1": {
				"object_class": "PC-9821",
				"occurrence": "トラブル発生",
				"post_content": "画面が映らない相談",
				"novelty_key": "pc98_display_trouble",
				"must_not": [],
				"titles": [
					"",
					"Re: 画面が映りません",
					"このタイトルは非常に長くて三拾六文字の制限を大幅に超えてしまっているため絶対に受理されてはならない無効な件名です"
				]
			}
		}
	}`
	apiResponse := `{
		"model":"gpt-test-structured",
		"output":[{"content":[{"type":"output_text","text":` + string(mustJSON(responseJSON)) + `}]}],
		"usage":{"input_tokens":100,"output_tokens":60,"total_tokens":160}
	}`

	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{
		Endpoint: "https://test.openai.azure.com",
		APIKey:   "test-key",
		Model:    "gpt-test",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(apiResponse)),
			}, nil
		})},
	}}

	req := BBSRootDraftRequest{
		HostName:     "テスト局",
		BoardID:      "1",
		BoardName:    "PC-98",
		WorldDate:    "1996-08-26",
		MultiVariant: true,
		Events: []BBSWorldWindowEvent{
			{EventID: "evt-1", Action: "thread_start", BoardID: "1", BoardName: "PC-98", AuthorHandle: "USER1"},
		},
	}

	_, err := provider.GenerateBBSRootDrafts(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when all titles are invalid")
	}
	if !strings.Contains(err.Error(), "no valid titles") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
