import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TerminalCore } from '../terminal/TerminalCore';
import { VirtualModem } from './VirtualModem';

class FakeSocket {
  readyState = 0;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  sent: string[] = [];

  open() { this.readyState = 1; this.onopen?.({} as Event); }
  receive(message: unknown) {
    this.onmessage?.({ data: JSON.stringify(message) } as MessageEvent);
  }
  send(data: string) { this.sent.push(data); }
  close() { this.readyState = 3; }
}

function visibleText(terminal: TerminalCore) {
  return terminal.cells.map(row => row.map(cell => cell.ch).join('')).join('\n');
}

describe('VirtualModem standalone line simulation', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('dials and returns BUSY without creating any server socket', () => {
    const terminal = new TerminalCore();
    const dial = vi.fn(() => 0.10);
    const busy = vi.fn(() => 4);
    const socketFactory = vi.fn(() => new FakeSocket());
    const modem = new VirtualModem(terminal, '', {
      socketFactory,
      offlineBusyExtraMs: 50,
      audio: { dial, busy, handshake: vi.fn() },
    });
    modem.setAutoRedial(false);

    modem.submitLine('ATDT0451234567');
    expect(socketFactory).not.toHaveBeenCalled();
    expect(dial).toHaveBeenCalledWith('0451234567', 'tone');

    vi.advanceTimersByTime(149);
    expect(busy).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);

    expect(busy).toHaveBeenCalledTimes(1);
    expect(visibleText(terminal)).toContain('BUSY');
    modem.dispose();
  });

  it('remembers pulse dialling mode for ATDL', () => {
    const terminal = new TerminalCore();
    const dial = vi.fn(() => 0);
    const modem = new VirtualModem(terminal, '', {
      offlineBusyExtraMs: 10,
      audio: { dial, busy: vi.fn(), handshake: vi.fn() },
    });
    modem.setAutoRedial(false);

    modem.submitLine('ATDP0451234567');
    vi.advanceTimersByTime(120);
    modem.submitLine('ATDL');

    expect(dial).toHaveBeenNthCalledWith(1, '0451234567', 'pulse');
    expect(dial).toHaveBeenNthCalledWith(2, '0451234567', 'pulse');
    modem.dispose();
  });
});

describe('VirtualModem serial-rate terminal output', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  function connectedAt(baud: number) {
    const terminal = new TerminalCore();
    const socket = new FakeSocket();
    const modem = new VirtualModem(terminal, 'ws://test', {
      socketFactory: () => socket,
      dialDelayMs: 0,
      serialTickMs: 16,
      audio: { dial: () => 0, busy: vi.fn(), handshake: vi.fn() },
    });
    modem.setAutoRedial(false);
    modem.submitLine('ATDT0450000001');
    socket.open();
    vi.runOnlyPendingTimers();
    socket.receive({ type: 'dial_result', result: 'connect', baud });
    return { modem, socket, terminal };
  }

  it('renders 9600 bps text faster than 2400 bps text', () => {
    const slow = connectedAt(2400);
    const fast = connectedAt(9600);

    slow.socket.receive({ type: 'terminal', text: 'XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX' });
    fast.socket.receive({ type: 'terminal', text: 'YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY' });

    vi.advanceTimersByTime(16);

    const slowCount = visibleText(slow.terminal).split('X').length - 1;
    const fastCount = visibleText(fast.terminal).split('Y').length - 1;
    expect(slowCount).toBeGreaterThan(0);
    expect(fastCount).toBeGreaterThan(slowCount * 3);

    slow.modem.dispose();
    fast.modem.dispose();
  });

  it('charges Japanese characters as two serial bytes', () => {
    const { modem, socket, terminal } = connectedAt(2400);
    socket.receive({ type: 'terminal', text: '日本語日本語日本語日本語' });

    vi.advanceTimersByTime(16);
    const rendered = visibleText(terminal);
    const count = [...rendered].filter(ch => '日本語'.includes(ch)).length;

    // 2400 bps / 10 bits per byte / 62.5 ticks per second ≈ 3.84 bytes/tick,
    // so only one two-byte Japanese character fits in the first 16 ms tick.
    expect(count).toBe(1);
    modem.dispose();
  });
});
