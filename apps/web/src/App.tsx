import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import { LocalTestStation, LOCAL_TEST_NUMBER } from './modem/LocalTestStation';
import { DEFAULT_COMM_SETTINGS, normalizeCommSettings } from './modem/CommSettings';
import type { CommSettings } from './modem/CommSettings';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { pseudoTariffTable } from './billing/pseudoTariffs';
import { Japan1996WorldClock } from './time/WorldClock';
import { playHandshake } from './audio/modemAudio';
import { playDialSequence, playStandaloneBusySequence } from './audio/dialLineAudio';
import type { DialMode } from './audio/dialLineAudio';
import './styles.css';

const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const isLocalHost = typeof window !== 'undefined'
  && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const wsURL = configuredWsURL || (isLocalHost ? 'ws://localhost:8080/ws' : '');
const standaloneLine = wsURL.length === 0;

const configuredWorldDate = (import.meta.env.VITE_WORLD_DATE as string | undefined)?.trim() ?? '';
const worldDate = /^\d{4}-\d{2}-\d{2}$/.test(configuredWorldDate)
  ? configuredWorldDate
  : '1996-08-26';

const configuredTelehodaiNumbers = (import.meta.env.VITE_TELEHODAI_NUMBERS as string | undefined)?.trim();
const telehodaiNumbers = (configuredTelehodaiNumbers || '0451234567,0450000001')
  .split(',')
  .map((phone: string) => phone.trim())
  .filter(Boolean);

const SETTINGS_KEY = 'zutto.commSettings.v1';

type ActiveCall = { phone: string; connectedAt: Date };
type BootWindow = Window & { __zuttoBootOk?: () => void };
type HandshakeRun = ReturnType<typeof playHandshake>;

function loadCommSettings(): CommSettings {
  if (typeof window === 'undefined') return { ...DEFAULT_COMM_SETTINGS };
  try {
    const raw = window.localStorage.getItem(SETTINGS_KEY);
    return raw
      ? normalizeCommSettings(JSON.parse(raw) as Partial<CommSettings>)
      : { ...DEFAULT_COMM_SETTINGS };
  } catch {
    return { ...DEFAULT_COMM_SETTINGS };
  }
}

export default function App() {
  const terminal = useMemo(() => new TerminalCore(), []);
  const clock = useMemo(() => new Japan1996WorldClock(worldDate), []);
  const tariff = useMemo(() => new PseudoTariffService(pseudoTariffTable, telehodaiNumbers), []);
  const modemRef = useRef<VirtualModem | null>(null);
  const localStationRef = useRef<LocalTestStation | null>(null);
  const lastDialWasLocalRef = useRef(false);
  const lastLocalDialModeRef = useRef<DialMode>('tone');

  // The hidden HTML input is the source of truth for keyboard/IME text.
  // echoedInputRef tracks what is currently painted in TerminalCore so an IME
  // composition can replace its preview instead of appending the committed text.
  const echoedInputRef = useRef('');
  const composingRef = useRef(false);
  const suppressEnterRef = useRef(false);

  const [status, setStatus] = useState('STANDALONE / MODEM IDLE');
  const [input, setInput] = useState('');
  const [autoRedial, setAutoRedial] = useState(true);
  const [worldNow, setWorldNow] = useState(() => clock.now());
  const [activeCall, setActiveCall] = useState<ActiveCall | null>(null);
  const [localTestConnected, setLocalTestConnected] = useState(false);
  const [completedCost, setCompletedCost] = useState(0);
  const [lastHandshake, setLastHandshake] = useState<HandshakeRun | null>(null);
  const [audioStatus, setAudioStatus] = useState('READY');
  const [commSettings, setCommSettings] = useState<CommSettings>(loadCommSettings);

  useEffect(() => {
    (window as BootWindow).__zuttoBootOk?.();
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(commSettings));
    } catch {
      // Persistence is optional.
    }
  }, [commSettings]);

  useEffect(() => {
    terminal.write('ZUTTO COMMUNICATION TERMINAL for PC-98\r\n');
    terminal.write(`1996 MODE / COM1 / ${commSettings.lineBaud}bps / ${commSettings.dataBits}${commSettings.parity === 'none' ? 'N' : commSettings.parity === 'even' ? 'E' : 'O'}${commSettings.stopBits}\r\n`);
    terminal.write('STANDALONE MODE / LINE CLOSED\r\n\r\n');
    terminal.write('AT\r\nOK\r\n');

    const modem = new VirtualModem(terminal, wsURL, standaloneLine ? {
      offlineBusyExtraMs: 0,
      audio: {
        dial: playStandaloneBusySequence,
        busy: () => 0,
        handshake: playHandshake,
      },
    } : {});

    modem.onStatus = setStatus;
    modem.onCallState = call => {
      const now = clock.now();
      setWorldNow(now);
      setActiveCall(previous => {
        if (previous) {
          setCompletedCost(value => value + tariff.chargeYen(previous.phone, previous.connectedAt, now));
        }
        return call ? { phone: call.phone, connectedAt: now } : null;
      });
    };
    modem.setAutoRedial(autoRedial);
    modemRef.current = modem;

    const localStation = new LocalTestStation(terminal, {
      audio: {
        dial: playDialSequence,
        handshake: playHandshake,
      },
    });
    localStation.onStatus = setStatus;
    localStation.onConnectionChange = connected => setLocalTestConnected(connected);
    localStationRef.current = localStation;

    return () => {
      modem.onStatus = undefined;
      modem.onCallState = undefined;
      modem.dispose();
      modemRef.current = null;
      localStation.onStatus = undefined;
      localStation.onConnectionChange = undefined;
      localStation.dispose();
      localStationRef.current = null;
    };
    // Settings currently affect the local UI/banner; the local test station
    // receives the latest snapshot at dial time.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clock, tariff, terminal]);

  useEffect(() => {
    modemRef.current?.setAutoRedial(autoRedial);
  }, [autoRedial]);

  useEffect(() => {
    const id = window.setInterval(() => setWorldNow(clock.now()), 1000);
    return () => window.clearInterval(id);
  }, [clock]);

  function syncTerminalInput(next: string) {
    const previousChars = Array.from(echoedInputRef.current);
    const nextChars = Array.from(next);
    let common = 0;

    while (
      common < previousChars.length
      && common < nextChars.length
      && previousChars[common] === nextChars[common]
    ) {
      common++;
    }

    for (let i = previousChars.length; i > common; i--) terminal.backspace();
    if (common < nextChars.length) terminal.write(nextChars.slice(common).join(''));
    echoedInputRef.current = next;
  }

  function change(e: React.ChangeEvent<HTMLInputElement>) {
    const next = e.currentTarget.value;
    syncTerminalInput(next);
    setInput(next);
  }

  function compositionStart() {
    composingRef.current = true;
  }

  function compositionEnd(e: React.CompositionEvent<HTMLInputElement>) {
    composingRef.current = false;
    const next = e.currentTarget.value;
    syncTerminalInput(next);
    setInput(next);
    suppressEnterRef.current = true;
    window.setTimeout(() => { suppressEnterRef.current = false; }, 0);
  }

  function routeCommand(raw: string) {
    const upper = raw.trim().toUpperCase();
    const station = localStationRef.current;

    if (station?.isConnected()) {
      station.submitLine(raw);
      return;
    }

    if (station?.isDialing()) {
      if (upper === 'ATH') station.hangup(true);
      return;
    }

    let localMode: DialMode | null = null;
    if (upper === `ATDT${LOCAL_TEST_NUMBER}`) localMode = 'tone';
    else if (upper === `ATDP${LOCAL_TEST_NUMBER}`) localMode = 'pulse';
    else if (upper === `ATD${LOCAL_TEST_NUMBER}`) localMode = commSettings.defaultDialMode;

    if (localMode) {
      lastDialWasLocalRef.current = true;
      lastLocalDialModeRef.current = localMode;
      station?.dial(localMode, commSettings);
      return;
    }

    if ((upper === 'ATDL' || upper === 'A/') && lastDialWasLocalRef.current) {
      station?.dial(lastLocalDialModeRef.current, commSettings);
      return;
    }

    if (upper.startsWith('ATDT') || upper.startsWith('ATDP') || /^ATD\d/.test(upper)) {
      lastDialWasLocalRef.current = false;
    }
    modemRef.current?.submitLine(raw);
  }

  function keyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    const native = e.nativeEvent as KeyboardEvent;

    if (e.key === 'Enter') {
      if (composingRef.current || native.isComposing || suppressEnterRef.current) {
        e.preventDefault();
        return;
      }
      terminal.write('\r\n');
      routeCommand(input);
      setInput('');
      echoedInputRef.current = '';
      e.preventDefault();
    }
  }

  function audition(baud: number) {
    try {
      setAudioStatus(`SYNTHESIZING ${baud}bps...`);
      const run = playHandshake(baud);
      setLastHandshake(run);
      setAudioStatus(`PLAYING ${baud}bps`);
    } catch (error) {
      setAudioStatus(`AUDIO ERROR: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  function setting<K extends keyof CommSettings>(key: K, value: CommSettings[K]) {
    setCommSettings(current => ({ ...current, [key]: value }));
  }

  const runningCost = activeCall
    ? tariff.chargeYen(activeCall.phone, activeCall.connectedAt, worldNow)
    : 0;
  const cost = completedCost + runningCost;
  const teleho = tariff.isTelehodaiWindow(worldNow);
  const registeredCall = activeCall && tariff.isTelehodaiCall(activeCall.phone, worldNow);
  const framing = `${commSettings.dataBits}${commSettings.parity === 'none' ? 'N' : commSettings.parity === 'even' ? 'E' : 'O'}${commSettings.stopBits}`;

  return (
    <main className="shell">
      <header className="titlebar">
        <span>ZUTTO COMMUNICATION TERMINAL Ver 0.01</span>
        <span>PC-9821 / 1996</span>
      </header>

      <section className="screen-wrap" onClick={() => document.getElementById('kbd')?.focus()}>
        <TerminalCanvas terminal={terminal} />
        <input
          id="kbd"
          className="keyboard-capture"
          value={input}
          onChange={change}
          onKeyDown={keyDown}
          onCompositionStart={compositionStart}
          onCompositionEnd={compositionEnd}
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
      </section>

      <footer className="statusbar">
        <span>{status}</span>
        <span>{localTestConnected ? 'CALL LOCAL TEST / ¥0' : `CALL ¥${cost}`}</span>
        <span>{localTestConnected ? 'LOCAL LOOP' : registeredCall ? 'TELEHODAI FIXED RATE' : teleho ? 'TELEHODAI TIME' : 'NORMAL TOLL'}</span>
        <label>
          <input type="checkbox" checked={autoRedial} onChange={e => setAutoRedial(e.target.checked)} disabled={localTestConnected} /> AUTO REDIAL
        </label>
      </footer>

      <aside className="quick-help">
        <strong>Stand-alone:</strong> no server connection until dialing. <code>ATDT0451234567</code> / <code>ATDP...</code> pulse / <code>ATDL</code> last number / <code>A/</code> redial / <code>ATH</code> hangup.<br />
        <strong>Local test station:</strong> <code>ATDT{LOCAL_TEST_NUMBER}</code> — always answers locally with the current communication settings.

        {!activeCall && !localTestConnected && (
          <>
            <details className="comm-panel">
              <summary>COMM SETTINGS / 通信設定</summary>
              <div className="settings-summary">
                LINE {commSettings.lineBaud} / DTE {commSettings.dteBaud} / {framing} / {commSettings.flowControl.toUpperCase()} / {commSettings.characterCode.toUpperCase()}
              </div>

              <div className="settings-group-title">MODEM / LINE</div>
              <div className="settings-grid">
                <label>MAX LINE SPEED
                  <select value={commSettings.lineBaud} onChange={e => setting('lineBaud', Number(e.target.value) as CommSettings['lineBaud'])}>
                    <option value={2400}>2400 bps / V.22bis</option>
                    <option value={9600}>9600 bps / V.32</option>
                    <option value={14400}>14400 bps / V.32bis</option>
                    <option value={28800}>28800 bps / V.34</option>
                  </select>
                </label>
                <label>DTE SPEED
                  <select value={commSettings.dteBaud} onChange={e => setting('dteBaud', Number(e.target.value) as CommSettings['dteBaud'])}>
                    {[9600, 19200, 38400, 57600, 115200].map(v => <option key={v} value={v}>{v} bps</option>)}
                  </select>
                </label>
                <label>DEFAULT DIAL
                  <select value={commSettings.defaultDialMode} onChange={e => setting('defaultDialMode', e.target.value as CommSettings['defaultDialMode'])}>
                    <option value="tone">TONE / DTMF</option>
                    <option value="pulse">PULSE / 10pps</option>
                  </select>
                </label>
                <label>ERROR CORRECTION
                  <select value={commSettings.errorCorrection} onChange={e => setting('errorCorrection', e.target.value as CommSettings['errorCorrection'])}>
                    <option value="auto">AUTO / V.42-MNP</option>
                    <option value="v42">V.42</option>
                    <option value="mnp4">MNP4</option>
                    <option value="off">OFF</option>
                  </select>
                </label>
                <label>COMPRESSION
                  <select value={commSettings.compression} onChange={e => setting('compression', e.target.value as CommSettings['compression'])}>
                    <option value="auto">AUTO</option>
                    <option value="v42bis">V.42bis</option>
                    <option value="mnp5">MNP5</option>
                    <option value="off">OFF</option>
                  </select>
                </label>
              </div>

              <div className="settings-group-title">SERIAL / TERMINAL</div>
              <div className="settings-grid">
                <label>DATA BITS
                  <select value={commSettings.dataBits} onChange={e => setting('dataBits', Number(e.target.value) as CommSettings['dataBits'])}>
                    <option value={8}>8 bit</option>
                    <option value={7}>7 bit</option>
                  </select>
                </label>
                <label>PARITY
                  <select value={commSettings.parity} onChange={e => setting('parity', e.target.value as CommSettings['parity'])}>
                    <option value="none">NONE</option>
                    <option value="even">EVEN</option>
                    <option value="odd">ODD</option>
                  </select>
                </label>
                <label>STOP BITS
                  <select value={commSettings.stopBits} onChange={e => setting('stopBits', Number(e.target.value) as CommSettings['stopBits'])}>
                    <option value={1}>1</option>
                    <option value={2}>2</option>
                  </select>
                </label>
                <label>FLOW CONTROL
                  <select value={commSettings.flowControl} onChange={e => setting('flowControl', e.target.value as CommSettings['flowControl'])}>
                    <option value="rtscts">RTS/CTS</option>
                    <option value="xonxoff">XON/XOFF</option>
                    <option value="none">NONE</option>
                  </select>
                </label>
                <label>CHARACTER CODE
                  <select value={commSettings.characterCode} onChange={e => setting('characterCode', e.target.value as CommSettings['characterCode'])}>
                    <option value="shift-jis">SHIFT-JIS</option>
                    <option value="jis">JIS</option>
                    <option value="ascii">ASCII</option>
                  </select>
                </label>
                <label>TERMINAL
                  <select value={commSettings.terminal} onChange={e => setting('terminal', e.target.value as CommSettings['terminal'])}>
                    <option value="ansi">ANSI</option>
                    <option value="vt100">VT100</option>
                    <option value="plain">PLAIN</option>
                  </select>
                </label>
                <label className="settings-check">
                  <input type="checkbox" checked={commSettings.localEcho} onChange={e => setting('localEcho', e.target.checked)} /> LOCAL ECHO
                </label>
              </div>
              <div className="settings-footnote">
                設定はこのブラウザに保存されます。ローカル試験局 <code>{LOCAL_TEST_NUMBER}</code> は全項目を受け入れ、設定確認・文字表示・ANSI・速度・エコー試験ができます。
              </div>
            </details>

            <details className="debug-panel">
              <summary>DEBUG / MODEM AUDIO</summary>
              <div className="debug-copy">
                通信せず、モデムのハンドシェイク合成だけを確認します。実行ごとに回線・検出待ち・スピーカー特性が少し変化します。
              </div>
              <div className="audition-row">
                {([
                  { label: 'V.22bis 2400', baud: 2400 },
                  { label: 'V.32 9600', baud: 9600 },
                  { label: 'V.32bis 14400', baud: 14400 },
                  { label: 'V.34 28800', baud: 28800 },
                ] as const).map(({ label, baud }) => (
                  <button key={baud} className="audition-btn" onClick={() => audition(baud)}>
                    {label}{commSettings.lineBaud === baud ? ' *' : ''}
                  </button>
                ))}
              </div>
              <div className="audition-meta">AUDIO: {audioStatus}</div>
              {lastHandshake && (
                <div className="audition-meta">
                  RUN {lastHandshake.seed} / {lastHandshake.baud}bps / {lastHandshake.duration.toFixed(2)}s / DETECT ±{lastHandshake.responseJitterMs}ms / SPKR {lastHandshake.speakerResonanceHz}Hz / LINE {lastHandshake.lineLevelDb >= 0 ? '+' : ''}{lastHandshake.lineLevelDb.toFixed(1)}dB
                </div>
              )}
            </details>
          </>
        )}
      </aside>
    </main>
  );
}
