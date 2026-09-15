import { useEffect, useState } from 'react';

type Arm = {
  model?: string;
  body?: string;
  subject?: string;
  latency_ms?: number;
  error?: string;
};

type Result = {
  canonical?: {
    post?: number;
    board?: string;
    author?: string;
    subject?: string;
    worker_intent?: string;
  };
  gemini?: Arm;
  luna?: Arm;
  persisted?: boolean;
  error?: string;
};

const panel: React.CSSProperties = {
  border: '1px solid #315f43',
  background: '#06110b',
  padding: 14,
  borderRadius: 4,
};

function ArmPanel({ title, arm }: { title: string; arm?: Arm }) {
  return (
    <section style={panel}>
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{title}</h2>
      <div style={{ color: '#9fe8b4', marginBottom: 8 }}>
        model={arm?.model || 'n/a'} / latency={arm?.latency_ms ?? 'n/a'}ms
      </div>
      {arm?.error ? (
        <pre style={{ whiteSpace: 'pre-wrap', color: '#ff9a9a' }}>{arm.error}</pre>
      ) : (
        <>
          <div style={{ fontWeight: 700, marginBottom: 8 }}>{arm?.subject || '(no subject)'}</div>
          <pre style={{ whiteSpace: 'pre-wrap', lineHeight: 1.65, margin: 0 }}>{arm?.body || '(no body)'}</pre>
        </>
      )}
    </section>
  );
}

export default function GeminiArticleWorkerViewerPage() {
  const query = new URLSearchParams(window.location.search);
  const [phone, setPhone] = useState(query.get('phone') || '0450000196');
  const [board, setBoard] = useState(query.get('board') || '5');
  const [post, setPost] = useState(query.get('post') || '1201');
  const [result, setResult] = useState<Result | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function run() {
    setLoading(true);
    setError('');
    setResult(null);
    try {
      const params = new URLSearchParams({ phone, board, post });
      const response = await fetch(`/api/article-worker-ab?${params.toString()}`, { cache: 'no-store' });
      const data = (await response.json()) as Result;
      if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
      setResult(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (query.get('run') === '1') void run();
    // Query parameters are intentionally read only once on debug-page entry.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const inputStyle: React.CSSProperties = {
    background: '#020805',
    color: '#d5ffe0',
    border: '1px solid #315f43',
    padding: '7px 8px',
    width: 150,
    fontFamily: 'monospace',
  };
  const buttonStyle: React.CSSProperties = {
    background: '#123b22',
    color: '#d5ffe0',
    border: '1px solid #4f9a67',
    padding: '8px 12px',
    cursor: 'pointer',
    fontFamily: 'monospace',
  };

  return (
    <main style={{ minHeight: '100vh', background: '#020805', color: '#d5ffe0', padding: 24, fontFamily: 'monospace' }}>
      <div style={{ maxWidth: 1180, margin: '0 auto' }}>
        <h1 style={{ marginTop: 0 }}>Gemini Article Worker Debug</h1>
        <p style={{ color: '#9fe8b4' }}>
          正本の記事Envelopeをそのまま使い、永続化せずに Gemini 3.8 Flash と比較用 Luna で本文だけを再生成します。
        </p>

        <div style={{ ...panel, display: 'flex', flexWrap: 'wrap', gap: 10, alignItems: 'end', marginBottom: 16 }}>
          <label>PHONE<br /><input style={inputStyle} value={phone} onChange={e => setPhone(e.target.value)} /></label>
          <label>BOARD<br /><input style={inputStyle} value={board} onChange={e => setBoard(e.target.value)} /></label>
          <label>POST<br /><input style={inputStyle} value={post} onChange={e => setPost(e.target.value)} /></label>
          <button style={buttonStyle} onClick={() => void run()} disabled={loading}>{loading ? 'GENERATING...' : 'RUN'}</button>
          <button style={buttonStyle} onClick={() => { setBoard('5'); setPost('1201'); }}>YMO sample</button>
          <button style={buttonStyle} onClick={() => { setBoard('4'); setPost('1208'); }}>GAME sample</button>
          <a href="/poc/materialization-lab-viewer" style={{ ...buttonStyle, textDecoration: 'none' }}>LAB VIEWER</a>
        </div>

        {error && <pre style={{ ...panel, color: '#ff9a9a', whiteSpace: 'pre-wrap' }}>{error}</pre>}

        {result?.canonical && (
          <section style={{ ...panel, marginBottom: 16 }}>
            <h2 style={{ marginTop: 0, fontSize: 18 }}>Canonical input</h2>
            <div>MSG {String(result.canonical.post ?? '').padStart(4, '0')} / {result.canonical.board} / {result.canonical.author}</div>
            <div style={{ fontWeight: 700, margin: '8px 0' }}>{result.canonical.subject}</div>
            <pre style={{ whiteSpace: 'pre-wrap', lineHeight: 1.5, margin: 0, color: '#a8d7b5' }}>{result.canonical.worker_intent}</pre>
          </section>
        )}

        {result && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(360px,1fr))', gap: 16 }}>
            <ArmPanel title="Gemini" arm={result.gemini} />
            <ArmPanel title="Luna (comparison)" arm={result.luna} />
          </div>
        )}

        {result && <p style={{ color: '#8bbd98' }}>persisted={String(result.persisted)} — この画面の再生成結果は世界DBへ保存しません。</p>}
      </div>
    </main>
  );
}
