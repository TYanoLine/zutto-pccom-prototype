import { readFileSync } from 'node:fs';

const raw = readFileSync(new URL('../vercel.json', import.meta.url), 'utf8');
const config = JSON.parse(raw);

if (!Array.isArray(config.rewrites)) {
  throw new Error('vercel.json must contain a rewrites array');
}
if (!config.rewrites.some(r => r?.source === '/poc/persona-timeline' && r?.destination === '/')) {
  throw new Error('vercel.json must rewrite /poc/persona-timeline to /');
}

console.log('vercel.json OK');
