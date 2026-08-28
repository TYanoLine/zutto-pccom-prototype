import type { DialMode } from '../audio/dialLineAudio';

export type RegisteredCenter = {
  id: string;
  name: string;
  phone: string;
  dialMode: DialMode;
  builtIn?: boolean;
};

export const DEFAULT_CENTERS: RegisteredCenter[] = [
  {
    id: 'yokohama-moonlight',
    name: 'YOKOHAMA MOONLIGHT NETWORK',
    phone: '0451234567',
    dialMode: 'tone',
    builtIn: true,
  },
];

export const CENTER_STORAGE_KEY = 'zutto.centers.v1';

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
  return { id, name, phone, dialMode, builtIn: value.builtIn === true };
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
