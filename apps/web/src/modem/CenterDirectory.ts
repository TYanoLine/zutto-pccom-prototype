import type { DialMode } from '../audio/dialLineAudio';

// One entry of the dialing directory. `name` is the display name: the station's
// name, followed by its software in brackets when it has one.
export type RegisteredCenter = {
  id: string;
  name: string;
  phone: string;
  dialMode: DialMode;
  maxBaud?: number;
};

const PRODUCTION_API_ORIGIN = 'https://zutto-pccom-prototype.onrender.com';
// The preset stations: fast, and the only ones that can be called today.
const DIRECTORY_PATH = '/api/directory';
// The stations the world generated for this browser: slow on the first visit (the
// naming model runs and the hosts are stored), and not callable yet.
const WORLD_CENTERS_PATH = '/api/world/bootstrap';
const WORLD_RESET_HOSTS_PATH = '/api/world/reset-hosts';
const WORLD_KEY_STORAGE_KEY = 'zutto.worldKey.v1';
// A first visit may wake the server (a free Render instance sleeps), and the
// first world visit also waits for the naming model.
const FETCH_TIMEOUT_MS = 120_000;
// Keys of the browser-local custom directory that no longer exists. The world key
// is not one of them: it identifies this browser's generated stations.
const LEGACY_STORAGE_KEYS = ['zutto.centers.v1'];

function digitsOnly(value: unknown): string {
  return typeof value === 'string' ? value.replace(/\D/g, '').slice(0, 20) : '';
}

function normalizeCenter(value: unknown, index: number): RegisteredCenter | null {
  if (!value || typeof value !== 'object') return null;
  const raw = value as Record<string, unknown>;
  const phone = digitsOnly(raw.phone);
  if (!phone) return null;
  const baseName = typeof raw.name === 'string' && raw.name.trim() ? raw.name.trim() : `CENTER ${index + 1}`;
  const software = typeof raw.software === 'string' ? raw.software.trim() : '';
  const name = (software ? `${baseName} [${software}]` : baseName).slice(0, 64);
  const dialMode: DialMode = raw.dialMode === 'pulse' ? 'pulse' : 'tone';
  const id = typeof raw.id === 'string' && raw.id.trim() ? raw.id.trim() : `center-${phone}`;
  const maxBaud = typeof raw.maxBaud === 'number' && Number.isFinite(raw.maxBaud) ? raw.maxBaud : undefined;
  return { id, name, phone, dialMode, maxBaud };
}

// parseDirectory turns a server response ({ centers: [...] }) into directory
// entries. Entries without a phone number are dropped, and the first entry wins
// when a number appears twice. Both the preset directory and the world's
// generated stations have this shape.
export function parseDirectory(payload: unknown): RegisteredCenter[] {
  const centers = (payload as { centers?: unknown } | null)?.centers;
  if (!Array.isArray(centers)) throw new Error('center directory: invalid response');
  const seen = new Set<string>();
  const out: RegisteredCenter[] = [];
  centers.forEach((value, index) => {
    const center = normalizeCenter(value, index);
    if (!center || seen.has(center.phone)) return;
    seen.add(center.phone);
    out.push(center);
  });
  return out;
}

// mergeCenters joins directory lists in priority order: the entries of earlier
// lists come first, and when a phone number appears in several lists the
// earliest wins. The preset stations are therefore listed first, and a generated
// station can never shadow one.
export function mergeCenters(...lists: RegisteredCenter[][]): RegisteredCenter[] {
  const seen = new Set<string>();
  const out: RegisteredCenter[] = [];
  for (const list of lists) {
    for (const center of list) {
      if (seen.has(center.phone)) continue;
      seen.add(center.phone);
      out.push(center);
    }
  }
  return out;
}

// apiEndpoint is the URL of an API path: on the origin of the WebSocket server
// when there is one, otherwise on this page (local development) or on the
// production server.
function apiEndpoint(path: string, wsURL: string, pageURL: string, localPage: boolean): string {
  if (wsURL) {
    const url = new URL(wsURL, pageURL);
    url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
    url.pathname = path;
    url.search = '';
    return url.toString();
  }
  return localPage ? path : `${PRODUCTION_API_ORIGIN}${path}`;
}

export function directoryEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
  return apiEndpoint(DIRECTORY_PATH, wsURL, pageURL, localPage);
}

export function worldCentersEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
  return apiEndpoint(WORLD_CENTERS_PATH, wsURL, pageURL, localPage);
}

export function worldResetHostsEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
  return apiEndpoint(WORLD_RESET_HOSTS_PATH, wsURL, pageURL, localPage);
}

// clearLegacyDirectoryStorage removes the keys of the old browser-local custom
// directory. Cleanup is optional: blocked storage must not break the directory.
export function clearLegacyDirectoryStorage(storage?: Pick<Storage, 'removeItem'>): void {
  try {
    const target = storage ?? (typeof window === 'undefined' ? undefined : window.localStorage);
    if (!target) return;
    for (const key of LEGACY_STORAGE_KEYS) target.removeItem(key);
  } catch {
    // Ignore.
  }
}

// getOrCreateWorldKey returns the key that identifies this browser's generated
// world, creating and storing one on the first visit. Blocked storage falls back
// to an in-memory key for this page load.
export function getOrCreateWorldKey(storage?: Pick<Storage, 'getItem' | 'setItem'>): string {
  let target: Pick<Storage, 'getItem' | 'setItem'> | undefined;
  try {
    target = storage ?? (typeof window === 'undefined' ? undefined : window.localStorage);
  } catch {
    target = undefined;
  }
  try {
    const existing = target?.getItem(WORLD_KEY_STORAGE_KEY);
    if (existing && /^[0-9a-f]{32}$/.test(existing)) return existing;
  } catch {
    // Continue with a new key.
  }
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  const key = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('');
  try { target?.setItem(WORLD_KEY_STORAGE_KEY, key); } catch { /* optional */ }
  return key;
}

async function fetchCenters(url: URL): Promise<RegisteredCenter[]> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const response = await fetch(url.toString(), { signal: controller.signal });
    if (!response.ok) {
      let detail = '';
      try { detail = ((await response.json()) as { error?: string }).error ?? ''; } catch { /* ignore */ }
      throw new Error(detail || `center directory: ${response.status}`);
    }
    return parseDirectory(await response.json());
  } finally {
    window.clearTimeout(timeout);
  }
}

// fetchDirectory loads the preset stations.
export async function fetchDirectory(wsURL = ''): Promise<RegisteredCenter[]> {
  const endpoint = directoryEndpoint(wsURL, window.location.href, isLocalPage());
  return fetchCenters(new URL(endpoint, window.location.href));
}

// fetchWorldCenters loads the stations the world generated for this browser. The
// first visit can take a long time: the server runs the naming model and stores
// the hosts. Later visits read them back.
export async function fetchWorldCenters(wsURL = ''): Promise<RegisteredCenter[]> {
  const endpoint = worldCentersEndpoint(wsURL, window.location.href, isLocalPage());
  const url = new URL(endpoint, window.location.href);
  url.searchParams.set('key', getOrCreateWorldKey());
  return fetchCenters(url);
}

export async function resetWorldHosts(wsURL = ''): Promise<RegisteredCenter[]> {
  const endpoint = worldResetHostsEndpoint(wsURL, window.location.href, isLocalPage());
  const url = new URL(endpoint, window.location.href);
  url.searchParams.set('key', getOrCreateWorldKey());
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const response = await fetch(url.toString(), { method: 'POST', signal: controller.signal });
    if (!response.ok) {
      let detail = '';
      try { detail = ((await response.json()) as { error?: string }).error ?? ''; } catch { /* ignore */ }
      throw new Error(detail || `reset hosts: ${response.status}`);
    }
    return parseDirectory(await response.json());
  } finally {
    window.clearTimeout(timeout);
  }
}

function isLocalPage(): boolean {
  return typeof window !== 'undefined'
    && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
}
