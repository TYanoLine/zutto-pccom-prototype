export type ServerBuildInfo = {
  commit: string;
  branch: string;
  started_at: string;
};

// Query the backend configured for this exact frontend build. In particular,
// a Vercel preview must not silently display the version of a different backend.
export function serverVersionEndpoint(wsURL: string): string | null {
  if (!wsURL) return null;
  let url: URL;
  try {
    url = new URL(wsURL, typeof window === 'undefined' ? 'http://localhost' : window.location.href);
  } catch {
    return null;
  }
  if (url.protocol !== 'ws:' && url.protocol !== 'wss:') return null;
  url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
  url.pathname = '/api/version';
  url.search = '';
  url.hash = '';
  return url.toString();
}

export async function fetchServerBuildInfo(
  wsURL: string,
  fetcher: typeof fetch = fetch,
  signal?: AbortSignal,
): Promise<ServerBuildInfo> {
  const endpoint = serverVersionEndpoint(wsURL);
  if (!endpoint) throw new Error('backend not configured');

  const controller = new AbortController();
  const onAbort = () => controller.abort();
  if (signal?.aborted) controller.abort();
  signal?.addEventListener('abort', onAbort, { once: true });
  // Metadata retrieval must never delay the terminal, directory or modem.
  // Allow a cold Render instance to wake while keeping the request bounded.
  const timer = setTimeout(() => controller.abort(), 30_000);
  try {
    const response = await fetcher(endpoint, {
      method: 'GET',
      cache: 'no-store',
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`server version: HTTP ${response.status}`);
    const value: unknown = await response.json();
    if (!value || typeof value !== 'object') throw new Error('invalid server version');
    const data = value as Record<string, unknown>;
    if (
      typeof data.commit !== 'string' ||
      typeof data.branch !== 'string' ||
      typeof data.started_at !== 'string' ||
      !data.commit.trim()
    ) {
      throw new Error('invalid server version');
    }
    return { commit: data.commit, branch: data.branch, started_at: data.started_at };
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener('abort', onAbort);
  }
}

export function shortBuildCommit(commit: string): string {
  return /^[0-9a-f]{12,}$/i.test(commit) ? commit.slice(0, 12) : (commit || '不明');
}
