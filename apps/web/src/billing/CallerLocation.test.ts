import { describe, expect, it } from 'vitest';
import {
  DEFAULT_CALLER_LOCATION,
  normalizeCallerLocation,
  resolve1996DistanceClass,
} from './CallerLocation';

describe('caller location', () => {
  it('defaults to the Fukuoka MA preset requested for the prototype', () => {
    const location = normalizeCallerLocation(null);
    expect(location.id).toBe('fukuoka-092');
    expect(location.maName).toBe('福岡');
    expect(location.areaCode).toBe('092');
  });

  it('does not assume every 092 number belongs to the Fukuoka MA', () => {
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0920000196')).toBe('local');
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0923200000')).toBe('over-160km');
  });

  it('keeps known Yokohama and Tokyo fixtures in the far-distance fallback from Fukuoka', () => {
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0451234567')).toBe('over-160km');
    expect(resolve1996DistanceClass(DEFAULT_CALLER_LOCATION, '0312345678')).toBe('over-160km');
  });
});
