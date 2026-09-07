const BACKEND_BASE = 'https://zutto-pccom-prototype.onrender.com';
const ONE_SHOT_KEY = 'fresh-conv-poc-7f4a2c9e';

export default async function handler(req, res) {
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  if (req.method !== 'GET') {
    res.status(405).json({ error: 'GET only' });
    return;
  }
  const incoming = new URL(req.url || '/api/materialization-lab-fresh-run-once', 'https://fresh-run.local');
  if (incoming.searchParams.get('key') !== ONE_SHOT_KEY) {
    res.status(404).json({ error: 'not found' });
    return;
  }
  const upstream = new URL('/api/debug/materialization-lab-fresh', BACKEND_BASE);
  upstream.searchParams.set('action', 'start');
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(upstream, {
      method: 'POST',
      headers: { Accept: 'application/json' },
      signal: controller.signal,
      cache: 'no-store',
    });
    const body = await response.text();
    res.status(response.status).send(body);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    res.status(502).json({ error: 'fresh materialization lab start failed', detail: message });
  } finally {
    clearTimeout(timeout);
  }
}
