import { describe, expect, it } from 'vitest';
import {
  clearLegacyDirectoryStorage,
  directoryEndpoint,
  getOrCreateWorldKey,
  mergeCenters,
  parseDirectory,
  worldCentersEndpoint,
} from './CenterDirectory';
import type { RegisteredCenter } from './CenterDirectory';

const HAKATA: RegisteredCenter = {
  id: 'hakata-canal-net',
  name: 'HAKATA CANAL NET [絵理香K版]',
  phone: '0920000196',
  dialMode: 'tone',
  maxBaud: 14400,
};

function center(phone: string, name = `CENTER ${phone}`): RegisteredCenter {
  return { id: `center-${phone}`, name, phone, dialMode: 'tone' };
}

function fakeStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => { data.set(key, value); },
    removeItem: (key: string) => { data.delete(key); },
  };
}

describe('parseDirectory', () => {
  it('formats HAKATA with its software label', () => {
    expect(parseDirectory({
      centers: [{
        id: 'hakata-canal-net',
        name: 'HAKATA CANAL NET',
        software: '絵理香K版',
        phone: '0920000196',
        dialMode: 'tone',
        maxBaud: 14400,
      }],
    })).toEqual([HAKATA]);
  });

  it('does not add brackets without software', () => {
    expect(parseDirectory({ centers: [{ name: 'PLAIN', phone: '0312345678' }] })[0].name).toBe('PLAIN');
  });

  it.each([undefined, null, {}, 'centers'])('rejects an invalid response: %p', payload => {
    expect(() => parseDirectory(payload)).toThrow('center directory: invalid response');
  });

  it('normalizes phone numbers, drops missing phones, and keeps the first duplicate', () => {
    expect(parseDirectory({
      centers: [
        { id: 'first', name: 'FIRST', phone: '092-000-0196' },
        { id: 'missing', name: 'MISSING' },
        { id: 'second', name: 'SECOND', phone: '0920000196' },
      ],
    })).toEqual([{
      id: 'first',
      name: 'FIRST',
      phone: '0920000196',
      dialMode: 'tone',
    }]);
  });

  it('normalizes dial mode and maximum speed', () => {
    expect(parseDirectory({
      centers: [
        { phone: '0111111111', dialMode: 'pulse', maxBaud: 9600 },
        { phone: '0222222222', dialMode: 'x', maxBaud: '14400' },
        { phone: '0333333333', maxBaud: Number.NaN },
      ],
    })).toEqual([
      { id: 'center-0111111111', name: 'CENTER 1', phone: '0111111111', dialMode: 'pulse', maxBaud: 9600 },
      { id: 'center-0222222222', name: 'CENTER 2', phone: '0222222222', dialMode: 'tone' },
      { id: 'center-0333333333', name: 'CENTER 3', phone: '0333333333', dialMode: 'tone' },
    ]);
  });

  it('accepts an empty directory', () => {
    expect(parseDirectory({ centers: [] })).toEqual([]);
  });
});

describe('mergeCenters', () => {
  it('lists the earlier list first, in order', () => {
    const merged = mergeCenters([HAKATA], [center('0111111111'), center('0222222222')]);
    expect(merged.map(c => c.phone)).toEqual(['0920000196', '0111111111', '0222222222']);
  });

  it('lets the earlier list win when a phone number appears in both', () => {
    // A generated station can never shadow a preset station.
    const merged = mergeCenters([HAKATA], [center('0920000196', 'IMPOSTOR'), center('0111111111')]);
    expect(merged).toEqual([HAKATA, center('0111111111')]);
  });

  it('drops duplicates inside one list, keeping the first', () => {
    const merged = mergeCenters([], [center('0111111111', 'FIRST'), center('0111111111', 'SECOND')]);
    expect(merged.map(c => c.name)).toEqual(['FIRST']);
  });

  it('handles empty lists', () => {
    expect(mergeCenters()).toEqual([]);
    expect(mergeCenters([], [])).toEqual([]);
    expect(mergeCenters([HAKATA], [])).toEqual([HAKATA]);
  });

  it('does not modify its inputs', () => {
    const presets = [HAKATA];
    const generated = [center('0111111111')];
    mergeCenters(presets, generated);
    expect(presets).toEqual([HAKATA]);
    expect(generated).toEqual([center('0111111111')]);
  });
});

describe('endpoints', () => {
  it('uses the WebSocket server origin when configured', () => {
    expect(directoryEndpoint('ws://localhost:8080/ws?query=1', 'http://localhost:5173/', true))
      .toBe('http://localhost:8080/api/directory');
    expect(directoryEndpoint('wss://api.example.com/ws', 'http://localhost:5173/', false))
      .toBe('https://api.example.com/api/directory');
    expect(worldCentersEndpoint('ws://localhost:8080/ws?query=1', 'http://localhost:5173/', true))
      .toBe('http://localhost:8080/api/world/bootstrap');
    expect(worldCentersEndpoint('wss://api.example.com/ws', 'http://localhost:5173/', false))
      .toBe('https://api.example.com/api/world/bootstrap');
  });

  it('uses the local or production endpoint without a WebSocket URL', () => {
    expect(directoryEndpoint('', 'http://localhost:5173/', true)).toBe('/api/directory');
    expect(directoryEndpoint('', 'https://zutto.example/', false))
      .toBe('https://zutto-pccom-prototype.onrender.com/api/directory');
    expect(worldCentersEndpoint('', 'http://localhost:5173/', true)).toBe('/api/world/bootstrap');
    expect(worldCentersEndpoint('', 'https://zutto.example/', false))
      .toBe('https://zutto-pccom-prototype.onrender.com/api/world/bootstrap');
  });
});

describe('getOrCreateWorldKey', () => {
  it('creates a 32-digit hex key and stores it', () => {
    const storage = fakeStorage();
    const key = getOrCreateWorldKey(storage);
    expect(key).toMatch(/^[0-9a-f]{32}$/);
    expect(storage.data.get('zutto.worldKey.v1')).toBe(key);
  });

  it('returns the stored key on later visits', () => {
    const stored = 'abcdef0123456789abcdef0123456789';
    expect(getOrCreateWorldKey(fakeStorage({ 'zutto.worldKey.v1': stored }))).toBe(stored);
  });

  it('replaces a stored value that is not a key', () => {
    const storage = fakeStorage({ 'zutto.worldKey.v1': 'not-a-key' });
    const key = getOrCreateWorldKey(storage);
    expect(key).toMatch(/^[0-9a-f]{32}$/);
    expect(storage.data.get('zutto.worldKey.v1')).toBe(key);
  });

  it('still returns a key when the storage is blocked', () => {
    const blocked = {
      getItem: () => { throw new Error('blocked'); },
      setItem: () => { throw new Error('blocked'); },
    };
    expect(getOrCreateWorldKey(blocked)).toMatch(/^[0-9a-f]{32}$/);
  });
});

describe('clearLegacyDirectoryStorage', () => {
  it('removes the old custom directory and keeps the world key', () => {
    const storage = fakeStorage({
      'zutto.centers.v1': '[]',
      'zutto.worldKey.v1': 'abcdef0123456789abcdef0123456789',
      other: 'x',
    });
    clearLegacyDirectoryStorage(storage);
    expect([...storage.data.keys()].sort()).toEqual(['other', 'zutto.worldKey.v1']);
  });

  it('ignores storage errors', () => {
    expect(() => clearLegacyDirectoryStorage({ removeItem: () => { throw new Error('blocked'); } })).not.toThrow();
  });
});
