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
  private listeners = new Set<() => void>();

  constructor() { this.clear(); }

  subscribe(fn: () => void) { this.listeners.add(fn); return () => { this.listeners.delete(fn); }; }
  private emit() { for (const fn of this.listeners) fn(); }

  get scrollbackLength() { return this.scrollback.length; }
  get maxScrollOffset() { return this.scrollback.length; }

  // offset=0 is the live terminal screen. Positive offsets expose lines that
  // physically scrolled off the top of the 80x25 screen. Keeping this in the
  // terminal core (rather than scraping rendered pixels) preserves ANSI colors
  // and full-width character metadata for historical display.
  viewportRows(offset = 0): Cell[][] {
    const clamped = Math.max(0, Math.min(this.maxScrollOffset, Math.trunc(offset)));
    if (clamped === 0) return this.cells;
    const history = [...this.scrollback, ...this.cells];
    const start = Math.max(0, history.length - HEIGHT - clamped);
    return history.slice(start, start + HEIGHT);
  }

  clear() {
    this.cells = Array.from({ length: HEIGHT }, () => blankRow());
    this.scrollback = [];
    this.cursorX = 0;
    this.cursorY = 0;
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
    if (ch === '\n') { this.newline(); return; }
    if (ch === '\b') { this.backspace(); return; }
    const w = isFullWidth(ch) ? 2 : 1;
    if (this.cursorX + w > WIDTH) this.newline();
    this.cells[this.cursorY][this.cursorX] = { ch, fg: this.fg, bg: this.bg, bold: this.bold };
    if (w === 2 && this.cursorX + 1 < WIDTH) {
      this.cells[this.cursorY][this.cursorX + 1] = { ch: '', fg: this.fg, bg: this.bg, bold: this.bold, continuation: true };
    }
    this.cursorX += w;
    if (this.cursorX >= WIDTH) this.newline();
  }

  private newline() {
    this.cursorX = 0;
    this.cursorY++;
    if (this.cursorY >= HEIGHT) {
      const scrolled = this.cells.shift();
      if (scrolled) {
        this.scrollback.push(cloneRow(scrolled));
        if (this.scrollback.length > MAX_SCROLLBACK) {
          this.scrollback.splice(0, this.scrollback.length - MAX_SCROLLBACK);
        }
      }
      this.cells.push(blankRow());
      this.cursorY = HEIGHT - 1;
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
      this.cursorY = Math.max(0, Math.min(HEIGHT - 1, (args[0] ?? 1) - 1));
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

export function isFullWidth(ch: string) {
  const cp = ch.codePointAt(0) ?? 0;
  return cp >= 0x1100 && (
    cp <= 0x115f || cp === 0x2329 || cp === 0x232a ||
    (cp >= 0x2e80 && cp <= 0xa4cf) || (cp >= 0xac00 && cp <= 0xd7a3) ||
    (cp >= 0xf900 && cp <= 0xfaff) || (cp >= 0xfe10 && cp <= 0xfe19) ||
    (cp >= 0xff01 && cp <= 0xff60) || (cp >= 0xffe0 && cp <= 0xffe6)
  );
}
