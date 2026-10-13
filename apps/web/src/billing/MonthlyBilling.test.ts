import { beforeEach, describe, expect, it } from 'vitest';
import {
  type KeyValueStorage,
  addMonthlyCharge,
  getMonthKey,
  loadMonthlyTotal,
} from './MonthlyBilling';

describe('MonthlyBilling', () => {
  let mockStorage: KeyValueStorage;
  let data: Record<string, string>;

  beforeEach(() => {
    data = {};
    mockStorage = {
      getItem: (k: string) => data[k] ?? null,
      setItem: (k: string, v: string) => { data[k] = v; },
      removeItem: (k: string) => { delete data[k]; },
    };
  });

  it('formats month key from Date', () => {
    expect(getMonthKey(new Date('1996-08-26T12:00:00Z'))).toBe('1996-08');
  });

  it('loads 0 when no data stored', () => {
    expect(loadMonthlyTotal('1996-08', mockStorage)).toBe(0);
  });

  it('accumulates charges for month', () => {
    expect(addMonthlyCharge('1996-08', 20, mockStorage)).toBe(20);
    expect(addMonthlyCharge('1996-08', 30, mockStorage)).toBe(50);
    expect(loadMonthlyTotal('1996-08', mockStorage)).toBe(50);
    expect(loadMonthlyTotal('1996-09', mockStorage)).toBe(0);
  });

  it('ignores zero or negative charge', () => {
    addMonthlyCharge('1996-08', 40, mockStorage);
    expect(addMonthlyCharge('1996-08', 0, mockStorage)).toBe(40);
    expect(addMonthlyCharge('1996-08', -10, mockStorage)).toBe(40);
  });
});
