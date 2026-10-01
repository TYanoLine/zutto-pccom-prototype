import { readFileSync } from 'node:fs';

const config = JSON.parse(readFileSync(new URL('../vercel.json', import.meta.url), 'utf8'));
if (!Array.isArray(config.rewrites)) throw new Error('vercel.json must contain a rewrites array');
if (!config.rewrites.some(r => r?.source === '/admin/research' && r?.destination === '/')) {
  throw new Error('vercel.json must rewrite /admin/research to /');
}
if (config.rewrites.some(r => r?.source?.startsWith('/poc/'))) {
  throw new Error('retired PoC routes must not be present in Vercel rewrites');
}
console.log('vercel.json OK');
