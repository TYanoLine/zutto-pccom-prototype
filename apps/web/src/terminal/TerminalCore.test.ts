import { describe, expect, it } from 'vitest';
import { TerminalCore } from './TerminalCore';

function rowText(row: ReturnType<TerminalCore['viewportRows']>[number]) {
  return row.filter(cell => !cell.continuation).map(cell => cell.ch).join('').trimEnd();
}

describe('TerminalCore scrollback', () => {
  it('retains lines that physically scroll off the 80x25 screen', () => {
    const terminal = new TerminalCore();
    for (let i = 0; i < 30; i++) terminal.write(`LINE-${String(i).padStart(2, '0')}\r\n`);

    expect(terminal.scrollbackLength).toBe(6);
    expect(rowText(terminal.viewportRows(0)[0])).toBe('LINE-06');
    expect(rowText(terminal.viewportRows(terminal.maxScrollOffset)[0])).toBe('LINE-00');
  });

  it('clamps viewport requests to the available history', () => {
    const terminal = new TerminalCore();
    for (let i = 0; i < 27; i++) terminal.write(`ROW-${i}\r\n`);

    expect(rowText(terminal.viewportRows(9999)[0])).toBe('ROW-0');
    expect(rowText(terminal.viewportRows(-99)[0])).toBe('ROW-3');
  });

  it('clears scrollback when the terminal screen is explicitly reset', () => {
    const terminal = new TerminalCore();
    for (let i = 0; i < 30; i++) terminal.write(`OLD-${i}\r\n`);
    expect(terminal.scrollbackLength).toBeGreaterThan(0);

    terminal.clear();

    expect(terminal.scrollbackLength).toBe(0);
    expect(rowText(terminal.viewportRows(0)[0])).toBe('');
  });
});
