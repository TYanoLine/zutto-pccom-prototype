import { useEffect, useMemo, useState } from 'react';
import './personaLab.css';

type HostProfile = { id:string; label:string; weights:Record<string,number> };
type Timing = { pool_generation_us:number; selection_us:number; formatting_us:number; quality_us:number; total_us:number };
type Check = { name:string; status:string; value:string; note:string };
type Quality = {
  unique_handle_ratio:number; exact_clone_ratio:number; average_host_fit:number; wildcard_ratio:number;
  average_age:number; age_min:number; age_max:number; occupation_kinds:number; style_signature_kinds:number;
  future_term_hits:number; age_occupation_warnings:number;
  activity_distribution:Record<string,number>; occupation_distribution:Record<string,number>; top_interest_distribution:Record<string,number>;
  checks:Check[];
};
type Persona = {
  id:string; handle:string; age:number; gender:string; occupation:string; activity_class:string; visit_days_per_week:number;
  lurker_bias:number; write_bias:number; reply_bias:number; thread_start_bias:number;
  interests:Record<string,number>; top_interests:string[]; style_tags:string[]; connect_window:string; quirk:string;
  host_fit:number; membership_source:string; detail_tier:string; profile_summary:string;
};
type Run = { seed:number; count:number; pool_size:number; profile:HostProfile; timing:Timing; quality:Quality; personas:Persona[]; generator_note:string };
type Benchmark = { count:number; pool_size:number; total_us:number; per_person_ns:number };
type Response = {
  generated_at:string; build_commit?:string; build_branch?:string; profiles:HostProfile[]; run:Run; benchmarks:Benchmark[];
  semantics:{ api_calls:number; llm_calls:number; persona_bank:boolean; host_profile_affects:string; canonical:boolean; note:string };
};

const interestLabels:Record<string,string> = {
  communications:'パソコン通信', modem:'モデム', software:'ソフトウェア', files:'ファイル',
  games:'ゲーム', music:'音楽', local:'地域', chat:'チャット'
};
const activityLabels:Record<string,string> = {
  regular:'常連', active:'活発', occasional:'時々', lurker:'ROM寄り', dormant:'休眠気味'
};
const tierLabels:Record<string,string> = {
  'core-candidate':'詳細化候補', active:'active', identity:'identityのみ'
};

function us(v:number) {
  if (v >= 1000) return `${(v/1000).toFixed(2)} ms`;
  return `${v} µs`;
}
function pct(v:number) { return `${(v*100).toFixed(1)}%`; }

export default function PersonaLabPage() {
  const [data,setData] = useState<Response|null>(null);
  const [count,setCount] = useState(100);
  const [seed,setSeed] = useState(19960826);
  const [profile,setProfile] = useState('general');
  const [tier,setTier] = useState('all');
  const [loading,setLoading] = useState(false);
  const [error,setError] = useState('');
  const [roundTrip,setRoundTrip] = useState(0);

  async function generate(nextSeed=seed) {
    setLoading(true); setError('');
    const started = performance.now();
    try {
      const q = new URLSearchParams({count:String(count),seed:String(nextSeed),profile});
      const res = await fetch(`/api/persona-lab?${q}`, {cache:'no-store'});
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setRoundTrip(performance.now()-started);
      setData(json);
      setSeed(nextSeed);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(()=>{ void generate(); },[]);

  const personas = useMemo(()=>{
    const all = data?.run.personas || [];
    if (tier==='all') return all;
    return all.filter(p=>p.detail_tier===tier);
  },[data,tier]);

  return <div className="personaLab">
    <header>
      <div>
        <div className="eyebrow">DEVELOPMENT PERSONA LAB</div>
        <h1>会員Identity生成 PoC</h1>
        <p>大量の会員をLLMなしで生成し、局傾向による加入選択・速度・多様性を確認します。永続世界には書き込みません。</p>
      </div>
      <a href="/">端末へ戻る</a>
    </header>

    <section className="controls">
      <label>会員数
        <select value={count} onChange={e=>setCount(Number(e.target.value))}>
          {[50,100,250,500,1000].map(n=><option key={n} value={n}>{n}人</option>)}
        </select>
      </label>
      <label>局傾向
        <select value={profile} onChange={e=>setProfile(e.target.value)}>
          {(data?.profiles || [
            {id:'general',label:'総合・雑談',weights:{}},{id:'tech',label:'通信・技術',weights:{}},
            {id:'games',label:'ゲーム中心',weights:{}},{id:'local',label:'地域・交流',weights:{}},{id:'music',label:'音楽・趣味',weights:{}}
          ]).map(p=><option key={p.id} value={p.id}>{p.label}</option>)}
        </select>
      </label>
      <label>seed<input value={seed} onChange={e=>setSeed(Number(e.target.value)||1)}/></label>
      <button onClick={()=>generate()} disabled={loading}>{loading?'生成中…':'同じseedで生成'}</button>
      <button onClick={()=>generate(Date.now()%2147483647)} disabled={loading}>別seedで生成</button>
    </section>

    {error && <div className="error">{error}</div>}
    {data && <>
      <section className="heroMetrics">
        <div><span>LOCAL TOTAL</span><strong>{us(data.run.timing.total_us)}</strong><small>{data.run.count}人 / pool {data.run.pool_size}</small></div>
        <div><span>HTTP ROUND TRIP</span><strong>{roundTrip.toFixed(1)} ms</strong><small>Vercel→Renderを含む</small></div>
        <div><span>LLM CALLS</span><strong>{data.semantics.llm_calls}</strong><small>API calls {data.semantics.api_calls}</small></div>
        <div><span>UNIQUE HANDLES</span><strong>{pct(data.run.quality.unique_handle_ratio)}</strong><small>exact clone {pct(data.run.quality.exact_clone_ratio)}</small></div>
        <div><span>HOST FIT</span><strong>{data.run.quality.average_host_fit.toFixed(3)}</strong><small>wildcard {pct(data.run.quality.wildcard_ratio)}</small></div>
      </section>

      <section className="panel timing">
        <h2>処理時間内訳</h2>
        <div className="timingGrid">
          <div><b>{us(data.run.timing.pool_generation_us)}</b><span>世界候補pool生成</span></div>
          <div><b>{us(data.run.timing.selection_us)}</b><span>局傾向による会員選択</span></div>
          <div><b>{us(data.run.timing.formatting_us)}</b><span>プロフィール文章化</span></div>
          <div><b>{us(data.run.timing.quality_us)}</b><span>品質診断</span></div>
        </div>
        <p className="note">プロフィール文章もルールベースです。ここでは「人物生成にLLMが本当に必要か」を切り分けるため、ネットワーク/モデル呼び出しを一切していません。</p>
      </section>

      <section className="panel benchmark">
        <h2>人数スケール</h2>
        <div className="benchRows">
          {data.benchmarks.map(b=><div key={b.count} className="benchRow">
            <b>{b.count}人</b><div className="bar"><i style={{width:`${Math.min(100, Math.max(2,b.total_us/(Math.max(...data.benchmarks.map(x=>x.total_us))||1)*100))}%`}}/></div>
            <span>{us(b.total_us)}</span><small>{b.per_person_ns.toLocaleString()} ns/人</small>
          </div>)}
        </div>
      </section>

      <section className="panel quality">
        <h2>品質チェック</h2>
        <div className="checks">
          {data.run.quality.checks.map(c=><div key={c.name} className={`check ${c.status}`}>
            <span>{c.status==='pass'?'PASS':'WARN'}</span><b>{c.name}</b><strong>{c.value}</strong><small>{c.note}</small>
          </div>)}
        </div>
        <div className="distGrid">
          <Distribution title="活動クラス" values={data.run.quality.activity_distribution} labels={activityLabels}/>
          <Distribution title="職業" values={data.run.quality.occupation_distribution}/>
          <Distribution title="第一関心" values={data.run.quality.top_interest_distribution} labels={interestLabels}/>
        </div>
        <div className="qualityFoot">年齢 {data.run.quality.age_min}–{data.run.quality.age_max} / 平均 {data.run.quality.average_age.toFixed(1)}歳 ・ 職業 {data.run.quality.occupation_kinds}種 ・ 文体signature {data.run.quality.style_signature_kinds}種</div>
      </section>

      <section className="panel members">
        <div className="memberHead">
          <div><h2>生成された会員</h2><p>{data.run.profile.label}への適合度で75%、多様性15%、wildcard10%を選択。</p></div>
          <label>表示
            <select value={tier} onChange={e=>setTier(e.target.value)}>
              <option value="all">全員</option><option value="core-candidate">詳細化候補</option><option value="active">active</option><option value="identity">identityのみ</option>
            </select>
          </label>
        </div>
        <div className="tableWrap"><table>
          <thead><tr><th>HANDLE</th><th>属性</th><th>活動</th><th>関心</th><th>局fit</th><th>選出</th><th>詳細度</th><th>プロフィール</th></tr></thead>
          <tbody>{personas.map(p=><tr key={p.id}>
            <td><b>{p.handle}</b><small>{p.id}</small></td>
            <td>{p.age}歳<br/>{p.occupation}</td>
            <td>{activityLabels[p.activity_class]||p.activity_class}<small>週{p.visit_days_per_week.toFixed(1)}日 / ROM {p.lurker_bias.toFixed(2)}</small></td>
            <td>{p.top_interests.map(x=>interestLabels[x]||x).join(' / ')}<small>{p.style_tags.join('・')}</small></td>
            <td className="number">{p.host_fit.toFixed(3)}</td>
            <td>{p.membership_source}</td>
            <td><span className={`tier ${p.detail_tier}`}>{tierLabels[p.detail_tier]||p.detail_tier}</span></td>
            <td className="summary">{p.profile_summary}</td>
          </tr>)}</tbody>
        </table></div>
      </section>

      <footer>
        <span>seed {data.run.seed}</span><span>build {data.build_commit?.slice(0,12)||'unknown'}</span>
        <span>{data.run.generator_note}</span>
      </footer>
    </>}
  </div>;
}

function Distribution({title,values,labels={}}:{title:string;values:Record<string,number>;labels?:Record<string,string>}) {
  const entries=Object.entries(values||{}).sort((a,b)=>b[1]-a[1]);
  const total=entries.reduce((s,[,n])=>s+n,0)||1;
  return <div className="distribution"><h3>{title}</h3>
    {entries.map(([k,n])=><div key={k}><span>{labels[k]||k}</span><i><em style={{width:`${n/total*100}%`}}/></i><b>{n}</b></div>)}
  </div>;
}
