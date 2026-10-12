import { describe, expect, it } from 'vitest';
import { getOrCreateCallerKey } from './CallerKey';

function fakeStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
  };
}

describe('getOrCreateCallerKey', () => {
  it('creates a 32 digit hexadecimal key once and keeps it', () => {
    const storage = fakeStorage();
    const first = getOrCreateCallerKey(storage);
    expect(first).toMatch(/^[0-9a-f]{32}$/);
    expect(getOrCreateCallerKey(storage)).toBe(first);
  });

  it('replaces a stored value that is not a key', () => {
    const storage = fakeStorage({ 'zutto.callerKey.v1': 'not a key' });
    expect(getOrCreateCallerKey(storage)).toMatch(/^[0-9a-f]{32}$/);
  });

  it('does not share the world key', () => {
    const worldKey = '0123456789abcdef0123456789abcdef';
    const storage = fakeStorage({ 'zutto.worldKey.v1': worldKey });
    expect(getOrCreateCallerKey(storage)).not.toBe(worldKey);
  });

  it('still returns a key when storage refuses to save', () => {
    const blocked = {
      getItem: () => { throw new Error('blocked'); },
      setItem: () => { throw new Error('blocked'); },
    };
    expect(getOrCreateCallerKey(blocked)).toMatch(/^[0-9a-f]{32}$/);
  });
});
