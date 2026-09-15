import { useEffect, useState } from 'react';
import './historicalResearch.css';

type ResearchSource = { url: string; title?: string };
type ResearchMessage = { role: string; body: string; createdAt?: string };
type BootWindow = Window & { __zuttoBootOk?: () => void };
type ResearchCase = {
  id: string;
  topic: string;
  question: string;
  worldDate: string;
  status: string;
  summary: string;
  provisionalAnswer: string;
  missingInfo: string[];
  confidence: number;
  sources: ResearchSource[];
  messages: ResearchMessage[];
};

const configuredApiURL = (import.meta.env.VITE_API_URL as string | undefined)?.trim();
const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const inferredApiURL = configuredWsURL?.replace(/^wss:/, 'https:').replace(/^ws:/, 'http:').replace(/\/ws\/?$/, '');
const apiBase = (configuredApiURL || inferredApiURL || (typeof window !== 'undefined' && /^(localhost|127\.0\.0\.1)$/.test(window.location.hostname) ? 'http://localhost:8080' : '')).replace(/\/$/, '');
const statuses = ['needs_review', 'provisional', 'operator_verified', 'canonical', 'rejected'];
const API_TIMEOUT_MS = 25000;

export default function HistoricalResearchPage() {
  const [cases, setCases] = useState<ResearchCase[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selected, setSelected] = useState<ResearchCase | null>(null);
  const [topic, setTopic] = useState('');
  const [question, setQuestion] = useState('');
  const [chat, setChat] = useState('');
  const [supplement, setSupplement] = useState('');
  const [status, setStatus] = useState('needs_review');
  const [busy, setBusy] = useState(false);
  const [loadingList, setLoadingList] = useState(false);
  const [notice, setNotice] = useState('TEST MODE: 認証なしで利用できます。');
  const canCall = apiBase.length > 0;

  useEffect(() => { (window as BootWindow).__zuttoBootOk?.(); }, []);

  async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!apiBase) throw new Error('APIサーバを特定できません');
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), API_TIMEOUT_MS);
    try {
      const response = await fetch(`${apiBase}${path}`, {
        ...init,
        signal: controller.signal,
        headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
      });
      const text = await response.text();
      let body: unknown = {};
      try { body = text ? JSON.parse(text) : {}; } catch { body = { error: text }; }
      if (!response.ok) {
        const message = typeof body === 'object' && body && 'error' in body ? String((body as { error: unknown }).error) : `${response.status} ${response.statusText}`;
        throw new Error(message);
      }
      return body as T;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') throw new Error('APIが25秒以内に応答しませんでした。Renderの起動/デプロイ状態を確認してください。');
      throw error;
    } finally {
      window.clearTimeout(timer);
    }
  }

  async function loadCases(preferId?: string | null) {
    if (!canCall) return;
    setLoadingList(true);
    try {
      const data = await api<{ cases: ResearchCase[] }>('/api/admin/research');
      setCases(data.cases || []);
      const nextId = preferId ?? selectedId ?? data.cases?.[0]?.id ?? null;
      if (nextId) await openCase(nextId, false);
    } finally {
      setLoadingList(false);
    }
  }

  async function openCase(id: string, reloadList = true) {
    const item = await api<ResearchCase>(`/api/admin/research/case?id=${encodeURIComponent(id)}`);
    setSelectedId(id); setSelected(item); setSupplement(item.provisionalAnswer || ''); setStatus(item.status || 'needs_review');
    if (reloadList) {
      const data = await api<{ cases: ResearchCase[] }>('/api/admin/research');
      setCases(data.cases || []);
    }
  }

  async function run(action: () => Promise<void>) {
    setBusy(true); setNotice('処理中...');
    try { await action(); setNotice('完了しました。'); }
    catch (error) { setNotice(error instanceof Error ? error.message : String(error)); }
    finally { setBusy(false); }
  }

  useEffect(() => {
    if (!canCall) { setNotice('APIサーバを特定できません。'); return; }
    void loadCases().then(() => setNotice('TEST MODE: 認証なしで利用できます。')).catch(error => setNotice(error instanceof Error ? error.message : String(error)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canCall]);

  return <main className="research-admin">
    <header className="research-header">
      <div><p className="eyebrow">ZUTTO PC COMMUNICATION / OPERATOR</p><h1>Historical Research Maintenance</h1><p>史実・カルチャー情報の自動調査、暫定採用、運営補完を確認するPoCです。</p></div>
      <a href="/" className="back-link">← 通信端末へ戻る</a>
    </header>

    <section className="research-card auth-card">
      <div><span className="api-label">TEST MODE</span><strong>認証なし</strong><p className="muted">PoC中のみResearch APIを開放しています。本番化時に運営認証へ戻します。</p></div>
      <div><span className="api-label">API</span><code>{apiBase || '(未設定)'}</code></div>
      <button type="button" disabled={!canCall || loadingList} onClick={() => void loadCases().then(() => setNotice('案件一覧を更新しました。')).catch(error => setNotice(error instanceof Error ? error.message : String(error)))}>{loadingList ? '読込中…' : '案件を再読込'}</button>
    </section>

    <section className="research-card">
      <h2>新規調査</h2>
      <div className="new-research-grid">
        <label>テーマ<input value={topic} onChange={e => setTopic(e.currentTarget.value)} placeholder="例: 1996年夏の福岡のモデム事情" /></label>
        <label>調べたいこと<textarea value={question} onChange={e => setQuestion(e.currentTarget.value)} rows={3} placeholder="14.4Kと28.8Kの普及状況を当時資料中心に確認して" /></label>
      </div>
      <button type="button" disabled={!canCall || busy || !topic.trim() || !question.trim()} onClick={() => void run(async () => {
        const item = await api<ResearchCase>('/api/admin/research/new', { method: 'POST', body: JSON.stringify({ topic, question }) });
        setTopic(''); setQuestion(''); await loadCases(item.id);
      })}>{busy ? '調査中…' : 'Web調査して案件を作成'}</button>
    </section>

    <div className="research-layout">
      <aside className="research-card case-list">
        <div className="section-heading"><h2>案件</h2><span>{cases.length}件</span></div>
        {cases.length === 0 ? <p className="muted">案件はまだありません。上の「新規調査」から最初の案件を作成してください。</p> : cases.map(item => <button type="button" key={item.id} className={`case-button ${selectedId === item.id ? 'active' : ''}`} disabled={busy} onClick={() => void run(() => openCase(item.id))}><strong>{item.topic}</strong><span>{item.status} / {item.worldDate}</span></button>)}
      </aside>

      <section className="research-card case-detail">
        {!selected ? <div className="empty-state">案件を作成するか、左の案件を選択してください。</div> : <>
          <div className="case-title-row"><div><p className="eyebrow">{selected.worldDate}</p><h2>{selected.topic}</h2><p>{selected.question}</p></div><span className={`status-pill status-${selected.status}`}>{selected.status}</span></div>
          <div className="confidence"><span>confidence</span><strong>{Math.round((selected.confidence || 0) * 100)}%</strong><div><i style={{ width: `${Math.max(0, Math.min(100, (selected.confidence || 0) * 100))}%` }} /></div></div>
          <h3>調査サマリー</h3><p className="preserve">{selected.summary || '(なし)'}</p>
          <h3>暫定結論</h3><div className="answer-box preserve">{selected.provisionalAnswer || '(未補完)'}</div>
          <h3>不足・不確実な情報</h3>{selected.missingInfo?.length ? <ul>{selected.missingInfo.map((x, i) => <li key={i}>{x}</li>)}</ul> : <p className="muted">特記事項なし</p>}
          <h3>出典</h3><div className="sources">{selected.sources?.length ? selected.sources.map((source, i) => <a key={`${source.url}-${i}`} href={source.url} target="_blank" rel="noreferrer">{source.title || source.url}</a>) : <p className="muted">出典なし</p>}</div>
          <h3>運営チャット / 再調査</h3><div className="chat-log">{selected.messages?.map((message, i) => <div key={i} className={`chat-message ${message.role === 'operator' ? 'operator' : 'assistant'}`}><strong>{message.role === 'operator' ? '運営' : 'Research Agent'}</strong><p className="preserve">{message.body}</p></div>)}</div>
          <textarea value={chat} onChange={e => setChat(e.currentTarget.value)} rows={4} placeholder="例: 九州全体まで範囲を広げて、当時の雑誌・料金表・BBSリストも探して" />
          <button type="button" disabled={busy || !chat.trim()} onClick={() => void run(async () => { const item = await api<ResearchCase>(`/api/admin/research/chat?id=${encodeURIComponent(selected.id)}`, { method: 'POST', body: JSON.stringify({ message: chat }) }); setChat(''); setSelected(item); setSupplement(item.provisionalAnswer || ''); setStatus(item.status); await loadCases(item.id); })}>送信して再調査</button>
          <h3>運営による最終補完</h3><textarea value={supplement} onChange={e => setSupplement(e.currentTarget.value)} rows={7} />
          <button type="button" disabled={busy || !supplement.trim()} onClick={() => void run(async () => { const item = await api<ResearchCase>(`/api/admin/research/supplement?id=${encodeURIComponent(selected.id)}`, { method: 'POST', body: JSON.stringify({ supplement }) }); setSelected(item); setStatus(item.status); await loadCases(item.id); })}>この内容で運営確認済みにする</button>
          <div className="status-editor"><select value={status} onChange={e => setStatus(e.currentTarget.value)}>{statuses.map(value => <option key={value} value={value}>{value}</option>)}</select><button type="button" disabled={busy} onClick={() => void run(async () => { const item = await api<ResearchCase>(`/api/admin/research/status?id=${encodeURIComponent(selected.id)}`, { method: 'POST', body: JSON.stringify({ status }) }); setSelected(item); await loadCases(item.id); })}>状態変更</button></div>
        </>}
      </section>
    </div>
    <p className="research-notice" role="status">{notice}</p>
  </main>;
}
