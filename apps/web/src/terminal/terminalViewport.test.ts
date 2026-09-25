import { describe, expect, it } from 'vitest';
import { terminalCursorScrollTop, terminalViewportHeight } from './terminalViewport';

describe('mobile terminal viewport', () => {
  it('caps the terminal viewport above the command dock while keeping a usable minimum', () => {
    expect(terminalViewportHeight(400, 80, 330, 28)).toBe(218);
    expect(terminalViewportHeight(400, 300, 330, 28)).toBe(96);
    expect(terminalViewportHeight(400, 20, 700, 28)).toBe(400);
  });

  it('scrolls only enough to keep the live cursor row visible', () => {
    expect(terminalCursorScrollTop(0, 220, 368, 16)).toBe(180);
    expect(terminalCursorScrollTop(180, 220, 368, 16)).toBe(180);
    expect(terminalCursorScrollTop(180, 220, 32, 16)).toBe(16);
  });
});
