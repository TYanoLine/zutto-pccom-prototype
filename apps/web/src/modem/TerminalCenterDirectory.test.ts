import { describe, expect, it } from 'vitest';
import { TerminalCenterDirectory } from './TerminalCenterDirectory';
import type { RegisteredCenter } from './CenterDirectory';
import type { TerminalCore } from '../terminal/TerminalCore';

function center(n: number): RegisteredCenter {
  const phone = `0${String(n).padStart(9, '0')}`;
  return { id: `c${n}`, name: `CENTER ${n}`, phone, dialMode: 'tone', maxBaud: 14400 };
}

// A terminal that keeps only what the last redraw wrote.
function setup(initial: RegisteredCenter[]) {
  let list = initial;
  const written: string[] = [];
  const terminal = {
    clear: () => { written.length = 0; },
    write: (text: string) => { written.push(text); },
  } as unknown as TerminalCore;
  const directory = new TerminalCenterDirectory(terminal, () => list, () => {}, () => {});
  return {
    directory,
    screen: () => written.join(''),
    setList: (next: RegisteredCenter[]) => { list = next; },
  };
}

describe('TerminalCenterDirectory', () => {
  it('shows the number of stations it has been given', () => {
    const { directory, screen } = setup([center(1)]);
    directory.show();
    expect(screen()).toContain('登録 1局');
  });

  it('redraws an open list when more stations arrive, keeping the selection', () => {
    const { directory, screen, setList } = setup([center(1), center(2)]);
    directory.show();
    directory.handleKey('ArrowDown');
    expect(screen()).toContain('>002');

    setList([center(1), center(2), center(3), center(4)]);
    directory.refresh();

    expect(screen()).toContain('登録 4局');
    expect(screen()).toContain('>002');
    expect(screen()).toContain('CENTER 4');
  });

  it('does nothing when the list is not open', () => {
    const { directory, screen } = setup([center(1)]);
    directory.refresh();
    expect(screen()).toBe('');

    directory.show();
    directory.handleKey('Escape');
    const closedScreen = screen();
    directory.refresh();
    expect(screen()).toBe(closedScreen);
  });

  it('shows a standing note under the list until it is cleared', () => {
    const { directory, screen } = setup([center(1)]);
    directory.setNotice('ほかのセンターを読み込み中...');
    directory.show();
    expect(screen()).toContain('ほかのセンターを読み込み中...');

    directory.setNotice('');
    directory.refresh();
    expect(screen()).not.toContain('ほかのセンターを読み込み中...');
  });

  it('does not redraw by itself when the note changes', () => {
    const { directory, screen } = setup([center(1)]);
    directory.show();
    directory.setNotice('ほかのセンターを読み込み中...');
    expect(screen()).not.toContain('ほかのセンターを読み込み中...');
  });
});
