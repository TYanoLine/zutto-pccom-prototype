package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type personaTimelinePocInterval struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	From       string `json:"from"`
	Until      string `json:"until,omitempty"`
	SourceKind string `json:"source_kind"`
}

type personaTimelinePocEvent struct {
	ID      string `json:"id"`
	At      string `json:"at"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

type personaTimelinePocPerson struct {
	ID               string                       `json:"id"`
	Handle           string                       `json:"handle"`
	BirthDate        string                       `json:"birth_date"`
	BaselineFacts    []string                     `json:"baseline_facts"`
	Occupation       []personaTimelinePocInterval `json:"occupation_history"`
	States           []personaTimelinePocInterval `json:"state_intervals"`
	Events           []personaTimelinePocEvent    `json:"events"`
	ObservationDates []string                     `json:"observation_dates"`
}

type personaTimelinePocSnapshot struct {
	At                 string                       `json:"at"`
	Age                int                          `json:"age"`
	Occupation         string                       `json:"occupation"`
	ActiveStates       []personaTimelinePocInterval `json:"active_states"`
	KnownEvents        []personaTimelinePocEvent    `json:"known_events"`
	HiddenFutureEvents int                          `json:"hidden_future_events"`
	RendererContext    []string                     `json:"renderer_context"`
}

func newPersonaTimelinePocHandler() http.HandlerFunc {
	people := personaTimelinePocPeople()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "GET only"})
			return
		}

		personID := strings.TrimSpace(r.URL.Query().Get("persona"))
		if personID == "" {
			personID = "worker"
		}
		person, ok := people[personID]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown persona"})
			return
		}
		at := strings.TrimSpace(r.URL.Query().Get("at"))
		if at == "" {
			at = person.ObservationDates[0]
		}
		snapshot, err := resolvePersonaTimelinePoc(person, at)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		catalog := make([]map[string]any, 0, len(people))
		keys := make([]string, 0, len(people))
		for id := range people {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			p := people[id]
			catalog = append(catalog, map[string]any{
				"id": id, "handle": p.Handle, "observation_dates": p.ObservationDates,
			})
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"canonical": false,
			"note": "development PoC only; dates and life events are fictional fixtures. The important part is temporal resolution and future isolation.",
			"persona": person,
			"snapshot": snapshot,
			"catalog": catalog,
		})
	}
}

func personaTimelinePocPeople() map[string]personaTimelinePocPerson {
	return map[string]personaTimelinePocPerson{
		"worker": {
			ID: "timeline-worker", Handle: "RYO-5", BirthDate: "1967-03-12",
			BaselineFacts: []string{
				"パソコン通信には時々接続する",
				"ファイルや通信の具体的な話題には反応しやすい",
			},
			Occupation: []personaTimelinePocInterval{
				{Key: "employment.occupation", Value: "会社員", From: "1994-04-01", Until: "1996-09-30", SourceKind: "fixture_history"},
				{Key: "employment.status", Value: "無職", From: "1996-10-01", Until: "1996-10-20", SourceKind: "fixture_event"},
				{Key: "employment.occupation", Value: "販売・サービス業", From: "1996-10-21", SourceKind: "fixture_event"},
			},
			States: []personaTimelinePocInterval{
				{Key: "health.common_cold", Value: "軽い風邪", From: "1996-07-18", Until: "1996-07-23", SourceKind: "fixture_event"},
			},
			Events: []personaTimelinePocEvent{
				{ID: "cold-start", At: "1996-07-18", Kind: "health.started", Summary: "軽い風邪をひいた"},
				{ID: "cold-end", At: "1996-07-24", Kind: "health.ended", Summary: "風邪から回復した"},
				{ID: "job-end", At: "1996-09-30", Kind: "employment.ended", Summary: "勤めていた会社を退職した"},
				{ID: "job-start", At: "1996-10-21", Kind: "employment.started", Summary: "販売・サービス業の仕事を始めた"},
			},
			ObservationDates: []string{"1996-07-20", "1996-08-15", "1996-10-10", "1996-11-01"},
		},
		"student": {
			ID: "timeline-student", Handle: "EMI-2", BirthDate: "1975-11-08",
			BaselineFacts: []string{
				"大学生",
				"ゲームと音楽の話題を読むことが多い",
			},
			Occupation: []personaTimelinePocInterval{
				{Key: "school.role", Value: "大学生", From: "1994-04-01", SourceKind: "fixture_history"},
			},
			States: []personaTimelinePocInterval{
				{Key: "school.calendar", Value: "夏季休業", From: "1996-07-20", Until: "1996-09-15", SourceKind: "fixture_calendar"},
			},
			Events: []personaTimelinePocEvent{
				{ID: "summer-start", At: "1996-07-20", Kind: "school.break.started", Summary: "学校カレンダー上の夏季休業に入った"},
				{ID: "summer-end", At: "1996-09-16", Kind: "school.break.ended", Summary: "夏季休業が終わった"},
			},
			ObservationDates: []string{"1996-07-10", "1996-08-10", "1996-09-20"},
		},
	}
}

func resolvePersonaTimelinePoc(person personaTimelinePocPerson, rawAt string) (personaTimelinePocSnapshot, error) {
	at, err := time.Parse("2006-01-02", rawAt)
	if err != nil {
		return personaTimelinePocSnapshot{}, err
	}
	birth, err := time.Parse("2006-01-02", person.BirthDate)
	if err != nil {
		return personaTimelinePocSnapshot{}, err
	}
	age := at.Year() - birth.Year()
	birthday := time.Date(at.Year(), birth.Month(), birth.Day(), 0, 0, 0, 0, time.UTC)
	if at.Before(birthday) {
		age--
	}

	occupation := "未設定"
	for _, interval := range person.Occupation {
		if timelinePocActive(interval, at) {
			occupation = interval.Value
			break
		}
	}

	active := make([]personaTimelinePocInterval, 0)
	for _, state := range person.States {
		if timelinePocActive(state, at) {
			active = append(active, state)
		}
	}

	known := make([]personaTimelinePocEvent, 0)
	hidden := 0
	for _, event := range person.Events {
		eventAt, parseErr := time.Parse("2006-01-02", event.At)
		if parseErr != nil {
			continue
		}
		if eventAt.After(at) {
			hidden++
			continue
		}
		known = append(known, event)
	}

	context := []string{
		"観測日: " + rawAt,
		"年齢: " + strconv.Itoa(age) + "歳",
		"現在の所属/職業: " + occupation,
	}
	for _, fact := range person.BaselineFacts {
		context = append(context, "既存事実: "+fact)
	}
	for _, state := range active {
		context = append(context, "現在状態: "+state.Value+" ("+state.From+"〜"+timelinePocUntil(state.Until)+")")
	}
	for _, event := range known {
		context = append(context, "既知の過去イベント: "+event.At+" "+event.Summary)
	}

	return personaTimelinePocSnapshot{
		At: rawAt, Age: age, Occupation: occupation, ActiveStates: active,
		KnownEvents: known, HiddenFutureEvents: hidden, RendererContext: context,
	}, nil
}

func timelinePocActive(interval personaTimelinePocInterval, at time.Time) bool {
	from, err := time.Parse("2006-01-02", interval.From)
	if err != nil || at.Before(from) {
		return false
	}
	if interval.Until == "" {
		return true
	}
	until, err := time.Parse("2006-01-02", interval.Until)
	return err == nil && !at.After(until)
}

func timelinePocUntil(value string) string {
	if value == "" {
		return "継続中"
	}
	return value
}


func newPersonaTimelinePocViewerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(personaTimelinePocViewerHTML))
	}
}

const personaTimelinePocViewerHTML = `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Persona Timeline PoC</title>
<style>
:root{color-scheme:dark}*{box-sizing:border-box}body{margin:0;background:#07110b;color:#d9f4df;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
main{max-width:1180px;margin:auto;padding:24px}.eyebrow{font-size:12px;letter-spacing:.16em;color:#6fe39a}h1{font-size:28px;margin:7px 0 8px}p{color:#9ab5a0;line-height:1.65}
.controls,.panel,.metric{border:1px solid #284b36;background:#0a1710}.controls{padding:14px;display:flex;gap:10px;align-items:end;flex-wrap:wrap;margin:18px 0}
label{display:flex;flex-direction:column;gap:6px;font-size:12px;color:#8fb69a}select,input,button{background:#07130d;color:#d9f4df;border:1px solid #397a53;padding:9px;font:inherit}button{cursor:pointer}
.presets{display:flex;gap:6px;flex-wrap:wrap}.metrics{display:grid;grid-template-columns:repeat(5,1fr);gap:10px;margin:0 0 18px}.metric{padding:12px;min-height:94px}.metric span{display:block;font-size:11px;color:#70927a}.metric b{display:block;font-size:20px;margin:9px 0 4px;color:#f0fff3}.metric small{color:#809f87}
.panel{padding:17px;margin:0 0 18px}.panel h2{margin:0 0 10px;font-size:17px}.safe{color:#7ff0a0;border:1px solid #397a53;padding:4px 7px;font-size:11px}.head{display:flex;justify-content:space-between;gap:14px;align-items:flex-start}
pre{white-space:pre-wrap;background:#050c08;border:1px solid #1c3e29;padding:14px;line-height:1.65;overflow:auto}.events{display:grid;gap:8px}.event{display:grid;grid-template-columns:96px 1fr auto;gap:10px;border-top:1px solid #173522;padding-top:9px}.event.future{opacity:.48}.event code{display:block;color:#71a47d;font-size:11px;margin-top:3px}.event .status{font-size:11px;color:#72d58e}.event.future .status{color:#a2aaa4}
table{width:100%;border-collapse:collapse;font-size:13px}th,td{text-align:left;border-top:1px solid #1e402b;padding:8px}th{color:#78a984}code{color:#8ee3a5}.note{font-size:12px;color:#758f7a}
@media(max-width:780px){main{padding:15px}.metrics{grid-template-columns:repeat(2,1fr)}.event{grid-template-columns:86px 1fr}.event .status{grid-column:2}.head{display:block}.safe{display:inline-block;margin-top:8px}}
</style>
</head>
<body>
<main>
<div class="eyebrow">DEVELOPMENT PERSONA TIMELINE PoC</div>
<h1>人物を「ある日付」で解決する</h1>
<p>風邪・夏休み・退職/転職のような時間変化を、期間状態とイベントとして解決します。将来イベントはデバッグ表示には残しますが、投稿rendererへ渡す人物コンテキストからは遮断します。</p>

<div class="controls">
<label>人物<select id="person"><option value="worker">RYO-5 / worker</option><option value="student">EMI-2 / student</option></select></label>
<label>観測日<input id="at" type="date"></label>
<button id="resolve">この日付で解決</button>
<div id="presets" class="presets"></div>
</div>

<div id="metrics" class="metrics"></div>

<section class="panel">
<div class="head"><div><h2>Renderer がこの日に見えるもの</h2><p>この断面だけを投稿本文生成へ渡す想定です。</p></div><span class="safe">FUTURE ISOLATED</span></div>
<pre id="context"></pre>
</section>

<section class="panel">
<h2>デバッグ用・全イベント時間軸</h2>
<p>KNOWN は観測日時点で既知。FUTURE / HIDDEN は世界データ上には存在してもrendererへ入りません。</p>
<div id="events" class="events"></div>
</section>

<section class="panel">
<h2>期間付き状態</h2>
<div style="overflow:auto"><table><thead><tr><th>key</th><th>value</th><th>from</th><th>until</th><th>source</th></tr></thead><tbody id="intervals"></tbody></table></div>
</section>

<div class="note">PoC fixture only — 人生イベント・日付は仕組み確認用の架空データです。</div>
</main>
<script>
var currentData = null;
function esc(v){return String(v == null ? '' : v).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
async function load(person, at){
  var url='/api/debug/persona-timeline?persona='+encodeURIComponent(person);
  if(at) url += '&at='+encodeURIComponent(at);
  var res=await fetch(url,{cache:'no-store'});
  var data=await res.json();
  if(!res.ok){alert(data.error||('HTTP '+res.status));return}
  currentData=data;
  document.getElementById('at').value=data.snapshot.at;
  var selected=data.catalog.find(function(x){return x.id===person});
  var presets=document.getElementById('presets');
  presets.innerHTML='';
  (selected?selected.observation_dates:[]).forEach(function(d){
    var b=document.createElement('button');b.textContent=d;b.onclick=function(){load(person,d)};presets.appendChild(b)
  });
  var states=data.snapshot.active_states.map(function(x){return x.value}).join(' / ')||'なし';
  document.getElementById('metrics').innerHTML=
    '<div class="metric"><span>HANDLE</span><b>'+esc(data.persona.handle)+'</b><small>'+esc(data.persona.id)+'</small></div>'+
    '<div class="metric"><span>OBSERVED AT</span><b>'+esc(data.snapshot.at)+'</b><small>この瞬間の世界断面</small></div>'+
    '<div class="metric"><span>AGE</span><b>'+esc(data.snapshot.age)+'歳</b><small>生年月日から導出</small></div>'+
    '<div class="metric"><span>CURRENT ROLE</span><b>'+esc(data.snapshot.occupation)+'</b><small>期間付き状態から解決</small></div>'+
    '<div class="metric"><span>FUTURE HIDDEN</span><b>'+esc(data.snapshot.hidden_future_events)+'</b><small>'+esc(states)+'</small></div>';
  document.getElementById('context').textContent=data.snapshot.renderer_context.join('\n');
  var known={};data.snapshot.known_events.forEach(function(x){known[x.id]=true});
  document.getElementById('events').innerHTML=data.persona.events.map(function(ev){
    var k=!!known[ev.id];
    return '<div class="event '+(k?'known':'future')+'"><time>'+esc(ev.at)+'</time><div><b>'+esc(ev.summary)+'</b><code>'+esc(ev.kind)+'</code></div><span class="status">'+(k?'KNOWN':'FUTURE / HIDDEN')+'</span></div>'
  }).join('');
  var all=data.persona.occupation_history.concat(data.persona.state_intervals);
  document.getElementById('intervals').innerHTML=all.map(function(x){
    return '<tr><td><code>'+esc(x.key)+'</code></td><td>'+esc(x.value)+'</td><td>'+esc(x.from)+'</td><td>'+esc(x.until||'∞')+'</td><td>'+esc(x.source_kind)+'</td></tr>'
  }).join('');
}
document.getElementById('person').addEventListener('change',function(e){load(e.target.value,'')});
document.getElementById('resolve').addEventListener('click',function(){load(document.getElementById('person').value,document.getElementById('at').value)});
load('worker','1996-07-20');
</script>
</body>
</html>`
