import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TerminalCore } from '../terminal/TerminalCore';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import { LocalTestStation, LOCAL_TEST_NUMBER } from './LocalTestStation';

function screenText(terminal: TerminalCore): string {
  return terminal.cells.map(row => row.map(cell => cell.continuation ? '' : cell.ch).join('')).join('\n');
}

function handshake(baud: number) {
  return {
    baud,
    seed: 'TEST0001',
    duration: 0.2,
    responseJitterMs: 0,
    speakerResonanceHz: 1900,
    lineLevelDb: 0,
  };
}

describe('LocalTestStation', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('answers the magic number locally using the selected line speed', () => {
    const terminal = new TerminalCore();
    const states: boolean[] = [];
    const dial = vi.fn(() => 0.1);
    const hs = vi.fn(handshake);
    const station = new LocalTestStation(terminal, {
      answerDelayMs: 50,
      audio: { dial, handshake: hs },
    });
    station.onConnectionChange = connected => states.push(connected);

    station.dial('tone', { ...DEFAULT_COMM_SETTINGS, lineBaud: 28800 });
    vi.advanceTimersByTime(150);
    expect(hs).toHaveBeenCalledWith(28800);
    vi.advanceTimersByTime(300);

    expect(station.isConnected()).toBe(true);
    expect(states).toEqual([true]);
    expect(dial).toHaveBeenCalledWith(LOCAL_TEST_NUMBER, 'tone');
    expect(screenText(terminal)).toContain('CONNECT 28800');
    station.dispose();
  });

  it('accepts legacy communication options and disconnects with Q', () => {
    const terminal = new TerminalCore();
    const station = new LocalTestStation(terminal, {
      answerDelayMs: 0,
      serialTickMs: 1,
      audio: { dial: () => 0, handshake },
    });
    const settings = {
      ...DEFAULT_COMM_SETTINGS,
      lineBaud: 2400 as const,
      dataBits: 7 as const,
      parity: 'even' as const,
      stopBits: 2 as const,
      flowControl: 'xonxoff' as const,
      characterCode: 'jis' as const,
      terminal: 'vt100' as const,
      errorCorrection: 'mnp4' as const,
      compression: 'mnp5' as const,
    };

    station.dial('pulse', settings);
    vi.advanceTimersByTime(400);
    vi.runOnlyPendingTimers();
    station.submitLine('I');
    vi.advanceTimersByTime(3000);

    const text = screenText(terminal);
    expect(text).toContain('LINE       : 2400 bps');
    expect(text).toContain('FRAMING    : 7E2');
    expect(text).toContain('FLOW       : XONXOFF');
    expect(text).toContain('ERROR CORR : MNP4');
    expect(text).toContain('COMPRESS   : MNP5');

    station.submitLine('Q');
    expect(station.isConnected()).toBe(false);
    expect(screenText(terminal)).toContain('NO CARRIER');
    station.dispose();
  });
});
