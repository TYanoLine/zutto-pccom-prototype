const BACKEND_BASE = 'https://zutto-pccom-prototype.onrender.com';

export default async function handler(req, res) {
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  if (req.method !== 'GET') {
    res.status(405).json({ error: 'GET only' });
    return;
  }

  const incoming = new URL(req.url || '/api/bbs-sample', 'https://bbs-sample.local');
  const upstream = new URL('/api/debug/bbs/sample', BACKEND_BASE);
  for (const key of ['action', 'phone', 'board']) {
    const value = incoming.searchParams.get(key);
    if (value !== null) upstream.searchParams.set(key, value);
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 15000);
  try {
    const response = await fetch(upstream, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      signal: controller.signal,
      cache: 'no-store',
    });
    const body = await response.text();
    res.status(response.status).send(body);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    res.status(502).json({ error: 'BBS sample upstream failed', detail: message });
  } finally {
    clearTimeout(timeout);
  }
}
