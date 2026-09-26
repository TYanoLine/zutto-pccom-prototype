export type TariffDistanceClass =
  | 'local'
  | 'adjacent-or-20km'
  | 'up-to-30km'
  | 'up-to-60km'
  | 'up-to-100km'
  | 'up-to-160km'
  | 'over-160km';

export type CallerLocation = {
  id: string;
  label: string;
  maName: string;
  areaCode: string;
  localDialPrefixes: string[];
};

export const CALLER_LOCATION_STORAGE_KEY = 'zutto.callerLocation.v1';

export const DEFAULT_CALLER_LOCATION: CallerLocation = {
  id: 'fukuoka-092',
  label: '福岡',
  maName: '福岡',
  areaCode: '092',
  // For the current prototype this preset represents the Fukuoka MA.
  // Future location UI can replace this preset with a canonical MA record.
  localDialPrefixes: ['092'],
};

export function normalizeCallerLocation(value: Partial<CallerLocation> | null | undefined): CallerLocation {
  const fallback = DEFAULT_CALLER_LOCATION;
  if (!value) return { ...fallback, localDialPrefixes: [...fallback.localDialPrefixes] };

  const cleanPrefix = (prefix: unknown) =>
    typeof prefix === 'string' ? prefix.replace(/\D/g, '').slice(0, 6) : '';
  const areaCode = cleanPrefix(value.areaCode) || fallback.areaCode;
  const prefixes = Array.isArray(value.localDialPrefixes)
    ? value.localDialPrefixes.map(cleanPrefix).filter(Boolean)
    : [];

  return {
    id: typeof value.id === 'string' && value.id.trim() ? value.id.trim() : fallback.id,
    label: typeof value.label === 'string' && value.label.trim() ? value.label.trim() : fallback.label,
    maName: typeof value.maName === 'string' && value.maName.trim() ? value.maName.trim() : fallback.maName,
    areaCode,
    localDialPrefixes: prefixes.length ? prefixes : [areaCode],
  };
}

export function loadCallerLocation(): CallerLocation {
  if (typeof window === 'undefined') return normalizeCallerLocation(null);
  try {
    const raw = window.localStorage.getItem(CALLER_LOCATION_STORAGE_KEY);
    return raw ? normalizeCallerLocation(JSON.parse(raw) as Partial<CallerLocation>) : normalizeCallerLocation(null);
  } catch {
    return normalizeCallerLocation(null);
  }
}

export function saveCallerLocation(location: CallerLocation): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(CALLER_LOCATION_STORAGE_KEY, JSON.stringify(normalizeCallerLocation(location)));
  } catch {
    // Location persistence is optional; the default preset remains usable.
  }
}

export function resolve1996DistanceClass(location: CallerLocation, phone: string): TariffDistanceClass {
  const digits = phone.replace(/\D/g, '');

  // A real tariff engine should resolve both endpoints to historical MAs and
  // measure the official MA-to-MA tariff distance. The current world only has
  // canonical MA metadata for the Fukuoka fixture and a few development
  // destinations, so keep the resolver explicit rather than pretending that
  // every fictional phone prefix is historically geocoded.
  if (location.id === 'fukuoka-092') {
    if (location.localDialPrefixes.some(prefix => digits.startsWith(prefix))) return 'local';
    if (digits.startsWith('045') || digits.startsWith('03')) return 'over-160km';
  }

  // Until generated centers carry canonical MA metadata, unknown destinations
  // use the far-distance band rather than inventing a false near-distance rate.
  return 'over-160km';
}
