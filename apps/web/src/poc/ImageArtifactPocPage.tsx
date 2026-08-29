import { useRef, useState } from 'react';

const PRESETS = [
  '飼い猫を室内で撮ったスナップ写真',
  'PC-9821のある1996年の自室の写真',
  '成人女性が描いたオリジナル美少女CG（登場人物は20歳以上、非性的）',
  '夏の海辺を撮った旅行写真',
  '秋葉原で買ったパソコン周辺機器を机に並べた写真',
];

type Generated = { image: string; model?: string };

export default function ImageArtifactPocPage() {
  const [prompt, setPrompt] = useState(PRESETS[0]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [original, setOriginal] = useState('');
  const [processed, setProcessed] = useState('');
  const [processedBytes, setProcessedBytes] = useState(0);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  async function generate() {
    setBusy(true); setError(''); setOriginal(''); setProcessed(''); setProcessedBytes(0);
    try {
      const res = await fetch('/api/image-poc', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({prompt}) });
      const data = await res.json() as Generated & { error?: string };
      if (!res.ok || !data.image) throw new Error(data.error || `HTTP ${res.status}`);
      setOriginal(data.image);
      await reduce(data.image);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  }

  async function reduce(src: string) {
    const img = new Image();
    img.onload = () => {
      const canvas = canvasRef.current!; canvas.width=640; canvas.height=400;
      const ctx=canvas.getContext('2d')!; ctx.imageSmoothingEnabled=true;
      const scale=Math.max(640/img.width,400/img.height), sw=640/scale, sh=400/scale;
      ctx.drawImage(img,(img.width-sw)/2,(img.height-sh)/2,sw,sh,0,0,640,400);
      const frame=ctx.getImageData(0,0,640,400), d=frame.data;
      // Deterministic RGB 3-3-2 quantization: exactly <=256 possible colors.
      for(let i=0;i<d.length;i+=4){d[i]=Math.round(d[i]/255*7)*255/7;d[i+1]=Math.round(d[i+1]/255*7)*255/7;d[i+2]=Math.round(d[i+2]/255*3)*255/3;}
      ctx.putImageData(frame,0,0);
      canvas.toBlob(blob=>{if(!blob)return;setProcessedBytes(blob.size);setProcessed(URL.createObjectURL(blob));},'image/png');
    };
    img.src=src;
  }

  return <main style={{fontFamily:'monospace',maxWidth:1100,margin:'0 auto',padding:24,color:'#d8ffe8',background:'#07130d',minHeight:'100vh'}}>
    <p><a href="/" style={{color:'#75ffac'}}>← ずっとパソコン通信</a></p>
    <h1>画像ファイル生成 PoC</h1>
    <p>OpenAIで素材を生成し、ブラウザ側で640×400・256色以下へ機械的に変換します。PoCなので変換後はPNGです。</p>
    <section style={{border:'1px solid #397a53',padding:16}}>
      <strong>題材</strong>
      <div style={{display:'grid',gap:8,marginTop:12}}>{PRESETS.map((p,i)=><label key={p}><input type="radio" name="preset" checked={prompt===p} onChange={()=>setPrompt(p)}/> {i+1}. {p}</label>)}</div>
      <label style={{display:'block',marginTop:16}}>自由入力</label>
      <textarea value={prompt} onChange={e=>setPrompt(e.target.value)} rows={4} style={{width:'100%',boxSizing:'border-box',marginTop:6}} />
      <button onClick={generate} disabled={busy||!prompt.trim()} style={{marginTop:12,padding:'8px 18px'}}>{busy?'生成・変換中...':'生成して当時化'}</button>
      <p style={{opacity:.75}}>自由入力もOpenAIの安全基準の範囲で生成されます。人物を含む場合は成人として扱うようサーバー側でも指示します。</p>
      {error&&<pre style={{color:'#ff9a9a',whiteSpace:'pre-wrap'}}>{error}</pre>}
    </section>
    {(original||processed)&&<section style={{display:'grid',gridTemplateColumns:'repeat(auto-fit,minmax(300px,1fr))',gap:20,marginTop:24}}>
      <div><h2>AI元画像</h2>{original&&<img src={original} style={{width:'100%'}}/>}</div>
      <div><h2>640×400 / ≤256色</h2>{processed&&<><img src={processed} style={{width:'100%',imageRendering:'auto'}}/><p>{processedBytes.toLocaleString()} bytes (PNG)</p></>}</div>
    </section>}
    <canvas ref={canvasRef} style={{display:'none'}} />
  </main>;
}
