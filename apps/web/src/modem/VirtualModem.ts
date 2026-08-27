import { playBusy, playDialSequence, playHandshake } from '../audio/modemAudio';
import type { TerminalCore } from '../terminal/TerminalCore';

type ServerMessage = {
  type: string;
  result?: string;
  baud?: number;
  line?: number;
  text?: string;
  host?: { name: string; phone: string };
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

type VirtualModemOptions = {
  socketFactory?: (url: string) => ModemSocket;
  dialDelayMs?: number;
  setTimeout?: typeof globalThis.setTimeout;
  clearTimeout?: typeof globalThis.clearTimeout;
  audio?: {
    dial(phone: string): void;
    busy(): void;
    handshake(baud: number): void;
  };
};

export type CallState = { phone: string; baud: number } | null;

const SOCKET_OPEN = 1;

type PendingDial = { phone: string; attempt: number };

export class VirtualModem {
  private ws?: ModemSocket;
  private connected = false;
  private dialing = false;
  private destroyed = false;
  private lastPhone = '';
  private attempt = 0;
  private pendingDial?: PendingDial;
  private retryTimer?: Timer;
  private dialTimer?: Timer;
  private readonly socketFactory: (url: string) => ModemSocket;
  private readonly dialDelayMs: number;
  private readonly schedule: typeof globalThis.setTimeout;
  private readonly cancel: typeof globalThis.clearTimeout;
  private readonly audio: NonNullable<VirtualModemOptions['audio']>;
  autoRedial = true;
  redialSeconds = 5;
  onStatus?: (status: string) => void;
  onCallState?: (call: CallState) => void;

  constructor(
    private readonly terminal: TerminalCore,
    private readonly url: string,
    options: VirtualModemOptions = {},
  ) {
    this.socketFactory = options.socketFactory ?? (socketURL => new WebSocket(socketURL));
    this.dialDelayMs = options.dialDelayMs ?? 650;
    this.schedule = options.setTimeout ?? globalThis.setTimeout.bind(globalThis);
    this.cancel = options.clearTimeout ?? globalThis.clearTimeout.bind(globalThis);
    this.audio = options.audio ?? {
      dial: playDialSequence,
      busy: playBusy,
      handshake: playHandshake,
    };
    // Deliberately do not open a socket here. The terminal boots standalone;
    // the transport represents the telephone call and is created by ATD.
  }

  submitLine(raw: string) {
    if (this.destroyed) return;
    if (this.connected) {
      this.send({ type: 'line', line: raw });
      return;
    }

    const line = raw.trim();
    const upper = line.toUpperCase();
    if (upper === 'AT' || upper === 'ATZ') { this.terminal.write('\r\nOK\r\n'); return; }
    if (upper === 'ATI') { this.terminal.write('\r\nZUTTO MODEM 14400/FAX prototype\r\nOK\r\n'); return; }
    if (upper === 'A/' && this.lastPhone) { this.dial(this.lastPhone, true); return; }
    if (upper === 'ATDL') {
      if (this.lastPhone) this.dial(this.lastPhone, false);
      else this.terminal.write('\r\nERROR\r\n');
      return;
    }
    if (upper === 'ATH') { this.hangup(); return; }
    if (upper.startsWith('ATDT') || upper.startsWith('ATDP')) {
      const phone = line.slice(4).replace(/\D/g, '');
      if (!phone) { this.terminal.write('\r\nERROR\r\n'); return; }
      this.dial(phone, false);
      return;
    }
    this.terminal.write('\r\nERROR\r\n');
  }

  setAutoRedial(on: boolean) {
    this.autoRedial = on;
    if (!on) this.clearRetryTimer();
  }

  hangup() {
    if (this.destroyed) return;
    const hadCarrier = this.connected;
    const wasCalling = hadCarrier || this.dialing || this.retryTimer !== undefined;
    this.clearCallTimers();
    this.pendingDial = undefined;
    if (hadCarrier) this.send({ type: 'hangup' });
    this.connected = false;
    this.dialing = false;
    if (hadCarrier) this.onCallState?.(null);
    this.releaseSocket('hangup');
    this.terminal.write(wasCalling ? '\r\nNO CARRIER\r\n' : '\r\nOK\r\n');
    this.onStatus?.('STANDALONE / MODEM IDLE');
  }

  dispose() {
    if (this.destroyed) return;
    this.destroyed = true;
    this.clearCallTimers();
    this.pendingDial = undefined;
    if (this.connected) this.onCallState?.(null);
    this.connected = false;
    this.dialing = false;
    this.releaseSocket('terminal disposed');
  }

  private ensureSocket() {
    if (this.destroyed) return;
    if (this.ws) {
      if (this.ws.readyState === SOCKET_OPEN) this.flushPendingDial();
      return;
    }

    let socket: ModemSocket;
    try {
      socket = this.socketFactory(this.url);
    } catch {
      this.failDialTone();
      return;
    }

    this.ws = socket;
    socket.onopen = () => {
      if (this.destroyed || socket !== this.ws) return;
      this.onStatus?.('MODEM READY / LINE OPEN');
      this.flushPendingDial();
    };
    socket.onclose = () => {
      if (this.destroyed || socket !== this.ws) return;
      this.ws = undefined;
      this.clearDialTimer();

      if (this.connected) {
        this.connected = false;
        this.dialing = false;
        this.pendingDial = undefined;
        this.terminal.write('\r\nNO CARRIER\r\n');
        this.onCallState?.(null);
        this.onStatus?.('STANDALONE / NO CARRIER');
        return;
      }

      if (this.dialing || this.pendingDial) {
        this.failDialTone(false);
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
      // Browsers normally deliver onclose after an error. Keep error handling
      // centralized there so a failed server cannot break standalone mode.
    };
  }

  private dial(phone: string, retry: boolean) {
    this.clearRetryTimer();
    this.clearDialTimer();
    this.lastPhone = phone;
    if (!retry) this.attempt = 0;
    this.attempt++;
    this.dialing = true;
    this.pendingDial = { phone, attempt: this.attempt };
    this.terminal.write(`\r\nDIALING ${phone} ...\r\n`);
    this.playAudio(() => this.audio.dial(phone));
    this.onStatus?.(`DIAL ${phone}`);
    this.ensureSocket();
  }

  private flushPendingDial() {
    if (!this.pendingDial || !this.ws || this.ws.readyState !== SOCKET_OPEN) return;
    const pending = this.pendingDial;
    this.clearDialTimer();
    this.dialTimer = this.schedule(() => {
      this.dialTimer = undefined;
      if (!this.pendingDial || this.pendingDial !== pending || !this.dialing) return;
      if (this.send({ type: 'dial', phone: pending.phone, attempt: pending.attempt })) {
        this.pendingDial = undefined;
      } else {
        this.failDialTone();
      }
    }, this.dialDelayMs);
  }

  private failDialTone(release = true) {
    this.clearDialTimer();
    this.pendingDial = undefined;
    this.dialing = false;
    if (release) this.releaseSocket('no dialtone');
    this.terminal.write('\r\nNO DIALTONE\r\n');
    this.onStatus?.('STANDALONE / NO SERVER');
  }

  private handleServer(msg: ServerMessage) {
    if (msg.type === 'terminal' && msg.text) { this.terminal.write(msg.text); return; }
    if (msg.type === 'carrier' && msg.result === 'off') {
      this.clearCallTimers();
      const hadCarrier = this.connected;
      this.connected = false;
      this.dialing = false;
      this.pendingDial = undefined;
      if (hadCarrier) {
        this.terminal.write('\r\nNO CARRIER\r\n');
        this.onCallState?.(null);
      }
      this.releaseSocket('carrier off');
      this.onStatus?.('STANDALONE / MODEM IDLE');
      return;
    }
    if (msg.type !== 'dial_result') return;

    this.clearDialTimer();
    this.pendingDial = undefined;
    this.dialing = false;
    if (msg.result === 'busy') {
      this.playAudio(() => this.audio.busy());
      this.terminal.write('\r\nBUSY\r\n');
      this.onStatus?.(`BUSY / RETRY ${this.attempt}`);
      this.releaseSocket('busy');
      if (this.autoRedial) {
        this.terminal.write(`AUTO REDIAL IN ${this.redialSeconds} SEC...\r\n`);
        this.retryTimer = this.schedule(
          () => this.dial(this.lastPhone, true),
          this.redialSeconds * 1000,
        );
      }
      return;
    }
    if (msg.result === 'no_answer') {
      this.terminal.write('\r\nNO ANSWER\r\n');
      this.onStatus?.('STANDALONE / NO ANSWER');
      this.releaseSocket('no answer');
      return;
    }
    if (msg.result === 'connect') {
      this.clearRetryTimer();
      this.connected = true;
      const baud = msg.baud ?? 9600;
      this.playAudio(() => this.audio.handshake(baud));
      this.terminal.write(`\r\nCONNECT ${baud}\r\n`);
      this.onStatus?.(`ONLINE ${baud}`);
      this.onCallState?.({ phone: msg.host?.phone ?? this.lastPhone, baud });
    }
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

  private playAudio(play: () => void) {
    try {
      play();
    } catch {
      // Audio is atmospheric. A missing/blocked Web Audio implementation must
      // never interrupt dialing, carrier handling, or auto-redial.
    }
  }

  private clearCallTimers() {
    this.clearRetryTimer();
    this.clearDialTimer();
  }

  private clearRetryTimer() {
    if (this.retryTimer !== undefined) this.cancel(this.retryTimer);
    this.retryTimer = undefined;
  }

  private clearDialTimer() {
    if (this.dialTimer !== undefined) this.cancel(this.dialTimer);
    this.dialTimer = undefined;
  }
}
