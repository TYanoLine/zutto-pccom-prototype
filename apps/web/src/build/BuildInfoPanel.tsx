import type { ServerBuildInfo } from './ServerBuildInfo';
import { shortBuildCommit } from './ServerBuildInfo';

export type BuildInfoState = 'loading' | 'ready' | 'unavailable' | 'not-configured';

export type BuildInfoPanelProps = {
  clientCommit: string;
  clientRef: string;
  clientBuildTime: string;
  server: ServerBuildInfo | null;
  serverState: BuildInfoState;
  onRetry: () => void;
};

function readableDate(value: string): string {
  if (!value || Number.isNaN(Date.parse(value))) return '不明';
  return new Date(value).toLocaleString('ja-JP');
}

export function BuildInfoPanel({
  clientCommit, clientRef, clientBuildTime, server, serverState, onRetry,
}: BuildInfoPanelProps) {
  const comparable = serverState === 'ready' && server && server.commit !== 'unknown' && clientCommit !== 'unknown';
  const match = comparable && server.commit === clientCommit;
  return <section className="build-info" aria-label="バージョン情報">
    <span className="build-info__heading">バージョン情報</span>
    <div>クライアント: <code title={clientCommit}>{shortBuildCommit(clientCommit)}</code></div>
    <div>ブランチ: <code>{clientRef || 'unknown'}</code></div>
    <div>ビルド日時: <time>{readableDate(clientBuildTime)}</time></div>
    {serverState === 'ready' && server ? <>
      <div>サーバー: <code title={server.commit}>{shortBuildCommit(server.commit)}</code></div>
      <div>ブランチ: <code>{server.branch}</code></div>
      <div>起動日時: <time>{readableDate(server.started_at)}</time></div>
      {comparable && <div className="build-info__comparison">{match ? '同一コミット' : 'コミットが異なります（新旧は未判定）'}</div>}
    </> : <div className="build-info__state">
      {serverState === 'loading' ? 'サーバー情報を取得中…' :
       serverState === 'not-configured' ? 'サーバー未設定' : 'サーバー情報を取得できません'}
    </div>}
    {serverState === 'unavailable' && <button type="button" onClick={onRetry}>再取得</button>}
  </section>;
}
