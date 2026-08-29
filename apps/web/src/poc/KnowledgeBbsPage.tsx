import { useEffect, useMemo, useState } from 'react';
import './knowledgeBbs.css';

type Source = { url: string; title?: string };
type Fact = { id:string; claim:string; confidence:number; status:string; sources:Source[]; researchId?:string };
type KnowledgeResult = { facts:Fact[]; coverage:number; confidence:number; researched:boolean; researchPending:boolean; researchId?:string; canUse:boolean; missing?:{description:string}[] };
type Article = { id:string; title:string; handle:string; time:string; preview:string; evidence:'atmospheric'|'verified'; body:string };
type Board = { id:string; title:string; description:string; articles:Article[] };
type Stage = 'entrance'|'boards'|'articles'|'article';

const configuredApiURL=(import.meta.env.VITE_API_URL as string|undefined)?.trim();
const configuredWsURL=(import.meta.env.VITE_WS_URL as string|undefined)?.trim();
const inferredApiURL=configuredWsURL?.replace(/^wss:/,'https:').replace(/^ws:/,'http:').replace(/\/ws\/?$/,'');
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

export default function KnowledgeBbsPage(){
 const [stage,setStage]=useState<Stage>('entrance');
 const [boardId,setBoardId]=useState('general');
 const [article,setArticle]=useState<Article|null>(null);
 const [knowledge,setKnowledge]=useState<KnowledgeResult|null>(null);
 const [busy,setBusy]=useState(false);
 const [log,setLog]=useState<string[]>(['入口を表示: Web検索 0回']);
 const board=useMemo(()=>boards.find(b=>b.id===boardId)!,[boardId]);
 useEffect(()=>{(window as Window&{__zuttoBootOk?:()=>void}).__zuttoBootOk?.();},[]);
 const push=(s:string)=>setLog(v=>[s,...v].slice(0,8));

 async function ensureKnowledge(){
  if(knowledge||!apiBase)return;
  setBusy(true);push('具体的な永続事実 → WorldEngine/EvidencePolicy は verified');
  try{
   const res=await fetch(`${apiBase}/api/internal/knowledge/resolve`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({kind:'technical_capability',subject:'日本の個人向けV.34モデム速度事情',worldDate:'1996-08-26',region:'JP',audience:['pc_communication_users'],need:'14.4kbps・28.8kbps・33.6kbps級モデムが、1996年8月時点で一般利用者にとってどのような位置づけだったかを確認する',requiredEvidence:'verified'})});
   const result=await res.json() as KnowledgeResult&{error?:string};if(!res.ok)throw new Error(result.error||res.statusText);setKnowledge(result);
   if(result.researched)push('KB不足 → bounded Research Sub-Agent がWeb検索してFact化');
   else if(result.researchPending)push('同じ調査が進行中 → 重複検索せず待機');
   else push('有効な共有Factあり → Web検索 0回で再利用');
  }catch(e){push(`Knowledge取得失敗: ${e instanceof Error?e.message:String(e)}`);}finally{setBusy(false);}
 }
 async function openArticle(a:Article){setArticle(a);setStage('article');if(a.evidence==='verified')await ensureKnowledge();else push(`記事「${a.title}」: atmospheric / Web検索 0回`);}
 const bestFact=knowledge?.facts?.[0];

 return <main className="kbpoc">
  <header><div><small>WORLD ENGINE × HISTORICAL KNOWLEDGE</small><h1>MIDI NIGHT NET</h1><p>1996-08-26 / selective evidence demo</p></div><a href="/admin/research">Research案件を見る →</a></header>
  <div className="kbgrid">
   <section className="terminal">
    {stage==='entrance'&&<div className="screen entrance"><pre>{`*** MIDI NIGHT NET ***\n\nいらっしゃいませ。\n夜間は混み合うことがあります。\n長時間接続はほどほどに(^^;\n\n[Enter] 掲示板へ`}</pre><button onClick={()=>{setStage('boards');push('掲示板一覧: atmospheric / Web検索 0回');}}>掲示板へ入る</button></div>}
    {stage==='boards'&&<div className="screen"><h2>掲示板一覧</h2>{boards.map((b,i)=><button className="row" key={b.id} onClick={()=>{setBoardId(b.id);setStage('articles');push(`「${b.title}」一覧: atmospheric / Web検索 0回`);}}><b>{i+1}. {b.title}</b><span>{b.description}</span></button>)}<button className="back" onClick={()=>setStage('entrance')}>← 戻る</button></div>}
    {stage==='articles'&&<div className="screen"><h2>{board.title}</h2>{board.articles.map(a=><button className="row articleRow" key={a.id} onClick={()=>void openArticle(a)}><span>{a.time} {a.handle}</span><b>{a.title}</b><em className={a.evidence}>{a.evidence==='verified'?'要考証':'雰囲気'}</em><small>{a.preview}</small></button>)}<button className="back" onClick={()=>setStage('boards')}>← 掲示板一覧</button></div>}
    {stage==='article'&&article&&<div className="screen"><h2>{article.title}</h2><p className="meta">{article.time} / {article.handle}</p><pre className="body">{article.body}</pre>{article.evidence==='verified'&&<div className="evidenceBox"><b>Historical Knowledge inspector</b>{busy?<p>共有KBを確認中...</p>:knowledge?<><p>{bestFact?.claim||'Verifiedとして使えるFactはまだありません。'}</p><p>confidence {Math.round(knowledge.confidence*100)}% / coverage {Math.round(knowledge.coverage*100)}% / {bestFact?.status||'pending review'}</p>{knowledge.missing?.map((m,i)=><small key={i}>{m.description}</small>)}<div>{bestFact?.sources?.slice(0,4).map((s,i)=><a key={i} href={s.url} target="_blank" rel="noreferrer">{s.title||s.url}</a>)}</div></>:<p>考証情報を取得できませんでした。本文はデモ用の暫定表示です。</p>}</div>}<button className="back" onClick={()=>setStage('articles')}>← 記事一覧</button></div>}
   </section>
   <aside className="trace"><h2>WorldEngine trace</h2><p>通常の画面・雑談は検索せず、永続化する具体的な歴史事実だけ EvidencePolicy が強い証拠を要求します。</p>{log.map((x,i)=><div key={i} className="traceRow"><span>{i===0?'NOW':'·'}</span>{x}</div>)}<div className="legend"><b>atmospheric</b><span>モデル知識/時代ルールだけ。検索しない</span><b>plausible</b><span>KBがあれば使う。不足しても同期検索しない</span><b>verified</b><span>共有Fact必須。不足時だけbounded Research Sub-Agent</span></div></aside>
  </div>
 </main>;
}
