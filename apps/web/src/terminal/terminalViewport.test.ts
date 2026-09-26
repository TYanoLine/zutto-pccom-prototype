import { describe, expect, it } from 'vitest';
import { mobileTerminalRows, terminalCursorTargetScrollTop, terminalViewportHeight } from './terminalViewport';

describe('mobile terminal viewport', () => {
  it('uses the visible keyboard viewport without reserving a command dock', () => {
    expect(terminalViewportHeight(400, 80, 330)).toBe(246);
    expect(terminalViewportHeight(400, 300, 330)).toBe(96);
    expect(terminalViewportHeight(400, 20, 700)).toBe(400);
  });

  it('derives rows from 80-column width and available height without stretching cells', () => {
    expect(mobileTerminalRows(390, 760)).toBe(77);
    expect(mobileTerminalRows(390, 620)).toBe(63);
    expect(mobileTerminalRows(640, 400)).toBe(25);
    expect(mobileTerminalRows(320, 100)).toBe(25);
  });

  it('derives a stable absolute scroll target from the cursor row', () => {
    expect(terminalCursorTargetScrollTop(400, 220, 368, 16)).toBe(180);
    expect(terminalCursorTargetScrollTop(400, 220, 32, 16)).toBe(0);
    expect(terminalCursorTargetScrollTop(400, 220, 200, 16)).toBe(12);
    expect(terminalCursorTargetScrollTop(180, 220, 160, 16)).toBe(0);
  });
});
