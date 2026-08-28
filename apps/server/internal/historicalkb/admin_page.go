package historicalkb

const AdminPageHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Historical Research Maintenance</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#f4f4f1;color:#222}.wrap{max-width:1100px;margin:auto;padding:20px}.bar,.card{background:#fff;border:1px solid #ccc;border-radius:8px;padding:14px;margin-bottom:14px}.grid{display:grid;grid-template-columns:340px 1fr;gap:14px}input,textarea,select,button{font:inherit;padding:9px;border:1px solid #aaa;border-radius:5px;box-sizing:border-box}input,textarea,select{width:100%}button{cursor:pointer;background:#222;color:#fff}.case{display:block;width:100%;text-align:left;background:#fff;color:#222;margin:6px 0}.case.active{outline:2px solid #222}.muted{color:#666;font-size:.9em}.msg{white-space:pre-wrap;border-left:3px solid #aaa;padding:8px 10px;margin:8px 0;background:#fafafa}.operator{border-left-color:#222}.row{display:flex;gap:8px;align-items:center}.row>*{flex:1}.sources a{display:block;word-break:break-all}.badge{display:inline-block;padding:2px 7px;border:1px solid #999;border-radius:999px;font-size:.8em}@media(max-width:760px){.grid{grid-template-columns:1fr}.row{flex-direction:column;align-items:stretch}}
</style></head><body><div class="wrap">
<h1>Historical Research Maintenance</h1>
<div class="bar"><label>Admin / debug token <input id="token" type="password" autocomplete="off"></label><p class="muted">PoCでは DEBUG_RESET_TOKEN を運営APIの認証にも利用します。</p></div>
<div class="card"><h2>新規調査</h2><div class="row"><input id="topic" placeholder="例: 1996年夏のモデム事情"><input id="question" placeholder="調べたいこと"></div><p><button id="create">Web調査して案件を作成</button></p></div>
<div class="grid"><section class="card"><h2>案件</h2><button id="reload">更新</button><div id="cases"></div></section>
<section class="card"><div id="empty">案件を選択してください。</div><div id="detail" hidden>
<h2 id="title"></h2><p><span id="status" class="badge"></span> confidence <strong id="confidence"></strong></p>
<p id="summary"></p><h3>暫定結論</h3><div id="answer" class="msg"></div><h3>不足情報</h3><ul id="missing"></ul><h3>Sources</h3><div id="sources" class="sources"></div>
<h3>運営チャット / 再調査</h3><div id="messages"></div><textarea id="chat" rows="3" placeholder="例: 九州全体まで範囲を広げて再調査して"></textarea><p><button id="send">送信して再調査</button></p>
<h3>運営による最終補完</h3><textarea id="supplement" rows="5" placeholder="最終的に採用する内容を入力"></textarea><p><button id="applySupplement">この内容で運営確認済みにする</button></p>
<div class="row"><select id="newStatus"><option value="needs_review">needs_review</option><option value="provisional">provisional</option><option value="operator_verified">operator_verified</option><option value="canonical">canonical</option><option value="rejected">rejected</option></select><button id="setStatus">状態変更</button></div>
</div></section></div><p id="notice" class="muted"></p></div>
<script>
(()=>{let selected=null;const $=id=>document.getElementById(id);const headers=()=>({'Content-Type':'application/json','X-Zutto-Debug-Token':$('token').value});
const api=async(url,opt={})=>{const r=await fetch(url,{...opt,headers:{...headers(),...(opt.headers||{})}});const t=await r.text();let d={};try{d=JSON.parse(t)}catch{d={error:t}}if(!r.ok)throw new Error(d.error||r.statusText);return d};
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
async function load(){try{const d=await api('/api/admin/research');$('cases').innerHTML=d.cases.map(c=>'<button class="case '+(selected===c.id?'active':'')+'" data-id="'+esc(c.id)+'"><b>'+esc(c.topic)+'</b><br><span class="muted">'+esc(c.status)+' / '+esc(c.worldDate)+'</span></button>').join('');document.querySelectorAll('.case').forEach(b=>b.addEventListener('click',()=>openCase(b.dataset.id)))}catch(e){$('notice').textContent=e.message}}
async function openCase(id){try{selected=id;const c=await api('/api/admin/research/case?id='+encodeURIComponent(id));render(c);load()}catch(e){$('notice').textContent=e.message}}
function render(c){$('empty').hidden=true;$('detail').hidden=false;$('title').textContent=c.topic+' — '+c.question;$('status').textContent=c.status;$('confidence').textContent=(c.confidence*100).toFixed(0)+'%';$('summary').textContent=c.summary;$('answer').textContent=c.provisionalAnswer||'(未補完)';$('missing').innerHTML=(c.missingInfo||[]).map(x=>'<li>'+esc(x)+'</li>').join('')||'<li>なし</li>';$('sources').innerHTML=(c.sources||[]).map(s=>'<a target="_blank" rel="noreferrer" href="'+esc(s.url)+'">'+esc(s.title||s.url)+'</a>').join('')||'出典なし';$('messages').innerHTML=(c.messages||[]).map(m=>'<div class="msg '+(m.role==='operator'?'operator':'')+'"><b>'+esc(m.role)+'</b><br>'+esc(m.body)+'</div>').join('');$('supplement').value=c.provisionalAnswer||'';$('newStatus').value=c.status;}
$('reload').addEventListener('click',load);$('create').addEventListener('click',async()=>{try{const c=await api('/api/admin/research/new',{method:'POST',body:JSON.stringify({topic:$('topic').value,question:$('question').value})});selected=c.id;render(c);load()}catch(e){$('notice').textContent=e.message}});
$('send').addEventListener('click',async()=>{if(!selected)return;try{const c=await api('/api/admin/research/chat?id='+encodeURIComponent(selected),{method:'POST',body:JSON.stringify({message:$('chat').value})});$('chat').value='';render(c);load()}catch(e){$('notice').textContent=e.message}});
$('applySupplement').addEventListener('click',async()=>{if(!selected)return;try{const c=await api('/api/admin/research/supplement?id='+encodeURIComponent(selected),{method:'POST',body:JSON.stringify({supplement:$('supplement').value})});render(c);load()}catch(e){$('notice').textContent=e.message}});
$('setStatus').addEventListener('click',async()=>{if(!selected)return;try{const c=await api('/api/admin/research/status?id='+encodeURIComponent(selected),{method:'POST',body:JSON.stringify({status:$('newStatus').value})});render(c);load()}catch(e){$('notice').textContent=e.message}});load();})();
</script></body></html>`
