import type { RegisteredCenter } from './CenterDirectory';
import type { TerminalCore } from '../terminal/TerminalCore';
import { terminalCellWidth } from '../terminal/TerminalCore';

const PAGE_SIZE = 15;

export class TerminalCenterDirectory {
  private open = false;
  private selected = 0;

  constructor(
    private readonly terminal: TerminalCore,
    private readonly centers: () => RegisteredCenter[],
    private readonly onDial: (center: RegisteredCenter) => void,
    private readonly onClose: () => void,
  ) {}

  isOpen() { return this.open; }

  show() {
    this.open = true;
    this.selected = Math.min(this.selected, Math.max(0, this.centers().length - 1));
    this.render();
  }

  close() {
    this.open = false;
    this.onClose();
  }

  handleKey(key: string) {
    if (!this.open) return false;
    const centers = this.centers();
    if (!centers.length) {
      if (key === 'Escape') this.close();
      return true;
    }

    switch (key) {
      case 'ArrowUp': this.selected = (this.selected - 1 + centers.length) % centers.length; this.render(); break;
      case 'ArrowDown': this.selected = (this.selected + 1) % centers.length; this.render(); break;
      case 'PageUp': this.selected = Math.max(0, this.selected - PAGE_SIZE); this.render(); break;
      case 'PageDown': this.selected = Math.min(centers.length - 1, this.selected + PAGE_SIZE); this.render(); break;
      case 'Home': this.selected = 0; this.render(); break;
      case 'End': this.selected = centers.length - 1; this.render(); break;
      case 's':
      case 'S': this.saveList(centers); break;
      case 'Enter': {
        const center = centers[this.selected];
        this.open = false;
        this.terminal.clear();
        this.terminal.write(`センター「${center.name}」を呼び出します。\r\n\r\n`);
        this.onDial(center);
        break;
      }
      case 'Escape': this.close(); break;
    }
    return true;
  }

  private saveList(centers: RegisteredCenter[]) {
    const exportedAt = new Date().toISOString();
    const payload = {
      format: 'zutto-center-directory',
      version: 1,
      exportedAt,
      count: centers.length,
      centers: centers.map(({ id, name, phone, dialMode, maxBaud }) => ({ id, name, phone, dialMode, maxBaud })),
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `zutto-centers-${exportedAt.slice(0, 10)}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    this.render('センター・リストをファイルに保存しました。');
  }

  private render(message = '') {
    const centers = this.centers();
    const page = Math.floor(this.selected / PAGE_SIZE);
    const pages = Math.max(1, Math.ceil(centers.length / PAGE_SIZE));
    const start = page * PAGE_SIZE;
    const rows = centers.slice(start, start + PAGE_SIZE);

    this.terminal.clear();
    this.terminal.write('\x1b[37;44m ずっとパソコン通信　センターの呼び出し                                      \x1b[0m\r\n');
    this.terminal.write('\r\n');
    this.terminal.write('              \x1b[30;46m　　　　セ　ン　タ　ー　・　リ　ス　ト　　　　\x1b[0m\r\n');
    this.terminal.write(`              Page ${String(page + 1).padStart(2, '0')} / ${String(pages).padStart(2, '0')}    登録 ${centers.length}局\r\n`);
    this.terminal.write('  No.  センター名                              電話番号       速度\r\n');
    this.terminal.write('  --------------------------------------------------------------------------\r\n');

    for (let i = 0; i < PAGE_SIZE; i++) {
      const center = rows[i];
      if (!center) { this.terminal.write('\r\n'); continue; }
      const absolute = start + i;
      const marker = absolute === this.selected ? '>' : ' ';
      const no = String(absolute + 1).padStart(3, '0');
      const name = fit(center.name, 38);
      const phone = formatPhone(center.phone).padEnd(14, ' ');
      const baud = `${center.maxBaud ?? 14400}`.padStart(5, ' ');
      const row = `${marker}${no}  ${name} ${phone} ${baud}`;
      this.terminal.write(absolute === this.selected ? `\x1b[30;46m${padCells(row, 78)}\x1b[0m\r\n` : `${row}\r\n`);
    }

    if (message) this.terminal.write(`\x1b[33m  ${fit(message, 76)}\x1b[0m\r\n`);
    else this.terminal.write('\r\n');
    this.terminal.write('\x1b[36m  ↑↓:選択  ROLL:頁移動  RETURN:呼出  S:リスト保存  ESC:メニュー\x1b[0m');
  }
}

function cellWidth(value: string) {
  return Array.from(value).reduce((width, ch) => width + terminalCellWidth(ch), 0);
}

function padCells(value: string, width: number) {
  return value + ' '.repeat(Math.max(0, width - cellWidth(value)));
}

function fit(value: string, width: number) {
  let used = 0;
  let out = '';
  for (const ch of Array.from(value)) {
    const w = terminalCellWidth(ch);
    if (used + w > width) break;
    out += ch;
    used += w;
  }
  return out + ' '.repeat(Math.max(0, width - used));
}

function formatPhone(phone: string) {
  const digits = phone.replace(/\D/g, '');
  if (digits.length === 10) return `${digits.slice(0, 3)}-${digits.slice(3, 6)}-${digits.slice(6)}`;
  return digits;
}
