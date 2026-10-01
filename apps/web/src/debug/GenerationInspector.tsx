import { useEffect, useState } from 'react';
import { serverVersionEndpoint } from '../build/ServerBuildInfo';
import './GenerationInspector.css';

export type GenerationTraceStep = {
  stage: string;
  status: 'running' | 'completed' | 'failed';
  started_at: string;
  finished_at?: string;
  prompt: string;
  result?: string;
  error?: string;
};

export type GenerationTraceRun = {
  id: string;
  host: string;
  board: string;
  kind: string;
  post_id?: number;
  status: 'running' | 'completed' | 'failed';
  started_at: string;
  finished_at?: string;
  error?: string;
  steps: GenerationTraceStep[];
};

export type GenerationTraceSnapshot = {
  runs: GenerationTraceRun[];
  running: boolean;
};

export function generationTraceEndpoint(wsURL: string): string | null {
  const base = serverVersionEndpoint(wsURL);
  if (!base) return null;
  const url = new URL(base);
  url.pathname = '/api/debug/bbs/generation-trace';
  return url.toString();
}

export async function fetchGenerationTrace(
  wsURL: string,
  token: string,
  fetcher: typeof fetch = fetch,
  signal?: AbortSignal,
): Promise<GenerationTraceSnapshot> {
  const endpoint = generationTraceEndpoint(wsURL);
  if (!endpoint) throw new Error('接続先サーバーが設定されていません');
  const response = await fetcher(endpoint, {
    method: 'GET',
    cache: 'no-store',
    headers: { 'X-Zutto-Debug-Token': token },
    signal,
  });
  if (response.status === 403) throw new Error('認証できません。サーバー側の DEBUG_RESET_TOKEN を確認してください。');
  if (!response.ok) throw new Error(`生成ログ取得失敗: HTTP ${response.status}`);
  const result: unknown = await response.json();
  if (!result || typeof result !== 'object' || !Array.isArray((result as GenerationTraceSnapshot).runs)) {
    throw new Error('サーバーからの応答が不正です');
  }
  return result as GenerationTraceSnapshot;
}

type Props = {
  wsURL: string;
  active: boolean;
  open: boolean;
  onClose: () => void;
  onRunningChange: (running: boolean) => void;
};

// This is a modern, operator-only overlay. It never writes to the PC-98
// terminal, changes host-program navigation or starts LLM generation.
export function GenerationInspector({ wsURL, active, open, onClose, onRunningChange }: Props) {
  // Deliberately in component memory only: never put the server token in an URL,
  // Vite environment variable, localStorage, or logs.
  const [tokenInput, setTokenInput] = useState('');
  const [token, setToken] = useState('');
  const [snapshot, setSnapshot] = useState<GenerationTraceSnapshot>({ runs: [], running: false });
  const [error, setError] = useState('');
  const [lastUpdated, setLastUpdated] = useState('');

  useEffect(() => {
    if (!active || !token || !generationTraceEndpoint(wsURL)) {
      onRunningChange(false);
      return;
    }
    let busy = false;
    const controller = new AbortController();
    const refresh = async () => {
      if (busy || controller.signal.aborted) return;
      busy = true;
      try {
        const next = await fetchGenerationTrace(wsURL, token, fetch, controller.signal);
        if (!controller.signal.aborted) {
          setSnapshot(next);
          setError('');
          setLastUpdated(new Date().toLocaleTimeString('ja-JP'));
          onRunningChange(next.running);
        }
      } catch (failure) {
        if (!controller.signal.aborted) {
          setError(failure instanceof Error ? failure.message : String(failure));
          onRunningChange(false);
        }
      } finally {
        busy = false;
      }
    };
    void refresh();
    const timer = window.setInterval(() => void refresh(), 2500);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [active, token, wsURL, onRunningChange]);

  if (!open) return null;
  return <div className="generation-inspector-backdrop">
    <section className="generation-inspector" role="dialog" aria-modal="true" aria-label="HAKATA 生成デバッグ">
      <header className="generation-inspector__header">
        <div><strong>HAKATA 生成デバッグ</strong><small>Situation → 件名 → 記事本文 / 2.5秒ごとに更新</small></div>
        <button type="button" onClick={onClose} aria-label="生成デバッグを閉じる">閉じる ×</button>
      </header>
      {!token ? <form className="generation-inspector__unlock" onSubmit={e => {
        e.preventDefault();
        if (tokenInput.trim()) { setToken(tokenInput.trim()); setTokenInput(''); setError(''); }
      }}>
        <p>この画面にはLLMへ送った全文プロンプトと応答が含まれます。閲覧にはサーバーのデバッグ認証キーが必要です。</p>
        <label>デバッグキー <input type="password" autoComplete="off" value={tokenInput} onChange={e => setTokenInput(e.target.value)} /></label>
        <button type="submit" disabled={!tokenInput.trim()}>表示する</button>
      </form> : <>
        <div className="generation-inspector__toolbar">
          <span className={snapshot.running ? 'generation-inspector__live' : ''}>{snapshot.running ? '● 生成中…' : '生成待機中 / 履歴表示'}</span>
          <span>最終更新: {lastUpdated || '取得中…'}</span>
          <button type="button" onClick={() => { setToken(''); setSnapshot({ runs: [], running: false }); onRunningChange(false); }}>認証解除</button>
        </div>
        {error && <p className="generation-inspector__error" role="alert">{error}</p>}
        <div className="generation-inspector__history">
          {snapshot.runs.length === 0 && !error && <p>まだ生成履歴はありません。HAKATAの板を開くか記事を読むと、ここに処理が表示されます。</p>}
          {snapshot.runs.map(run => <details key={run.id} className="generation-inspector__run" open={run.status === 'running' ? true : undefined}>
            <summary>
              <strong>{run.board} / {run.kind === 'headers' ? '件名生成' : '本文生成'}</strong>
              {run.post_id ? ` 記事 #${run.post_id}` : ''}
              {'　'}{run.status === 'running' ? '生成中…' : run.status === 'failed' ? '失敗' : '完了'}
              <small>{new Date(run.started_at).toLocaleTimeString('ja-JP')}</small>
            </summary>
            {run.error && <pre className="generation-inspector__error">{run.error}</pre>}
            {run.steps.length === 0 && <p>処理準備中（LLM呼び出し前）</p>}
            {run.steps.map((step, index) => <details key={index} className="generation-inspector__step" open={step.status === 'running' ? true : undefined}>
              <summary>{index + 1}. {step.stage}　{step.status === 'running' ? '生成中…' : step.status === 'failed' ? 'APIエラー' : '応答受信'} <small>{new Date(step.started_at).toLocaleTimeString('ja-JP')}</small></summary>
              <h4>送信プロンプト</h4>
              <pre>{step.prompt}</pre>
              <h4>LLM応答 {step.status === 'running' ? '（待機中）' : ''}</h4>
              <pre>{step.result || (step.status === 'running' ? '応答を待っています…' : '出力テキストなし')}</pre>
              {step.error && <pre className="generation-inspector__error">{step.error}</pre>}
            </details>)}
          </details>)}
        </div>
      </>}
      <footer>この記録はプロセス内の一時診断情報です。再起動で消去され、生成処理や世界状態は変更しません。</footer>
    </section>
  </div>;
}
