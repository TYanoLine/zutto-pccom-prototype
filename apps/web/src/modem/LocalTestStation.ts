import { playDialSequence } from '../audio/dialLineAudio';
import type { DialMode } from '../audio/dialLineAudio';
import { playHandshake } from '../audio/modemAudio';
import type { HandshakeRun } from '../audio/modemAudio';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import type { CommSettings } from './CommSettings';
import { protocolStateForSettings } from './ModemTelemetry';
import type { ModemPhase, ModemTelemetry } from './ModemTelemetry';
import type { TerminalCore } from '../terminal/TerminalCore';

export const LOCAL_TEST_NUMBER = '0312345678';

type Timer = ReturnType<typeof globalThis.setTimeout>;

type LocalTestStationOptions = {
  answerDelayMs?: number;
  serialTickMs?: number;
  setTimeout?: typeof globalThis.setTimeout;
  clearTimeout?: typeof globalThis.clearTimeout;
  audio?: {
    dial(phone: string, mode: DialMode): number | void;
    handshake(baud: number): HandshakeRun;
  };
};

function framing(settings: CommSettings): string {
  const parity = settings.parity === 'none' ? 'N' : settings.parity === 'even' ? 'E' : 'O';
  return `${settings.dataBits}${parity}${settings.stopBits}`;
}

function resolvedCorrection(settings: CommSettings): string {
  if (settings.errorCorrection === 'auto') return 'V.42';
  if (settings.errorCorrection === 'v42') return 'V.42';
  if (settings.errorCorrection === 'mnp4') return 'MNP4';
  return 'OFF';
}

function resolvedCompression(settings: CommSettings): string {
  if (settings.compression === 'auto') return 'V.42bis';
  if (settings.compression === 'v42bis') return 'V.42bis';
  if (settings.compression === 'mnp5') return 'MNP5';
  return 'OFF';
}

function compressionFactor(settings: CommSettings): number {
  const mode = resolvedCompression(settings);
  if (mode === 'V.42bis') return 1.7;
  if (mode === 'MNP5') return 1.3;
  return 1;
}

export class LocalTestStation {
  private connected = false;
  private dialing = false;
  private negotiating = false;
  private destroyed = false;
  private settings?: CommSettings;
  private displaySettings: CommSettings = { ...DEFAULT_COMM_SETTINGS };
  private currentBaud = 0;
  private txActive = false;
  private rxActive = false;
  private mode: DialMode = 'tone';
  private answerTimer?: Timer;
  private connectTimer?: Timer;
  private serialTimer?: Timer;
  private txLampTimer?: Timer;
  private rxLampTimer?: Timer;
  private txQueue = '';
  private txByteCredit = 0;
  private readonly answerDelayMs: number;
  private readonly serialTickMs: number;
  private readonly schedule: typeof globalThis.setTimeout;
  private readonly cancel: typeof globalThis.clearTimeout;
  private readonly audio: NonNullable<LocalTestStationOptions['audio']>;

  onStatus?: (status: string) => void;
  onConnectionChange?: (connected: boolean, baud?: number) => void;
  onTelemetry?: (telemetry: ModemTelemetry) => void;

  constructor(
    private readonly terminal: TerminalCore,
    options: LocalTestStationOptions = {},
  ) {
    this.answerDelayMs = options.answerDelayMs ?? 850;
    this.serialTickMs = options.serialTickMs ?? 16;
    this.schedule = options.setTimeout ?? globalThis.setTimeout.bind(globalThis);
    this.cancel = options.clearTimeout ?? globalThis.clearTimeout.bind(globalThis);
    this.audio = options.audio ?? {
      dial: playDialSequence,
      handshake: playHandshake,
    };
  }

  isConnected(): boolean { return this.connected; }
  isDialing(): boolean { return this.dialing; }

  setCommunicationSettings(settings: CommSettings): void {
    this.displaySettings = { ...settings };
    this.emitTelemetry();
  }

  getTelemetry(): ModemTelemetry {
    return this.telemetrySnapshot();
  }

  dial(mode: DialMode, settings: CommSettings): void {
    if (this.destroyed) return;
    this.hangup(false);
    this.mode = mode;
    this.settings = { ...settings };
    this.displaySettings = { ...settings };
    this.dialing = true;
    this.negotiating = false;
    this.currentBaud = 0;
    this.pulseActivity('tx');
    this.emitTelemetry();
    this.terminal.write(`\r\nDIALING ${LOCAL_TEST_NUMBER} ${mode === 'pulse' ? '(PULSE)' : '(TONE)'} ...\r\n`);
    this.onStatus?.(`DIAL LOCAL TEST / ${mode.toUpperCase()}`);

    let dialSeconds = 0;
    try { dialSeconds = this.audio.dial(LOCAL_TEST_NUMBER, mode) ?? 0; } catch { /* audio is optional */ }

    this.answerTimer = this.schedule(() => {
      this.answerTimer = undefined;
      if (!this.dialing || this.destroyed || !this.settings) return;
      this.negotiating = true;
      this.currentBaud = this.settings.lineBaud;
      this.emitTelemetry();
      this.onStatus?.(`LOCAL TEST / HANDSHAKE ${this.settings.lineBaud}`);

      let run: HandshakeRun | undefined;
      try { run = this.audio.handshake(this.settings.lineBaud); } catch { /* still connect without Web Audio */ }
      const handshakeMs = Math.max(100, Math.round((run?.duration ?? 1.0) * 1000));
      this.connectTimer = this.schedule(() => this.finishConnect(), handshakeMs + 80);
    }, Math.max(0, Math.round(dialSeconds * 1000)) + this.answerDelayMs);
  }

  submitLine(raw: string): void {
    if (!this.connected || !this.settings || this.destroyed) return;
    if (raw.length > 0) this.pulseActivity('tx');
    const line = raw.trim();
    const upper = line.toUpperCase();

    if (upper === 'ATH' || upper === 'Q' || upper === 'QUIT' || upper === 'BYE') {
      this.hangup(true);
      return;
    }
    if (upper === '' || upper === 'H' || upper === '?' || upper === 'HELP') {
      this.write(this.menu());
      return;
    }
    if (upper === 'I' || upper === 'INFO' || upper === '1') {
      this.write(this.info());
      return;
    }
    if (upper === 'J' || upper === 'JP' || upper === '2') {
      this.write(this.japaneseTest());
      return;
    }
    if (upper === 'A' || upper === 'ASCII' || upper === '3') {
      this.write('\r\n[ASCII TEST]\r\nABCDEFGHIJKLMNOPQRSTUVWXYZ\r\nabcdefghijklmnopqrstuvwxyz\r\n0123456789 !"#$%&\'()*+,-./:;<=>?@[\\]^_`{|}~\r\nOK\r\n');
      return;
    }
    if (upper === 'C' || upper === 'COLOR' || upper === 'ANSI' || upper === '4') {
      this.write(this.ansiTest());
      return;
    }
    if (upper === 'S' || upper === 'SPEED' || upper === '5') {
      this.write(this.speedTest());
      return;
    }
    if (upper === 'E' || upper === 'ECHO' || upper.startsWith('E ')) {
      const text = line.length > 1 ? line.replace(/^E(?:CHO)?\s*/i, '') : 'THE QUICK BROWN FOX / 0123456789 / テスト';
      this.write(`\r\n[REMOTE ECHO]\r\n${text}\r\nOK\r\n`);
      return;
    }

    this.write(`\r\n? UNKNOWN COMMAND: ${line}\r\nH=HELP\r\n`);
  }

  hangup(showResult = true): void {
    const wasActive = this.connected || this.dialing;
    this.clearTimers();
    this.clearSerial();
    this.connected = false;
    this.dialing = false;
    this.negotiating = false;
    this.currentBaud = 0;
    this.settings = undefined;
    if (wasActive) this.onConnectionChange?.(false);
    this.emitTelemetry();
    if (showResult && wasActive) this.terminal.write('\r\nNO CARRIER\r\n');
    if (wasActive) this.onStatus?.('STANDALONE / MODEM IDLE');
  }

  dispose(): void {
    if (this.destroyed) return;
    this.destroyed = true;
    this.clearTimers();
    this.clearSerial();
    this.connected = false;
    this.dialing = false;
    this.negotiating = false;
    this.currentBaud = 0;
    this.settings = undefined;
    this.clearActivityTimers();
  }

  private finishConnect(): void {
    this.connectTimer = undefined;
    if (!this.dialing || this.destroyed || !this.settings) return;
    this.dialing = false;
    this.negotiating = false;
    this.connected = true;
    this.txByteCredit = 0;
    const baud = this.settings.lineBaud;
    this.currentBaud = baud;
    this.emitTelemetry();
    this.terminal.write(`\r\nCONNECT ${baud}\r\n`);
    this.pulseActivity('rx');
    this.onStatus?.(`LOCAL TEST ONLINE ${baud}`);
    this.onConnectionChange?.(true, baud);
    this.write(this.banner());
  }

  private banner(): string {
    return [
      '\r\n',
      '========================================\r\n',
      ' ZUTTO LOCAL MODEM TEST STATION\r\n',
      ` TEL ${LOCAL_TEST_NUMBER} / MULTISTANDARD MODEM\r\n`,
      ' ALL COMMUNICATION OPTIONS ACCEPTED\r\n',
      '========================================\r\n',
      this.info(),
      this.menu(),
    ].join('');
  }

  private info(): string {
    const s = this.settings!;
    return [
      '\r\n[NEGOTIATED SETTINGS]\r\n',
      `LINE       : ${s.lineBaud} bps\r\n`,
      `DTE        : ${s.dteBaud} bps\r\n`,
      `FRAMING    : ${framing(s)}\r\n`,
      `FLOW       : ${s.flowControl.toUpperCase()}\r\n`,
      `CHAR CODE  : ${s.characterCode.toUpperCase()}\r\n`,
      `TERMINAL   : ${s.terminal.toUpperCase()}\r\n`,
      `ERROR CORR : ${resolvedCorrection(s)}\r\n`,
      `COMPRESS   : ${resolvedCompression(s)}\r\n`,
      `DIAL MODE  : ${this.mode.toUpperCase()}\r\n`,
      'RESULT     : ACCEPTED / TEST LOOP\r\n',
    ].join('');
  }

  private menu(): string {
    return [
      '\r\n[TEST MENU]\r\n',
      '1 / I  NEGOTIATED SETTINGS\r\n',
      '2 / J  JAPANESE TEXT TEST\r\n',
      '3 / A  ASCII TEXT TEST\r\n',
      '4 / C  ANSI COLOR TEST\r\n',
      '5 / S  THROUGHPUT TEST\r\n',
      'E text REMOTE ECHO TEST\r\n',
      'H / ?  HELP\r\n',
      'Q      DISCONNECT\r\n',
      '> ',
    ].join('');
  }

  private japaneseTest(): string {
    if (this.settings?.characterCode === 'ascii') {
      return '\r\n[JAPANESE TEXT TEST]\r\nCHARACTER CODE IS ASCII: JAPANESE NOT AVAILABLE\r\nOK\r\n';
    }
    return [
      '\r\n[日本語表示テスト]\r\n',
      'これは「ずっとパソコン通信」ローカル試験局です。\r\n',
      '漢字・ひらがな・カタカナ：通信試験　あいうえお　アイウエオ\r\n',
      '全角記号：［］（）「」・。、！？\r\n',
      '表示できれば文字コード試験はOKです。\r\n',
      'OK\r\n',
    ].join('');
  }

  private ansiTest(): string {
    if (this.settings?.terminal === 'plain') {
      return '\r\n[ANSI TEST]\r\nTERMINAL=PLAIN: ESCAPE COLOR TEST SKIPPED\r\nOK\r\n';
    }
    return '\r\n[ANSI COLOR TEST]\r\n\x1b[31mRED \x1b[32mGREEN \x1b[33mYELLOW \x1b[34mBLUE \x1b[35mMAGENTA \x1b[36mCYAN\x1b[0m\r\nOK\r\n';
  }

  private speedTest(): string {
    const s = this.settings!;
    const effective = Math.round(this.effectiveBytesPerSecond());
    const rows = Array.from({ length: 12 }, (_, i) =>
      `${String(i + 1).padStart(2, '0')} 0123456789 ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz\r\n`,
    ).join('');
    return `\r\n[THROUGHPUT TEST]\r\nLINE=${s.lineBaud} DTE=${s.dteBaud} EFFECTIVE~${effective} byte/s\r\n${rows}END\r\n`;
  }

  private effectiveBytesPerSecond(): number {
    const s = this.settings!;
    const parityBits = s.parity === 'none' ? 0 : 1;
    const serialBitsPerByte = 1 + s.dataBits + parityBits + s.stopBits;
    const dteBytes = s.dteBaud / serialBitsPerByte;
    const lineBytes = (s.lineBaud / 10) * compressionFactor(s);
    return Math.max(1, Math.min(dteBytes, lineBytes));
  }

  private charBytes(ch: string): number {
    if ((ch.codePointAt(0) ?? 0) <= 0x7f) return 1;
    if (this.settings?.characterCode === 'ascii') return 1;
    return 2;
  }

  private write(text: string): void {
    if (!this.connected || !this.settings) return;
    const visible = this.settings.characterCode === 'ascii'
      ? Array.from(text).map(ch => (ch.codePointAt(0) ?? 0) <= 0x7f ? ch : '?').join('')
      : text;
    this.txQueue += visible;
    if (this.serialTimer === undefined) this.scheduleSerialTick();
  }

  private scheduleSerialTick(): void {
    this.serialTimer = this.schedule(() => {
      this.serialTimer = undefined;
      this.drainSerial();
    }, this.serialTickMs);
  }

  private drainSerial(): void {
    if (!this.connected || !this.settings || this.txQueue.length === 0) return;
    this.txByteCredit += this.effectiveBytesPerSecond() * (this.serialTickMs / 1000);

    let count = 0;
    const chars = Array.from(this.txQueue);
    while (count < chars.length) {
      const bytes = this.charBytes(chars[count]);
      if (this.txByteCredit < bytes) break;
      this.txByteCredit -= bytes;
      count++;
    }

    if (count > 0) {
      this.terminal.write(chars.slice(0, count).join(''));
      this.txQueue = chars.slice(count).join('');
      this.pulseActivity('rx');
    }
    if (this.txQueue.length > 0) this.scheduleSerialTick();
  }

  private clearTimers(): void {
    if (this.answerTimer !== undefined) this.cancel(this.answerTimer);
    if (this.connectTimer !== undefined) this.cancel(this.connectTimer);
    this.answerTimer = undefined;
    this.connectTimer = undefined;
  }

  private clearSerial(): void {
    if (this.serialTimer !== undefined) this.cancel(this.serialTimer);
    this.serialTimer = undefined;
    this.txQueue = '';
    this.txByteCredit = 0;
  }

  private phase(): ModemPhase {
    if (this.negotiating) return 'negotiating';
    if (this.connected) return 'online';
    if (this.dialing) return 'dialing';
    return 'idle';
  }

  private telemetrySnapshot(): ModemTelemetry {
    const phase = this.phase();
    const ready = !this.destroyed;
    const carrier = this.connected && !this.negotiating;
    const settings = this.settings ?? this.displaySettings;
    return {
      phase,
      baud: this.currentBaud > 0 ? this.currentBaud : null,
      mr: ready,
      tr: ready,
      sd: this.txActive,
      rd: this.rxActive,
      oh: this.dialing || this.connected,
      cd: carrier,
      aa: false,
      hs: carrier && this.currentBaud >= 9600,
      dsr: ready,
      cts: ready,
      protocol: protocolStateForSettings(settings, phase),
    };
  }

  private emitTelemetry(): void {
    this.onTelemetry?.(this.telemetrySnapshot());
  }

  private pulseActivity(direction: 'tx' | 'rx'): void {
    if (direction === 'tx') {
      this.txActive = true;
      if (this.txLampTimer !== undefined) this.cancel(this.txLampTimer);
      this.txLampTimer = this.schedule(() => {
        this.txLampTimer = undefined;
        this.txActive = false;
        this.emitTelemetry();
      }, 140);
    } else {
      this.rxActive = true;
      if (this.rxLampTimer !== undefined) this.cancel(this.rxLampTimer);
      this.rxLampTimer = this.schedule(() => {
        this.rxLampTimer = undefined;
        this.rxActive = false;
        this.emitTelemetry();
      }, 140);
    }
    this.emitTelemetry();
  }

  private clearActivityTimers(): void {
    if (this.txLampTimer !== undefined) this.cancel(this.txLampTimer);
    if (this.rxLampTimer !== undefined) this.cancel(this.rxLampTimer);
    this.txLampTimer = undefined;
    this.rxLampTimer = undefined;
    this.txActive = false;
    this.rxActive = false;
  }
}
