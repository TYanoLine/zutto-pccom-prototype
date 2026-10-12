import { describe, expect, it } from 'vitest';
import {
  CALLER_LOCATION_PRESETS,
  DEFAULT_CALLER_LOCATION,
  normalizeCallerLocation,
  resolve1996DistanceClass,
} from './CallerLocation';

describe('caller location', () => {
  it('defaults to the Tokyo (03) preset', () => {
    const location = normalizeCallerLocation(null);
    expect(location.id).toBe('tokyo-03');
    expect(location.maName).toBe('東京');
    expect(location.areaCode).toBe('03');
  });

  it('treats destinations with the configured 03 area code as local-area calls', () => {
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0312345678')).toBe('local');
  });

  it('keeps known Yokohama and Fukuoka fixtures in the far-distance fallback from Tokyo', () => {
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0920000196')).toBe('over-160km');
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0451234567')).toBe('over-160km');
  });

  it('supports Fukuoka preset for local Fukuoka calls', () => {
    const fukuoka = CALLER_LOCATION_PRESETS.find(p => p.id === 'fukuoka-092')!;
    expect(resolve1996DistanceClass(fukuoka, '0920000196')).toBe('local');
  });
});
