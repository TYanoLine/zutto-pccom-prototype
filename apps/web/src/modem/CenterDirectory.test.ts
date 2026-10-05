import { describe, expect, it } from 'vitest';
import {
  clearLegacyDirectoryStorage,
  directoryEndpoint,
  parseDirectory,
} from './CenterDirectory';

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
    })).toEqual([{
      id: 'hakata-canal-net',
      name: 'HAKATA CANAL NET [絵理香K版]',
      phone: '0920000196',
      dialMode: 'tone',
      maxBaud: 14400,
    }]);
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

describe('directoryEndpoint', () => {
  it('uses the WebSocket server origin when configured', () => {
    expect(directoryEndpoint('ws://localhost:8080/ws?query=1', 'http://localhost:5173/', true))
      .toBe('http://localhost:8080/api/directory');
    expect(directoryEndpoint('wss://api.example.com/ws', 'http://localhost:5173/', false))
      .toBe('https://api.example.com/api/directory');
  });

  it('uses the local or production endpoint without a WebSocket URL', () => {
    expect(directoryEndpoint('', 'http://localhost:5173/', true)).toBe('/api/directory');
    expect(directoryEndpoint('', 'https://zutto.example/', false))
      .toBe('https://zutto-pccom-prototype.onrender.com/api/directory');
  });
});

describe('clearLegacyDirectoryStorage', () => {
  it('removes only the two legacy keys', () => {
    const removed: string[] = [];
    clearLegacyDirectoryStorage({ removeItem: key => removed.push(key) });
    expect(removed).toEqual(['zutto.centers.v1', 'zutto.worldKey.v1']);
  });

  it('ignores storage errors', () => {
    expect(() => clearLegacyDirectoryStorage({ removeItem: () => { throw new Error('blocked'); } })).not.toThrow();
  });
});
