import { describe, expect, it } from 'vitest';
import { Japan1996WorldClock } from './WorldClock';

describe('Japan1996WorldClock', () => {
  it('maps Japan time onto the configured 1996 date', () => {
    const source = () => new Date('2026-08-26T13:59:30.000Z'); // 22:59 JST
    const clock = new Japan1996WorldClock('1996-08-26', source);
    expect(clock.now().toISOString()).toBe('1996-08-26T22:59:30.000Z');
  });
});
