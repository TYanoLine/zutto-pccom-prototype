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

export const CALLER_LOCATION_STORAGE_KEY = 'zutto.callerLocation.v2';

export const CALLER_LOCATION_PRESETS: CallerLocation[] = [
  { id: 'tokyo-03', label: '東京', maName: '東京', areaCode: '03', localDialPrefixes: ['03'] },
  { id: 'fukuoka-092', label: '福岡', maName: '福岡', areaCode: '092', localDialPrefixes: ['092'] },
  { id: 'osaka-06', label: '大阪', maName: '大阪', areaCode: '06', localDialPrefixes: ['06'] },
  { id: 'nagoya-052', label: '名古屋', maName: '名古屋', areaCode: '052', localDialPrefixes: ['052'] },
  { id: 'sapporo-011', label: '札幌', maName: '札幌', areaCode: '011', localDialPrefixes: ['011'] },
  { id: 'sendai-022', label: '仙台', maName: '仙台', areaCode: '022', localDialPrefixes: ['022'] },
  { id: 'hiroshima-082', label: '広島', maName: '広島', areaCode: '082', localDialPrefixes: ['082'] },
  { id: 'yokohama-045', label: '横浜', maName: '横浜', areaCode: '045', localDialPrefixes: ['045'] },
  { id: 'kyoto-075', label: '京都', maName: '京都', areaCode: '075', localDialPrefixes: ['075'] },
];

export const DEFAULT_CALLER_LOCATION: CallerLocation = CALLER_LOCATION_PRESETS[0];

export function normalizeCallerLocation(value: Partial<CallerLocation> | null | undefined): CallerLocation {
  const fallback = DEFAULT_CALLER_LOCATION;
  if (!value) return {
    ...fallback,
    localDialPrefixes: [...fallback.localDialPrefixes],
  };

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

  if (location.localDialPrefixes.some(prefix => digits.startsWith(prefix))) return 'local';

  // Generated centers do not yet carry enough geographic metadata to resolve
  // the intermediate distance bands. Until they do, non-local destinations use
  // the far-distance fallback rather than inventing a false near-distance rate.
  return 'over-160km';
}
