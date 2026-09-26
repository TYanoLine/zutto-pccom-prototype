import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TerminalCore } from '../terminal/TerminalCore';
import type { ModemTelemetry } from './ModemTelemetry';
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

describe('VirtualModem telemetry', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('tracks off-hook, carrier, high-speed and data activity from real modem state', () => {
    const socket = new FakeSocket();
    const telemetry: ModemTelemetry[] = [];
    const modem = new VirtualModem(new TerminalCore(), 'ws://test', {
      socketFactory: () => socket,
      dialDelayMs: 0,
      serialTickMs: 16,
      audio: { dial: () => 0, ringback: () => 0, busy: () => 0, handshake: () => 0 },
    });
    modem.setAutoRedial(false);
    modem.onTelemetry = state => telemetry.push(state);

    modem.submitLine('ATDT0920000196');
    expect(telemetry.some(state => state.phase === 'dialing' && state.oh && !state.cd)).toBe(true);

    socket.open();
    vi.runOnlyPendingTimers();
    socket.receive({
      type: 'dial_result',
      result: 'connect',
      baud: 14400,
      session_id: 'telemetry-test',
      host: { name: 'TEST', phone: '0920000196' },
    });

    expect(telemetry.at(-1)).toMatchObject({
      phase: 'online',
      baud: 14400,
      oh: true,
      cd: true,
      hs: true,
    });

    modem.submitLine('HELLO');
    expect(telemetry.at(-1)?.sd).toBe(true);

    socket.receive({ type: 'terminal', text: 'WELCOME' });
    vi.advanceTimersByTime(16);
    expect(telemetry.some(state => state.rd)).toBe(true);

    modem.hangup();
    expect(telemetry.at(-1)).toMatchObject({
      phase: 'idle',
      baud: null,
      oh: false,
      cd: false,
    });
    modem.dispose();
  });
});
