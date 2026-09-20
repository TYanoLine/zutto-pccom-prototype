import { useEffect, useMemo, useState } from 'react';
import './personaTimelinePoc.css';

type Interval = { key:string; value:string; from:string; until?:string; source_kind:string };
type Event = { id:string; at:string; kind:string; summary:string };
type Person = {
  id:string; handle:string; birth_date:string; baseline_facts:string[];
  occupation_history:Interval[]; state_intervals:Interval[]; events:Event[]; observation_dates:string[];
};
type Snapshot = {
  at:string; age:number; occupation:string; active_states:Interval[];
  known_events:Event[]; hidden_future_events:number; renderer_context:string[];
};
type CatalogItem = { id:string; handle:string; observation_dates:string[] };
type Response = { canonical:boolean; note:string; persona:Person; snapshot:Snapshot; catalog:CatalogItem[] };

export default function PersonaTimelinePocPage() {
  const [data,setData] = useState<Response|null>(null);
  const [person,setPerson] = useState('worker');
  const [at,setAt] = useState('1996-07-20');
  const [loading,setLoading] = useState(false);
  const [error,setError] = useState('');

  async function load(nextPerson=person,nextAt?:string) {
    setLoading(true); setError('');
    try {
      const q = new URLSearchParams({persona:nextPerson});
      if (nextAt) q.set('at',nextAt);
      const res = await fetch(`/api/persona-timeline?${q}`,{cache:'no-store'});
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setData(json);
      setPerson(nextPerson);
      setAt(json.snapshot.at);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(()=>{ void load('worker'); },[]);

  const selectedCatalog = useMemo(
    ()=>data?.catalog.find(x=>x.id===person),
    [data,person]
  );
  const knownIds = useMemo(()=>new Set(data?.snapshot.known_events.map(x=>x.id)||[]),[data]);

  return <div className="timelinePoc">
    <header className="timelineHero">
      <div>
        <div className="eyebrow">DEVELOPMENT PERSONA TIMELINE PoC</div>
        <h1>人物を「ある日付」で解決する</h1>
        <p>静的プロフィールではなく、永続事実・一時状態・世界イベントを時間軸で合成します。未来の出来事はデバッグ用タイムラインには見えても、renderer context からは遮断します。</p>
      </div>
      <div className="heroLinks"><a href="/poc/persona-lab">PERSONA LAB</a><a href="/">端末へ戻る</a></div>
    </header>

    <section className="timelineControls">
      <label>人物
        <select value={person} onChange={e=>{ const id=e.target.value; void load(id); }}>
          {(data?.catalog||[]).map(x=><option key={x.id} value={x.id}>{x.handle} / {x.id}</option>)}
        </select>
      </label>
      <label>観測日
        <input type="date" value={at} onChange={e=>setAt(e.target.value)}/>
      </label>
      <button onClick={()=>load(person,at)} disabled={loading}>{loading?'解決中…':'この日付で解決'}</button>
      <div className="presetDates">
        {selectedCatalog?.observation_dates.map(d=><button key={d} className={d===data?.snapshot.at?'active':''} onClick={()=>load(person,d)}>{d}</button>)}
      </div>
    </section>

    {error && <div className="timelineError">{error}</div>}

    {data && <>
      <section className="snapshotGrid">
        <article>
          <span>HANDLE</span><strong>{data.persona.handle}</strong><small>{data.persona.id}</small>
        </article>
        <article>
          <span>OBSERVED AT</span><strong>{data.snapshot.at}</strong><small>この瞬間の世界断面</small>
        </article>
        <article>
          <span>AGE</span><strong>{data.snapshot.age}歳</strong><small>生年月日から導出</small>
        </article>
        <article>
          <span>CURRENT ROLE</span><strong>{data.snapshot.occupation}</strong><small>期間付き状態から解決</small>
        </article>
        <article>
          <span>ACTIVE TEMP STATE</span><strong>{data.snapshot.active_states.length}</strong><small>{data.snapshot.active_states.map(x=>x.value).join(' / ') || 'なし'}</small>
        </article>
        <article className={data.snapshot.hidden_future_events?'futureMetric':''}>
          <span>FUTURE HIDDEN</span><strong>{data.snapshot.hidden_future_events}</strong><small>rendererへ渡さないイベント数</small>
        </article>
      </section>

      <section className="timelinePanel">
        <div className="panelHead">
          <div><h2>Renderer がこの日に見えるもの</h2><p>投稿本文を生成するなら、この断面だけを人物コンテキストとして使います。</p></div>
          <span className="safeBadge">FUTURE ISOLATED</span>
        </div>
        <pre className="rendererContext">{data.snapshot.renderer_context.join('\n')}</pre>
      </section>

      <section className="twoCol">
        <div className="timelinePanel">
          <h2>現在だけ有効な状態</h2>
          {data.snapshot.active_states.length===0
            ? <p className="empty">この日には一時状態なし。</p>
            : data.snapshot.active_states.map(s=><div className="stateCard" key={s.key}>
                <b>{s.value}</b><code>{s.key}</code>
                <span>{s.from} → {s.until||'継続中'}</span>
                <small>{s.source_kind}</small>
              </div>)}
        </div>
        <div className="timelinePanel">
          <h2>ベース事実</h2>
          <p className="explain">これらは投稿ネタではなく、矛盾防止・解釈用の背景です。</p>
          <ul>{data.persona.baseline_facts.map(x=><li key={x}>{x}</li>)}</ul>
        </div>
      </section>

      <section className="timelinePanel">
        <div className="panelHead">
          <div><h2>デバッグ用・全イベント時間軸</h2><p>ここだけは将来イベントも表示します。緑は観測日時点で既知、灰色は未来なのでrendererには入りません。</p></div>
        </div>
        <div className="eventTimeline">
          {data.persona.events.map(ev=>{
            const known=knownIds.has(ev.id);
            return <div key={ev.id} className={known?'event known':'event future'}>
              <div className="dot"/>
              <time>{ev.at}</time>
              <div><b>{ev.summary}</b><code>{ev.kind}</code></div>
              <span>{known?'KNOWN':'FUTURE / HIDDEN'}</span>
            </div>;
          })}
        </div>
      </section>

      <section className="timelinePanel">
        <h2>期間付き状態の正本イメージ</h2>
        <div className="intervalTable"><table>
          <thead><tr><th>key</th><th>value</th><th>from</th><th>until</th><th>source</th></tr></thead>
          <tbody>
            {[...data.persona.occupation_history,...data.persona.state_intervals].map((x,i)=><tr key={x.key+x.from+i}>
              <td><code>{x.key}</code></td><td>{x.value}</td><td>{x.from}</td><td>{x.until||'∞'}</td><td>{x.source_kind}</td>
            </tr>)}
          </tbody>
        </table></div>
      </section>

      <footer>
        <b>PoC only</b> — {data.note}
      </footer>
    </>}
  </div>;
}
