export type Cell = {
  ch: string;
  fg: number;
  bg: number;
  bold: boolean;
  continuation?: boolean;
};

const WIDTH = 80;
const HEIGHT = 25;
const MAX_SCROLLBACK = 2000;

// Unicode's East Asian Width=Ambiguous characters are commonly rendered as
// double-cell glyphs in Japanese terminals. The service intentionally follows
// that Japanese legacy convention for symbols that occur in JIS-era BBS text.
// This includes the exact families seen in period-style host screens: ruled
// lines, geometric blocks, arrows, stars, circled numbers and math symbols.
const JAPANESE_DOUBLE_CELL_SYMBOL_RANGES: ReadonlyArray<readonly [number, number]> = [
  [0x2010, 0x203b], // dashes, quotes, ellipsis, ※
  [0x2103, 0x2103], // ℃
  [0x212b, 0x212b], // Ångström sign
  [0x2190, 0x2193], // arrows
  [0x21d2, 0x21d2],
  [0x21d4, 0x21d4],
  [0x2200, 0x22bf], // common mathematical symbols
  [0x2312, 0x2312],
  [0x2460, 0x24ff], // enclosed/circled numbers and letters
  [0x2500, 0x257f], // box drawing
  [0x25a0, 0x25ff], // ■ □ ◆ ◇ ○ etc.
  [0x2605, 0x2606], // ★ ☆
  [0x2640, 0x2642],
  [0x266a, 0x266f], // music / accidental marks
  [0x0391, 0x03c9], // Greek used by JIS X 0208
  [0x0401, 0x0451], // Cyrillic used by JIS X 0208
];

function inRanges(cp: number, ranges: ReadonlyArray<readonly [number, number]>) {
  return ranges.some(([start, end]) => cp >= start && cp <= end);
}

function isVariationSelector(cp: number) {
  return (cp >= 0xfe00 && cp <= 0xfe0f) || (cp >= 0xe0100 && cp <= 0xe01ef);
}

function isCombiningMark(cp: number) {
  return (cp >= 0x0300 && cp <= 0x036f)
    || (cp >= 0x1ab0 && cp <= 0x1aff)
    || (cp >= 0x1dc0 && cp <= 0x1dff)
    || (cp >= 0x20d0 && cp <= 0x20ff)
    || (cp >= 0xfe20 && cp <= 0xfe2f)
    || cp === 0x3099
    || cp === 0x309a;
}

function blankRow(): Cell[] {
  return Array.from({ length: WIDTH }, () => ({ ch: ' ', fg: 7, bg: 0, bold: false }));
}

function cloneRow(row: Cell[]): Cell[] {
  return row.map(cell => ({ ...cell }));
}

export class TerminalCore {
  readonly width = WIDTH;
  readonly height = HEIGHT;
  cells: Cell[][] = [];
  cursorX = 0;
  cursorY = 0;
  private scrollback: Cell[][] = [];
  private fg = 7;
  private bg = 0;
  private bold = false;
  private autoWrapped = false;
  private listeners = new Set<() => void>();

  constructor() { this.clear(); }

  subscribe(fn: () => void) { this.listeners.add(fn); return () => { this.listeners.delete(fn); }; }
  private emit() { for (const fn of this.listeners) fn(); }

  get scrollbackLength() { return this.scrollback.length; }
  get maxScrollOffset() { return this.maxScrollOffsetForRows(this.height); }

  maxScrollOffsetForRows(rowCount = this.height) {
    const rows = Math.max(1, Math.min(120, Math.trunc(rowCount)));
    return Math.max(0, this.scrollback.length + this.cells.length - rows);
  }

  // The emulated terminal itself stays a historical 80x25 screen. Presentation
  // layers may request a taller read-only viewport; those extra rows come from
  // existing scrollback instead of mutating the live terminal buffer height.
  viewportRows(offset = 0, rowCount = this.height): Cell[][] {
    const rows = Math.max(1, Math.min(120, Math.trunc(rowCount)));
    const history = [...this.scrollback, ...this.cells];
    const maxOffset = this.maxScrollOffsetForRows(rows);
    const clamped = Math.max(0, Math.min(maxOffset, Math.trunc(offset)));
    const end = Math.max(0, history.length - clamped);
    const start = Math.max(0, end - rows);
    const visible = history.slice(start, end);
    while (visible.length < rows) visible.push(blankRow());
    return visible;
  }

  viewportCursorY(rowCount = this.height) {
    const rows = Math.max(1, Math.min(120, Math.trunc(rowCount)));
    const historyLength = this.scrollback.length + this.cells.length;
    const start = Math.max(0, historyLength - rows);
    const absoluteCursorY = this.scrollback.length + this.cursorY;
    return Math.max(0, Math.min(rows - 1, absoluteCursorY - start));
  }

  clear() {
    this.cells = Array.from({ length: this.height }, () => blankRow());
    this.scrollback = [];
    this.cursorX = 0;
    this.cursorY = 0;
    this.autoWrapped = false;
    this.emit();
  }

  write(text: string) {
    for (let i = 0; i < text.length; ) {
      if (text[i] === '\x1b' && text[i + 1] === '[') {
        const end = this.findAnsiEnd(text, i + 2);
        if (end >= 0) {
          this.handleAnsi(text.slice(i + 2, end + 1));
          i = end + 1;
          continue;
        }
      }
      const cp = text.codePointAt(i)!;
      const ch = String.fromCodePoint(cp);
      i += ch.length;
      this.put(ch);
    }
    this.emit();
  }

  backspace() {
    if (this.cursorX > 0) {
      this.cursorX--;
      if (this.cells[this.cursorY][this.cursorX].continuation && this.cursorX > 0) this.cursorX--;
      this.cells[this.cursorY][this.cursorX] = { ch: ' ', fg: this.fg, bg: this.bg, bold: this.bold };
      if (this.cursorX + 1 < WIDTH && this.cells[this.cursorY][this.cursorX + 1].continuation) {
        this.cells[this.cursorY][this.cursorX + 1] = { ch: ' ', fg: this.fg, bg: this.bg, bold: this.bold };
      }
      this.emit();
    }
  }

  private put(ch: string) {
    if (ch === '\r') { this.cursorX = 0; return; }
    if (ch === '\n') {
      // Filling column 80 already performed the visual wrap. A following CR/LF
      // belongs to that same physical line and must not create a blank row.
      if (this.autoWrapped) { this.autoWrapped = false; return; }
      this.newline();
      return;
    }
    if (ch === '\b') { this.autoWrapped = false; this.backspace(); return; }

    const cp = ch.codePointAt(0) ?? 0;
    const w = terminalCellWidth(ch);
    if (w === 0) {
      // Presentation selectors (notably VS16 in "▫️") must not consume a
      // terminal column. Ignore them so period text symbols stay text glyphs.
      if (isVariationSelector(cp) || cp === 0x200d) return;

      // Preserve decomposed accents/dakuten on the previous leading cell while
      // still treating the mark itself as zero columns.
      if (isCombiningMark(cp)) {
        let x = this.cursorX - 1;
        if (x >= 0 && this.cells[this.cursorY][x]?.continuation) x--;
        if (x >= 0) this.cells[this.cursorY][x].ch += ch;
      }
      return;
    }

    this.autoWrapped = false;
    if (this.cursorX + w > WIDTH) this.newline();
    this.cells[this.cursorY][this.cursorX] = { ch, fg: this.fg, bg: this.bg, bold: this.bold };
    if (w === 2 && this.cursorX + 1 < WIDTH) {
      this.cells[this.cursorY][this.cursorX + 1] = { ch: '', fg: this.fg, bg: this.bg, bold: this.bold, continuation: true };
    }
    this.cursorX += w;
    if (this.cursorX >= WIDTH) {
      this.newline();
      this.autoWrapped = true;
    }
  }

  private newline() {
    this.cursorX = 0;
    this.cursorY++;
    if (this.cursorY >= this.height) {
      const scrolled = this.cells.shift();
      if (scrolled) {
        this.scrollback.push(cloneRow(scrolled));
        if (this.scrollback.length > MAX_SCROLLBACK) {
          this.scrollback.splice(0, this.scrollback.length - MAX_SCROLLBACK);
        }
      }
      this.cells.push(blankRow());
      this.cursorY = this.height - 1;
    }
  }

  private findAnsiEnd(text: string, start: number) {
    for (let i = start; i < text.length; i++) {
      const c = text.charCodeAt(i);
      if (c >= 0x40 && c <= 0x7e) return i;
    }
    return -1;
  }

  private handleAnsi(seq: string) {
    const final = seq.at(-1)!;
    const args = seq.slice(0, -1).split(';').filter(Boolean).map(Number);
    if (final === 'J' && (args[0] ?? 0) === 2) { this.clear(); return; }
    if (final === 'H' || final === 'f') {
      this.cursorY = Math.max(0, Math.min(this.height - 1, (args[0] ?? 1) - 1));
      this.cursorX = Math.max(0, Math.min(WIDTH - 1, (args[1] ?? 1) - 1));
      return;
    }
    if (final === 'm') {
      const codes = args.length ? args : [0];
      for (const code of codes) {
        if (code === 0) { this.fg = 7; this.bg = 0; this.bold = false; }
        else if (code === 1) this.bold = true;
        else if (code >= 30 && code <= 37) this.fg = code - 30;
        else if (code >= 40 && code <= 47) this.bg = code - 40;
      }
    }
  }
}

export function terminalCellWidth(ch: string): 0 | 1 | 2 {
  const cp = ch.codePointAt(0) ?? 0;

  if (isVariationSelector(cp) || cp === 0x200d || isCombiningMark(cp)) return 0;

  // Explicitly retain the Japanese ideographic space as two cells. This is
  // redundant with the broad CJK range below but documents a key invariant.
  if (cp === 0x3000) return 2;

  if (cp >= 0x1100 && (
    cp <= 0x115f || cp === 0x2329 || cp === 0x232a ||
    (cp >= 0x2e80 && cp <= 0xa4cf) || (cp >= 0xac00 && cp <= 0xd7a3) ||
    (cp >= 0xf900 && cp <= 0xfaff) || (cp >= 0xfe10 && cp <= 0xfe19) ||
    (cp >= 0xff01 && cp <= 0xff60) || (cp >= 0xffe0 && cp <= 0xffe6)
  )) return 2;

  if (inRanges(cp, JAPANESE_DOUBLE_CELL_SYMBOL_RANGES)) return 2;

  return 1;
}

export function isFullWidth(ch: string) {
  return terminalCellWidth(ch) === 2;
}
