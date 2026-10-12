// The key this browser keeps so the server can tell the same user across calls
// (spec 019). It is a bearer secret: it is sent only with a dial, and the
// server stores a one-way digest, never the key. It identifies a browser, not a
// login ID; it proves nothing about who is typing.
const CALLER_KEY_STORAGE_KEY = 'zutto.callerKey.v1';

export function getOrCreateCallerKey(storage?: Pick<Storage, 'getItem' | 'setItem'>): string {
  let target: Pick<Storage, 'getItem' | 'setItem'> | undefined;
  try {
    target = storage ?? (typeof window === 'undefined' ? undefined : window.localStorage);
  } catch {
    target = undefined;
  }
  try {
    const existing = target?.getItem(CALLER_KEY_STORAGE_KEY);
    if (existing && /^[0-9a-f]{32}$/.test(existing)) return existing;
  } catch {
    // Continue with a new key.
  }
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  const key = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('');
  try { target?.setItem(CALLER_KEY_STORAGE_KEY, key); } catch { /* optional */ }
  return key;
}
