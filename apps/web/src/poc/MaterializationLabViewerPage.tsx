import { useEffect, useMemo, useState } from 'react';

type Board = { id: string; name: string };
type Article = {
  id: number; board_id: string; parent_id?: number; author: string; created_at: string;
  subject: string; body: string; action?: string; anchor_key?: string; cause_kind?: string;
  discourse_mode?: string; source_post_id?: number; responds_to_post_id?: number;
  situation_kind?: string; situation_summary?: string; situation_facts?: string[];
};
type Job = {
  id: string; status: string; situation_mode?: string; board_count?: number; shell_limit?: number;
  boards?: Board[]; created_at: string; finished_at?: string; duration_ms?: number; post_count?: number;
  body_count?: number; failures?: number; usage?: string; situation_diagnostic?: string; articles?: Article[];
};
type JobSummary = Pick<Job, 'id'|'status'|'situation_mode'|'board_count'|'shell_limit'|'created_at'|'finished_at'|'post_count'|'body_count'|'failures'>;

const fallbackBoards: Record<string,string> = {
  '1':'フリートーク','2':'パソコン通信・モデム','3':'地域の話題','4':'ゲーム','5':'音楽','6':'ソフトウェア'
};
const jst = new Intl.DateTimeFormat('ja-JP', { timeZone:'Asia/Tokyo', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', hour12:false });

function fmt(ts?: string) { return ts ? jst.format(new Date(ts)) : '-'; }
function threadPosts(root: Article, articles: Article[]) {
  const out = [root];
  const seen = new Set<number>([root.id]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const post of articles) {
      if (seen.has(post.id) || !post.parent_id || !seen.has(post.parent_id)) continue;
      seen.add(post.id); out.push(post); changed = true;
    }
  }
  return out.sort((a,b) => new Date(a.created_at).getTime()-new Date(b.created_at).getTime() || a.id-b.id);
}

export default function MaterializationLabViewerPage() {
  const [summaries,setSummaries] = useState<JobSummary[]>([]);
  const [job,setJob] = useState<Job|null>(null);
  const [boardID,setBoardID] = useState('');
  const [threadID,setThreadID] = useState<number|undefined>();
  const [debug,setDebug] = useState(false);
  const [error,setError] = useState('');
  const [loading,setLoading] = useState(true);

  async function loadJob(id?: string) {
    setLoading(true); setError('');
    try {
      const qs = id ? `?id=${encodeURIComponent(id)}` : '';
      const res = await fetch(`/api/materialization-lab-viewer${qs}`, { cache:'no-store' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const next: Job = await res.json();
      setJob(next);
      if (id) history.replaceState(null,'',`/poc/materialization-lab-viewer?job=${encodeURIComponent(id)}`);
      const firstBoard = next.boards?.[0]?.id || next.articles?.[0]?.board_id || '1';
      setBoardID(firstBoard); setThreadID(undefined);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); setJob(null); }
    finally { setLoading(false); }
  }

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch('/api/materialization-lab-viewer?list=1&limit=30', { cache:'no-store' });
        if (res.ok) { const data = await res.json(); setSummaries(data.jobs || []); }
      } catch { /* full job fetch below gives the useful error */ }
      const requested = new URLSearchParams(location.search).get('job') || undefined;
      await loadJob(requested);
    })();
  }, []);

  const articles = job?.articles || [];
  const boards = useMemo(() => {
    if (job?.boards?.length) return job.boards;
    const ids = Array.from(new Set(articles.map(a=>a.board_id))).sort();
    return ids.map(id => ({ id, name:fallbackBoards[id] || `BOARD ${id}` }));
  }, [job,articles]);
  const boardArticles = articles.filter(a=>a.board_id===boardID);
  const roots = boardArticles.filter(a=>!a.parent_id).sort((a,b)=>new Date(a.created_at).getTime()-new Date(b.created_at).getTime());
  const selectedRoot = roots.find(r=>r.id===threadID) || roots[0];
  const selectedPosts = selectedRoot ? threadPosts(selectedRoot, boardArticles) : [];

  useEffect(() => { if (selectedRoot && threadID !== selectedRoot.id) setThreadID(selectedRoot.id); }, [boardID, job?.id, selectedRoot?.id]);

  return <div className="labviewer">
    <style>{css}</style>
    <header>
      <div><div className="eyebrow">DEVELOPMENT MATERIALIZATION LAB</div><h1>生成BBS 評価ビュー</h1></div>
      <div className="readonly">READ ONLY</div>
    </header>
    <div className="note">実験用のisolated fresh worldを閲覧しています。ここから書込・返信・生成・RESETはできません。</div>

    <section className="runbar">
      <label>実験run
        <select value={job?.id || ''} onChange={e=>loadJob(e.target.value)}>
          {job && !summaries.some(s=>s.id===job.id) && <option value={job.id}>{job.id}</option>}
          {summaries.map(s=><option key={s.id} value={s.id}>{fmt(s.finished_at || s.created_at)} · {s.situation_mode || '-'} · {s.post_count || 0}件 · {s.id}</option>)}
        </select>
      </label>
      <button onClick={()=>loadJob(job?.id)} disabled={loading}>再読込</button>
      <label className="debug"><input type="checkbox" checked={debug} onChange={e=>setDebug(e.target.checked)}/> 内部Situationを表示</label>
    </section>

    {error && <div className="error">読み込み失敗: {error}</div>}
    {loading && <div className="loading">読み込み中...</div>}
    {job && <>
      <section className="metrics">
        <span>MODE <b>{job.situation_mode || '-'}</b></span><span>BOARDS <b>{job.board_count || boards.length}</b></span>
        <span>POSTS <b>{job.post_count || articles.length}</b></span><span>BODIES <b>{job.body_count || 0}</b></span>
        <span>FAIL <b>{job.failures || 0}</b></span><span>TIME <b>{job.duration_ms ? (job.duration_ms/1000).toFixed(1)+'s' : '-'}</b></span>
      </section>

      <nav className="boards">
        {boards.map(b => <button key={b.id} className={b.id===boardID?'active':''} onClick={()=>{setBoardID(b.id);setThreadID(undefined)}}>
          <small>{b.id}</small>{b.name}<em>{articles.filter(a=>a.board_id===b.id).length}</em>
        </button>)}
      </nav>

      <main>
        <aside className="threads">
          <div className="paneTitle">スレッド一覧</div>
          {roots.length===0 && <div className="empty">記事なし</div>}
          {roots.map(root => {
            const count = threadPosts(root, boardArticles).length;
            return <button key={root.id} className={root.id===selectedRoot?.id?'selected':''} onClick={()=>setThreadID(root.id)}>
              <strong>{root.subject}</strong><span>{root.author} · {fmt(root.created_at)}{count>1?` · ${count} posts`:''}</span>
            </button>;
          })}
        </aside>
        <section className="conversation">
          <div className="paneTitle">{selectedRoot?.subject || '本文'}</div>
          {selectedPosts.map((post,i)=><article key={post.id}>
            <div className="postHead"><b>{post.author}</b><span>#{post.id} · {fmt(post.created_at)}</span></div>
            {i>0 && <div className="subject">{post.subject}</div>}
            <div className="body">{post.body}</div>
            {debug && <details open className="worldDebug"><summary>WORLD / SITUATION</summary>
              <dl><dt>action</dt><dd>{post.action || '-'}</dd><dt>anchor</dt><dd>{post.anchor_key || '-'}</dd><dt>cause</dt><dd>{post.cause_kind || '-'}</dd><dt>discourse</dt><dd>{post.discourse_mode || '-'}</dd><dt>source</dt><dd>{post.source_post_id || '-'}</dd><dt>situation</dt><dd>{post.situation_kind || '-'}</dd></dl>
              {post.situation_summary && <p>{post.situation_summary}</p>}
              {!!post.situation_facts?.length && <ul>{post.situation_facts.map((f,n)=><li key={n}>{f}</li>)}</ul>}
            </details>}
          </article>)}
        </section>
      </main>
      <footer>JOB {job.id} · finished {fmt(job.finished_at)} · archived experimental output; not canonical BBS world</footer>
    </>}
  </div>;
}

const css = `
:root{background:#07100c;color:#d8f6df;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,"Noto Sans Mono CJK JP",monospace}*{box-sizing:border-box}body{margin:0;background:#07100c}.labviewer{min-height:100vh;padding:22px;max-width:1500px;margin:auto}header{display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #335844;padding-bottom:12px}.eyebrow{font-size:11px;letter-spacing:.18em;color:#70a981}h1{font-size:25px;margin:5px 0 0;font-weight:600}.readonly{border:1px solid #72c38b;color:#9ff6b8;padding:7px 10px;font-size:12px}.note{color:#92ad9b;font-size:12px;padding:10px 0}.runbar{display:flex;gap:10px;align-items:end;flex-wrap:wrap;background:#0c1711;border:1px solid #263b2d;padding:10px}.runbar label{font-size:11px;color:#8fad98}.runbar select{display:block;min-width:430px;max-width:70vw;margin-top:4px;background:#07100c;color:#d8f6df;border:1px solid #3d634b;padding:8px}.runbar button,.boards button,.threads button{font:inherit}.runbar>button{background:#13241a;color:#c7edcf;border:1px solid #42644c;padding:8px 12px}.debug{margin-left:auto;display:flex!important;gap:7px;align-items:center;padding-bottom:7px}.metrics{display:flex;gap:18px;flex-wrap:wrap;padding:10px 2px;font-size:11px;color:#779183}.metrics b{color:#dbf6e1;font-size:13px}.boards{display:flex;gap:5px;flex-wrap:wrap;border-bottom:1px solid #35513d;padding:3px 0 9px}.boards button{background:#0b150f;color:#9abb9f;border:1px solid #294333;padding:8px 12px;cursor:pointer}.boards button.active{background:#183121;color:#e1ffe8;border-color:#5b946d}.boards small{color:#5c8168;margin-right:6px}.boards em{font-style:normal;color:#6f9d7c;margin-left:8px}main{display:grid;grid-template-columns:minmax(270px,34%) 1fr;gap:12px;margin-top:12px;min-height:60vh}.threads,.conversation{border:1px solid #2c4635;background:#09130d}.paneTitle{padding:8px 10px;border-bottom:1px solid #2c4635;color:#8ebc9a;font-size:12px;letter-spacing:.08em}.threads button{width:100%;display:block;text-align:left;background:transparent;color:#c4dfca;border:0;border-bottom:1px solid #18291e;padding:11px;cursor:pointer}.threads button.selected{background:#14271a;border-left:3px solid #6bc184}.threads strong{display:block;font-size:13px;font-weight:500}.threads span{display:block;margin-top:5px;font-size:10px;color:#708b78}.conversation article{padding:16px 18px;border-bottom:1px dashed #294233}.postHead{display:flex;justify-content:space-between;gap:10px;color:#9ee3ae;font-size:13px}.postHead span{font-size:10px;color:#718d79}.subject{font-size:11px;color:#7ea488;margin-top:6px}.body{white-space:pre-wrap;line-height:1.75;margin-top:12px;color:#e1f6e5;font-family:inherit;font-size:14px}.worldDebug{margin-top:14px;background:#050b07;border:1px solid #273b2e;padding:8px;color:#8fab96;font-size:10px}.worldDebug summary{cursor:pointer;color:#72ab80}.worldDebug dl{display:grid;grid-template-columns:80px 1fr;gap:3px 8px}.worldDebug dt{color:#577762}.worldDebug dd{margin:0}.worldDebug p,.worldDebug ul{line-height:1.5}.error{margin:14px 0;padding:12px;border:1px solid #8d4949;background:#2b1212;color:#ffc6c6}.loading,.empty{padding:18px;color:#789080}footer{font-size:10px;color:#536a59;padding:14px 2px}@media(max-width:800px){.labviewer{padding:12px}.runbar select{min-width:0;width:80vw}.debug{margin-left:0}main{grid-template-columns:1fr}.threads{max-height:34vh;overflow:auto}.conversation{min-height:40vh}}
`;
