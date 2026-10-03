import { describe, expect, it } from 'vitest';
import { isFullWidth, terminalCellWidth, TerminalCore } from './TerminalCore';

describe('PC-9801 / Shift_JIS terminal cell width', () => {
  it('uses one cell for ASCII and JIS X 0201 half-width kana', () => {
    for (const ch of ['A', '#', ' ', 'ｱ', '｡', 'ﾞ']) {
      expect(terminalCellWidth(ch), ch).toBe(1);
      expect(isFullWidth(ch), ch).toBe(false);
    }
  });

  it('uses two cells for PC-98 full-width Japanese and symbols', () => {
    for (const ch of ['福', '　', '■', '□', '＊', '※', '○', '◎', '◇', '◆', '★', '☆', '→', '―']) {
      expect(terminalCellWidth(ch), ch).toBe(2);
      expect(isFullWidth(ch), ch).toBe(true);
    }
  });

  it('does not let Unicode presentation controls consume a terminal column', () => {
    expect(terminalCellWidth('\ufe0f')).toBe(0);
    const terminal = new TerminalCore();
    terminal.write('□A');
    expect(terminal.cursorX).toBe(3);
    expect(terminal.cells[0][0].ch).toBe('□');
    expect(terminal.cells[0][1].continuation).toBe(true);
    expect(terminal.cells[0][2].ch).toBe('A');
  });

  it('stores U+3000 as a real two-cell PC-98 blank', () => {
    const terminal = new TerminalCore();
    terminal.write('　A');
    expect(terminal.cursorX).toBe(3);
    expect(terminal.cells[0][0].ch).toBe('　');
    expect(terminal.cells[0][1].continuation).toBe(true);
    expect(terminal.cells[0][2].ch).toBe('A');
  });

  it('does not insert a blank row when an exact 80-cell line is followed by CRLF', () => {
    const terminal = new TerminalCore();
    terminal.write('■'.repeat(40) + '\r\nX');
    expect(terminal.cells[0][0].ch).toBe('■');
    expect(terminal.cells[0][1].continuation).toBe(true);
    expect(terminal.cells[1][0].ch).toBe('X');
    expect(terminal.cursorY).toBe(1);
    expect(terminal.cursorX).toBe(1);
  });
});
