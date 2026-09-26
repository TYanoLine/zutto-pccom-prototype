import { describe, expect, it } from 'vitest';
import { terminalGlyphPaintStyle } from './TerminalCanvas';

describe('terminal fixed-cell glyph rendering', () => {
  it('uses an 8px natural-width font profile for half-width text without Canvas maxWidth squeezing', () => {
    const latin = terminalGlyphPaintStyle('A');
    expect(latin.fullWidth).toBe(false);
    expect(latin.glyphWidth).toBe(8);
    expect(latin.yOffset).toBe(1);
    expect(latin.font).toContain('13px');
    expect(latin.font).toContain('SFMono-Regular');
  });

  it('uses a separate 16px profile for Japanese full-width glyphs', () => {
    const japanese = terminalGlyphPaintStyle('福');
    expect(japanese.fullWidth).toBe(true);
    expect(japanese.glyphWidth).toBe(16);
    expect(japanese.yOffset).toBe(0);
    expect(japanese.font).toContain('16px');
    expect(japanese.font).toContain('Hiragino');
  });

  it('keeps ASCII box drawing in one half-width cell', () => {
    expect(terminalGlyphPaintStyle('#').glyphWidth).toBe(8);
    expect(terminalGlyphPaintStyle('-').glyphWidth).toBe(8);
  });
});
