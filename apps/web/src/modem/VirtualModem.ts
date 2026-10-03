import { playHandshake } from '../audio/modemAudio';
import { playBusy, playDialSequence, playRingback } from '../audio/dialLineAudio';
import type { DialMode } from '../audio/dialLineAudio';
import type { TerminalCore } from '../terminal/TerminalCore';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import type { CommSettings } from './CommSettings';
import { protocolStateForSettings } from './ModemTelemetry';
import type { ModemPhase, ModemTelemetry } from './ModemTelemetry';

type ServerMessage = {
  type: string;
  result?: string;
  baud?: number;
  line?: number;
  session_id?: string;
  text?: string;
  host?: { name: string; phone: string; role?: string };
};

type ModemSocket = {
  readyState: number;
  onopen: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  send(data: string): void;
  close(code?: number, reason?: string): void;
};

type Timer = ReturnType<typeof globalThis.setTimeout>;
type HandshakePlayback = void | number | { duration?: number };

type VirtualModemAudio = {
  dial?(phone: string, mode: DialMode): number | void;
  ringback?(): number | void;
  busy?(): number | void;
  handshake?(baud: number): HandshakePlayback;
};

type ResolvedAudio = {
  dial(phone: string, mode: DialMode): number | void;
  ringback(): number | void;
  busy(): number | void;
  handshake(baud: number): HandshakePlayback;
};

type VirtualModemOptions = {
  socketFactory?: (url: string) => ModemSocket;
  dialDelayMs?: number;
  serialTickMs?: number;
  offlineBusyExtraMs?: number;
  remoteConnectGraceMs?: number;
  reconnectDelayMs?: number;
  setTimeout?: typeof globalThis.setTimeout;
  clearTimeout?: typeof globalThis.clearTimeout;
  audio?: VirtualModemAudio;
};

// role is the host's operational role as reported by the server (for example
// "experiment"); it is absent for ordinary hosts and for older servers.
export type CallState = { phone: string; baud: number; role?: string } | null;

const SOCKET_OPEN = 1;

type PendingDial = {
  phone: string;
  attempt: number;
  mode: DialMode;
  dialDurationMs: number;
};

export class VirtualModem {
  private ws?: ModemSocket;
  private connected = false;
  private dialing = false;
  private negotiating = false;
  private recoveringCarrier = false;
  private destroyed = false;
  private lastPhone = '';
  private lastDialMode: DialMode = 'tone';
  private attempt = 0;
  private sessionID = '';
  private pendingDial?: PendingDial;
  private pendingLines: string[] = [];
  private preConnectRx = '';
  private retryTimer?: Timer;
  private dialTimer?: Timer;
  private connectTimer?: Timer;
  private offlineBusyTimer?: Timer;
  private serialTimer?: Timer;
  private txLampTimer?: Timer;
  private rxLampTimer?: Timer;
  private currentBaud = 0;
  private rxQueue = '';
  private rxByteCredit = 0;
  private ringing = false;
  private txActive = false;
  private rxActive = false;
  private commSettings: CommSettings = { ...DEFAULT_COMM_SETTINGS };
  private readonly socketFactory: (url: string) => ModemSocket;
  private readonly dialDelayMs: number;
  private readonly serialTickMs: number;
  private readonly offlineBusyExtraMs: number;
  private readonly remoteConnectGraceMs: number;
  private readonly reconnectDelayMs: number;
  private readonly schedule: typeof globalThis.setTimeout;
  private readonly cancel: typeof globalThis.clearTimeout;
  private readonly audio: ResolvedAudio;
  autoRedial = true;
  redialSeconds = 5;
  onStatus?: (status: string) => void;
  onCallState?: (call: CallState) => void;
  onTelemetry?: (telemetry: ModemTelemetry) => void;

  constructor(
    private readonly terminal: TerminalCore,
    private readonly url: string,
    options: VirtualModemOptions = {},
  ) {
    this.socketFactory = options.socketFactory ?? (socketURL => new WebSocket(socketURL));
    this.dialDelayMs = options.dialDelayMs ?? 650;
    this.serialTickMs = options.serialTickMs ?? 16;
    this.offlineBusyExtraMs = options.offlineBusyExtraMs ?? 850;
    this.remoteConnectGraceMs = options.remoteConnectGraceMs ?? 45_000;
    this.reconnectDelayMs = options.reconnectDelayMs ?? 1_500;
    this.schedule = options.setTimeout ?? globalThis.setTimeout.bind(globalThis);
    this.cancel = options.clearTimeout ?? globalThis.clearTimeout.bind(globalThis);

    const supplied = options.audio;
    this.audio = {
      dial: supplied?.dial ?? playDialSequence,
      // A custom audio object is generally used by unit tests or the complete
      // standalone BUSY sequence; keep those deterministic unless ringback was
      // explicitly supplied. Real remote calls use the default ringback sound.
      ringback: supplied ? (supplied.ringback ?? (() => 0)) : playRingback,
      busy: supplied?.busy ?? playBusy,
      handshake: supplied?.handshake ?? playHandshake,
    };
  }

  submitLine(raw: string) {
    if (this.destroyed) return;
    if (raw.length > 0) this.pulseActivity('tx');
    if (this.connected) {
      if (this.negotiating) {
        this.pendingLines.push(raw);
        this.onStatus?.('MODEM NEGOTIATING / INPUT QUEUED');
        return;
      }
      if (!this.send({ type: 'line', line: raw }) && this.recoveringCarrier) {
        this.pendingLines.push(raw);
        this.onStatus?.('LINE INTERRUPTED / INPUT QUEUED');
      }
      return;
    }

    const line = raw.trim();
    const upper = line.toUpperCase();
    if (upper === 'AT' || upper === 'ATZ') { this.terminal.write('\r\nOK\r\n'); return; }
    if (upper === 'ATI') { this.terminal.write('\r\nZUTTO MODEM 14400/FAX prototype\r\nOK\r\n'); return; }
    if (upper === 'A/' && this.lastPhone) {
      this.dial(this.lastPhone, this.lastDialMode, true);
      return;
    }
    if (upper === 'ATDL') {
      if (this.lastPhone) this.dial(this.lastPhone, this.lastDialMode, false);
      else this.terminal.write('\r\nERROR\r\n');
      return;
    }
    if (upper === 'ATH') { this.hangup(); return; }
    if (upper.startsWith('ATDT') || upper.startsWith('ATDP')) {
      const phone = line.slice(4).replace(/\D/g, '');
      if (!phone) { this.terminal.write('\r\nERROR\r\n'); return; }
      const mode: DialMode = upper.startsWith('ATDP') ? 'pulse' : 'tone';
      this.dial(phone, mode, false);
      return;
    }
    this.terminal.write('\r\nERROR\r\n');
  }

  setAutoRedial(on: boolean) {
    this.autoRedial = on;
    if (!on) this.clearRetryTimer();
  }

  setCommunicationSettings(settings: CommSettings) {
    this.commSettings = { ...settings };
    this.emitTelemetry();
  }

  getTelemetry(): ModemTelemetry {
    return this.telemetrySnapshot();
  }

  hangup() {
    if (this.destroyed) return;
    const hadCarrier = this.connected;
    const wasCalling = hadCarrier || this.dialing || this.retryTimer !== undefined;
    this.clearCallTimers();
    this.clearSerialOutput();
    this.pendingDial = undefined;
    this.pendingLines = [];
    this.preConnectRx = '';
    if (hadCarrier) this.send({ type: 'hangup' });
    this.connected = false;
    this.dialing = false;
    this.ringing = false;
    this.negotiating = false;
    this.recoveringCarrier = false;
    this.sessionID = '';
    if (hadCarrier) this.onCallState?.(null);
    this.releaseSocket('hangup');
    this.terminal.write(wasCalling ? '\r\nNO CARRIER\r\n' : '\r\nOK\r\n');
    this.onStatus?.('STANDALONE / MODEM IDLE');
    this.emitTelemetry();
  }

  dispose() {
    if (this.destroyed) return;
    this.destroyed = true;
    this.clearCallTimers();
    this.clearSerialOutput();
    this.pendingDial = undefined;
    this.pendingLines = [];
    this.preConnectRx = '';
    if (this.connected) this.onCallState?.(null);
    this.connected = false;
    this.dialing = false;
    this.ringing = false;
    this.negotiating = false;
    this.recoveringCarrier = false;
    this.sessionID = '';
    this.clearActivityTimers();
    this.releaseSocket('terminal disposed');
  }

  private ensureSocket() {
    if (this.destroyed || !this.url) return;
    if (this.ws) {
      if (this.ws.readyState === SOCKET_OPEN) {
        if (this.recoveringCarrier && this.sessionID) this.requestResume();
        else this.flushPendingDial();
      }
      return;
    }

    let socket: ModemSocket;
    try {
      socket = this.socketFactory(this.url);
    } catch {
      this.scheduleRemoteReconnect();
      return;
    }

    this.ws = socket;
    socket.onopen = () => {
      if (this.destroyed || socket !== this.ws) return;
      if (this.recoveringCarrier && this.sessionID) {
        this.onStatus?.('LINE RESTORING / SESSION RESUME');
        this.requestResume();
        return;
      }
      this.clearOfflineBusyTimer();
      this.onStatus?.('MODEM READY / LINE OPEN');
      this.flushPendingDial();
    };
    socket.onclose = () => {
      if (this.destroyed || socket !== this.ws) return;
      this.ws = undefined;
      this.clearDialTimer();

      if (this.connected) {
        if (this.sessionID) {
          this.recoveringCarrier = true;
          this.emitTelemetry();
          this.onStatus?.(this.negotiating
            ? 'NEGOTIATION INTERRUPTED / RECONNECTING'
            : 'LINE INTERRUPTED / RECONNECTING');
          this.scheduleRemoteReconnect();
        } else {
          this.finishCarrierLoss('STANDALONE / NO CARRIER');
        }
        return;
      }

      if (this.dialing || this.pendingDial) {
        if (this.url) {
          this.onStatus?.('SERVER WAKING / RETRYING');
          this.scheduleRemoteReconnect();
        } else {
          this.scheduleOfflineBusyFallback(350);
          this.onStatus?.('STANDALONE / LINE SIMULATION');
        }
      } else {
        this.onStatus?.('STANDALONE / MODEM IDLE');
      }
    };
    socket.onmessage = event => {
      if (this.destroyed || socket !== this.ws) return;
      try {
        this.handleServer(JSON.parse(String(event.data)) as ServerMessage);
      } catch {
        this.terminal.write('\r\nMODEM PROTOCOL ERROR\r\n');
      }
    };
    socket.onerror = () => {
      // Browsers normally deliver onclose after an error. Reconnect handling
      // is centralized there so a sleeping remote server can wake cleanly.
    };
  }

  private scheduleRemoteReconnect() {
    if (this.destroyed || !this.url || (!this.dialing && !this.recoveringCarrier)) return;
    this.clearDialTimer();
    this.dialTimer = this.schedule(() => {
      this.dialTimer = undefined;
      this.ensureSocket();
    }, this.reconnectDelayMs);
  }

  private requestResume() {
    if (!this.recoveringCarrier || !this.sessionID) return;
    if (!this.send({ type: 'resume', session_id: this.sessionID })) {
      this.scheduleRemoteReconnect();
    }
  }

  private dial(phone: string, mode: DialMode, retry: boolean) {
    this.clearRetryTimer();
    this.clearDialTimer();
    this.clearConnectTimer();
    this.clearOfflineBusyTimer();
    this.sessionID = '';
    this.recoveringCarrier = false;
    this.ringing = false;
    this.negotiating = false;
    this.pendingLines = [];
    this.preConnectRx = '';
    this.lastPhone = phone;
    this.lastDialMode = mode;
    if (!retry) this.attempt = 0;
    this.attempt++;
    this.dialing = true;
    this.emitTelemetry();

    const dialDurationMs = Math.max(0, Math.round(this.playDialAudio(phone, mode) * 1000));
    this.pendingDial = { phone, attempt: this.attempt, mode, dialDurationMs };
    this.terminal.write(`\r\nDIALING ${phone} ${mode === 'pulse' ? '(PULSE)' : '(TONE)'} ...\r\n`);
    this.onStatus?.(`DIAL ${phone} / ${mode.toUpperCase()}`);

    if (this.url) {
      this.scheduleOfflineBusyFallback(this.remoteConnectGraceMs);
    } else {
      this.scheduleOfflineBusyFallback(dialDurationMs + this.offlineBusyExtraMs);
    }
    this.ensureSocket();
  }

  private flushPendingDial() {
    if (!this.pendingDial || !this.ws || this.ws.readyState !== SOCKET_OPEN) return;
    const pending = this.pendingDial;
    this.clearDialTimer();
    const delay = Math.max(this.dialDelayMs, pending.dialDurationMs);
    this.dialTimer = this.schedule(() => {
      this.dialTimer = undefined;
      if (!this.pendingDial || this.pendingDial !== pending || !this.dialing) return;
      if (this.send({ type: 'dial', phone: pending.phone, attempt: pending.attempt })) {
        this.pendingDial = undefined;
      } else if (this.url) {
        this.scheduleRemoteReconnect();
      } else {
        this.scheduleOfflineBusyFallback(300);
      }
    }, delay);
  }

  private scheduleOfflineBusyFallback(delayMs: number) {
    if (!this.dialing || this.destroyed) return;
    this.clearOfflineBusyTimer();
    this.offlineBusyTimer = this.schedule(() => {
      this.offlineBusyTimer = undefined;
      this.finishOfflineBusy();
    }, Math.max(120, delayMs));
  }

  private finishOfflineBusy() {
    if (!this.dialing || this.destroyed || this.connected) return;
    this.clearDialTimer();
    this.pendingDial = undefined;
    this.dialing = false;
    this.ringing = false;
    this.emitTelemetry();
    this.releaseSocket(this.url ? 'remote timeout' : 'standalone busy');

    if (this.url) {
      this.terminal.write('\r\nNO DIALTONE\r\n');
      this.onStatus?.('SERVER UNAVAILABLE / NO DIALTONE');
    } else {
      this.playAudio(() => this.audio.busy());
      this.terminal.write('\r\nBUSY\r\n');
      this.onStatus?.(`BUSY / STANDALONE LINE / RETRY ${this.attempt}`);
    }
    this.scheduleAutoRedial();
  }

  private handleServer(msg: ServerMessage) {
    if (msg.type === 'terminal' && msg.text) {
      // Servers are allowed to send their welcome screen immediately after
      // accepting the call.  A real terminal cannot see those bytes until the
      // modem handshake has finished, so hold them here during ringing/training.
      if (this.negotiating) this.preConnectRx += msg.text;
      else this.writeReceived(msg.text);
      return;
    }
    if (msg.type === 'carrier' && msg.result === 'off') {
      this.finishCarrierLoss('STANDALONE / MODEM IDLE');
      return;
    }
    if (msg.type === 'resume_result') {
      if (msg.result === 'ok') {
        this.recoveringCarrier = false;
        this.sessionID = msg.session_id ?? this.sessionID;
        if (msg.baud) this.currentBaud = Math.max(300, msg.baud);
        this.emitTelemetry();
        if (this.negotiating) {
          this.onStatus?.(`MODEM NEGOTIATING ${this.currentBaud} / RESUMED`);
        } else {
          this.onStatus?.(`ONLINE ${this.currentBaud} / RESUMED`);
          this.flushPendingLines();
        }
      } else {
        this.finishCarrierLoss(`SESSION ${String(msg.result ?? 'LOST').toUpperCase()} / NO CARRIER`);
      }
      return;
    }
    if (msg.type !== 'dial_result') return;

    this.clearDialTimer();
    this.clearOfflineBusyTimer();
    this.pendingDial = undefined;
    this.dialing = false;

    if (msg.result === 'busy') {
      this.ringing = false;
      this.emitTelemetry();
      this.playAudio(() => this.audio.busy());
      this.terminal.write('\r\nBUSY\r\n');
      this.onStatus?.(`BUSY / RETRY ${this.attempt}`);
      this.releaseSocket('busy');
      this.scheduleAutoRedial();
      return;
    }
    if (msg.result === 'no_answer') {
      this.ringing = true;
      this.emitTelemetry();
      const ringSeconds = this.playAudioDuration(() => this.audio.ringback());
      this.onStatus?.('RINGING / NO ANSWER');
      const finish = () => {
        this.connectTimer = undefined;
        this.ringing = false;
        this.emitTelemetry();
        this.terminal.write('\r\nNO ANSWER\r\n');
        this.onStatus?.('STANDALONE / NO ANSWER');
        this.releaseSocket('no answer');
      };
      if (ringSeconds > 0) this.connectTimer = this.schedule(finish, Math.round(ringSeconds * 1000));
      else finish();
      return;
    }
    if (msg.result === 'no_carrier') {
      this.ringing = false;
      this.negotiating = false;
      this.connected = false;
      this.emitTelemetry();
      this.terminal.write('\r\nNO CARRIER\r\n');
      this.onStatus?.('NO CARRIER');
      this.releaseSocket('no carrier');
      this.scheduleAutoRedial();
      return;
    }

    if (msg.result === 'connect') {
      this.clearRetryTimer();
      this.connected = true;
      this.ringing = true;
      this.negotiating = true;
      this.recoveringCarrier = false;
      this.sessionID = msg.session_id ?? '';
      const baud = msg.baud ?? 9600;
      const phone = msg.host?.phone ?? this.lastPhone;
      const role = msg.host?.role;
      this.currentBaud = Math.max(300, baud);
      this.rxByteCredit = 0;
      this.preConnectRx = '';
      this.emitTelemetry();

      // Let the caller hear the remote line ring before the answering modem
      // grabs the call.  Only after that do the modem training tones begin.
      const ringSeconds = this.playAudioDuration(() => this.audio.ringback());
      this.onStatus?.(`RINGING / ${msg.host?.name ?? phone}`);
      const begin = () => {
        this.connectTimer = undefined;
        this.beginRemoteHandshake(phone, baud, role);
      };
      if (ringSeconds > 0) this.connectTimer = this.schedule(begin, Math.round(ringSeconds * 1000));
      else begin();
    }
  }

  private beginRemoteHandshake(phone: string, baud: number, role?: string) {
    if (this.destroyed || !this.connected || !this.negotiating) return;
    this.ringing = false;
    this.emitTelemetry();
    this.onStatus?.(`MODEM NEGOTIATING ${baud}`);
    const handshakeSeconds = this.playAudioDuration(() => this.audio.handshake(baud));
    const finish = () => {
      this.connectTimer = undefined;
      this.finishRemoteConnect(phone, baud, role);
    };
    if (handshakeSeconds > 0) {
      this.connectTimer = this.schedule(finish, Math.round(handshakeSeconds * 1000) + 80);
    } else {
      finish();
    }
  }

  private finishRemoteConnect(phone: string, baud: number, role?: string) {
    if (this.destroyed || !this.connected || !this.negotiating) return;
    if (this.recoveringCarrier) {
      this.connectTimer = this.schedule(() => {
        this.connectTimer = undefined;
        this.finishRemoteConnect(phone, baud, role);
      }, 200);
      return;
    }

    this.negotiating = false;
    this.ringing = false;
    this.emitTelemetry();
    this.terminal.write(`\r\nCONNECT ${baud}\r\n`);
    this.pulseActivity('rx');
    this.onStatus?.(`ONLINE ${baud}`);
    this.onCallState?.(role ? { phone, baud, role } : { phone, baud });

    const buffered = this.preConnectRx;
    this.preConnectRx = '';
    if (buffered) this.writeReceived(buffered);
    this.flushPendingLines();
  }

  private finishCarrierLoss(status: string) {
    this.clearCallTimers();
    const hadCarrier = this.connected;
    this.connected = false;
    this.dialing = false;
    this.ringing = false;
    this.negotiating = false;
    this.recoveringCarrier = false;
    this.sessionID = '';
    this.pendingDial = undefined;
    this.pendingLines = [];
    this.preConnectRx = '';
    this.clearSerialOutput();
    this.emitTelemetry();
    if (hadCarrier) {
      this.terminal.write('\r\nNO CARRIER\r\n');
      this.onCallState?.(null);
    }
    this.releaseSocket('carrier lost');
    this.onStatus?.(status);
  }

  private flushPendingLines() {
    if (!this.connected || this.negotiating || this.recoveringCarrier || this.pendingLines.length === 0) return;
    const queued = this.pendingLines;
    this.pendingLines = [];
    for (let i = 0; i < queued.length; i++) {
      if (!this.send({ type: 'line', line: queued[i] })) {
        this.pendingLines = queued.slice(i);
        this.recoveringCarrier = true;
        this.emitTelemetry();
        this.onStatus?.('LINE INTERRUPTED / RECONNECTING');
        this.scheduleRemoteReconnect();
        break;
      }
    }
  }

  private scheduleAutoRedial() {
    if (!this.autoRedial || !this.lastPhone || this.destroyed) return;
    this.terminal.write(`AUTO REDIAL IN ${this.redialSeconds} SEC...\r\n`);
    this.retryTimer = this.schedule(
      () => this.dial(this.lastPhone, this.lastDialMode, true),
      this.redialSeconds * 1000,
    );
  }

  private writeReceived(text: string) {
    if (!this.connected || this.currentBaud <= 0) {
      this.terminal.write(text);
      return;
    }
    this.rxQueue += text;
    if (this.serialTimer === undefined) this.scheduleSerialTick();
  }

  private scheduleSerialTick() {
    this.serialTimer = this.schedule(() => {
      this.serialTimer = undefined;
      this.drainSerialOutput();
    }, this.serialTickMs);
  }

  private drainSerialOutput() {
    if (!this.connected || this.currentBaud <= 0 || this.rxQueue.length === 0) return;

    // 8N1 = start + 8 data + stop = 10 line bits per byte. For the current
    // Unicode prototype, ASCII costs one serial byte and Japanese characters
    // approximate their CP932-era two-byte wire cost.
    this.rxByteCredit += (this.currentBaud / 10) * (this.serialTickMs / 1000);

    let count = 0;
    while (count < this.rxQueue.length) {
      const bytes = this.rxQueue.charCodeAt(count) <= 0x7f ? 1 : 2;
      if (this.rxByteCredit < bytes) break;
      this.rxByteCredit -= bytes;
      count++;
    }

    if (count > 0) {
      this.terminal.write(this.rxQueue.slice(0, count));
      this.rxQueue = this.rxQueue.slice(count);
      this.pulseActivity('rx');
    }
    if (this.rxQueue.length > 0) this.scheduleSerialTick();
  }

  private clearSerialOutput() {
    if (this.serialTimer !== undefined) this.cancel(this.serialTimer);
    this.serialTimer = undefined;
    this.rxQueue = '';
    this.rxByteCredit = 0;
    this.currentBaud = 0;
  }

  private phase(): ModemPhase {
    if (this.recoveringCarrier) return 'recovering';
    if (this.ringing) return 'ringing';
    if (this.negotiating) return 'negotiating';
    if (this.connected) return 'online';
    if (this.dialing) return 'dialing';
    return 'idle';
  }

  private telemetrySnapshot(): ModemTelemetry {
    const phase = this.phase();
    const ready = !this.destroyed;
    const carrier = this.connected && !this.negotiating && !this.recoveringCarrier;
    const offHook = this.dialing || this.ringing || this.connected;
    return {
      phase,
      baud: this.currentBaud > 0 ? this.currentBaud : null,
      mr: ready,
      tr: ready,
      sd: this.txActive,
      rd: this.rxActive,
      oh: offHook,
      cd: carrier,
      aa: false,
      hs: carrier && this.currentBaud >= 9600,
      dsr: ready,
      cts: ready && !this.recoveringCarrier,
      protocol: protocolStateForSettings(this.commSettings, phase),
    };
  }

  private emitTelemetry() {
    this.onTelemetry?.(this.telemetrySnapshot());
  }

  private pulseActivity(direction: 'tx' | 'rx') {
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

  private clearActivityTimers() {
    if (this.txLampTimer !== undefined) this.cancel(this.txLampTimer);
    if (this.rxLampTimer !== undefined) this.cancel(this.rxLampTimer);
    this.txLampTimer = undefined;
    this.rxLampTimer = undefined;
    this.txActive = false;
    this.rxActive = false;
  }

  private send(obj: unknown) {
    if (this.ws?.readyState === SOCKET_OPEN) {
      this.ws.send(JSON.stringify(obj));
      return true;
    }
    return false;
  }

  private releaseSocket(reason: string) {
    const socket = this.ws;
    this.ws = undefined;
    if (!socket) return;
    socket.onopen = null;
    socket.onclose = null;
    socket.onmessage = null;
    socket.onerror = null;
    try { socket.close(1000, reason); } catch { /* best effort */ }
  }

  private playDialAudio(phone: string, mode: DialMode): number {
    try {
      return this.audio.dial(phone, mode) ?? 0;
    } catch {
      return 0;
    }
  }

  private playAudio(play: () => unknown) {
    try {
      play();
    } catch {
      // Audio is atmospheric. A missing/blocked Web Audio implementation must
      // never interrupt dialing, carrier handling, or auto-redial.
    }
  }

  private playAudioDuration(play: () => HandshakePlayback): number {
    try {
      const result = play();
      if (typeof result === 'number' && Number.isFinite(result)) return Math.max(0, result);
      if (result && typeof result === 'object' && typeof result.duration === 'number') {
        return Math.max(0, result.duration);
      }
    } catch {
      // Continue without audio or timing metadata.
    }
    return 0;
  }

  private clearCallTimers() {
    this.clearRetryTimer();
    this.clearDialTimer();
    this.clearConnectTimer();
    this.clearOfflineBusyTimer();
  }

  private clearRetryTimer() {
    if (this.retryTimer !== undefined) this.cancel(this.retryTimer);
    this.retryTimer = undefined;
  }

  private clearDialTimer() {
    if (this.dialTimer !== undefined) this.cancel(this.dialTimer);
    this.dialTimer = undefined;
  }

  private clearConnectTimer() {
    if (this.connectTimer !== undefined) this.cancel(this.connectTimer);
    this.connectTimer = undefined;
  }

  private clearOfflineBusyTimer() {
    if (this.offlineBusyTimer !== undefined) this.cancel(this.offlineBusyTimer);
    this.offlineBusyTimer = undefined;
  }
}
