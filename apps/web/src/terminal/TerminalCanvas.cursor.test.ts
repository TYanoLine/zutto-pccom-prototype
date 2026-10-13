import { describe, expect, it } from 'vitest';
import { PC98_CURSOR_BLINK_INTERVAL_MS, terminalCursorRect, terminalCursorWidth } from './TerminalCanvas';

describe('PC-98 cursor blink rate and geometry', () => {
  it('defines the historical PC-98 GDC cursor blink interval (32 frames @ 56.42Hz ~= 567ms)', () => {
    // NEC uPD7220 text GDC: 32 frames on, 32 frames off.
    // 32 / 56.422 Hz = 567.15 ms.
    expect(PC98_CURSOR_BLINK_INTERVAL_MS).toBe(567);

    // Full blink period is 2 * 567 ms = 1134 ms (~0.88 Hz blink frequency)
    const totalCycleMs = PC98_CURSOR_BLINK_INTERVAL_MS * 2;
    expect(totalCycleMs).toBe(1134);
    const blinkHz = 1000 / totalCycleMs;
    expect(blinkHz).toBeGreaterThan(0.85);
    expect(blinkHz).toBeLessThan(0.95);
  });

  it('calculates 8px cursor width for half-width cells', () => {
    expect(terminalCursorWidth({ ch: 'A' })).toBe(8);
    expect(terminalCursorWidth({ ch: '1' })).toBe(8);
    expect(terminalCursorWidth({ ch: ' ' })).toBe(8);
    expect(terminalCursorWidth({ ch: '#' })).toBe(8);
    expect(terminalCursorWidth(undefined)).toBe(8);
  });

  it('calculates 16px cursor width for full-width double-byte cells', () => {
    expect(terminalCursorWidth({ ch: '福' })).toBe(16);
    expect(terminalCursorWidth({ ch: 'あ' })).toBe(16);
    expect(terminalCursorWidth({ ch: '■' })).toBe(16);
    expect(terminalCursorWidth({ ch: '全' })).toBe(16);
  });

  it('draws the cursor as a full 100% cell block', () => {
    expect(terminalCursorRect(2, 3, { ch: 'A' })).toEqual({ x: 16, y: 48, width: 8, height: 16 });
    expect(terminalCursorRect(2, 3, { ch: '福' })).toEqual({ x: 16, y: 48, width: 16, height: 16 });
  });
});
