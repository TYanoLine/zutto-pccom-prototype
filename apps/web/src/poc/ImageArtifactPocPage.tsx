import { useRef, useState } from 'react';

type PaletteMode = '256' | '16';
type Preset = { label: string; prompt: string; palette: PaletteMode };

const PRESETS: Preset[] = [
  { label:'飼い猫のスナップ写真', prompt:'飼い猫を室内で撮ったスナップ写真', palette:'256' },
  { label:'1996年のPC-9821のある自室', prompt:'PC-9821のある1996年の自室の写真', palette:'256' },
  { label:'1990年代の美少女CG・256色', prompt:'1990年代半ばの日本のパソコン通信で配布されていそうな、成人女性を描いたオリジナル美少女CG。登場人物は20歳以上、非性的。256色程度のパレットを意識した当時のデジタルCG表現', palette:'256' },
  { label:'1990年代の美少女CG・16色', prompt:'1990年代半ばの日本のPC-98系パソコンで描かれたような、成人女性を描いたオリジナル美少女CG。登場人物は20歳以上、非性的。最初から16色だけで描くことを強く意識し、色数の少なさをディザや面塗りで補う当時のCG表現', palette:'16' },
  { label:'夏の海辺の旅行写真', prompt:'夏の海辺を撮った旅行写真', palette:'256' },
  { label:'秋葉原で買った周辺機器', prompt:'秋葉原で買ったパソコン周辺機器を机に並べた写真', palette:'256' },
];

const PALETTE16: readonly (readonly [number,number,number])[] = [
  [0,0,0],[128,0,0],[0,128,0],[128,128,0],
  [0,0,128],[128,0,128],[0,128,128],[192,192,192],
  [128,128,128],[255,0,0],[0,255,0],[255,255,0],
  [0,0,255],[255,0,255],[0,255,255],[255,255,255],
];

const configuredApiURL=(import.meta.env.VITE_API_URL as string|undefined)?.trim();
const configuredWsURL=(import.meta.env.VITE_WS_URL as string|undefined)?.trim();
const inferredApiURL=configuredWsURL?.replace(/^wss:/,'https:').replace(/^ws:/,'http:').replace(/\/ws\/?$/,'');
const apiBase=(configuredApiURL||inferredApiURL||'').replace(/\/$/,'');
const buildTime=(import.meta.env.VITE_BUILD_TIME as string|undefined)||'unknown';
const buildCommit=(import.meta.env.VITE_BUILD_COMMIT as string|undefined)||'unknown';
const buildRef=(import.meta.env.VITE_BUILD_REF as string|undefined)||'unknown';
const converterRevision='jpeg-compat-preview-v3';

type Generated = { image: string; model?: string };

function nextPaint() {
  return new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
}

export default function ImageArtifactPocPage() {
  const [prompt, setPrompt] = useState(PRESETS[0].prompt);
  const [paletteMode, setPaletteMode] = useState<PaletteMode>(PRESETS[0].palette);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [original, setOriginal] = useState('');
  const [convertedReady, setConvertedReady] = useState(false);
  const [compatPreview, setCompatPreview] = useState('');
  const [processedBytes, setProcessedBytes] = useState(0);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  function choosePreset(preset: Preset) {
    setPrompt(preset.prompt);
    setPaletteMode(preset.palette);
  }

  async function generate() {
    setBusy(true); setError(''); setOriginal(''); setConvertedReady(false); setCompatPreview(''); setProcessedBytes(0);
    try {
      if (!apiBase) throw new Error('Render backend URL is not configured');
      const paletteInstruction = paletteMode === '16'
        ? 'Generate the source image as deliberately limited-color 16-color computer artwork; avoid gradients that depend on many colors and use period-appropriate dithering or flat color areas where useful.'
        : 'Generate the source image as period-inspired computer artwork or imagery that can survive conversion to a 256-color palette; avoid relying on subtle modern HDR-like gradients.';
      const requestPrompt = `${prompt}\n\nTechnical source-image instruction: ${paletteInstruction}`;
      const res = await fetch(`${apiBase}/api/poc/image-artifact`, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({prompt:requestPrompt}) });
      const data = await res.json() as Generated & { error?: string };
      if (!res.ok || !data.image) throw new Error(data.error || `HTTP ${res.status}`);

      setOriginal(data.image);
      setConvertedReady(true);
      await nextPaint();
      await reduce(data.image, paletteMode);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  }

  function reduce(src: string, mode: PaletteMode) {
    return new Promise<void>((resolve, reject) => {
      const img = new Image();
      img.onload = () => {
        try {
          const canvas = canvasRef.current;
          if (!canvas) throw new Error('conversion canvas is not available');
          canvas.width=640; canvas.height=400;
          const ctx=canvas.getContext('2d');
          if (!ctx) throw new Error('2D canvas is not available');
          ctx.imageSmoothingEnabled=true;
          const scale=Math.max(640/img.width,400/img.height), sw=640/scale, sh=400/scale;
          ctx.drawImage(img,(img.width-sw)/2,(img.height-sh)/2,sw,sh,0,0,640,400);
          const frame=ctx.getImageData(0,0,640,400), d=frame.data;

          if (mode === '16') {
            for(let i=0;i<d.length;i+=4){
              let best=PALETTE16[0], bestDistance=Number.POSITIVE_INFINITY;
              for(const candidate of PALETTE16){
                const dr=d[i]-candidate[0], dg=d[i+1]-candidate[1], db=d[i+2]-candidate[2];
                const distance=dr*dr+dg*dg+db*db;
                if(distance<bestDistance){bestDistance=distance;best=candidate;}
              }
              d[i]=best[0]; d[i+1]=best[1]; d[i+2]=best[2];
            }
          } else {
            for(let i=0;i<d.length;i+=4){
              d[i]=Math.round(d[i]/255*7)*255/7;
              d[i+1]=Math.round(d[i+1]/255*7)*255/7;
              d[i+2]=Math.round(d[i+2]/255*3)*255/3;
            }
          }
          ctx.putImageData(frame,0,0);

          // Compatibility experiment: re-encode the already quantized pixels as
          // ordinary full-color JPEG. JPEG itself is not palette-indexed and may
          // introduce additional colors through compression, but the source look
          // still comes from the 16/256-color quantized frame above.
          setCompatPreview(canvas.toDataURL('image/jpeg',0.92));
          canvas.toBlob(blob=>{if(blob)setProcessedBytes(blob.size); resolve();},'image/png');
        } catch (e) {
          reject(e);
        }
      };
      img.onerror = () => reject(new Error('generated image could not be loaded for conversion'));
      img.src=src;
    });
  }

  const colorLabel=paletteMode==='16'?'16色':'≤256色';
  const showResults=Boolean(original||convertedReady);

  return <main style={{fontFamily:'monospace',maxWidth:1100,margin:'0 auto',padding:24,color:'#d8ffe8',background:'#07130d',minHeight:'100vh'}}>
    <p><a href="/" style={{color:'#75ffac'}}>← ずっとパソコン通信</a></p>
    <h1>画像ファイル生成 PoC</h1>
    <p style={{fontSize:12,opacity:.72,lineHeight:1.6,border:'1px dashed #397a53',padding:10}}>
      BUILD: {buildTime}<br/>
      COMMIT: {buildCommit}<br/>
      REF: {buildRef}<br/>
      CONVERTER: {converterRevision}
    </p>
    <p>OpenAIで素材を生成し、生成時にも色数を意識させたうえで、ブラウザ側で640×400・指定色数へ機械的に再変換します。互換性確認用に、量子化後の見た目を通常のフルカラーJPEGにも再エンコードして表示します。</p>
    <section style={{border:'1px solid #397a53',padding:16}}>
      <strong>題材</strong>
      <div style={{display:'grid',gap:8,marginTop:12}}>{PRESETS.map((p,i)=><label key={p.label}><input type="radio" name="preset" checked={prompt===p.prompt} onChange={()=>choosePreset(p)}/> {i+1}. {p.label}</label>)}</div>
      <fieldset style={{marginTop:16,border:'1px solid #397a53'}}>
        <legend>生成時の色数イメージ + 最終変換</legend>
        <label style={{marginRight:16}}><input type="radio" name="palette" checked={paletteMode==='256'} onChange={()=>setPaletteMode('256')}/> 256色</label>
        <label><input type="radio" name="palette" checked={paletteMode==='16'} onChange={()=>setPaletteMode('16')}/> 16色</label>
      </fieldset>
      <label style={{display:'block',marginTop:16}}>自由入力</label>
      <textarea value={prompt} onChange={e=>setPrompt(e.target.value)} rows={5} style={{width:'100%',boxSizing:'border-box',marginTop:6}} />
      <button onClick={generate} disabled={busy||!prompt.trim()} style={{marginTop:12,padding:'8px 18px'}}>{busy?'生成・変換中...':'生成して当時化'}</button>
      <p style={{opacity:.75}}>自由入力もOpenAIの安全基準の範囲で生成されます。人物を含む場合は成人として扱うようRender側でも指示します。</p>
      {error&&<pre style={{color:'#ff9a9a',whiteSpace:'pre-wrap'}}>{error}</pre>}
    </section>
    <section style={{display:showResults?'grid':'none',gridTemplateColumns:'repeat(auto-fit,minmax(300px,1fr))',gap:20,marginTop:24}}>
      <div><h2>AI元画像</h2>{original&&<img src={original} alt="AI生成元画像" style={{width:'100%',display:'block'}}/>}</div>
      <div>
        <h2>変換後 640×400 / {colorLabel}</h2>
        <canvas ref={canvasRef} width={640} height={400} aria-label={`640×400 ${colorLabel}変換後`} style={{width:'100%',height:'auto',display:convertedReady?'block':'none',background:'#000'}} />
        {processedBytes>0&&<p>{processedBytes.toLocaleString()} bytes (PNG計測)</p>}
        {compatPreview&&<><h3>互換表示：フルカラーJPEG</h3><img src={compatPreview} alt={`量子化後 ${colorLabel} をフルカラーJPEGで再エンコード`} style={{width:'100%',display:'block'}}/><p style={{opacity:.75}}>JPEGは多色フォーマットです。見た目の元は{colorLabel}量子化ですが、JPEG圧縮により実画素色数は増えます。</p></>}
      </div>
    </section>
  </main>;
}
