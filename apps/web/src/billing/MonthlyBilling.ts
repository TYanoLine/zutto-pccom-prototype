export const MONTHLY_BILLING_STORAGE_KEY = 'zutto.monthlyBilling.v1';

export type KeyValueStorage = {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem?(key: string): void;
};

function getStorage(custom?: KeyValueStorage): KeyValueStorage | null {
  if (custom) return custom;
  if (typeof window !== 'undefined' && window.localStorage) return window.localStorage;
  return null;
}

export function getMonthKey(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  return `${y}-${m}`;
}

export function loadMonthlyTotal(monthKey: string, storage?: KeyValueStorage): number {
  const store = getStorage(storage);
  if (!store) return 0;
  try {
    const raw = store.getItem(MONTHLY_BILLING_STORAGE_KEY);
    if (!raw) return 0;
    const parsed = JSON.parse(raw) as Record<string, number>;
    return typeof parsed[monthKey] === 'number' ? parsed[monthKey] : 0;
  } catch {
    return 0;
  }
}

export function addMonthlyCharge(monthKey: string, yen: number, storage?: KeyValueStorage): number {
  if (yen <= 0) return loadMonthlyTotal(monthKey, storage);
  const store = getStorage(storage);
  if (!store) return 0;
  try {
    const raw = store.getItem(MONTHLY_BILLING_STORAGE_KEY);
    const parsed = raw ? (JSON.parse(raw) as Record<string, number>) : {};
    const current = typeof parsed[monthKey] === 'number' ? parsed[monthKey] : 0;
    const next = current + yen;
    parsed[monthKey] = next;
    store.setItem(MONTHLY_BILLING_STORAGE_KEY, JSON.stringify(parsed));
    return next;
  } catch {
    return loadMonthlyTotal(monthKey, storage);
  }
}
