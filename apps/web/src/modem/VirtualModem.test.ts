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

describe('VirtualModem standalone lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });
  afterEach(() => vi.useRealTimers());

  it('does not create a server socket until a dial command is issued', () => {
    const sockets: FakeSocket[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
    });

    expect(sockets).toHaveLength(0);
    modem.submitLine('AT');
    modem.submitLine('ATI');
    expect(sockets).toHaveLength(0);

    modem.submitLine('ATDT0450000001');
    expect(sockets).toHaveLength(1);
    modem.dispose();
  });

  it('waits for the lazy socket to open before sending the dial request', () => {
    const socket = new FakeSocket();
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      audio: silentAudio,
      dialDelayMs: 10,
    });

    modem.submitLine('ATDT0450000001');
    vi.advanceTimersByTime(100);
    expect(socket.sent).toEqual([]);

    socket.open();
    vi.advanceTimersByTime(10);
    expect(socket.sent).toEqual([
      JSON.stringify({ type: 'dial', phone: '0450000001', attempt: 1 }),
    ]);
    modem.dispose();
  });

  it('shows NO CARRIER when the server rejects a dial after line setup', () => {
    const socket = new FakeSocket();
    const terminal = new TerminalCore();
    const statuses: string[] = [];
    const modem = new VirtualModem(terminal, 'ws://test', {
      socketFactory: () => socket,
      audio: silentAudio,
      dialDelayMs: 0,
    });
    modem.onStatus = status => statuses.push(status);

    modem.submitLine('ATDT0920000196');
    socket.open();
    vi.runOnlyPendingTimers();
    socket.receive({ type: 'dial_result', result: 'no_carrier' });

    const screen = terminal.viewportRows().map(row => row.map(cell => cell.ch).join('')).join('\n');
    expect(screen).toContain('NO CARRIER');
    expect(statuses.at(-1)).toBe('NO CARRIER');
    expect(socket.closed).toBe(true);
    modem.dispose();
  });

  it('supports ATDL as dial-last-number and opens a fresh call transport', () => {
    const sockets: FakeSocket[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
      dialDelayMs: 0,
    });

    modem.submitLine('ATDT0451234567');
    sockets[0].open();
    vi.runOnlyPendingTimers();
    sockets[0].receive({ type: 'dial_result', result: 'no_answer' });

    modem.submitLine('ATDL');
    expect(sockets).toHaveLength(2);
    sockets[1].open();
    vi.runOnlyPendingTimers();

    expect(sockets[1].sent).toEqual([
      JSON.stringify({ type: 'dial', phone: '0451234567', attempt: 1 }),
    ]);
    modem.dispose();
  });

  it('cleans up pending auto-redial when disposed', () => {
    const sockets: FakeSocket[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
      dialDelayMs: 10,
    });

    modem.submitLine('ATDT0459999999');
    sockets[0].open();
    vi.advanceTimersByTime(10);
    sockets[0].receive({ type: 'dial_result', result: 'busy' });

    modem.dispose();
    vi.runAllTimers();

    expect(sockets[0].sent).toHaveLength(1);
    expect(sockets).toHaveLength(1);
    expect(sockets[0].closed).toBe(true);
  });

  it('resumes a logical call after the websocket transport is interrupted', () => {
    const sockets: FakeSocket[] = [];
    const calls: unknown[] = [];
    const statuses: string[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
      dialDelayMs: 0,
      reconnectDelayMs: 10,
    });
    modem.onCallState = call => calls.push(call);
    modem.onStatus = status => statuses.push(status);

    modem.submitLine('ATDT0450000001');
    sockets[0].open();
    vi.runOnlyPendingTimers();
    sockets[0].receive({
      type: 'dial_result',
      result: 'connect',
      baud: 28800,
      session_id: 'session-123',
      host: { name: 'TEST', phone: '0450000001' },
    });

    sockets[0].disconnect();
    expect(calls).toEqual([{ phone: '0450000001', baud: 28800 }]);
    expect(statuses.at(-1)).toBe('LINE INTERRUPTED / RECONNECTING');

    vi.advanceTimersByTime(10);
    expect(sockets).toHaveLength(2);
    sockets[1].open();
    expect(sockets[1].sent).toEqual([
      JSON.stringify({ type: 'resume', session_id: 'session-123' }),
    ]);

    sockets[1].receive({
      type: 'resume_result',
      result: 'ok',
      baud: 28800,
      session_id: 'session-123',
      host: { name: 'TEST', phone: '0450000001' },
    });
    expect(calls).toEqual([{ phone: '0450000001', baud: 28800 }]);
    expect(statuses.at(-1)).toBe('ONLINE 28800 / RESUMED');
    modem.dispose();
  });

  it('drops carrier when the server says the resumable session is gone', () => {
    const sockets: FakeSocket[] = [];
    const calls: unknown[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
      dialDelayMs: 0,
      reconnectDelayMs: 10,
    });
    modem.onCallState = call => calls.push(call);

    modem.submitLine('ATDT0450000001');
    sockets[0].open();
    vi.runOnlyPendingTimers();
    sockets[0].receive({
      type: 'dial_result',
      result: 'connect',
      baud: 14400,
      session_id: 'session-expired',
      host: { name: 'TEST', phone: '0450000001' },
    });
    sockets[0].disconnect();
    vi.advanceTimersByTime(10);
    sockets[1].open();
    sockets[1].receive({ type: 'resume_result', result: 'expired', session_id: 'session-expired' });

    expect(calls).toEqual([{ phone: '0450000001', baud: 14400 }, null]);
    modem.dispose();
  });

  it('drops carrier exactly once with a legacy server that provides no session id', () => {
    const socket = new FakeSocket();
    const calls: unknown[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      audio: silentAudio,
      dialDelayMs: 0,
    });
    modem.onCallState = call => calls.push(call);

    modem.submitLine('ATDT0450000001');
    socket.open();
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

  it('passes the host role from the server to the call state, and omits it when absent', () => {
    const sockets: FakeSocket[] = [];
    const calls: unknown[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => {
        const socket = new FakeSocket();
        sockets.push(socket);
        return socket;
      },
      audio: silentAudio,
      dialDelayMs: 0,
    });
    modem.onCallState = call => calls.push(call);

    modem.submitLine('ATDT0920000196');
    sockets[0].open();
    vi.runOnlyPendingTimers();
    sockets[0].receive({
      type: 'dial_result',
      result: 'connect',
      baud: 14400,
      session_id: 'session-role',
      host: { name: 'STATION', phone: '0920000196', role: 'experiment' },
    });
    modem.hangup();

    modem.submitLine('ATDT0459999999');
    sockets[1].open();
    vi.runOnlyPendingTimers();
    sockets[1].receive({
      type: 'dial_result',
      result: 'connect',
      baud: 9600,
      session_id: 'session-plain',
      host: { name: 'PLAIN', phone: '0459999999' },
    });

    expect(calls[0]).toStrictEqual({ phone: '0920000196', baud: 14400, role: 'experiment' });
    expect(calls[1]).toBeNull();
    expect(calls[2]).toStrictEqual({ phone: '0459999999', baud: 9600 });
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

    modem.submitLine('ATDT0450000001');
    socket.open();
    vi.advanceTimersByTime(10);

    expect(socket.sent).toEqual([
      JSON.stringify({ type: 'dial', phone: '0450000001', attempt: 1 }),
    ]);

    socket.receive({ type: 'dial_result', result: 'connect', baud: 28800 });
    expect(calls).toEqual([{ phone: '0450000001', baud: 28800 }]);
    modem.dispose();
  });
});
