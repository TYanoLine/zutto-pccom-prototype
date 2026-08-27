import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { pseudoTariffTable } from './billing/pseudoTariffs';
import { Japan1996WorldClock } from './time/WorldClock';
import { playHandshake } from './audio/modemAudio';
import './styles.css';

const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const isLocalHost = typeof window !== 'undefined'
  && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const wsURL = configuredWsURL || (isLocalHost ? 'ws://localhost:8080/ws' : '');
const auditionOnly = wsURL.length === 0;

const configuredWorldDate = (import.meta.env.VITE_WORLD_DATE as string | undefined)?.trim() ?? '';
const worldDate = /^\d{4}-\d{2}-\d{2}$/.test(configuredWorldDate)
  ? configuredWorldDate
  : '1996-08-26';

const configuredTelehodaiNumbers = (import.meta.env.VITE_TELEHODAI_NUMBERS as string | undefined)?.trim();
const telehodaiNumbers = (configuredTelehodaiNumbers || '0451234567,0450000001')
  .split(',')
  .map((phone: string) => phone.trim())
  .filter(Boolean);

type ActiveCall = { phone: string; connectedAt: Date };
type BootWindow = Window & { __zuttoBootOk?: () => void };
type HandshakeRun = ReturnType<typeof playHandshake>;

function markBootOk() {
  (window as BootWindow).__zuttoBootOk?.();
}

function AuditionApp() {
  const [lastHandshake, setLastHandshake] = useState<HandshakeRun | null>(null);
  const [status, setStatus] = useState('READY');

  useEffect(() => {
    markBootOk();
  }, []);

  function audition(baud: number) {
    try {
      setStatus(`SYNTHESIZING ${baud}bps...`);
      const run = playHandshake(baud);
      setLastHandshake(run);
      setStatus(`PLAYING ${baud}bps`);
    } catch (error) {
      setStatus(`AUDIO ERROR: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  return (
    <main style={{ minHeight: '100vh', padding: 18, background: '#181818', color: '#ddd', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' }}>
      <section style={{ width: 'min(760px, 100%)', margin: '0 auto' }}>
        <header style={{ padding: '10px 12px', background: '#bdbdbd', color: '#111', border: '2px solid #eee', borderRightColor: '#555', borderBottomColor: '#555' }}>
          <strong>ZUTTO MODEM HANDSHAKE LAB</strong>
          <div style={{ fontSize: 12, marginTop: 4 }}>procedural PCM synthesis / analogue variation on every run</div>
        </header>

        <div style={{ marginTop: 16, padding: 14, border: '1px solid #555', background: '#080808' }}>
          <div style={{ marginBottom: 12, color: '#aaa', lineHeight: 1.6 }}>
            同じ規格でも毎回、検出待ち・回線レベル・スピーカー共振・クロック誤差・残留キャリアが少しずつ変化します。
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 10 }}>
            {([
              { label: 'V.22bis', baud: 2400 },
              { label: 'V.32', baud: 9600 },
              { label: 'V.32bis', baud: 14400 },
              { label: 'V.34', baud: 28800 },
            ] as const).map(({ label, baud }) => (
              <button
                key={baud}
                onClick={() => audition(baud)}
                style={{
                  minHeight: 64,
                  padding: '10px 8px',
                  font: 'inherit',
                  cursor: 'pointer',
                  color: '#ddd',
                  background: '#262626',
                  border: '1px solid #666',
                  borderRadius: 3,
                }}
              >
                <strong>{label}</strong><br />
                <span style={{ fontSize: 13 }}>{baud.toLocaleString()} bps</span>
              </button>
            ))}
          </div>

          <div style={{ marginTop: 14, padding: '9px 10px', background: '#101010', border: '1px solid #333', fontSize: 12, lineHeight: 1.6 }}>
            STATUS: {status}
          </div>

          {lastHandshake && (
            <div style={{ marginTop: 10, padding: '9px 10px', background: '#101010', border: '1px solid #333', fontSize: 12, lineHeight: 1.7, overflowWrap: 'anywhere' }}>
              RUN {lastHandshake.seed}<br />
              {lastHandshake.baud}bps / {lastHandshake.duration.toFixed(2)}s<br />
              DETECT ±{lastHandshake.responseJitterMs}ms<br />
              SPKR {lastHandshake.speakerResonanceHz}Hz<br />
              LINE {lastHandshake.lineLevelDb >= 0 ? '+' : ''}{lastHandshake.lineLevelDb.toFixed(1)}dB
            </div>
          )}
        </div>
      </section>
    </main>
  );
}

function TerminalApp() {
  const terminal = useMemo(() => new TerminalCore(), []);
  const clock = useMemo(() => new Japan1996WorldClock(worldDate), []);
  const tariff = useMemo(() => new PseudoTariffService(pseudoTariffTable, telehodaiNumbers), []);
  const modemRef = useRef<VirtualModem | null>(null);
  const [status, setStatus] = useState('INITIALIZING');
  const [input, setInput] = useState('');
  const [autoRedial, setAutoRedial] = useState(true);
  const [worldNow, setWorldNow] = useState(() => clock.now());
  const [activeCall, setActiveCall] = useState<ActiveCall | null>(null);
  const [completedCost, setCompletedCost] = useState(0);
  const [lastHandshake, setLastHandshake] = useState<HandshakeRun | null>(null);

  useEffect(() => {
    markBootOk();
  }, []);

  useEffect(() => {
    terminal.write('ZUTTO COMMUNICATION TERMINAL for PC-98\r\n');
    terminal.write('1996 MODE / COM1 / 14400bps / 8N1\r\n\r\n');
    terminal.write('AT\r\nOK\r\n');

    const modem = new VirtualModem(terminal, wsURL);
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
    return () => {
      modem.onStatus = undefined;
      modem.onCallState = undefined;
      modem.dispose();
      modemRef.current = null;
    };
  }, [clock, tariff, terminal]);

  useEffect(() => { modemRef.current?.setAutoRedial(autoRedial); }, [autoRedial]);

  useEffect(() => {
    const id = window.setInterval(() => setWorldNow(clock.now()), 1000);
    return () => window.clearInterval(id);
  }, [clock]);

  function keyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      terminal.write('\r\n');
      modemRef.current?.submitLine(input);
      setInput('');
      e.preventDefault();
      return;
    }
    if (e.key === 'Backspace') terminal.backspace();
  }

  function change(e: React.ChangeEvent<HTMLInputElement>) {
    const next = e.target.value;
    if (next.length > input.length) terminal.write(next.slice(input.length));
    setInput(next);
  }

  const runningCost = activeCall
    ? tariff.chargeYen(activeCall.phone, activeCall.connectedAt, worldNow)
    : 0;
  const cost = completedCost + runningCost;
  const teleho = tariff.isTelehodaiWindow(worldNow);
  const registeredCall = activeCall && tariff.isTelehodaiCall(activeCall.phone, worldNow);

  return (
    <main className="shell">
      <header className="titlebar">
        <span>ZUTTO COMMUNICATION TERMINAL Ver 0.01</span>
        <span>PC-9821 / 1996</span>
      </header>

      <section className="screen-wrap" onClick={() => document.getElementById('kbd')?.focus()}>
        <TerminalCanvas terminal={terminal} />
        <input id="kbd" className="keyboard-capture" value={input} onChange={change} onKeyDown={keyDown} autoFocus />
      </section>

      <footer className="statusbar">
        <span>{status}</span>
        <span>CALL ¥{cost}</span>
        <span>{registeredCall ? 'TELEHODAI FIXED RATE' : teleho ? 'TELEHODAI TIME' : 'NORMAL TOLL'}</span>
        <label><input type="checkbox" checked={autoRedial} onChange={e => setAutoRedial(e.target.checked)} /> AUTO REDIAL</label>
      </footer>

      <aside className="quick-help">
        <strong>Prototype:</strong> <code>ATDT0451234567</code> / <code>A/</code> redial / <code>ATH</code> hangup. 接続後は <code>H</code>, <code>B</code>, <code>W</code>, <code>U</code>, <code>G</code>。
        <div className="audition-row">
          <span>ハンドシェイク試聴:</span>
          {([
            { label: 'V.22bis 2400', baud: 2400 },
            { label: 'V.32 9600', baud: 9600 },
            { label: 'V.32bis 14400', baud: 14400 },
            { label: 'V.34 28800', baud: 28800 },
          ] as const).map(({ label, baud }) => (
            <button
              key={baud}
              className="audition-btn"
              onClick={() => {
                try {
                  setLastHandshake(playHandshake(baud));
                } catch (error) {
                  setStatus(`AUDIO ERROR: ${error instanceof Error ? error.message : String(error)}`);
                }
              }}
            >
              {label}
            </button>
          ))}
        </div>
        {lastHandshake && (
          <div className="audition-meta">
            RUN {lastHandshake.seed} / {lastHandshake.baud}bps / {lastHandshake.duration.toFixed(2)}s / DETECT ±{lastHandshake.responseJitterMs}ms / SPKR {lastHandshake.speakerResonanceHz}Hz / LINE {lastHandshake.lineLevelDb >= 0 ? '+' : ''}{lastHandshake.lineLevelDb.toFixed(1)}dB
          </div>
        )}
      </aside>
    </main>
  );
}

export default function App() {
  return auditionOnly ? <AuditionApp /> : <TerminalApp />;
}
