import { describe, expect, it } from 'vitest';
import { CalendarWorldClock, Japan1996WorldClock } from './WorldClock';

describe('Japan1996WorldClock', () => {
  it('maps Japan time onto the configured 1996 date', () => {
    const source = () => new Date('2026-08-26T13:59:30.000Z'); // 22:59 JST
    const clock = new Japan1996WorldClock('1996-08-26', source);
    expect(clock.now().toISOString()).toBe('1996-08-26T22:59:30.000Z');
  });
});

// Shared in content with apps/server/internal/worldclock/clock_test.go
// (spec 015 plan §3.3). Real times are Japan time.
const calendarClockCases: Array<[string, string]> = [
  ['2026-10-12T11:54:00+09:00', '1996-10-12T11:54:00.000Z'],
  ['2026-12-24T22:59:59+09:00', '1996-12-24T22:59:59.000Z'],
  ['2026-12-24T23:00:00+09:00', '1996-12-24T23:00:00.000Z'],
  ['2026-12-31T23:59:59+09:00', '1996-12-31T23:59:59.000Z'],
  ['2027-01-01T00:00:00+09:00', '1997-01-01T00:00:00.000Z'],
];

describe('CalendarWorldClock', () => {
  it.each(calendarClockCases)('maps real %s to world %s', (real, world) => {
    const clock = new CalendarWorldClock(1996, 2026, () => new Date(real));
    expect(clock.now().toISOString()).toBe(world);
  });

  it('uses the 1996 weekday', () => {
    const clock = new CalendarWorldClock(1996, 2026, () => new Date('2026-12-24T21:00:00+09:00'));
    expect(clock.now().getUTCDay()).toBe(2); // Tuesday
  });

  it('supports another base year', () => {
    const clock = new CalendarWorldClock(1990, 2026, () => new Date('2026-10-12T11:54:00+09:00'));
    expect(clock.now().toISOString()).toBe('1990-10-12T11:54:00.000Z');
  });

  it('rejects a base year after the start year', () => {
    expect(() => new CalendarWorldClock(2030, 2026)).toThrow();
  });
});
