import { describe, expect, it } from 'vitest';
import { isLocalPc98FontAvailable, terminalGlyphPaintStyle } from './TerminalCanvas';

describe('terminal fixed-cell glyph rendering', () => {
  it('uses an 8px natural-width font profile for half-width text without Canvas maxWidth squeezing', () => {
    const latin = terminalGlyphPaintStyle('A');
    expect(latin.fullWidth).toBe(false);
    expect(latin.glyphWidth).toBe(8);
    expect(latin.yOffset).toBe(1);
    expect(latin.font).toContain('13px');
    expect(latin.font).toContain('SFMono-Regular');
  });

  it('uses a separate 16px profile for Japanese full-width glyphs with PC-98 priority', () => {
    const japanese = terminalGlyphPaintStyle('福');
    expect(japanese.fullWidth).toBe(true);
    expect(japanese.glyphWidth).toBe(16);
    expect(japanese.yOffset).toBe(0);
    expect(japanese.font).toContain('16px');
    expect(japanese.font).toContain('TS PC98');
    expect(japanese.font).toContain('Hiragino');
  });

  it('uses a 16px 8x16 native bitmap profile when local PC-98 font is selected', () => {
    const latin = terminalGlyphPaintStyle('A', 'pc98');
    expect(latin.fullWidth).toBe(false);
    expect(latin.glyphWidth).toBe(8);
    expect(latin.yOffset).toBe(0);
    expect(latin.font).toContain('16px');
    expect(latin.font).toContain('TS PC98');
    expect(latin.font).toContain('天翔 PC98');

    const japanese = terminalGlyphPaintStyle('福', 'pc98');
    expect(japanese.fullWidth).toBe(true);
    expect(japanese.glyphWidth).toBe(16);
    expect(japanese.yOffset).toBe(0);
    expect(japanese.font).toContain('16px');
    expect(japanese.font).toContain('TS PC98');
    expect(japanese.font).toContain('天翔 PC98');
  });

  it('keeps ASCII box drawing in one half-width cell', () => {
    expect(terminalGlyphPaintStyle('#').glyphWidth).toBe(8);
    expect(terminalGlyphPaintStyle('-').glyphWidth).toBe(8);
    expect(terminalGlyphPaintStyle('#', 'pc98').glyphWidth).toBe(8);
  });

  it('safely checks local font availability without throwing in test environment', () => {
    expect(typeof isLocalPc98FontAvailable()).toBe('boolean');
  });
});
