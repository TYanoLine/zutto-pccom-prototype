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
  semantics:{
    api_calls:number; llm_calls:number; persona_bank:boolean; host_profile_affects:string; canonical:boolean; note:string;
    profile_generation_available?:boolean; jev_profile_audit_available?:boolean; profile_batch_size?:number;
  };
};

type ProfileBatch = {
  batch:number; count:number; llm_duration_ms:number; llm_model?:string; llm_input_tokens:number; llm_output_tokens:number;
  llm_total_tokens:number; jev_duration_ms:number; jev_model?:string; jev_input_tokens:number; jev_error?:string;
};
type ProfileResult = {
  persona_id:string; handle:string; detail_tier:string; skeleton:string;
  distinctive_hook:string; core_traits:string[]; social_dynamics:string[]; participation_habits:string[];
  everyday_context:string[]; voice_notes:string[]; profile:string; jev_checked:boolean;
  future_probability:number; external_review_probability:number; future_flag:boolean; external_review_flag:boolean;
};
type ProfileSummary = {
  llm_calls:number; llm_duration_ms:number; llm_input_tokens:number; llm_output_tokens:number; llm_total_tokens:number;
  jev_calls:number; jev_duration_ms:number; jev_input_tokens:number; jev_errors:number; future_flagged:number;
  external_review_flagged:number; max_future_probability:number; max_external_review_probability:number;
  unique_hook_ratio:number; unique_trait_signature_ratio:number; max_profile_similarity:number; avg_nearest_profile_similarity:number;
  total_duration_ms:number; profiles_per_second:number;
};
type ProfileJob = {
  id:string; status:string; seed:number; profile_id:string; profile_label:string; account_count:number; profile_count:number;
  pool_size:number; batch_size:number; identity_generation_us:number; jev_available:boolean;
  created_at:string; started_at?:string; finished_at?:string; progress:{completed:number;total:number};
  batches?:ProfileBatch[]; results?:ProfileResult[]; summary:ProfileSummary; error?:string;
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
function ms(v:number) {
  if (v >= 1000) return `${(v/1000).toFixed(2)} s`;
  return `${v} ms`;
}
function pct(v:number) { return `${(v*100).toFixed(1)}%`; }
function probability(v:number) { return `${(v*100).toFixed(0)}%`; }

export default function PersonaLabPage() {
  const [data,setData] = useState<Response|null>(null);
  const [count,setCount] = useState(100);
  const [seed,setSeed] = useState(19960826);
  const [profile,setProfile] = useState('general');
  const [tier,setTier] = useState('all');
  const [loading,setLoading] = useState(false);
  const [error,setError] = useState('');
  const [roundTrip,setRoundTrip] = useState(0);
  const [profileCount,setProfileCount] = useState(100);
  const [profileJob,setProfileJob] = useState<ProfileJob|null>(null);
  const [profileError,setProfileError] = useState('');
  const [profileStarting,setProfileStarting] = useState(false);

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

  async function refreshProfileJob(id:string) {
    const q = new URLSearchParams({action:'profile-status',id});
    const res = await fetch(`/api/persona-lab?${q}`, {cache:'no-store'});
    const json = await res.json();
    if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
    setProfileJob(json);
  }

  async function startProfiles() {
    setProfileStarting(true); setProfileError('');
    try {
      const q = new URLSearchParams({
        action:'start-profiles', count:String(count), seed:String(seed), profile, profile_count:String(profileCount)
      });
      const res = await fetch(`/api/persona-lab?${q}`, {method:'POST',cache:'no-store'});
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setProfileJob(json);
    } catch (e) {
      setProfileError(e instanceof Error ? e.message : String(e));
    } finally {
      setProfileStarting(false);
    }
  }

  useEffect(()=>{ void generate(); },[]);
  useEffect(()=>{
    const id=profileJob?.id;
    if (!id || (profileJob?.status!=='queued' && profileJob?.status!=='running')) return;
    const timer=window.setInterval(()=>{ void refreshProfileJob(id).catch(e=>setProfileError(e instanceof Error?e.message:String(e))); },1200);
    return ()=>window.clearInterval(timer);
  },[profileJob?.id,profileJob?.status]);

  const personas = useMemo(()=>{
    const all = data?.run.personas || [];
    if (tier==='all') return all;
    return all.filter(p=>p.detail_tier===tier);
  },[data,tier]);

  const profileOptions=[10,20,50,100].filter(n=>n<=count);
  const progress=profileJob?.progress.total ? profileJob.progress.completed/profileJob.progress.total : 0;

  return <div className="personaLab">
    <header>
      <div>
        <div className="eyebrow">DEVELOPMENT PERSONA LAB</div>
        <h1>会員Identity生成 + 非同期人格具現化 PoC</h1>
        <p>大量の会員骨格はLLMなしで生成し、必要人数だけOpenAIで「別人として振る舞える」構造化人格へ具現化します。Jevで未来情報を監査し、固有性・文章類似度・速度まで比較します。永続世界には書き込みません。</p>
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
        <div><span>IDENTITY LLM</span><strong>{data.semantics.llm_calls}</strong><small>骨格生成はAPI 0</small></div>
        <div><span>UNIQUE HANDLES</span><strong>{pct(data.run.quality.unique_handle_ratio)}</strong><small>exact clone {pct(data.run.quality.exact_clone_ratio)}</small></div>
        <div><span>HOST FIT</span><strong>{data.run.quality.average_host_fit.toFixed(3)}</strong><small>wildcard {pct(data.run.quality.wildcard_ratio)}</small></div>
      </section>

      <section className="panel timing">
        <h2>Identity処理時間内訳</h2>
        <div className="timingGrid">
          <div><b>{us(data.run.timing.pool_generation_us)}</b><span>世界候補pool生成</span></div>
          <div><b>{us(data.run.timing.selection_us)}</b><span>局傾向による会員選択</span></div>
          <div><b>{us(data.run.timing.formatting_us)}</b><span>骨格プロフィール整形</span></div>
          <div><b>{us(data.run.timing.quality_us)}</b><span>品質診断</span></div>
        </div>
        <p className="note">ここまでは完全ローカルです。下の実験だけがOpenAI/Jevを呼びます。</p>
      </section>

      <section className="panel profileExperiment">
        <div className="memberHead">
          <div>
            <h2>非同期人格具現化 + Jev未来監査</h2>
            <p>活動度の高い core → active → identity の順で対象を選び、{data.semantics.profile_batch_size||10}人ずつ人格を生成。前バッチの固有フックも渡して100人全体の同型化を抑えます。</p>
          </div>
          <div className="profileActions">
            <label>具現化人数
              <select value={profileCount} onChange={e=>setProfileCount(Number(e.target.value))}>
                {profileOptions.map(n=><option key={n} value={n}>{n}人</option>)}
              </select>
            </label>
            <button onClick={startProfiles} disabled={profileStarting || !data.semantics.profile_generation_available || profileJob?.status==='running' || profileJob?.status==='queued'}>
              {profileStarting?'開始中…':'非同期生成を開始'}
            </button>
          </div>
        </div>
        <div className="experimentAvailability">
          <span className={data.semantics.profile_generation_available?'ok':'warn'}>OpenAI {data.semantics.profile_generation_available?'READY':'UNAVAILABLE'}</span>
          <span className={data.semantics.jev_profile_audit_available?'ok':'warn'}>Jev {data.semantics.jev_profile_audit_available?'READY':'UNAVAILABLE'}</span>
          <span>未来flag閾値 50%</span>
        </div>
        {profileError && <div className="error">{profileError}</div>}
        {profileJob && <div className="jobBox">
          <div className="jobTop">
            <b>{profileJob.status.toUpperCase()}</b><span>{profileJob.id}</span>
            <span>{profileJob.progress.completed}/{profileJob.progress.total}人</span>
          </div>
          <div className="progressTrack"><i style={{width:`${Math.max(1,progress*100)}%`}}/></div>
          <div className="profileMetrics">
            <div><span>IDENTITY</span><strong>{us(profileJob.identity_generation_us)}</strong><small>{profileJob.account_count} accounts</small></div>
            <div><span>OPENAI PROFILE</span><strong>{ms(profileJob.summary?.llm_duration_ms||0)}</strong><small>{profileJob.summary?.llm_calls||0} calls / {(profileJob.summary?.llm_total_tokens||0).toLocaleString()} tokens</small></div>
            <div><span>JEV AUDIT</span><strong>{ms(profileJob.summary?.jev_duration_ms||0)}</strong><small>{profileJob.summary?.jev_calls||0} calls / errors {profileJob.summary?.jev_errors||0}</small></div>
            <div><span>END TO END</span><strong>{ms(profileJob.summary?.total_duration_ms||0)}</strong><small>{(profileJob.summary?.profiles_per_second||0).toFixed(2)} profiles/s</small></div>
            <div><span>FUTURE FLAGS</span><strong>{profileJob.summary?.future_flagged||0}</strong><small>max {probability(profileJob.summary?.max_future_probability||0)}</small></div>
            <div><span>REVIEW FLAGS</span><strong>{profileJob.summary?.external_review_flagged||0}</strong><small>max {probability(profileJob.summary?.max_external_review_probability||0)}</small></div>
            <div><span>HOOK UNIQUE</span><strong>{pct(profileJob.summary?.unique_hook_ratio||0)}</strong><small>固有フック完全一致を検出</small></div>
            <div><span>TRAIT UNIQUE</span><strong>{pct(profileJob.summary?.unique_trait_signature_ratio||0)}</strong><small>性格+対人+投稿癖 signature</small></div>
            <div><span>TEXT NEAREST</span><strong>{pct(profileJob.summary?.avg_nearest_profile_similarity||0)}</strong><small>3文字gram / max {pct(profileJob.summary?.max_profile_similarity||0)}</small></div>
          </div>
          {profileJob.error && <div className="error">{profileJob.error}</div>}
          {!!profileJob.batches?.length && <div className="batchTable"><table>
            <thead><tr><th>batch</th><th>人数</th><th>OpenAI</th><th>tokens</th><th>Jev</th><th>Jev input</th><th>状態</th></tr></thead>
            <tbody>{profileJob.batches.map(b=><tr key={b.batch}>
              <td>#{b.batch}</td><td>{b.count}</td><td>{ms(b.llm_duration_ms)}</td><td>{b.llm_total_tokens.toLocaleString()}</td>
              <td>{ms(b.jev_duration_ms)}</td><td>{b.jev_input_tokens.toLocaleString()}</td><td>{b.jev_error?<span className="warn">ERROR</span>:<span className="ok">OK</span>}</td>
            </tr>)}</tbody>
          </table></div>}
          {!!profileJob.results?.length && <div className="profileResultTable"><table>
            <thead><tr><th>HANDLE</th><th>tier</th><th>骨格</th><th>LLM人物固有情報</th><th>プロフィール要約</th><th>未来</th><th>史実review</th></tr></thead>
            <tbody>{profileJob.results.map(r=><tr key={r.persona_id}>
              <td><b>{r.handle}</b><small>{r.persona_id}</small></td>
              <td>{tierLabels[r.detail_tier]||r.detail_tier}</td>
              <td className="summary">{r.skeleton}</td>
              <td className="personaDetail">
                <b>{r.distinctive_hook}</b>
                <small><em>性格</em> {r.core_traits?.join(' / ')}</small>
                <small><em>対人</em> {r.social_dynamics?.join(' / ')}</small>
                <small><em>投稿</em> {r.participation_habits?.join(' / ')}</small>
                <small><em>日常</em> {r.everyday_context?.join(' / ')}</small>
                <small><em>声</em> {r.voice_notes?.join(' / ')}</small>
              </td>
              <td className="generatedProfile">{r.profile}</td>
              <td className={r.future_flag?'risk':'number'}>{r.jev_checked?probability(r.future_probability):'-'}</td>
              <td className={r.external_review_flag?'risk':'number'}>{r.jev_checked?probability(r.external_review_probability):'-'}</td>
            </tr>)}</tbody>
          </table></div>}
        </div>}
      </section>

      <section className="panel benchmark">
        <h2>Identity人数スケール</h2>
        <div className="benchRows">
          {data.benchmarks.map(b=><div key={b.count} className="benchRow">
            <b>{b.count}人</b><div className="bar"><i style={{width:`${Math.min(100, Math.max(2,b.total_us/(Math.max(...data.benchmarks.map(x=>x.total_us))||1)*100))}%`}}/></div>
            <span>{us(b.total_us)}</span><small>{b.per_person_ns.toLocaleString()} ns/人</small>
          </div>)}
        </div>
      </section>

      <section className="panel quality">
        <h2>Identity品質チェック</h2>
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
          <thead><tr><th>HANDLE</th><th>属性</th><th>活動</th><th>関心</th><th>局fit</th><th>選出</th><th>詳細度</th><th>骨格プロフィール</th></tr></thead>
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
