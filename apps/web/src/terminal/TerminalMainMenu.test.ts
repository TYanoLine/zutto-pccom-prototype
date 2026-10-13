import { describe, expect, it, vi } from 'vitest';
import { TerminalMainMenu, DEFAULT_MAIN_MENU_ITEMS } from './TerminalMainMenu';
import type { TerminalCore } from './TerminalCore';

function setup() {
  const written: string[] = [];
  const terminal = {
    clear: () => { written.length = 0; },
    write: (text: string) => { written.push(text); },
  } as unknown as TerminalCore;

  const onSelect = vi.fn();
  const menu = new TerminalMainMenu(terminal, '0.28', () => '10局', onSelect);
  return {
    menu,
    screen: () => written.join(''),
    onSelect,
  };
}

describe('TerminalMainMenu', () => {
  it('highlights the first item by default on render', () => {
    const { menu, screen } = setup();
    menu.render();

    const output = screen();
    expect(output).toContain('メイン・メニュー');
    // First item is highlighted with cyan background (\x1b[30;46m)
    expect(output).toContain('\x1b[30;46m  1　センターの呼び出し');
    // Second item is not highlighted
    expect(output).toContain('                     2　通信パラメータの設定');
    expect(output).toContain('登録センター: 10局');
    expect(output).toContain('番号を選択してください (↑↓:選択 RETURN:決定) > ');
    expect(menu.getSelectedIndex()).toBe(0);
    expect(menu.getSelectedItem().key).toBe('1');
  });

  it('moves selection down on ArrowDown and updates highlight', () => {
    const { menu, screen } = setup();
    menu.render();

    const handled = menu.handleKey('ArrowDown');
    expect(handled).toBe(true);
    expect(menu.getSelectedIndex()).toBe(1);
    expect(menu.getSelectedItem().key).toBe('2');

    const output = screen();
    expect(output).toContain('                     1　センターの呼び出し');
    expect(output).toContain('\x1b[30;46m  2　通信パラメータの設定');
  });

  it('wraps selection to the last item on ArrowUp from the first item', () => {
    const { menu, screen } = setup();
    menu.render();

    const handled = menu.handleKey('ArrowUp');
    expect(handled).toBe(true);
    expect(menu.getSelectedIndex()).toBe(DEFAULT_MAIN_MENU_ITEMS.length - 1);
    expect(menu.getSelectedItem().key).toBe('7');

    const output = screen();
    expect(output).toContain('                     1　センターの呼び出し');
    expect(output).toContain('\x1b[30;46m  7　終　了');
  });

  it('triggers onSelect when Enter is pressed', () => {
    const { menu, onSelect } = setup();
    menu.render();
    menu.handleKey('ArrowDown'); // item 2
    menu.handleKey('ArrowDown'); // item 3

    expect(menu.getSelectedItem().key).toBe('3');
    const handled = menu.handleKey('Enter');
    expect(handled).toBe(true);
    expect(onSelect).toHaveBeenCalledWith(DEFAULT_MAIN_MENU_ITEMS[2]);
  });

  it('updates selection by key using setSelectedByKey', () => {
    const { menu, screen } = setup();
    menu.setSelectedByKey('3');
    expect(menu.getSelectedIndex()).toBe(2);

    menu.render();
    expect(screen()).toContain('\x1b[30;46m  3　ターミナル・モード');
  });

  it('ignores unknown keys and returns false', () => {
    const { menu } = setup();
    expect(menu.handleKey('a')).toBe(false);
    expect(menu.handleKey('Escape')).toBe(false);
  });
});
