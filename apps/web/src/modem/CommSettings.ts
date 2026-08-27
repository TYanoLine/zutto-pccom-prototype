export type CharacterCode = 'shift-jis' | 'jis' | 'ascii';
export type FlowControl = 'rtscts' | 'xonxoff' | 'none';
export type ErrorCorrection = 'auto' | 'v42' | 'mnp4' | 'off';
export type Compression = 'auto' | 'v42bis' | 'mnp5' | 'off';
export type Parity = 'none' | 'even' | 'odd';
export type TerminalEmulation = 'ansi' | 'vt100' | 'plain';
export type DefaultDialMode = 'tone' | 'pulse';

export type CommSettings = {
  lineBaud: 2400 | 9600 | 14400 | 28800;
  dteBaud: 9600 | 19200 | 38400 | 57600 | 115200;
  dataBits: 7 | 8;
  parity: Parity;
  stopBits: 1 | 2;
  flowControl: FlowControl;
  characterCode: CharacterCode;
  terminal: TerminalEmulation;
  errorCorrection: ErrorCorrection;
  compression: Compression;
  defaultDialMode: DefaultDialMode;
  localEcho: boolean;
};

export const DEFAULT_COMM_SETTINGS: CommSettings = {
  lineBaud: 14400,
  dteBaud: 38400,
  dataBits: 8,
  parity: 'none',
  stopBits: 1,
  flowControl: 'rtscts',
  characterCode: 'shift-jis',
  terminal: 'ansi',
  errorCorrection: 'auto',
  compression: 'auto',
  defaultDialMode: 'tone',
  localEcho: false,
};

export function normalizeCommSettings(value: Partial<CommSettings> | null | undefined): CommSettings {
  const base = DEFAULT_COMM_SETTINGS;
  if (!value) return { ...base };

  const oneOf = <T extends string | number>(candidate: unknown, allowed: readonly T[], fallback: T): T =>
    allowed.includes(candidate as T) ? candidate as T : fallback;

  return {
    lineBaud: oneOf(value.lineBaud, [2400, 9600, 14400, 28800] as const, base.lineBaud),
    dteBaud: oneOf(value.dteBaud, [9600, 19200, 38400, 57600, 115200] as const, base.dteBaud),
    dataBits: oneOf(value.dataBits, [7, 8] as const, base.dataBits),
    parity: oneOf(value.parity, ['none', 'even', 'odd'] as const, base.parity),
    stopBits: oneOf(value.stopBits, [1, 2] as const, base.stopBits),
    flowControl: oneOf(value.flowControl, ['rtscts', 'xonxoff', 'none'] as const, base.flowControl),
    characterCode: oneOf(value.characterCode, ['shift-jis', 'jis', 'ascii'] as const, base.characterCode),
    terminal: oneOf(value.terminal, ['ansi', 'vt100', 'plain'] as const, base.terminal),
    errorCorrection: oneOf(value.errorCorrection, ['auto', 'v42', 'mnp4', 'off'] as const, base.errorCorrection),
    compression: oneOf(value.compression, ['auto', 'v42bis', 'mnp5', 'off'] as const, base.compression),
    defaultDialMode: oneOf(value.defaultDialMode, ['tone', 'pulse'] as const, base.defaultDialMode),
    localEcho: typeof value.localEcho === 'boolean' ? value.localEcho : base.localEcho,
  };
}
