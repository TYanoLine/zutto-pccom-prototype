import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas, type TerminalCanvasHandle } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import { LocalTestStation, LOCAL_TEST_NUMBER } from './modem/LocalTestStation';
import { fetchWorldCenters, loadCenters } from './modem/CenterDirectory';
import type { RegisteredCenter } from './modem/CenterDirectory';
import { TerminalCenterDirectory } from './modem/TerminalCenterDirectory';
import { DEFAULT_COMM_SETTINGS, normalizeCommSettings } from './modem/CommSettings';
import type { CommSettings } from './modem/CommSettings';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { pseudoTariffTable } from './billing/pseudoTariffs';
import { Japan1996WorldClock } from './time/WorldClock';
import { playHandshake } from './audio/modemAudio';
import { playDialSequence, playStandaloneBusySequence } from './audio/dialLineAudio';
import type { DialMode } from './audio/dialLineAudio';
import './styles.css';

const APP_VERSION = '0.19';
const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const isLocalHost = typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const wsURL = configuredWsURL || (isLocalHost ? 'ws://localhost:8080/ws' : '');
const standaloneLine = wsURL.length === 0;
const configuredWorldDate = (import.meta.env.VITE_WORLD_DATE as string | undefined)?.trim() ?? '';
const worldDate = /^\d{4}-\d{2}-\d{2}$/.test(configuredWorldDate) ? configuredWorldDate : '1996-08-26';
const configuredTelehodaiNumbers = (import.meta.env.VITE_TELEHODAI_NUMBERS as string | undefined)?.trim();
const telehodaiNumbers = (configuredTelehodaiNumbers || '0451234567,0450000001').split(',').map((phone: string) => phone.trim()).filter(Boolean);
const SETTINGS_KEY = 'zutto.commSettings.v1';

type ActiveCall = { phone: string; connectedAt: Date };
type BootWindow = Window & { __zuttoBootOk?: () => void };
type HandshakeRun = ReturnType<typeof playHandshake>;
type DirectoryLoadState = 'loading' | 'ready' | 'error';
type ScreenMode = 'main' | 'terminal';

function loadCommSettings(): CommSettings {
  if (typeof window === 'undefined') return { ...DEFAULT_COMM_SETTINGS };
  try { const raw = window.localStorage.getItem(SETTINGS_KEY); return raw ? normalizeCommSettings(JSON.parse(raw) as Partial<CommSettings>) : { ...DEFAULT_COMM_SETTINGS }; }
  catch { return { ...DEFAULT_COMM_SETTINGS }; }
}
function formatElapsed(startedAt: Date | null, now: Date) {
  if (!startedAt) return '00:00:00';
  const totalSeconds = Math.max(0, Math.floor((now.getTime() - startedAt.getTime()) / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return [hours, minutes, seconds].map(value => String(value).padStart(2, '0')).join(':');
}

export default function App() {
  const terminal = useMemo(() => new TerminalCore(), []);
  const terminalCanvasRef = useRef<TerminalCanvasHandle | null>(null);
  const clock = useMemo(() => new Japan1996WorldClock(worldDate), []);
  const tariff = useMemo(() => new PseudoTariffService(pseudoTariffTable, telehodaiNumbers), []);
  const modemRef = useRef<VirtualModem | null>(null);
  const localStationRef = useRef<LocalTestStation | null>(null);
  const directoryRef = useRef<TerminalCenterDirectory | null>(null);
  const centersRef = useRef<RegisteredCenter[]>(loadCenters());
  const directoryLoadStateRef = useRef<DirectoryLoadState>('loading');
  const openDirectoryWhenReadyRef = useRef(false);
  const screenModeRef = useRef<ScreenMode>('main');
  const lastDialWasLocalRef = useRef(false);
  const lastLocalDialModeRef = useRef<DialMode>('tone');
  const echoedInputRef = useRef('');
  const composingRef = useRef(false);
  const suppressEnterRef = useRef(false);
  const [status, setStatus] = useState('STANDALONE / MODEM IDLE');
  const [input, setInput] = useState('');
  const [autoRedial, setAutoRedial] = useState(true);
  const [worldNow, setWorldNow] = useState(() => clock.now());
  const [activeCall, setActiveCall] = useState<ActiveCall | null>(null);
  const [localTestConnected, setLocalTestConnected] = useState(false);
  const [localTestConnectedAt, setLocalTestConnectedAt] = useState<Date | null>(null);
  const [completedCost, setCompletedCost] = useState(0);
  const [lastHandshake, setLastHandshake] = useState<HandshakeRun | null>(null);
  const [audioStatus, setAudioStatus] = useState('READY');
  const [commSettings, setCommSettings] = useState<CommSettings>(loadCommSettings);
  const [directoryCount, setDirectoryCount] = useState(0);
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [directoryStatus, setDirectoryStatus] = useState('センター情報読込中...');
  const [commandFocused, setCommandFocused] = useState(false);

  useEffect(() => { (window as BootWindow).__zuttoBootOk?.(); }, []);
  useEffect(() => { try { window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(commSettings)); } catch { /* optional */ } }, [commSettings]);
  useEffect(() => {
    fetchWorldCenters(wsURL).then(centers => {
      if (!centers.length) throw new Error('empty center directory');
      centersRef.current = centers;
      directoryLoadStateRef.current = 'ready';
      setDirectoryCount(centers.length);
      setDirectoryStatus(`センター情報読込完了 (${centers.length}局)`);
      if (openDirectoryWhenReadyRef.current) {
        openDirectoryWhenReadyRef.current = false;
        directoryRef.current?.show();
        setDirectoryOpen(true);
      }
    }).catch(error => {
      directoryLoadStateRef.current = 'error';
      openDirectoryWhenReadyRef.current = false;
      const message = error instanceof Error ? error.message : String(error);
      setDirectoryStatus(`CENTER API ERROR: ${message}`);
    });
  }, []);

  useEffect(() => {
    showMainMenu();
    const modem = new VirtualModem(terminal, wsURL, standaloneLine ? { offlineBusyExtraMs: 0, audio: { dial: playStandaloneBusySequence, busy: () => 0, handshake: playHandshake } } : {});
    modem.onStatus = setStatus;
    modem.onCallState = call => { const now = clock.now(); setWorldNow(now); setActiveCall(previous => { if (previous) setCompletedCost(value => value + tariff.chargeYen(previous.phone, previous.connectedAt, now)); return call ? { phone: call.phone, connectedAt: now } : null; }); };
    modem.setAutoRedial(autoRedial); modemRef.current = modem;
    const localStation = new LocalTestStation(terminal, { audio: { dial: playDialSequence, handshake: playHandshake } });
    localStation.onStatus = setStatus; localStation.onConnectionChange = connected => {
      setLocalTestConnected(connected);
      setLocalTestConnectedAt(connected ? clock.now() : null);
    }; localStationRef.current = localStation;
    directoryRef.current = new TerminalCenterDirectory(terminal, () => centersRef.current, center => { setDirectoryOpen(false); dialCenter(center); }, () => { setDirectoryOpen(false); showMainMenu(); });
    return () => { modem.onStatus = undefined; modem.onCallState = undefined; modem.dispose(); modemRef.current = null; localStation.onStatus = undefined; localStation.onConnectionChange = undefined; localStation.dispose(); localStationRef.current = null; directoryRef.current = null; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clock, tariff, terminal]);
  useEffect(() => { modemRef.current?.setAutoRedial(autoRedial); }, [autoRedial]);
  useEffect(() => { const id = window.setInterval(() => setWorldNow(clock.now()), 1000); return () => window.clearInterval(id); }, [clock]);

  function resetInput() { setInput(''); echoedInputRef.current = ''; }
  function showMainMenu() {
    screenModeRef.current = 'main';
    terminal.clear(); terminal.write(`\x1b[37;44m ずっとパソコン通信 Ver ${APP_VERSION}                         Copyright (C) 1996 ZUTTO \x1b[0m\r\n\r\n`);
    terminal.write('                     \x1b[30;46m　メイン・メニュー　\x1b[0m\r\n\r\n');
    terminal.write('                     1　センターの呼び出し\r\n                     2　通信パラメータの設定\r\n                     3　ターミナル・モード\r\n                     4　ノート・パッド\r\n                     5　ディスク・ユーティリティ\r\n                     6　MS-DOS コマンドへ\r\n                     7　終　了\r\n\r\n');
    const countText = directoryLoadStateRef.current === 'ready' ? `${centersRef.current.length}局` : '---';
    terminal.write(` 登録センター: ${countText}　　　　　　　　　使用する電話回線: ダイアル(10)\r\n 番号を選択してください > `); resetInput();
  }
  function showTerminalMode() {
    screenModeRef.current = 'terminal';
    terminal.clear();
    terminal.write(`\x1b[37;44m ずっとパソコン通信　ターミナル・モード                         Ver ${APP_VERSION} \x1b[0m\r\n\r\n`);
    terminal.write('                     \x1b[30;46m　ターミナル・モード　\x1b[0m\r\n\r\n');
    terminal.write(` 通信条件: ${commSettings.dteBaud}bps / ${commSettings.dataBits}${commSettings.parity === 'none' ? 'N' : commSettings.parity === 'even' ? 'E' : 'O'}${commSettings.stopBits} / ${commSettings.flowControl.toUpperCase()}\r\n`);
    terminal.write(' モデムコマンドを直接入力できます。\r\n');
    terminal.write(' 例: ATDT0450000196\r\n');
    terminal.write('     ATDL          （直前の番号へ再発信）\r\n');
    terminal.write('     ATH           （切断）\r\n\r\n');
    terminal.write(' オフライン時は ESC キーでメイン・メニューへ戻ります。\r\n');
    terminal.write('------------------------------------------------------------\r\n');
    resetInput();
  }
  function showDirectoryLoading() {
    terminal.clear();
    terminal.write(`\x1b[37;44m ずっとパソコン通信　センター情報読み込み                         Ver ${APP_VERSION} \x1b[0m\r\n\r\n`);
    terminal.write('                     \x1b[30;46m　センター情報の読み込み　\x1b[0m\r\n\r\n');
    terminal.write(' センター情報をディスクから読み込んでいます。\r\n');
    terminal.write(' しばらくお待ちください...\r\n\r\n');
    terminal.write(' ※ 読み込みが終わると、自動的にセンター・リストを表示します。');
  }
  function showDirectoryError() {
    terminal.clear();
    terminal.write(`\x1b[37;44m ずっとパソコン通信　センター情報読み込み                         Ver ${APP_VERSION} \x1b[0m\r\n\r\n`);
    terminal.write('\r\n センター情報を読み込むことができませんでした。\r\n');
    terminal.write(' ESCキーでメイン・メニューに戻ってください。');
  }
  function syncTerminalInput(next: string) { const p = Array.from(echoedInputRef.current), n = Array.from(next); let c = 0; while (c < p.length && c < n.length && p[c] === n[c]) c++; for (let i = p.length; i > c; i--) terminal.backspace(); if (c < n.length) terminal.write(n.slice(c).join('')); echoedInputRef.current = next; }
  function followLiveInput() { terminalCanvasRef.current?.returnToLive(); }
  function change(e: React.ChangeEvent<HTMLInputElement>) { if (directoryRef.current?.isOpen()) return; followLiveInput(); const next = e.currentTarget.value; syncTerminalInput(next); setInput(next); }
  function compositionStart() { composingRef.current = true; followLiveInput(); }
  function compositionEnd(e: React.CompositionEvent<HTMLInputElement>) { composingRef.current = false; followLiveInput(); const next = e.currentTarget.value; syncTerminalInput(next); setInput(next); suppressEnterRef.current = true; window.setTimeout(() => { suppressEnterRef.current = false; }, 0); }
  function routeCommand(raw: string) { const upper = raw.trim().toUpperCase(), station = localStationRef.current; if (station?.isConnected()) { station.submitLine(raw); return; } if (station?.isDialing()) { if (upper === 'ATH') station.hangup(true); return; } let localMode: DialMode | null = null; if (upper === `ATDT${LOCAL_TEST_NUMBER}`) localMode = 'tone'; else if (upper === `ATDP${LOCAL_TEST_NUMBER}`) localMode = 'pulse'; else if (upper === `ATD${LOCAL_TEST_NUMBER}`) localMode = commSettings.defaultDialMode; if (localMode) { lastDialWasLocalRef.current = true; lastLocalDialModeRef.current = localMode; station?.dial(localMode, commSettings); return; } if ((upper === 'ATDL' || upper === 'A/') && lastDialWasLocalRef.current) { station?.dial(lastLocalDialModeRef.current, commSettings); return; } if (upper.startsWith('ATDT') || upper.startsWith('ATDP') || /^ATD\d/.test(upper)) lastDialWasLocalRef.current = false; modemRef.current?.submitLine(raw); }
  function keyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (directoryRef.current?.isOpen()) { directoryRef.current.handleKey(e.key); e.preventDefault(); return; }
    const native = e.nativeEvent as KeyboardEvent;
    if (e.key === 'Escape' && !activeCall && !localTestConnected) {
      if (openDirectoryWhenReadyRef.current) openDirectoryWhenReadyRef.current = false;
      if (screenModeRef.current === 'terminal' || openDirectoryWhenReadyRef.current === false) showMainMenu();
      e.preventDefault(); return;
    }
    if (e.key !== 'Enter') return;
    if (composingRef.current || native.isComposing || suppressEnterRef.current) { e.preventDefault(); return; }
    e.preventDefault();
    submitInput();
  }
  function submitInput() {
    // A soft Enter and a physical Enter share the same IME guard and routing.
    followLiveInput();
    if (composingRef.current || suppressEnterRef.current) return;
    if (directoryRef.current?.isOpen()) { softKey('Enter'); return; }
    terminal.write('\r\n');
    const raw = input;
    const command = raw.trim();
    resetInput();
    if (!activeCall && !localTestConnected && screenModeRef.current === 'main') {
      if (command === '1') {
        if (directoryLoadStateRef.current === 'ready') { directoryRef.current?.show(); setDirectoryOpen(true); }
        else if (directoryLoadStateRef.current === 'loading') { openDirectoryWhenReadyRef.current = true; showDirectoryLoading(); }
        else showDirectoryError();
      } else if (command === '3') {
        showTerminalMode();
      } else if (command !== '') {
        terminal.write('\r\n 現在この項目は未実装です。\r\n\r\n');
        showMainMenu();
      } else {
        showMainMenu();
      }
    } else {
      routeCommand(raw);
    }
  }
  function softKey(key: string) { directoryRef.current?.handleKey(key); }
  function dialCenter(center: RegisteredCenter) { screenModeRef.current = 'terminal'; const command = `${center.dialMode === 'pulse' ? 'ATDP' : 'ATDT'}${center.phone}`; terminal.write(`${command}\r\n`); routeCommand(command); resetInput(); }
  function audition(baud: number) { try { setAudioStatus(`SYNTHESIZING ${baud}bps...`); const run = playHandshake(baud); setLastHandshake(run); setAudioStatus(`PLAYING ${baud}bps`); } catch (error) { setAudioStatus(`AUDIO ERROR: ${error instanceof Error ? error.message : String(error)}`); } }
  function setting<K extends keyof CommSettings>(key: K, value: CommSettings[K]) { setCommSettings(current => ({ ...current, [key]: value })); }

  const runningCost = activeCall ? tariff.chargeYen(activeCall.phone, activeCall.connectedAt, worldNow) : 0, cost = completedCost + runningCost, teleho = tariff.isTelehodaiWindow(worldNow), registeredCall = activeCall && tariff.isTelehodaiCall(activeCall.phone, worldNow), framing = `${commSettings.dataBits}${commSettings.parity === 'none' ? 'N' : commSettings.parity === 'even' ? 'E' : 'O'}${commSettings.stopBits}`;
  const activeCenter = activeCall ? centersRef.current.find(center => center.phone === activeCall.phone) : undefined;
  const mobileConnectionName = localTestConnected ? 'LOCAL TEST' : activeCall ? (activeCenter?.name ?? activeCall.phone) : 'OFFLINE';
  const mobileConnectedAt = localTestConnected ? localTestConnectedAt : activeCall?.connectedAt ?? null;
  const mobileElapsed = formatElapsed(mobileConnectedAt, worldNow);
  const mobileSessionCost = localTestConnected ? 0 : runningCost;
  return <main className="shell">
    <header className="titlebar"><span>ZUTTO COMMUNICATION TERMINAL Ver {APP_VERSION}</span><span>PC-9821 / 1996</span></header>
    <div className="mobile-statusbar" role="status" aria-label="接続状態">
      <span className="mobile-statusbar__name">{mobileConnectionName}</span>
      <span className="mobile-statusbar__stats">{mobileElapsed}&nbsp;&nbsp;¥{mobileSessionCost}</span>
    </div>
    <section className="screen-wrap"><TerminalCanvas
      ref={terminalCanvasRef}
      terminal={terminal}
      keyboardActive={commandFocused}
      bottomControlsActive={directoryOpen}
      keyboardInput={{
        value: input,
        readOnly: directoryOpen,
        onChange: change,
        onKeyDown: keyDown,
        onCompositionStart: compositionStart,
        onCompositionEnd: compositionEnd,
        onFocus: () => { setCommandFocused(true); followLiveInput(); },
        onBlur: () => setCommandFocused(false),
      }}
    /></section>
    {directoryOpen && <nav className="directory-softkeys" aria-label="センターリスト操作">
      <button type="button" onClick={() => softKey('ArrowUp')}>▲<small>上</small></button><button type="button" onClick={() => softKey('ArrowDown')}>▼<small>下</small></button>
      <button type="button" onClick={() => softKey('PageUp')}>◀<small>前頁</small></button><button type="button" onClick={() => softKey('PageDown')}>▶<small>次頁</small></button>
      <button type="button" className="softkey-call" onClick={() => softKey('Enter')}>CALL<small>呼出</small></button><button type="button" onClick={() => softKey('Escape')}>ESC<small>戻る</small></button>
    </nav>}
    <footer className="statusbar"><span>{status}</span><span>{localTestConnected ? 'CALL LOCAL TEST / ¥0' : `CALL ¥${cost}`}</span><span>{localTestConnected ? 'LOCAL LOOP' : registeredCall ? 'TELEHODAI FIXED RATE' : teleho ? 'TELEHODAI TIME' : 'NORMAL TOLL'}</span><label><input type="checkbox" checked={autoRedial} onChange={e => setAutoRedial(e.target.checked)} disabled={localTestConnected} /> AUTO REDIAL</label></footer>
    <aside className="quick-help"><strong>センター:</strong> {directoryStatus}<br /><strong>センターの呼び出し:</strong> メインメニューで <code>1</code>。現在 {directoryCount || '---'}局。<br /><strong>ターミナル・モード:</strong> メインメニューで <code>3</code>。電話番号を直接指定できます。<br /><strong>Local test station:</strong> <code>ATDT{LOCAL_TEST_NUMBER}</code>
      {!activeCall && !localTestConnected && <><details className="comm-panel"><summary>COMM SETTINGS / 通信設定</summary><div className="settings-summary">LINE {commSettings.lineBaud} / DTE {commSettings.dteBaud} / {framing} / {commSettings.flowControl.toUpperCase()}</div><div className="settings-grid"><label>MAX LINE SPEED<select value={commSettings.lineBaud} onChange={e => setting('lineBaud', Number(e.target.value) as CommSettings['lineBaud'])}><option value={2400}>2400 bps</option><option value={9600}>9600 bps</option><option value={14400}>14400 bps</option><option value={28800}>28800 bps</option></select></label></div></details><details className="debug-panel"><summary>DEBUG / MODEM AUDIO</summary><div className="audition-row">{([2400, 9600, 14400, 28800] as const).map(baud => <button key={baud} className="audition-btn" onClick={() => audition(baud)}>{baud}bps</button>)}</div><div className="audition-meta">AUDIO: {audioStatus}</div>{lastHandshake && <div className="audition-meta">RUN {lastHandshake.seed} / {lastHandshake.baud}bps</div>}</details></>}
    </aside>
  </main>;
}
