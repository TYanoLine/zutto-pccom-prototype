import { describe, expect, it, vi } from 'vitest';
import {
  isDotGothicAvailable,
  isLocalPc98FontAvailable,
  isRetroDotFontAvailable,
  measureRetroFontYOffset,
  terminalGlyphPaintStyle,
} from './TerminalCanvas';

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
    expect(japanese.font).toContain('DotGothic16');
    expect(japanese.font).toContain('Hiragino');
  });

  it('applies calibrated retroYOffset to Japanese glyphs and PC-98 mode', () => {
    const shiftedJapanese = terminalGlyphPaintStyle('福', 'default', -5);
    expect(shiftedJapanese.yOffset).toBe(-5);

    const defaultLatin = terminalGlyphPaintStyle('A', 'default', -5);
    expect(defaultLatin.yOffset).toBe(1);

    const pc98Japanese = terminalGlyphPaintStyle('福', 'pc98', -5);
    expect(pc98Japanese.yOffset).toBe(-5);

    const pc98Latin = terminalGlyphPaintStyle('A', 'pc98', -5);
    expect(pc98Latin.yOffset).toBe(-5);
  });

  it('uses a 16px 8x16 native bitmap profile when local PC-98 font is selected', () => {
    const latin = terminalGlyphPaintStyle('A', 'pc98');
    expect(latin.fullWidth).toBe(false);
    expect(latin.glyphWidth).toBe(8);
    expect(latin.yOffset).toBe(0);
    expect(latin.font).toContain('16px');
    expect(latin.font).toContain('TS PC98');
    expect(latin.font).toContain('天翔 PC98');
    expect(latin.font).toContain('DotGothic16');

    const japanese = terminalGlyphPaintStyle('福', 'pc98');
    expect(japanese.fullWidth).toBe(true);
    expect(japanese.glyphWidth).toBe(16);
    expect(japanese.yOffset).toBe(0);
    expect(japanese.font).toContain('16px');
    expect(japanese.font).toContain('TS PC98');
    expect(japanese.font).toContain('天翔 PC98');
    expect(japanese.font).toContain('DotGothic16');
  });

  it('keeps ASCII box drawing in one half-width cell', () => {
    expect(terminalGlyphPaintStyle('#').glyphWidth).toBe(8);
    expect(terminalGlyphPaintStyle('-').glyphWidth).toBe(8);
    expect(terminalGlyphPaintStyle('#', 'pc98').glyphWidth).toBe(8);
  });

  it('safely checks font availability without throwing in test environment', () => {
    expect(typeof isLocalPc98FontAvailable()).toBe('boolean');
    expect(typeof isDotGothicAvailable()).toBe('boolean');
    expect(typeof isRetroDotFontAvailable()).toBe('boolean');
    expect(typeof measureRetroFontYOffset()).toBe('number');
  });

  it('calibrates negative vertical offset when canvas text starts below row 1', () => {
    const dummyData = new Uint8ClampedArray(32 * 32 * 4);
    // Simulate WebKit + DotGothic16 where ink starts at y = 5
    dummyData[(5 * 32 + 8) * 4 + 3] = 255;
    const mockCtx = {
      font: '',
      textBaseline: '',
      textAlign: '',
      fillStyle: '',
      fillText: vi.fn(),
      getImageData: vi.fn().mockReturnValue({ data: dummyData }),
    };
    const mockCanvas = {
      width: 32,
      height: 32,
      getContext: (type: string) => (type === '2d' ? mockCtx : null),
    } as unknown as HTMLCanvasElement;

    const originalDoc = (globalThis as unknown as { document?: unknown }).document;
    (globalThis as unknown as { document?: unknown }).document = {
      createElement: (tagName: string) => (tagName === 'canvas' ? mockCanvas : {}),
    };

    try {
      const offset = measureRetroFontYOffset();
      expect(offset).toBe(-5);
    } finally {
      if (originalDoc === undefined) {
        delete (globalThis as unknown as { document?: unknown }).document;
      } else {
        (globalThis as unknown as { document?: unknown }).document = originalDoc;
      }
    }
  });

  it('returns zero offset when canvas text starts at row 0 or 1', () => {
    const dummyData = new Uint8ClampedArray(32 * 32 * 4);
    // Simulate normal font where ink starts at y = 0
    dummyData[(0 * 32 + 8) * 4 + 3] = 255;
    const mockCtx = {
      font: '',
      textBaseline: '',
      textAlign: '',
      fillStyle: '',
      fillText: vi.fn(),
      getImageData: vi.fn().mockReturnValue({ data: dummyData }),
    };
    const mockCanvas = {
      width: 32,
      height: 32,
      getContext: (type: string) => (type === '2d' ? mockCtx : null),
    } as unknown as HTMLCanvasElement;

    const originalDoc = (globalThis as unknown as { document?: unknown }).document;
    (globalThis as unknown as { document?: unknown }).document = {
      createElement: (tagName: string) => (tagName === 'canvas' ? mockCanvas : {}),
    };

    try {
      const offset = measureRetroFontYOffset();
      expect(offset).toBe(0);
    } finally {
      if (originalDoc === undefined) {
        delete (globalThis as unknown as { document?: unknown }).document;
      } else {
        (globalThis as unknown as { document?: unknown }).document = originalDoc;
      }
    }
  });
});
