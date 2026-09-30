package main

import "net/http"

func newPersonaHistoryPocViewerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(personaHistoryPocViewerHTML))
	}
}

const personaHistoryPocViewerHTML = `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Persona History Growth PoC</title>
<style>
:root{color-scheme:dark}*{box-sizing:border-box}body{margin:0;background:#07110b;color:#d9f4df;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
main{max-width:1280px;margin:auto;padding:24px}.eyebrow{font-size:12px;letter-spacing:.16em;color:#6fe39a}h1{font-size:29px;margin:7px 0 10px}p{color:#9ab5a0;line-height:1.65}
.top{display:flex;justify-content:space-between;align-items:flex-start;gap:20px}.top a{color:#9fffc0;text-decoration:none;border:1px solid #397a53;padding:8px 10px;background:#0a1a11;white-space:nowrap}
.controls,.panel,.profileCard,.roundCard,.historyItem,.metric{border:1px solid #284b36;background:#0a1710}.controls{padding:14px;margin:18px 0;display:flex;gap:12px;align-items:center;flex-wrap:wrap}
button{background:#12301e;color:#e8ffed;border:1px solid #4a9362;padding:10px 14px;font:inherit;cursor:pointer}button:disabled{opacity:.45;cursor:not-allowed}.status{font-size:12px;color:#8fb69a}
.metrics{display:grid;grid-template-columns:repeat(5,1fr);gap:10px;margin-bottom:18px}.metric{padding:12px}.metric span{display:block;font-size:10px;color:#70927a}.metric b{display:block;font-size:19px;color:#f0fff3;margin:7px 0 3px}.metric small{color:#789081}
.profiles{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px;margin:0 0 18px}.profileCard{padding:12px}.profileCard h3{margin:0 0 5px;font-size:17px;color:#effff2}.profileCard .id{font-size:10px;color:#63856c}.profileCard dl{margin:10px 0 0}.profileCard dt{font-size:10px;color:#698773;margin-top:8px}.profileCard dd{margin:3px 0 0;line-height:1.45;color:#c8e4cf;font-size:12px}
.panel{padding:17px;margin-bottom:18px}.panel h2{margin:0 0 8px;font-size:18px}.panelHead{display:flex;justify-content:space-between;align-items:center;gap:12px}.badge{font-size:10px;padding:4px 7px;border:1px solid #397a53;color:#87e9a2}
.personTabs{display:flex;gap:7px;flex-wrap:wrap;margin:12px 0}.personTabs button{background:#09170f;border-color:#2e6040;padding:7px 10px}.personTabs button.active{background:#1a452b}
.timeline{display:grid;gap:12px}.roundCard{padding:14px}.roundTop{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}.roundTop h3{margin:0;font-size:16px}.board{font-size:11px;color:#7eb58d}.situation{font-size:12px;color:#839e89;margin:7px 0 12px;line-height:1.5}
.post{background:#050c08;border:1px solid #1b3927;padding:13px}.subject{color:#f2fff4;font-weight:bold;margin-bottom:8px}.body{white-space:pre-wrap;line-height:1.7;color:#d2eed8}
.changes{margin-top:12px}.changes h4{font-size:12px;color:#82bb91;margin:0 0 7px}.decision{display:grid;grid-template-columns:auto 1fr;gap:8px 10px;padding:8px 0;border-top:1px solid #173522}.decision:first-of-type{border-top:0}.decision .mark{font-size:10px;padding:3px 5px;height:max-content}.decision.accepted .mark{color:#87efa4;border:1px solid #397a53}.decision.rejected{opacity:.55}.decision.rejected .mark{color:#e9b383;border:1px solid #7d5b37}.decision b{display:block;color:#dfffe6;font-size:12px}.decision code{display:block;color:#74ab82;font-size:10px;margin:2px 0}.evidence{font-size:11px;color:#a5bdaa}.reason{font-size:10px;color:#718679;margin-top:3px}
.historyNow{margin-top:16px;border-top:1px solid #274b34;padding-top:12px}.historyNow h3{font-size:14px;margin:0 0 8px}.historyList{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.historyItem{padding:9px}.historyItem .kind{font-size:9px;color:#6f9c7a}.historyItem b{display:block;margin:4px 0;color:#dfffe5;font-size:12px}.historyItem code{display:block;font-size:9px;color:#70a17c}.historyItem small{display:block;color:#839b89;margin-top:4px;line-height:1.45}
.progress{height:8px;background:#061009;border:1px solid #244b34;min-width:240px;flex:1;max-width:500px}.progress i{display:block;height:100%;background:#55a971}.empty{color:#718679;font-style:italic;font-size:12px}.error{border:1px solid #874343;background:#2b1212;color:#ffbaba;padding:10px;margin:12px 0}
.note{font-size:11px;color:#738779;line-height:1.55}
@media(max-width:1000px){.profiles{grid-template-columns:repeat(2,1fr)}.metrics{grid-template-columns:repeat(3,1fr)}}@media(max-width:680px){main{padding:15px}.top{display:block}.top a{display:inline-block;margin-top:10px}.profiles,.metrics,.historyList{grid-template-columns:1fr}.roundTop{display:block}.controls{align-items:stretch}.progress{max-width:none;width:100%}}
</style>
</head>
<body>
<main>
<div class="top">
<div>
<div class="eyebrow">PERSONA HISTORY GROWTH PoC</div>
<h1>5人が「投稿した結果」で人物史を持ちはじめる</h1>
<p>初期状態は年齢・職業・関心・活動傾向だけ。投稿本文はAzure OpenAIで生成し、その本文を別の抽出パスが読み、明示的な自己申告や観測可能な行動だけをhistory候補にします。採用されたhistoryは次ラウンドの投稿生成へ渡されます。</p>
</div>
<a href="/poc/persona-timeline">TIME PoC</a>
</div>

<div class="controls">
<button id="start">新しい3ラウンド実験を開始</button>
<div class="progress"><i id="progressBar" style="width:0%"></i></div>
<span id="status" class="status">読み込み中…</span>
</div>
<div id="error"></div>

<section>
<h2 style="font-size:17px">初期プロフィール（固定fixture）</h2>
<div id="profiles" class="profiles"></div>
</section>

<div id="metrics" class="metrics" style="display:none"></div>

<section class="panel">
<div class="panelHead"><div><h2>投稿 → History更新</h2><p style="margin:0">人物を切り替えて、各ラウンドで何が積み上がったか確認できます。</p></div><span class="badge">2-PASS</span></div>
<div id="tabs" class="personTabs"></div>
<div id="timeline" class="timeline"><div class="empty">実験を開始するとここに投稿とhistory更新が表示されます。</div></div>
<div class="historyNow"><h3>現在までに蓄積したhistory</h3><div id="historyNow" class="historyList"><div class="empty">まだありません。</div></div></div>
</section>

<p class="note">PoC only. 初期5人は比較のため固定しています。投稿内容とhistory候補は固定台本ではありません。history抽出は投稿本文の完全一致evidenceを必須にし、1投稿だけの observed_behavior は弱い観測として扱います。</p>
</main>
<script>
var state=null, selected='P01', timer=null;
function esc(v){return String(v==null?'':v).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function profileById(id){return (state&&state.profiles||[]).find(function(p){return p.id===id})}
function renderProfiles(){
  var profiles=(state&&state.profiles)||[];
  document.getElementById('profiles').innerHTML=profiles.map(function(p){
    return '<article class="profileCard"><h3>'+esc(p.handle)+'</h3><div class="id">'+esc(p.id)+'</div><dl>'+
      '<dt>年齢 / 職業</dt><dd>'+esc(p.age)+'歳 / '+esc(p.occupation)+'</dd>'+
      '<dt>関心</dt><dd>'+esc((p.interests||[]).join(' / '))+'</dd>'+
      '<dt>活動</dt><dd>'+esc(p.activity)+'</dd></dl></article>'
  }).join('');
}
function renderTabs(){
  var profiles=(state&&state.profiles)||[];
  document.getElementById('tabs').innerHTML=profiles.map(function(p){
    return '<button data-id="'+esc(p.id)+'" class="'+(p.id===selected?'active':'')+'">'+esc(p.handle)+'</button>'
  }).join('');
  document.querySelectorAll('#tabs button').forEach(function(b){b.onclick=function(){selected=b.dataset.id;renderAll()}});
}
function getSituation(round,id){return (round.situations||[]).find(function(s){return s.persona_id===id})}
function getPost(round,id){return (round.posts||[]).find(function(p){return p.persona_id===id})}
function renderTimeline(){
  if(!state||!state.rounds||!state.rounds.length){document.getElementById('timeline').innerHTML='<div class="empty">実験を開始するとここに投稿とhistory更新が表示されます。</div>';return}
  var html=state.rounds.map(function(r){
    var sit=getSituation(r,selected), post=getPost(r,selected);
    if(!post)return '';
    var decisions=(r.decisions||[]).filter(function(d){return d.candidate.persona_id===selected});
    var changes=decisions.length?decisions.map(function(d){
      var c=d.candidate;
      return '<div class="decision '+(d.accepted?'accepted':'rejected')+'"><span class="mark">'+(d.accepted?'ACCEPT':'REJECT')+'</span><div>'+
        '<b>'+esc(c.value)+'</b><code>'+esc(c.kind)+' / '+esc(c.key)+' / confidence '+Math.round((c.confidence||0)*100)+'%</code>'+
        '<div class="evidence">evidence: 「'+esc(c.evidence)+'」</div><div class="reason">'+esc(d.reason)+'</div></div></div>'
    }).join(''):'<div class="empty">この投稿から新しいhistoryは抽出されませんでした。</div>';
    return '<article class="roundCard"><div class="roundTop"><h3>Round '+r.round+'</h3><span class="board">'+esc(sit?sit.board:'')+'</span></div>'+
      '<div class="situation">World situation: '+esc(sit?sit.situation:'')+'</div>'+
      '<div class="post"><div class="subject">'+esc(post.subject)+'</div><div class="body">'+esc(post.body)+'</div></div>'+
      '<div class="changes"><h4>この投稿を読んだhistory extractor</h4>'+changes+'</div></article>'
  }).join('');
  document.getElementById('timeline').innerHTML=html||'<div class="empty">まだこの人物の投稿はありません。</div>';
}
function renderHistory(){
  var entries=(state&&state.history&&state.history[selected])||[];
  if(!entries.length){document.getElementById('historyNow').innerHTML='<div class="empty">まだありません。</div>';return}
  document.getElementById('historyNow').innerHTML=entries.map(function(h){
    return '<div class="historyItem"><span class="kind">'+esc(h.kind)+'</span><b>'+esc(h.value)+'</b><code>'+esc(h.key)+'</code>'+
      '<small>round '+esc(h.first_round)+'→'+esc(h.last_round)+' / observations '+esc(h.observations)+' / confidence '+Math.round((h.confidence||0)*100)+'%</small>'+
      '<small>'+esc((h.evidence||[]).join(' / '))+'</small></div>'
  }).join('');
}
function renderMetrics(){
  var m=state&&state.summary;
  if(!m||!state.id){document.getElementById('metrics').style.display='none';return}
  var box=document.getElementById('metrics');box.style.display='grid';
  box.innerHTML=
    '<div class="metric"><span>POST CALLS</span><b>'+esc(m.post_calls||0)+'</b><small>投稿生成pass</small></div>'+
    '<div class="metric"><span>EXTRACT CALLS</span><b>'+esc(m.extract_calls||0)+'</b><small>history抽出pass</small></div>'+
    '<div class="metric"><span>ACCEPTED</span><b>'+esc(m.accepted||0)+'</b><small>history更新</small></div>'+
    '<div class="metric"><span>REJECTED</span><b>'+esc(m.rejected||0)+'</b><small>evidence/矛盾チェック</small></div>'+
    '<div class="metric"><span>TOKENS</span><b>'+Number(m.total_tokens||0).toLocaleString()+'</b><small>'+((m.total_duration_ms||0)/1000).toFixed(1)+' sec</small></div>';
}
function renderAll(){renderProfiles();renderTabs();renderTimeline();renderHistory();renderMetrics();
  var p=state&&state.progress;var pct=p&&p.total_steps?Math.round(p.completed_steps/p.total_steps*100):0;
  document.getElementById('progressBar').style.width=pct+'%';
  document.getElementById('status').textContent=state&&state.status?state.status.toUpperCase()+(state.id?' / '+state.id:''):'IDLE';
  document.getElementById('start').disabled=!!(state&&(state.status==='queued'||state.status==='running'));
  document.getElementById('error').innerHTML=state&&state.error?'<div class="error">'+esc(state.error)+'</div>':'';
}
async function refresh(id){
  var url='/api/debug/persona-history?action=status'+(id?'&id='+encodeURIComponent(id):'');
  var res=await fetch(url,{cache:'no-store'});var j=await res.json();if(!res.ok)throw new Error(j.error||('HTTP '+res.status));
  state=j;if(!state.profiles&&j.status==='idle')state.profiles=j.profiles||[];renderAll();
  if(state.id&&(state.status==='queued'||state.status==='running')){clearTimeout(timer);timer=setTimeout(function(){refresh(state.id).catch(showError)},1200)}
}
async function start(){
  document.getElementById('start').disabled=true;
  var res=await fetch('/api/debug/persona-history?action=start',{method:'POST',cache:'no-store'});var j=await res.json();
  if(!res.ok)throw new Error(j.error||('HTTP '+res.status));state=j;selected='P01';renderAll();refresh(j.id).catch(showError)
}
function showError(e){document.getElementById('error').innerHTML='<div class="error">'+esc(e&&e.message?e.message:String(e))+'</div>';document.getElementById('start').disabled=false}
document.getElementById('start').onclick=function(){start().catch(showError)};
refresh('').catch(showError);
</script>
</body>
</html>`
