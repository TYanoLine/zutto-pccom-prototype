import { describe, expect, it } from 'vitest';
import { isFullWidth, terminalCellWidth, TerminalCore } from './TerminalCore';

describe('Japanese terminal cell width', () => {
  it('treats JIS-era ambiguous symbols as double-cell in the Japanese terminal', () => {
    for (const ch of ['■', '□', '◆', '◇', '★', '☆', '→', '―', '①']) {
      expect(terminalCellWidth(ch), ch).toBe(2);
      expect(isFullWidth(ch), ch).toBe(true);
    }
  });

  it('keeps ideographic space at two cells and ASCII space at one', () => {
    expect(terminalCellWidth('　')).toBe(2);
    expect(terminalCellWidth(' ')).toBe(1);
    expect(terminalCellWidth('ｱ')).toBe(1);
  });

  it('does not let emoji/text variation selectors consume a terminal column', () => {
    expect(terminalCellWidth('\ufe0f')).toBe(0);
    const terminal = new TerminalCore();
    terminal.write('▫️A');
    expect(terminal.cursorX).toBe(3);
    expect(terminal.cells[0][0].ch).toBe('▫');
    expect(terminal.cells[0][1].continuation).toBe(true);
    expect(terminal.cells[0][2].ch).toBe('A');
  });

  it('stores U+3000 as a real two-cell blank', () => {
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
