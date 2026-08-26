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
  closed = false;

  open() {
    this.readyState = 1;
    this.onopen?.({} as Event);
  }

  receive(message: unknown) {
    this.onmessage?.({ data: JSON.stringify(message) } as MessageEvent);
  }

  disconnect() {
    this.readyState = 3;
    this.onclose?.({} as CloseEvent);
  }

  send(data: string) { this.sent.push(data); }
  close() { this.closed = true; this.readyState = 3; }
}

const silentAudio = { dial: vi.fn(), busy: vi.fn(), handshake: vi.fn() };

describe('VirtualModem lifecycle', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('cleans up pending auto-redial when disposed', () => {
    const socket = new FakeSocket();
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      audio: silentAudio,
      dialDelayMs: 10,
    });
    socket.open();
    modem.submitLine('ATDT0459999999');
    vi.advanceTimersByTime(10);
    socket.receive({ type: 'dial_result', result: 'busy' });

    modem.dispose();
    vi.runAllTimers();

    expect(socket.sent).toHaveLength(1);
    expect(socket.closed).toBe(true);
  });

  it('reconnects after the server socket closes', () => {
    const sockets: FakeSocket[] = [];
    const statuses: string[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      reconnectDelaysMs: [100],
      audio: silentAudio,
    });
    modem.onStatus = status => statuses.push(status);
    sockets[0].open();
    sockets[0].disconnect();

    vi.advanceTimersByTime(100);
    expect(sockets).toHaveLength(2);
    sockets[1].open();

    expect(statuses).toEqual([
      'MODEM READY',
      'SERVER OFFLINE / RECONNECTING',
      'MODEM READY',
    ]);
    modem.dispose();
  });

  it('drops carrier exactly once when a connected socket closes', () => {
    const socket = new FakeSocket();
    const calls: unknown[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      audio: silentAudio,
      dialDelayMs: 0,
    });
    modem.onCallState = call => calls.push(call);
    socket.open();
    modem.submitLine('ATDT0450000001');
    vi.runOnlyPendingTimers();
    socket.receive({
      type: 'dial_result',
      result: 'connect',
      baud: 28800,
      host: { name: 'TEST', phone: '0450000001' },
    });
    socket.disconnect();

    expect(calls).toEqual([{ phone: '0450000001', baud: 28800 }, null]);
    modem.dispose();
  });

  it('continues dialing when Web Audio is unavailable', () => {
    const socket = new FakeSocket();
    const calls: unknown[] = [];
    const unavailableAudio = {
      dial: () => { throw new Error('AudioContext unavailable'); },
      busy: () => { throw new Error('AudioContext unavailable'); },
      handshake: () => { throw new Error('AudioContext unavailable'); },
    };
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      audio: unavailableAudio,
      dialDelayMs: 10,
    });
    modem.onCallState = call => calls.push(call);
    socket.open();
    modem.submitLine('ATDT0450000001');
    vi.advanceTimersByTime(10);

    expect(socket.sent).toEqual([
      JSON.stringify({ type: 'dial', phone: '0450000001', attempt: 1 }),
    ]);

    socket.receive({ type: 'dial_result', result: 'connect', baud: 28800 });
    expect(calls).toEqual([{ phone: '0450000001', baud: 28800 }]);
    modem.dispose();
  });
});
