import type { DialMode } from '../audio/dialLineAudio';

export type RegisteredCenter = {
  id: string;
  name: string;
  phone: string;
  dialMode: DialMode;
  maxBaud?: number;
  builtIn?: boolean;
};

export const DEFAULT_CENTERS: RegisteredCenter[] = [
  {
    id: 'yokohama-moonlight',
    name: 'YOKOHAMA MOONLIGHT NETWORK',
    phone: '0451234567',
    dialMode: 'tone',
    maxBaud: 28800,
    builtIn: true,
  },
  {
    id: 'hakata-canal-net',
    name: 'HAKATA CANAL NET [絵理香K版]',
    phone: '0920000196',
    dialMode: 'tone',
    maxBaud: 14400,
    builtIn: true,
  },
];

export const CENTER_STORAGE_KEY = 'zutto.centers.v1';
export const WORLD_KEY_STORAGE_KEY = 'zutto.worldKey.v1';
const PRODUCTION_API_FALLBACK = 'https://zutto-pccom-prototype.onrender.com/api/world/bootstrap';
const CENTER_FETCH_TIMEOUT_MS = 120_000;

function digitsOnly(value: unknown): string {
  return typeof value === 'string' ? value.replace(/\D/g, '').slice(0, 20) : '';
}

function normalizeCenter(value: Partial<RegisteredCenter>, index: number): RegisteredCenter | null {
  const phone = digitsOnly(value.phone);
  if (!phone) return null;
  const name = typeof value.name === 'string' && value.name.trim()
    ? value.name.trim().slice(0, 64)
    : `CENTER ${index + 1}`;
  const dialMode: DialMode = value.dialMode === 'pulse' ? 'pulse' : 'tone';
  const id = typeof value.id === 'string' && value.id.trim()
    ? value.id.trim()
    : `center-${phone}-${index}`;
  const maxBaud = typeof value.maxBaud === 'number' ? value.maxBaud : undefined;
  return { id, name, phone, dialMode, maxBaud, builtIn: value.builtIn === true };
}

export function getOrCreateWorldKey(): string {
  if (typeof window === 'undefined') return '';
  try {
    const existing = window.localStorage.getItem(WORLD_KEY_STORAGE_KEY);
    if (existing && /^[0-9a-f]{32}$/.test(existing)) return existing;
  } catch {
    // Continue with an in-memory key for this page load if storage is blocked.
  }

  const bytes = new Uint8Array(16);
  window.crypto.getRandomValues(bytes);
  const key = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('');
  try { window.localStorage.setItem(WORLD_KEY_STORAGE_KEY, key); } catch { /* optional */ }
  return key;
}

export function loadCenters(): RegisteredCenter[] {
  const defaults = DEFAULT_CENTERS.map(center => ({ ...center }));
  if (typeof window === 'undefined') return defaults;
  try {
    const raw = window.localStorage.getItem(CENTER_STORAGE_KEY);
    if (!raw) return defaults;
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return defaults;
    const custom = parsed
      .map((value, index) => normalizeCenter(value as Partial<RegisteredCenter>, index))
      .filter((value): value is RegisteredCenter => value !== null && !value.builtIn);
    const seen = new Set(defaults.map(center => center.phone));
    return defaults.concat(custom.filter(center => {
      if (seen.has(center.phone)) return false;
      seen.add(center.phone);
      return true;
    }));
  } catch {
    return defaults;
  }
}

export function saveCenters(centers: RegisteredCenter[]): void {
  if (typeof window === 'undefined') return;
  const custom = centers.filter(center => !center.builtIn);
  try {
    window.localStorage.setItem(CENTER_STORAGE_KEY, JSON.stringify(custom));
  } catch {
    // Persistence is optional.
  }
}

export async function fetchWorldCenters(wsURL = ''): Promise<RegisteredCenter[]> {
  const worldKey = getOrCreateWorldKey();
  let endpoint = isLocalPage() ? '/api/world/bootstrap' : PRODUCTION_API_FALLBACK;
  if (wsURL) {
    const url = new URL(wsURL, window.location.href);
    url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
    url.pathname = '/api/world/bootstrap';
    url.search = '';
    endpoint = url.toString();
  }
  const url = new URL(endpoint, window.location.href);
  url.searchParams.set('key', worldKey);

  // A first visit may wake Render, create the world, call the naming model and
  // commit 100 hosts. Later visits to the same browser key should be DB reads.
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), CENTER_FETCH_TIMEOUT_MS);
  try {
    const response = await fetch(url.toString(), { signal: controller.signal });
    if (!response.ok) {
      let detail = '';
      try { detail = ((await response.json()) as { error?: string }).error ?? ''; } catch { /* ignore */ }
      throw new Error(detail || `center directory: ${response.status}`);
    }
    const payload = await response.json() as { centers?: Partial<RegisteredCenter>[] };
    if (!Array.isArray(payload.centers)) throw new Error('center directory: invalid response');
    const generated = payload.centers
      .map((value, index) => normalizeCenter({ ...value, builtIn: true }, index))
      .filter((value): value is RegisteredCenter => value !== null);
    return generated;
  } finally {
    window.clearTimeout(timeout);
  }
}

function isLocalPage(): boolean {
  return typeof window !== 'undefined'
    && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
}
