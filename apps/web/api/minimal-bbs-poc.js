const BACKEND_BASE = 'https://zutto-pccom-prototype.onrender.com';

export default async function handler(req, res) {
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  if (req.method !== 'GET') {
    res.status(405).json({ error: 'GET only' });
    return;
  }

  const incoming = new URL(req.url || '/api/minimal-bbs-poc', 'https://minimal-bbs-poc.local');
  const batch = incoming.searchParams.get('mode') === 'batch';
  const upstream = new URL(
    batch ? '/api/debug/minimal-situation-title-batch-poc' : '/api/debug/minimal-bbs-poc',
    BACKEND_BASE
  );
  for (const key of batch ? ['model', 'phone', 'board', 'count', 'offset', 'lookback_days'] : ['model']) {
    const value = incoming.searchParams.get(key);
    if (value !== null) upstream.searchParams.set(key, value);
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), batch ? 180000 : 120000);
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
    res.status(502).json({ error: 'minimal BBS PoC upstream failed', detail: message });
  } finally {
    clearTimeout(timeout);
  }
}