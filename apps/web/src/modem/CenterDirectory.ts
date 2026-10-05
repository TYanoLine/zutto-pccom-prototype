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
const DIRECTORY_PATH = '/api/directory';
// A first visit may wake the server (a free Render instance sleeps).
const DIRECTORY_FETCH_TIMEOUT_MS = 120_000;
// Keys of the browser-local directory that no longer exists.
const LEGACY_STORAGE_KEYS = ['zutto.centers.v1', 'zutto.worldKey.v1'];

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

// parseDirectory turns the server's response into directory entries. Entries
// without a phone number are dropped, and the first entry wins when a number
// appears twice.
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

// directoryEndpoint is the URL of the directory API: on the origin of the
// WebSocket server when there is one, otherwise on this page (local
// development) or on the production server.
export function directoryEndpoint(wsURL: string, pageURL: string, localPage: boolean): string {
  if (wsURL) {
    const url = new URL(wsURL, pageURL);
    url.protocol = url.protocol === 'wss:' ? 'https:' : 'http:';
    url.pathname = DIRECTORY_PATH;
    url.search = '';
    return url.toString();
  }
  return localPage ? DIRECTORY_PATH : `${PRODUCTION_API_ORIGIN}${DIRECTORY_PATH}`;
}

// clearLegacyDirectoryStorage removes the keys of the old browser-local
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

export async function fetchDirectory(wsURL = ''): Promise<RegisteredCenter[]> {
  const endpoint = directoryEndpoint(wsURL, window.location.href, isLocalPage());
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), DIRECTORY_FETCH_TIMEOUT_MS);
  try {
    const response = await fetch(new URL(endpoint, window.location.href).toString(), { signal: controller.signal });
    if (!response.ok) throw new Error(`center directory: ${response.status}`);
    return parseDirectory(await response.json());
  } finally {
    window.clearTimeout(timeout);
  }
}

function isLocalPage(): boolean {
  return typeof window !== 'undefined'
    && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
}
