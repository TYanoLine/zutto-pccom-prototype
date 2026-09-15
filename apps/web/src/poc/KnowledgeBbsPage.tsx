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
  {id:'general-2',title:'夜更かし組(^^;',handle:'KEN',time:'08/26 00:41',preview:'こんな時間でもけっこう人いますね',evidence:'atmospheric',body:'こんな時間なのにWHO見るとけっこう人いますね(笑)\n明日つらいの分かってるんだけど、つい電話しちゃうんだよなあ。'},
  {id:'general-3',title:'電話代こわいので巡回順見直し中',handle:'PON',time:'08/26 00:58',preview:'23時から入るつもりがつい早くつないでしまう…',evidence:'atmospheric',body:'今月ちょっと電話代が重いので巡回順を見直してます。\n23時からにしようと思ってるのに、つい22時台に入ってしまう(^^;\nみなさん「ここだけは毎日見る」板ってあります？'}]},
 {id:'pc',title:'PC・通信',description:'本体、モデム、通信ソフトなど',articles:[
  {id:'modem-288',title:'モデム、みんなどのくらい？',handle:'TAKA',time:'08/26 01:08',preview:'28.8Kとか33.6Kとか最近よく聞くけど…',evidence:'verified',body:'ども、TAKAです。\n\n最近28.8Kとか33.6Kとか聞くようになったけど、実際みんな何使ってます？\nうちはまだ14.4K。速いのを見ると欲しくなるんだけど(^^;'},
  {id:'pc-2',title:'通信ソフト何使ってる？',handle:'NEKO',time:'08/26 02:03',preview:'最近ちょっと乗り換えようか迷ってます',evidence:'atmospheric',body:'最近ちょっと通信ソフトを乗り換えようか迷ってます。\n自動巡回が楽なのがいいんだけど、設定やり直すのも面倒で(^^;\nみなさん何使ってます？'},
  {id:'pc-3',title:'ATコマンドで切ったあと再接続が遅い件',handle:'RAY',time:'08/25 23:47',preview:'ATH→ATDTですぐつなぐとたまにNO CARRIER',evidence:'atmospheric',body:'ちょっと相談です。\nATHで切ってすぐATDTすると、たまにNO CARRIERが出ます。\n数秒待つと通るので回線側かな？\n同じような人いたら設定教えてください m(_ _)m'}]},
 {id:'games',title:'ゲーム',description:'家庭用ゲーム・PCゲームの話題',articles:[
  {id:'games-1',title:'サターン版NiGHTS、パッドどれ使ってる？',handle:'M2',time:'08/25 23:31',preview:'アナログでやると気持ちいいけど慣れが必要(^^;',evidence:'atmospheric',body:'NiGHTSを久しぶりにやってるんですが、3Dパッドに慣れるまで時間かかりますね。\nみんなは普通のパッド派？ 3Dパッド派？\nおすすめ設定あれば知りたいです。'},
  {id:'games-2',title:'PSのメモリーカード整理術',handle:'SACHI',time:'08/25 21:56',preview:'セーブデータがパンパンで困ってます',evidence:'atmospheric',body:'RPGのセーブが増えてメモカがいっぱいです(^^;\nみなさん古いデータって残す派ですか？\n私は消せなくてどんどん増えるタイプ…。'},
  {id:'games-3',title:'夏休み中に遊んだ一本',handle:'GON',time:'08/26 00:18',preview:'「これは当たりだった」ってやつ教えてください',evidence:'atmospheric',body:'夏休みの終わりなので、みんなの当たりソフト聞きたいです。\nジャンル問わず「これは良かった」一本だけぜひ！\n> 私は最近だとアクション多めでした。'}]},
 {id:'music',title:'音楽・MIDI',description:'CD、ライブ、MIDI打ち込み',articles:[
  {id:'music-1',title:'SC-55mkIIとMU80、迷ってます',handle:'HIDE',time:'08/25 22:52',preview:'どっちも良さそうで決めきれない…',evidence:'atmospheric',body:'MIDI音源を増やしたくて、SC-55mkIIとMU80で迷い中です。\n主にゲーム曲コピー用途。\n使ってる人いたら長所短所を教えてください(^^)'},
  {id:'music-2',title:'深夜の作業BGMスレ',handle:'YUKI',time:'08/26 01:34',preview:'巡回しながら何聴いてます？',evidence:'atmospheric',body:'巡回しながら聴くCD、最近固定化してきたので新規開拓したいです。\nジャンルは何でもOK。\n「夜向けの一枚」おすすめお願いします m(_ _)m'},
  {id:'music-3',title:'FM音源っぽい音作りのコツ',handle:'LIME',time:'08/25 23:09',preview:'PCM寄りになってしまって難しい…',evidence:'atmospheric',body:'打ち込みでFMっぽさを出したいんですが、つい厚くしすぎてしまいます(^^;\nアタック短め＋ビブラート浅めで試してるところ。\n他にもコツがあればぜひ。'}]},
 {id:'local',title:'地域の話題',description:'街情報、イベント、交通',articles:[
  {id:'local-1',title:'横浜駅西口の中古屋さん情報',handle:'AKI',time:'08/25 20:43',preview:'今週末に見に行く予定です',evidence:'atmospheric',body:'横浜駅西口あたりで中古ソフト強い店、最近のおすすめあります？\n価格帯が分かると助かります。\n週末に何件か回る予定です。'},
  {id:'local-2',title:'雨の日のオフ会集合場所どうしてる？',handle:'YU',time:'08/26 00:04',preview:'駅前だと混みやすいので悩み中',evidence:'atmospheric',body:'次の小規模オフ、天気が怪しいので集合場所を考え中です。\n駅前だと分かりやすいけど混むし、少し離れると迷う人が出るし…。\nみなさんの定番があれば教えてください。'},
  {id:'local-3',title:'深夜バス逃したときの対処法',handle:'KAZ',time:'08/26 01:49',preview:'終電後に通信してるとたまにやらかす(^^;',evidence:'atmospheric',body:'終電後に「もう一本だけ読むか」で遅くなってしまうことが…(^^;\nみなさん深夜移動どうしてます？\n始発待ちスポット情報とかあれば助かります。'}]},
 {id:'buysell',title:'売ります・買います',description:'個人売買・交換連絡',articles:[
  {id:'buy-1',title:'[売ります] 14.4K外付けモデム',handle:'NORI',time:'08/25 21:21',preview:'箱なし・動作品です',evidence:'atmospheric',body:'14.4K外付けモデムをお譲りします。\n本体＋ACアダプタのみ、端子は問題なし。\n希望は6,000円、横浜周辺で手渡し優先です。'},
  {id:'buy-2',title:'[買います] PC-9821用増設メモリ',handle:'HAL',time:'08/25 23:58',preview:'16MBか32MBを探しています',evidence:'atmospheric',body:'PC-9821用の増設メモリを探しています。\n16MBまたは32MB希望、相場感が分からないので提示歓迎です。\n状態と型番を書いていただけると助かります。'},
  {id:'buy-3',title:'[交換] MIDIケーブル余ってる方',handle:'SEI',time:'08/26 00:27',preview:'フロッピーケースと交換希望',evidence:'atmospheric',body:'MIDIケーブル(短め)を1本探してます。\nこちらは未使用のフロッピーケース数枚あります。\n等価交換でよければメールください。'}]},
 {id:'newbie',title:'はじめまして・初心者',description:'初参加あいさつ、質問歓迎',articles:[
  {id:'newbie-1',title:'はじめまして、接続テスト兼ねて書き込み',handle:'MOMO',time:'08/25 20:11',preview:'無事に投稿できてますか？',evidence:'atmospheric',body:'はじめまして、MOMOです。\n昨日モデムをつないだばかりで、まだ操作に不慣れです(^^;\nこの投稿が見えていたらレスいただけるとうれしいです。'},
  {id:'newbie-2',title:'引用返信の作法これで合ってます？',handle:'TOMO',time:'08/25 22:06',preview:'> を付ける位置でいつも迷います',evidence:'atmospheric',body:'初心者質問です。\n> 相手の文を必要な分だけ引用\nという形で大丈夫でしょうか？\n長くなりすぎないコツがあれば知りたいです。'},
  {id:'newbie-3',title:'ROM専からそろそろ脱出したい',handle:'FUMI',time:'08/26 01:02',preview:'どの板から書き始めるのが入りやすい？',evidence:'atmospheric',body:'ずっと読んでばかりだったんですが、そろそろ書いてみたいです。\n最初の一歩として、みなさんはどんな話題で入っていきましたか？\n緊張するので背中押してください(^^;)'}]},
 {id:'sysop',title:'局への要望・連絡',description:'運用連絡・不具合報告',articles:[
  {id:'sysop-1',title:'深夜帯のレスポンスについて',handle:'SYSOP',time:'08/25 19:50',preview:'23時台は混雑しやすいです',evidence:'atmospheric',body:'いつもご利用ありがとうございます。\n23時〜24時はアクセスが集中するため、表示が遅くなる場合があります。\n切断せず少し待って再試行してみてください。'},
  {id:'sysop-2',title:'迷惑投稿を見つけたときは',handle:'SYSOP',time:'08/25 21:02',preview:'板違い・連続投稿の報告先',evidence:'atmospheric',body:'板違い投稿や連続投稿を見つけた場合は、この板かSYSOPメールでお知らせください。\n確認後、必要に応じて移動・整理します。\nご協力お願いします m(_ _)m'},
  {id:'sysop-3',title:'次回メンテ予定（予告）',handle:'SYSOP',time:'08/26 00:36',preview:'週末早朝に短時間停止の可能性',evidence:'atmospheric',body:'週末の早朝に短時間メンテを予定しています。\n時間が決まり次第この板で再告知します。\nご不便をおかけしますがよろしくお願いします。'}]}
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
