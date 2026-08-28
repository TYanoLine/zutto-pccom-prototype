import { useEffect, useMemo, useState } from 'react';
import './knowledgeBbs.css';

type Source = { url: string; title?: string };
type ResearchCase = { id:string; topic:string; question:string; status:string; provisionalAnswer:string; confidence:number; sources:Source[] };
type Article = { id:string; title:string; handle:string; time:string; preview:string; evidence:'atmospheric'|'verified'; body:string };
type Board = { id:string; title:string; description:string; articles:Article[] };

type Stage = 'entrance'|'boards'|'articles'|'article';

const configuredApiURL = (import.meta.env.VITE_API_URL as string | undefined)?.trim();
const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const inferredApiURL = configuredWsURL?.replace(/^wss:/,'https:').replace(/^ws:/,'http:').replace(/\/ws\/?$/,'');
const apiBase=(configuredApiURL||inferredApiURL||'').replace(/\/$/,'');

const boards:Board[]=[
 {id:'general',title:'フリートーク',description:'とりあえず何でもどうぞ',articles:[
  {id:'general-1',title:'暑いですねえ',handle:'MARI',time:'08/25 22:14',preview:'昼間は暑くてPCの前に座ってるのもつらい…',evidence:'atmospheric',body:'ども、MARIです。\n\n今日も暑かったですねえ。昼間はPCの前に座ってるだけで汗だく(^^;\n夜になってやっと落ち着いたので巡回中です。\n\nみなさん夏バテしてません？'},
  {id:'general-2',title:'夜更かし組(^^;',handle:'KEN',time:'08/26 00:41',preview:'こんな時間でもけっこう人いますね',evidence:'atmospheric',body:'こんな時間なのにWHO見るとけっこう人いますね(笑)\n明日つらいの分かってるんだけど、つい電話しちゃうんだよなあ。'}]},
 {id:'pc',title:'PC・通信',description:'本体、モデム、通信ソフトなど',articles:[
  {id:'modem-288',title:'モデム、みんなどのくらい？',handle:'TAKA',time:'08/26 01:08',preview:'28.8Kとか33.6Kとか最近よく聞くけど…',evidence:'verified',body:'ども、TAKAです。\n\n最近28.8Kとか33.6Kとか聞くようになったけど、実際みんな何使ってます？\nうちはまだ14.4K。速いのを見ると欲しくなるんだけど(^^;'},
  {id:'pc-2',title:'通信ソフト何使ってる？',handle:'NEKO',time:'08/26 02:03',preview:'最近ちょっと乗り換えようか迷ってます',evidence:'atmospheric',body:'最近ちょっと通信ソフトを乗り換えようか迷ってます。\n自動巡回が楽なのがいいんだけど、設定やり直すのも面倒で(^^;\nみなさん何使ってます？'}]},
 {id:'chat',title:'趣味・雑談',description:'ゲーム、音楽、そのほか',articles:[
  {id:'chat-1',title:'週末なにしてた？',handle:'JUN',time:'08/26 00:12',preview:'こっちはずっと家でごろごろしてました(笑)',evidence:'atmospheric',body:'週末は特に出かけず家でごろごろしてました(笑)\n夜になってから通信してるので、結局いつも通り。'}]}
];

const researchTopic='PoC: 1996年8月のモデム速度事情';
const researchQuestion='1996年8月時点の日本の個人向けパソコン通信利用者にとって、14.4kbps・28.8kbps・33.6kbps級モデムはそれぞれどのような位置づけだったか。発売済みか、一般的か、先端的かを区別し、断定できない点も示す。';

export default function KnowledgeBbsPage(){
 const [stage,setStage]=useState<Stage>('entrance');
 const [boardId,setBoardId]=useState<string>('general');
 const [article,setArticle]=useState<Article|null>(null);
 const [research,setResearch]=useState<ResearchCase|null>(null);
 const [busy,setBusy]=useState(false);
 const [log,setLog]=useState<string[]>(['入口を表示: Web検索 0回']);
 const board=useMemo(()=>boards.find(b=>b.id===boardId)!,[boardId]);
 useEffect(()=>{(window as Window & {__zuttoBootOk?:()=>void}).__zuttoBootOk?.();},[]);

 const push=(s:string)=>setLog(v=>[s,...v].slice(0,8));
 async function ensureResearch(){
  if(research||!apiBase)return;
  setBusy(true); push('考証が必要 → HistoricalKnowledgeService を照会');
  try{
   const listRes=await fetch(`${apiBase}/api/admin/research`); const list=await listRes.json() as {cases?:ResearchCase[]};
   let found=list.cases?.find(c=>c.topic===researchTopic&&c.question===researchQuestion)||null;
   if(found){push('共有KBに既存案件あり → Web検索 0回'); setResearch(found); return;}
   push('共有KBに不足 → Research Agent がWeb検索');
   const res=await fetch(`${apiBase}/api/admin/research/new`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({topic:researchTopic,question:researchQuestion})});
   const created=await res.json() as ResearchCase; if(!res.ok)throw new Error((created as unknown as {error?:string}).error||res.statusText);
   setResearch(created); push('調査結果を共有KBへ保存 / needs_review候補化');
  }catch(e){push(`調査失敗: ${e instanceof Error?e.message:String(e)}`);}finally{setBusy(false);}
 }
 async function openArticle(a:Article){setArticle(a);setStage('article'); if(a.evidence==='verified')await ensureResearch(); else push(`記事「${a.title}」: 雰囲気生成扱い / Web検索 0回`);}

 return <main className="kbpoc">
  <header><div><small>WORLD ENGINE × HISTORICAL KNOWLEDGE PoC</small><h1>MIDI NIGHT NET</h1><p>1996-08-26 / selective evidence demo</p></div><a href="/admin/research">Research案件を見る →</a></header>
  <div className="kbgrid">
   <section className="terminal">
    {stage==='entrance'&&<div className="screen entrance"><pre>{`*** MIDI NIGHT NET ***\n\nいらっしゃいませ。\n夜間は混み合うことがあります。\n長時間接続はほどほどに(^^;\n\n[Enter] 掲示板へ`}</pre><button onClick={()=>{setStage('boards');push('掲示板一覧: atmosphere / Web検索 0回');}}>掲示板へ入る</button></div>}
    {stage==='boards'&&<div className="screen"><h2>掲示板一覧</h2>{boards.map((b,i)=><button className="row" key={b.id} onClick={()=>{setBoardId(b.id);setStage('articles');push(`「${b.title}」一覧: atmosphere / Web検索 0回`);}}><b>{i+1}. {b.title}</b><span>{b.description}</span></button>)}<button className="back" onClick={()=>setStage('entrance')}>← 戻る</button></div>}
    {stage==='articles'&&<div className="screen"><h2>{board.title}</h2>{board.articles.map(a=><button className="row articleRow" key={a.id} onClick={()=>void openArticle(a)}><span>{a.time} {a.handle}</span><b>{a.title}</b><em className={a.evidence}>{a.evidence==='verified'?'要考証':'雰囲気'}</em><small>{a.preview}</small></button>)}<button className="back" onClick={()=>setStage('boards')}>← 掲示板一覧</button></div>}
    {stage==='article'&&article&&<div className="screen"><h2>{article.title}</h2><p className="meta">{article.time} / {article.handle}</p><pre className="body">{article.body}</pre>{article.evidence==='verified'&&<div className="evidenceBox"><b>Historical Knowledge</b>{busy?<p>考証情報を調査中...</p>:research?<><p>{research.provisionalAnswer}</p><p>confidence {Math.round(research.confidence*100)}% / {research.status}</p><div>{research.sources?.slice(0,4).map((s,i)=><a key={i} href={s.url} target="_blank" rel="noreferrer">{s.title||s.url}</a>)}</div></>:<p>考証情報を取得できませんでした。本文自体は暫定表示しています。</p>}</div>}<button className="back" onClick={()=>setStage('articles')}>← 記事一覧</button></div>}
   </section>
   <aside className="trace"><h2>WorldEngine trace</h2><p>このPoCでは、通常の画面・雑談は検索しません。具体的な歴史事実だけ共有KBを確認します。</p>{log.map((x,i)=><div key={i} className="traceRow"><span>{i===0?'NOW':'·'}</span>{x}</div>)}<div className="legend"><b>atmospheric</b><span>モデル知識/時代ルールで十分</span><b>verified</b><span>具体的事実なのでKB→不足ならWeb調査</span></div></aside>
  </div>
 </main>;
}
