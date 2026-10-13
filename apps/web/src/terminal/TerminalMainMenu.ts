import type { TerminalCore } from './TerminalCore';
import { padTerminalCells } from './TerminalCore';

export type MainMenuItem = {
  key: string;
  label: string;
};

export const DEFAULT_MAIN_MENU_ITEMS: MainMenuItem[] = [
  { key: '1', label: '1　センターの呼び出し' },
  { key: '2', label: '2　通信パラメータの設定' },
  { key: '3', label: '3　ターミナル・モード' },
  { key: '4', label: '4　ノート・パッド' },
  { key: '5', label: '5　ディスク・ユーティリティ' },
  { key: '6', label: '6　MS-DOS コマンドへ' },
  { key: '7', label: '7　終　了' },
];

export class TerminalMainMenu {
  private selected = 0;

  constructor(
    private readonly terminal: TerminalCore,
    private readonly appVersion: string,
    private readonly getCenterCountText: () => string,
    private readonly onSelect: (item: MainMenuItem) => void,
    private readonly items: readonly MainMenuItem[] = DEFAULT_MAIN_MENU_ITEMS,
  ) {}

  getSelectedIndex(): number {
    return this.selected;
  }

  getSelectedItem(): MainMenuItem {
    return this.items[this.selected] ?? this.items[0];
  }

  setSelectedIndex(index: number) {
    if (this.items.length === 0) return;
    this.selected = Math.max(0, Math.min(this.items.length - 1, index));
  }

  setSelectedByKey(key: string) {
    const idx = this.items.findIndex(item => item.key.toLowerCase() === key.toLowerCase());
    if (idx !== -1) {
      this.selected = idx;
    }
  }

  selectNext() {
    if (this.items.length === 0) return;
    this.selected = (this.selected + 1) % this.items.length;
    this.render();
  }

  selectPrevious() {
    if (this.items.length === 0) return;
    this.selected = (this.selected - 1 + this.items.length) % this.items.length;
    this.render();
  }

  handleKey(key: string): boolean {
    switch (key) {
      case 'ArrowUp':
        this.selectPrevious();
        return true;
      case 'ArrowDown':
        this.selectNext();
        return true;
      case 'Enter':
        this.onSelect(this.getSelectedItem());
        return true;
      default:
        return false;
    }
  }

  render() {
    this.terminal.clear();
    this.terminal.write(
      `\x1b[37;44m ずっとパソコン通信 Ver ${this.appVersion}                         Copyright (C) 1996 ZUTTO \x1b[0m\r\n\r\n`
    );
    this.terminal.write('                     \x1b[30;46m　メイン・メニュー　\x1b[0m\r\n\r\n');

    for (let i = 0; i < this.items.length; i++) {
      const item = this.items[i];
      if (i === this.selected) {
        const inner = `  ${item.label}`;
        const padded = padTerminalCells(inner, 32);
        this.terminal.write(`                   \x1b[30;46m${padded}\x1b[0m\r\n`);
      } else {
        this.terminal.write(`                     ${item.label}\r\n`);
      }
    }

    this.terminal.write('\r\n');
    const countText = this.getCenterCountText();
    this.terminal.write(
      ` 登録センター: ${countText}　　　　　　　　　使用する電話回線: ダイアル(10)\r\n 番号を選択してください (↑↓:選択 RETURN:決定) > `
    );
  }
}
