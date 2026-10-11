package actormemory

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
)

const testHost = "h"

var (
	mari = world.ActorRef{Kind: "persona", ID: "mari"}
	taro = world.ActorRef{Kind: "member", ID: "TARO"}
)

func at(min int) time.Time { return time.Date(1996, 8, 29, 12, min, 0, 0, time.UTC) }

type seeded struct {
	store                         *world.MemoryStore
	root, reply, nested, otherBrd int64
}

// seed builds a small world: a thread on board 1 between MARI (persona) and TARO
// (member), a separate MARI post on board 2, and a post on another host.
func seed(t *testing.T) seeded {
	t.Helper()
	s := world.NewMemoryStore()
	root := s.AddPost(testHost, world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "mari",
		Subject: "グラボの設定", Body: "GA-98NBはSW2の3番をONにしています。", CreatedAt: at(0)})
	reply := s.AddPost(testHost, world.Post{BoardID: "1", ParentID: root.ID, Author: "TARO", AuthorMemberID: "TARO",
		Subject: "Re: グラボの設定", Body: "ディップスイッチの位置を教えてください。", CreatedAt: at(5)})
	nested := s.AddPost(testHost, world.Post{BoardID: "1", ParentID: reply.ID, Author: "MARI", AuthorPersonaID: "mari",
		Subject: "Re^2: グラボの設定", Body: "基板の右下です。", CreatedAt: at(10)})
	other := s.AddPost(testHost, world.Post{BoardID: "2", Author: "MARI", AuthorPersonaID: "mari",
		Subject: "別の話題", Body: strings.Repeat("あ", 300), CreatedAt: at(15)})
	s.AddPost("elsewhere", world.Post{BoardID: "1", Author: "MARI", AuthorPersonaID: "mari",
		Subject: "別の局の秘密", Body: "この局の人は見えない。", CreatedAt: at(20)})

	ledger := []world.InteractionEvent{
		{EventID: "post:h:" + itoa(reply.ID), HostID: testHost, Channel: "board_reply", Actor: taro,
			AudienceScope: "public_board:h:1", AddressedTo: []world.ActorRef{mari}, ContentRef: itoa(reply.ID), OccurredAt: at(5)},
		{EventID: "post:h:" + itoa(nested.ID), HostID: testHost, Channel: "board_reply", Actor: mari,
			AudienceScope: "public_board:h:1", AddressedTo: []world.ActorRef{taro}, ContentRef: itoa(nested.ID), OccurredAt: at(10)},
		{EventID: "post:h:" + itoa(root.ID), HostID: testHost, Channel: "board_root", Actor: mari,
			AudienceScope: "public_board:h:1", ContentRef: itoa(root.ID), OccurredAt: at(0)},
	}
	// Append oldest first so the store's newest-first listing is deterministic.
	for _, ev := range []world.InteractionEvent{ledger[2], ledger[0], ledger[1]} {
		if err := s.AppendInteractionEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	s.SavePersonaFact(world.PersonaFact{PersonaID: "mari", Key: "pc_model", Topic: "computer", Value: "PC-9821Xa", MaterializedAt: at(1)})
	s.SavePersonaFact(world.PersonaFact{PersonaID: "mari", Key: "home_region", Topic: "location", Value: "福岡市", MaterializedAt: at(2)})
	return seeded{store: s, root: root.ID, reply: reply.ID, nested: nested.ID, otherBrd: other.ID}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func call(name string, args any) llm.ToolCall {
	raw, _ := json.Marshal(args)
	return llm.ToolCall{ID: "c1", Name: name, Arguments: raw}
}

func run(t *testing.T, e *Executor, actor world.ActorRef, name string, args any) string {
	t.Helper()
	res, err := e.Execute(context.Background(), actor.String(), call(name, args))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError || res.Name != name || res.ToolCallID != "c1" {
		t.Fatalf("%s: result=%+v", name, res)
	}
	return res.Content
}

func TestGetMyRecentPostsNewestFirstAndBoardFilter(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)

	got := strings.Split(run(t, e, mari, llm.ToolGetMyRecentPosts, map[string]any{}), "\n")
	if len(got) != 3 || !strings.HasPrefix(got[0], "#"+itoa(w.otherBrd)) ||
		!strings.HasPrefix(got[1], "#"+itoa(w.nested)) || !strings.HasPrefix(got[2], "#"+itoa(w.root)) {
		t.Fatalf("recent posts=%q", got)
	}

	onBoard1 := strings.Split(run(t, e, mari, llm.ToolGetMyRecentPosts, map[string]any{"board_id": "1", "limit": 1}), "\n")
	if len(onBoard1) != 1 || !strings.HasPrefix(onBoard1[0], "#"+itoa(w.nested)) {
		t.Fatalf("board filter + limit=%q", onBoard1)
	}

	taroPosts := run(t, e, taro, llm.ToolGetMyRecentPosts, map[string]any{})
	if strings.Count(taroPosts, "\n") != 0 || !strings.Contains(taroPosts, "ディップスイッチ") {
		t.Fatalf("member's own posts=%q", taroPosts)
	}
}

func TestPostBodiesAreExcerpted(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)
	out := run(t, e, mari, llm.ToolGetMyRecentPosts, map[string]any{"board_id": "2"})
	body := out[strings.Index(out, ": ")+2:]
	if n := len([]rune(body)); n != excerptRunes+1 || !strings.HasSuffix(body, "…") {
		t.Fatalf("excerpt runes=%d body=%q", n, body)
	}
}

func TestGetThreadContextStartsAtRootAndKeepsNewest(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)

	// Asking by a reply id resolves to the same thread; root first, then the newest.
	got := strings.Split(run(t, e, mari, llm.ToolGetThreadContext, map[string]any{"thread_root_id": w.nested, "limit": 2}), "\n")
	if len(got) != 2 || !strings.HasPrefix(got[0], "#"+itoa(w.root)) || !strings.HasPrefix(got[1], "#"+itoa(w.nested)) {
		t.Fatalf("thread=%q", got)
	}

	all := strings.Split(run(t, e, mari, llm.ToolGetThreadContext, map[string]any{"thread_root_id": w.root}), "\n")
	if len(all) != 3 || !strings.HasPrefix(all[1], "#"+itoa(w.reply)) {
		t.Fatalf("whole thread=%q", all)
	}
	if out := run(t, e, mari, llm.ToolGetThreadContext, map[string]any{"thread_root_id": 99999}); !strings.Contains(out, "見つかりません") {
		t.Fatalf("missing thread=%q", out)
	}
}

func TestGetInteractionHistoryWithCounterpart(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)

	for _, id := range []string{"member:TARO", "TARO"} {
		got := strings.Split(run(t, e, mari, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": id}), "\n")
		if len(got) != 2 || !strings.Contains(got[0], "#"+itoa(w.nested)) || !strings.Contains(got[1], "#"+itoa(w.reply)) {
			t.Fatalf("history with %q = %q", id, got)
		}
		if !strings.HasPrefix(got[0], "[board_reply] ") {
			t.Fatalf("channel tag missing: %q", got[0])
		}
	}
	// From TARO's side the same two exchanges are visible.
	if got := run(t, e, taro, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": "persona:mari"}); strings.Count(got, "\n") != 1 {
		t.Fatalf("taro's history=%q", got)
	}
	if got := run(t, e, mari, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": "member:NOBODY"}); got != msgNothing {
		t.Fatalf("no history=%q", got)
	}
}

func TestChatAndMailFiltersAreEmptyNotErrors(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, NewIndex(&fakeEmbedder{}, IndexOptions{}))
	for _, ch := range []string{"chat", "mail", "telegram"} {
		if got := run(t, e, mari, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": "TARO", "channel_filter": ch}); got != msgNothing {
			t.Fatalf("history %s=%q", ch, got)
		}
		if got := run(t, e, mari, llm.ToolSearchMyObservedEvents, map[string]any{"query": "グラボ", "channel_filter": ch}); got != msgNothing {
			t.Fatalf("search %s=%q", ch, got)
		}
	}
	if got := run(t, e, mari, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": "TARO", "channel_filter": "board"}); got == msgNothing {
		t.Fatal("board filter must still return board events")
	}
}

func TestGetMyProfileFactsAndOpenLoops(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)

	all := strings.Split(run(t, e, mari, llm.ToolGetMyProfileFacts, map[string]any{}), "\n")
	if len(all) != 2 || all[0] != "- home_region: 福岡市" || all[1] != "- pc_model: PC-9821Xa" {
		t.Fatalf("facts=%q", all)
	}
	if got := run(t, e, mari, llm.ToolGetMyProfileFacts, map[string]any{"topic": "Computer"}); got != "- pc_model: PC-9821Xa" {
		t.Fatalf("topic filter=%q", got)
	}
	if got := run(t, e, taro, llm.ToolGetMyProfileFacts, map[string]any{}); got != msgNoFacts {
		t.Fatalf("member facts=%q", got)
	}
	if got := run(t, e, mari, llm.ToolGetMyOpenLoops, map[string]any{}); got != msgNoOpenLoops {
		t.Fatalf("open loops=%q", got)
	}
}

func TestLimitIsClamped(t *testing.T) {
	for in, want := range map[int]int{-4: 3, 0: 3, 1: 1, 5: 5, 6: 5, 99: 5} {
		if got := clampLimit(in); got != want {
			t.Errorf("clampLimit(%d)=%d, want %d", in, got, want)
		}
	}
}

func TestToolsNeverReturnAnotherHostsData(t *testing.T) {
	w := seed(t)
	emb := &fakeEmbedder{vecs: map[string][]float32{"グラボ": {1, 0, 0}}}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: 5 * time.Millisecond})
	defer ix.Close()
	ix.Enqueue(Item{EventID: "post:elsewhere:1", HostID: "elsewhere", PostID: 1, BoardID: "1",
		Scope: "public_board:elsewhere:1", Author: mari, Subject: "別の局の秘密", Text: "グラボ", OccurredAt: at(20)})
	waitFor(t, "indexed", func() bool { return ix.Len() == 1 })
	e := NewExecutor(testHost, w.store, ix)

	outputs := []string{
		run(t, e, mari, llm.ToolGetMyRecentPosts, map[string]any{"limit": 5}),
		run(t, e, mari, llm.ToolGetThreadContext, map[string]any{"thread_root_id": w.root, "limit": 5}),
		run(t, e, mari, llm.ToolGetInteractionHistoryWith, map[string]any{"counterpart_id": "TARO", "limit": 5}),
		run(t, e, mari, llm.ToolSearchMyObservedEvents, map[string]any{"query": "グラボ", "limit": 5}),
	}
	for i, out := range outputs {
		if strings.Contains(out, "別の局の秘密") {
			t.Fatalf("tool %d leaked another host's data: %q", i, out)
		}
	}
	if outputs[3] != msgNothing {
		t.Fatalf("search must find nothing on this host, got %q", outputs[3])
	}
}

func TestSearchFindsVisibleItemsOnly(t *testing.T) {
	w := seed(t)
	emb := &fakeEmbedder{vecs: map[string][]float32{
		"グラボ": {1, 0, 0}, "公開記事": {1, 0, 0}, "自分宛の私信": {0.9, 0.1, 0}, "他人の私信": {1, 0, 0},
	}}
	ix := NewIndex(emb, IndexOptions{BatchSize: 10, FlushInterval: 5 * time.Millisecond})
	defer ix.Close()
	base := Item{HostID: testHost, BoardID: "1", OccurredAt: at(0)}
	pub := base
	pub.EventID, pub.PostID, pub.Scope, pub.Author, pub.Text = "e1", 1, "public_board:h:1", taro, "公開記事"
	toMe := base
	toMe.EventID, toMe.PostID, toMe.Scope, toMe.Author, toMe.Addressed, toMe.Text = "e2", 2, "mail:h:x", taro, []world.ActorRef{mari}, "自分宛の私信"
	secret := base
	secret.EventID, secret.PostID, secret.Scope, secret.Author, secret.Text = "e3", 3, "mail:h:y", taro, "他人の私信"
	for _, it := range []Item{pub, toMe, secret} {
		ix.Enqueue(it)
	}
	waitFor(t, "indexed", func() bool { return ix.Len() == 3 })

	e := NewExecutor(testHost, w.store, ix)
	out := run(t, e, mari, llm.ToolSearchMyObservedEvents, map[string]any{"query": "グラボ", "limit": 5})
	if !strings.Contains(out, "公開記事") || !strings.Contains(out, "自分宛の私信") || strings.Contains(out, "他人の私信") {
		t.Fatalf("search=%q", out)
	}
	if strings.Index(out, "公開記事") > strings.Index(out, "自分宛の私信") {
		t.Fatalf("more similar item must come first: %q", out)
	}
}

func TestSearchDegradesToANormalResult(t *testing.T) {
	w := seed(t)

	noIndex := NewExecutor(testHost, w.store, nil)
	if got := run(t, noIndex, mari, llm.ToolSearchMyObservedEvents, map[string]any{"query": "グラボ"}); got != msgSearchDisabled {
		t.Fatalf("nil index=%q", got)
	}

	failing := NewIndex(&fakeEmbedder{failNext: 100}, IndexOptions{})
	defer failing.Close()
	broken := NewExecutor(testHost, w.store, failing)
	if got := run(t, broken, mari, llm.ToolSearchMyObservedEvents, map[string]any{"query": "グラボ"}); got != msgSearchDisabled {
		t.Fatalf("failing embedder=%q", got)
	}
	// The direct-lookup tools are untouched by the search outage.
	if got := run(t, broken, mari, llm.ToolGetMyRecentPosts, map[string]any{}); got == msgNothing || got == msgSearchDisabled {
		t.Fatalf("recent posts affected by search outage: %q", got)
	}
}

func TestExecuteRejectsBadInput(t *testing.T) {
	w := seed(t)
	e := NewExecutor(testHost, w.store, nil)
	cases := map[string]struct {
		actor string
		call  llm.ToolCall
	}{
		"bad actor":       {"mari", call(llm.ToolGetMyRecentPosts, map[string]any{})},
		"unknown kind":    {"robot:x", call(llm.ToolGetMyRecentPosts, map[string]any{})},
		"unknown tool":    {mari.String(), call("get_everything", map[string]any{})},
		"bad json":        {mari.String(), llm.ToolCall{ID: "c", Name: llm.ToolGetMyRecentPosts, Arguments: json.RawMessage(`{`)}},
		"thread no id":    {mari.String(), call(llm.ToolGetThreadContext, map[string]any{})},
		"history no peer": {mari.String(), call(llm.ToolGetInteractionHistoryWith, map[string]any{})},
		"search no query": {mari.String(), call(llm.ToolSearchMyObservedEvents, map[string]any{"query": "  "})},
	}
	for name, c := range cases {
		if _, err := e.Execute(context.Background(), c.actor, c.call); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if res, err := e.Execute(context.Background(), mari.String(), llm.ToolCall{ID: "c", Name: llm.ToolGetMyRecentPosts}); err != nil || res.IsError {
		t.Fatalf("empty arguments must be allowed: res=%+v err=%v", res, err)
	}
}

type countingTurnModel struct {
	calls     int
	toolCalls []llm.ToolCall
}

func (m *countingTurnModel) Complete(_ context.Context, req llm.ToolTurnRequest) (llm.ToolTurnResponse, error) {
	m.calls++
	if len(req.Tools) > 0 {
		return llm.ToolTurnResponse{ToolCalls: m.toolCalls}, nil
	}
	return llm.ToolTurnResponse{Text: "さっき書いた件ですね"}, nil
}

// The executor and the index take no model at all; going through the registry and
// a full tool turn must cost exactly the two turn calls and nothing else.
func TestToolTurnThroughRegistryMakesNoExtraModelCalls(t *testing.T) {
	w := seed(t)
	emb := &fakeEmbedder{}
	ix := NewIndex(emb, IndexOptions{BatchSize: 1, FlushInterval: time.Millisecond})
	defer ix.Close()
	reg := llm.NewToolRegistry(llm.StandardActorTools(), NewExecutor(testHost, w.store, ix))
	model := &countingTurnModel{toolCalls: []llm.ToolCall{{ID: "t1", Name: llm.ToolGetMyRecentPosts, Arguments: json.RawMessage(`{"limit":1}`)}}}

	out, err := llm.RunToolTurn(context.Background(), model, reg, llm.DefaultToolExecutionPolicy(), mari.String(),
		[]llm.ToolTurnMessage{{Role: "user", Content: "さっき掲示板に書いてた件だけど"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.ModelCalls != 2 || model.calls != 2 || out.ToolCallsExecuted != 1 {
		t.Fatalf("outcome=%+v model.calls=%d", out, model.calls)
	}
}
