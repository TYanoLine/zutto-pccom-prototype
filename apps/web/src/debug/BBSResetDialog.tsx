import { useEffect, useState } from 'react';
import { serverVersionEndpoint } from '../build/ServerBuildInfo';
import './BBSResetDialog.css';

export type BBSResetMode = 'generated' | 'all';

export type BBSResetResult = {
  ok: boolean;
  phone: string;
  host_id: string;
  removed_posts: number;
  mode: BBSResetMode;
};

export function bbsResetEndpoint(wsURL: string, phone: string, mode: BBSResetMode): string | null {
  const base = serverVersionEndpoint(wsURL);
  if (!base) return null;
  const url = new URL(base);
  url.pathname = '/api/debug/bbs/reset';
  url.searchParams.set('phone', phone);
  if (mode === 'all') {
    url.searchParams.set('mode', 'all');
  }
  return url.toString();
}

export async function requestBBSReset(
  wsURL: string,
  phone: string,
  mode: BBSResetMode,
  token?: string,
  fetcher: typeof fetch = fetch,
): Promise<BBSResetResult> {
  const endpoint = bbsResetEndpoint(wsURL, phone, mode);
  if (!endpoint) throw new Error('接続先サーバーが設定されていません');
  const headers: Record<string, string> = {};
  if (token && token.trim()) {
    headers['X-Zutto-Debug-Token'] = token.trim();
  }
  const response = await fetcher(endpoint, {
    method: 'POST',
    cache: 'no-store',
    headers,
  });
  let data: Record<string, unknown> = {};
  try {
    data = (await response.json()) as Record<string, unknown>;
  } catch {
    throw new Error(`HTTP ${response.status} ${response.statusText}`);
  }
  if (!response.ok) {
    const errorMsg = typeof data.error === 'string' ? data.error : `リセット失敗: HTTP ${response.status}`;
    throw new Error(errorMsg);
  }
  return {
    ok: Boolean(data.ok),
    phone: String(data.phone ?? phone),
    host_id: String(data.host_id ?? ''),
    removed_posts: Number(data.removed_posts ?? data.removed_generated_posts ?? 0),
    mode: data.mode === 'all' ? 'all' : 'generated',
  };
}

const STORAGE_TOKEN_KEY = 'zutto.debugResetToken';
const DEFAULT_TARGET_PHONE = '0920000196';

type Props = {
  wsURL: string;
  open: boolean;
  activePhone?: string;
  activeHostName?: string;
  onClose: () => void;
  onResetComplete?: (result: BBSResetResult) => void;
};

export function BBSResetDialog({
  wsURL,
  open,
  activePhone,
  activeHostName,
  onClose,
  onResetComplete,
}: Props) {
  const targetPhone = activePhone || DEFAULT_TARGET_PHONE;
  const targetName = activeHostName || (targetPhone === DEFAULT_TARGET_PHONE ? 'HAKATA CANAL NET' : targetPhone);

  const [mode, setMode] = useState<BBSResetMode>('generated');
  const [token, setToken] = useState(() => {
    try {
      return sessionStorage.getItem(STORAGE_TOKEN_KEY) ?? '';
    } catch {
      return '';
    }
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    if (open) {
      setError('');
      setSuccess('');
    }
  }, [open]);

  if (!open) return null;

  async function handleReset() {
    setBusy(true);
    setError('');
    setSuccess('');
    try {
      if (token) {
        try { sessionStorage.setItem(STORAGE_TOKEN_KEY, token); } catch { /* ignore */ }
      }
      const result = await requestBBSReset(wsURL, targetPhone, mode, token);
      const modeText = result.mode === 'all' ? '全記事' : '生成記事';
      setSuccess(`リセット完了: ${result.removed_posts}件の${modeText}を消去しました。`);
      onResetComplete?.(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="bbs-reset-backdrop" role="dialog" aria-modal="true" aria-labelledby="bbs-reset-title">
      <div className="bbs-reset-dialog">
        <header className="bbs-reset-dialog__header">
          <strong id="bbs-reset-title">[DEBUG] 掲示板データリセット</strong>
          <button type="button" onClick={onClose} aria-label="閉じる" disabled={busy}>×</button>
        </header>

        <div className="bbs-reset-dialog__body">
          <div className="bbs-reset-dialog__field">
            <span className="bbs-reset-dialog__label">対象ホスト:</span>
            <span className="bbs-reset-dialog__target">{targetName} ({targetPhone})</span>
          </div>

          <div className="bbs-reset-dialog__field">
            <span className="bbs-reset-dialog__label">リセット種別:</span>
            <div className="bbs-reset-dialog__options">
              <label className="bbs-reset-dialog__option">
                <input
                  type="radio"
                  name="reset-mode"
                  value="generated"
                  checked={mode === 'generated'}
                  onChange={() => setMode('generated')}
                  disabled={busy}
                />
                <span>
                  <strong>AI生成記事のみリセット</strong>
                  <small>人間ユーザーの投稿・スレッドは保持されます</small>
                </span>
              </label>
              <label className="bbs-reset-dialog__option">
                <input
                  type="radio"
                  name="reset-mode"
                  value="all"
                  checked={mode === 'all'}
                  onChange={() => setMode('all')}
                  disabled={busy}
                />
                <span>
                  <strong>全記事リセット (完全初期化)</strong>
                  <small>人間の投稿を含むすべての記事を消去し、初回状態に戻します</small>
                </span>
              </label>
            </div>
          </div>

          <div className="bbs-reset-dialog__field">
            <label htmlFor="bbs-reset-token" className="bbs-reset-dialog__label">Debug Token (必要な場合):</label>
            <input
              id="bbs-reset-token"
              type="password"
              className="bbs-reset-dialog__input"
              placeholder="DEBUG_RESET_TOKEN（未設定サーバーは空欄可）"
              value={token}
              onChange={e => setToken(e.target.value)}
              disabled={busy}
            />
          </div>

          {error && <div className="bbs-reset-dialog__error">{error}</div>}
          {success && <div className="bbs-reset-dialog__success">{success}</div>}
        </div>

        <footer className="bbs-reset-dialog__footer">
          <button type="button" className="bbs-reset-dialog__btn-cancel" onClick={onClose} disabled={busy}>
            閉じる
          </button>
          <button
            type="button"
            className="bbs-reset-dialog__btn-execute"
            onClick={handleReset}
            disabled={busy}
          >
            {busy ? 'リセット実行中…' : 'リセット実行'}
          </button>
        </footer>
      </div>
    </div>
  );
}
