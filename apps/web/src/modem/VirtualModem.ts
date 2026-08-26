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
  reconnectDelaysMs?: number[];
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

export class VirtualModem {
  private ws?: ModemSocket;
  private connected = false;
  private dialing = false;
  private destroyed = false;
  private lastPhone = '';
  private attempt = 0;
  private retryTimer?: Timer;
  private dialTimer?: Timer;
  private reconnectTimer?: Timer;
  private reconnectAttempt = 0;
  private readonly socketFactory: (url: string) => ModemSocket;
  private readonly reconnectDelaysMs: number[];
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
    this.reconnectDelaysMs = options.reconnectDelaysMs ?? [1_000, 2_000, 5_000, 10_000];
    this.dialDelayMs = options.dialDelayMs ?? 650;
    this.schedule = options.setTimeout ?? globalThis.setTimeout.bind(globalThis);
    this.cancel = options.clearTimeout ?? globalThis.clearTimeout.bind(globalThis);
    this.audio = options.audio ?? {
      dial: playDialSequence,
      busy: playBusy,
      handshake: playHandshake,
    };
    this.openSocket();
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
    if (hadCarrier) this.send({ type: 'hangup' });
    this.connected = false;
    this.dialing = false;
    if (hadCarrier) this.onCallState?.(null);
    this.terminal.write(wasCalling ? '\r\nNO CARRIER\r\n' : '\r\nOK\r\n');
    this.onStatus?.('OFFLINE');
  }

  dispose() {
    if (this.destroyed) return;
    this.destroyed = true;
    this.clearCallTimers();
    this.clearReconnectTimer();
    if (this.connected) this.onCallState?.(null);
    this.connected = false;
    this.dialing = false;
    const socket = this.ws;
    this.ws = undefined;
    if (socket) {
      socket.onopen = null;
      socket.onclose = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.close(1000, 'terminal disposed');
    }
  }

  private openSocket() {
    if (this.destroyed) return;
    const socket = this.socketFactory(this.url);
    this.ws = socket;
    socket.onopen = () => {
      if (this.destroyed || socket !== this.ws) return;
      this.reconnectAttempt = 0;
      this.onStatus?.('MODEM READY');
    };
    socket.onclose = () => {
      if (this.destroyed || socket !== this.ws) return;
      this.ws = undefined;
      this.clearCallTimers();
      if (this.connected) {
        this.connected = false;
        this.terminal.write('\r\nNO CARRIER\r\n');
        this.onCallState?.(null);
      }
      this.dialing = false;
      this.onStatus?.('SERVER OFFLINE / RECONNECTING');
      this.scheduleReconnect();
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
      // Browsers deliver onclose after an error; reconnection is centralized there.
    };
  }

  private scheduleReconnect() {
    if (this.destroyed || this.reconnectTimer !== undefined) return;
    const index = Math.min(this.reconnectAttempt, this.reconnectDelaysMs.length - 1);
    const delay = this.reconnectDelaysMs[index];
    this.reconnectAttempt++;
    this.reconnectTimer = this.schedule(() => {
      this.reconnectTimer = undefined;
      this.openSocket();
    }, delay);
  }

  private dial(phone: string, retry: boolean) {
    this.clearRetryTimer();
    this.clearDialTimer();
    this.lastPhone = phone;
    if (!retry) this.attempt = 0;
    this.attempt++;
    this.dialing = true;
    this.terminal.write(`\r\nDIALING ${phone} ...\r\n`);
    this.playAudio(() => this.audio.dial(phone));
    this.onStatus?.(`DIAL ${phone}`);
    this.dialTimer = this.schedule(() => {
      this.dialTimer = undefined;
      if (!this.send({ type: 'dial', phone, attempt: this.attempt })) this.dialing = false;
    }, this.dialDelayMs);
  }

  private handleServer(msg: ServerMessage) {
    if (msg.type === 'terminal' && msg.text) { this.terminal.write(msg.text); return; }
    if (msg.type === 'carrier' && msg.result === 'off') {
      this.clearCallTimers();
      const hadCarrier = this.connected;
      this.connected = false;
      this.dialing = false;
      if (hadCarrier) {
        this.terminal.write('\r\nNO CARRIER\r\n');
        this.onCallState?.(null);
      }
      this.onStatus?.('OFFLINE');
      return;
    }
    if (msg.type !== 'dial_result') return;

    this.clearDialTimer();
    this.dialing = false;
    if (msg.result === 'busy') {
      this.playAudio(() => this.audio.busy());
      this.terminal.write('\r\nBUSY\r\n');
      this.onStatus?.(`BUSY / RETRY ${this.attempt}`);
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
      this.onStatus?.('NO ANSWER');
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
    this.terminal.write('\r\nNO DIALTONE\r\n');
    return false;
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

  private clearReconnectTimer() {
    if (this.reconnectTimer !== undefined) this.cancel(this.reconnectTimer);
    this.reconnectTimer = undefined;
  }
}
