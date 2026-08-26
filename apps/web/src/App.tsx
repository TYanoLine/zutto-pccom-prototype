import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { pseudoTariffTable } from './billing/pseudoTariffs';
import { Japan1996WorldClock } from './time/WorldClock';
import './styles.css';

const wsURL = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws';
const worldDate = import.meta.env.VITE_WORLD_DATE ?? '1996-08-26';
const telehodaiNumbers = (import.meta.env.VITE_TELEHODAI_NUMBERS ?? '0451234567,0450000001')
  .split(',')
  .map((phone: string) => phone.trim())
  .filter(Boolean);

type ActiveCall = { phone: string; connectedAt: Date };

export default function App() {
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
      </aside>
    </main>
  );
}
