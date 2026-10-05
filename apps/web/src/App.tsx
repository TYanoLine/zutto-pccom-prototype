import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas, type TerminalCanvasHandle, type TerminalScreenMode } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import type { HostCapabilities } from './modem/HostCapabilities';
import { LocalTestStation, LOCAL_TEST_NUMBER } from './modem/LocalTestStation';
import { clearLegacyDirectoryStorage, fetchDirectory, fetchWorldCenters, mergeCenters } from './modem/CenterDirectory';
import type { RegisteredCenter } from './modem/CenterDirectory';
import { TerminalCenterDirectory } from './modem/TerminalCenterDirectory';
import { DEFAULT_COMM_SETTINGS, normalizeCommSettings } from './modem/CommSettings';
import type { CommSettings } from './modem/CommSettings';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { ntt1996TariffTable } from './billing/pseudoTariffs';
import { loadCallerLocation, saveCallerLocation } from './billing/CallerLocation';
import { Japan1996WorldClock } from './time/WorldClock';
import { playHandshake } from './audio/modemAudio';
import { playDialSequence, playStandaloneBusySequence } from './audio/dialLineAudio';
import type { DialMode } from './audio/dialLineAudio';
import { BuildInfoPanel } from './build/BuildInfoPanel';
import type { BuildInfoState } from './build/BuildInfoPanel';
import { fetchServerBuildInfo, serverVersionEndpoint } from './build/ServerBuildInfo';
import type { ServerBuildInfo } from './build/ServerBuildInfo';
import { createIdleModemTelemetry } from './modem/ModemTelemetry';
import type { ModemTelemetry } from './modem/ModemTelemetry';
import { resolveMobileConnectionStatus } from './modem/MobileConnectionStatus';
import {
  loadModemStatusDisplayMode,
  ModemStatusDisplay,
  saveModemStatusDisplayMode,
} from './modem/ModemStatusDisplay';
import type { ModemStatusDisplayMode } from './modem/ModemStatusDisplay';
import './styles.css';
import { GenerationInspector } from './debug/GenerationInspector';

const APP_VERSION = '0.28';
const CLIENT_BUILD_COMMIT = (import.meta.env.VITE_BUILD_COMMIT as string | undefined) || 'unknown';
const CLIENT_BUILD_REF = (import.meta.env.VITE_BUILD_REF as string | undefined) || 'unknown';
const CLIENT_BUILD_TIME = (import.meta.env.VITE_BUILD_TIME as string | undefined) || '';
const SCREEN_MODE_KEY = 'zutto.terminalScreenMode.v1';
const WEEKDAYS = ['日', '月', '火', '水', '木', '金', '土'];
function loadTerminalScreenMode(): TerminalScreenMode {
  if (typeof window === 'undefined') return 'variable';
  try { return window.localStorage.getItem(SCREEN_MODE_KEY) === 'fixed25' ? 'fixed25' : 'variable'; }
  catch { return 'variable'; }
}
function formatWorldDate(date: Date) {
  const weekday = WEEKDAYS[date.getUTCDay()];
  return `${date.getUTCFullYear()}年${date.getUTCMonth() + 1}月${date.getUTCDate()}日（${weekday}） ${String(date.getUTCHours()).padStart(2, '0')}:${String(date.getUTCMinutes()).padStart(2, '0')}`;
}
const configuredWsURL = (import.meta.env.VITE_WS_URL as string | undefined)?.trim();
const isLocalHost = typeof window !== 'undefined' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const wsURL = configuredWsURL || (isLocalHost ? 'ws://localhost:8080/ws' : '');
const standaloneLine = wsURL.length === 0;
const configuredWorldDate = (import.meta.env.VITE_WORLD_DATE as string | undefined)?.trim() ?? '';
const worldDate = /^\d{4}-\d{2}-\d{2}$/.test(configuredWorldDate) ? configuredWorldDate : '1996-08-26';
const configuredTelehodaiNumbers = (import.meta.env.VITE_TELEHODAI_NUMBERS as string | undefined)?.trim();
const telehodaiNumbers = (configuredTelehodaiNumbers || '0920000196').split(',').map((phone: string) => phone.trim()).filter(Boolean);
const SETTINGS_KEY = 'zutto.commSettings.v1';

type ActiveCall = { phone: string; connectedAt: Date; capabilities: HostCapabilities };
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
  const [callerLocation] = useState(loadCallerLocation);
  const tariff = useMemo(() => new PseudoTariffService(ntt1996TariffTable, telehodaiNumbers, callerLocation), [callerLocation]);
  const modemRef = useRef<VirtualModem | null>(null);
  const localStationRef = useRef<LocalTestStation | null>(null);
  const directoryRef = useRef<TerminalCenterDirectory | null>(null);
  const centersRef = useRef<RegisteredCenter[]>([]);
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
  const [modemTelemetry, setModemTelemetry] = useState<ModemTelemetry>(() => createIdleModemTelemetry(loadCommSettings()));
  const [modemStatusMode, setModemStatusMode] = useState<ModemStatusDisplayMode>(loadModemStatusDisplayMode);
  const [terminalScreenMode, setTerminalScreenMode] = useState<TerminalScreenMode>(loadTerminalScreenMode);
  const [desktopMenuOpen, setDesktopMenuOpen] = useState(false);
  const [serverBuildInfo, setServerBuildInfo] = useState<ServerBuildInfo | null>(null);
  const [serverBuildState, setServerBuildState] = useState<BuildInfoState>('loading');
  const [versionRefresh, setVersionRefresh] = useState(0);
  const [directoryCount, setDirectoryCount] = useState(0);
  const [directoryOpen, setDirectoryOpen] = useState(false);
  const [directoryStatus, setDirectoryStatus] = useState('センター情報読込中...');
  const [commandFocused, setCommandFocused] = useState(false);
  const [traceOpen, setTraceOpen] = useState(false);
  const [traceRunning, setTraceRunning] = useState(false);

  useEffect(() => { (window as BootWindow).__zuttoBootOk?.(); }, []);
  useEffect(() => {
    if (!serverVersionEndpoint(wsURL)) {
      setServerBuildState('not-configured');
      return;
    }
    const controller = new AbortController();
    setServerBuildState('loading');
    // This request never blocks modem startup, directory loading, or the terminal.
    void fetchServerBuildInfo(wsURL, undefined, controller.signal).then(info => {
      if (!controller.signal.aborted) {
        setServerBuildInfo(info);
        setServerBuildState('ready');
      }
    }).catch(() => {
      if (!controller.signal.aborted) setServerBuildState('unavailable');
    });
    return () => controller.abort();
  }, [versionRefresh]);
  useEffect(() => { saveCallerLocation(callerLocation); }, [callerLocation]);
  useEffect(() => { saveModemStatusDisplayMode(modemStatusMode); }, [modemStatusMode]);
  useEffect(() => {
    try { window.localStorage.setItem(SCREEN_MODE_KEY, terminalScreenMode); } catch { /* optional preference */ }
  }, [terminalScreenMode]);
  useEffect(() => {
    try { window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(commSettings)); } catch { /* optional */ }
    modemRef.current?.setCommunicationSettings(commSettings);
    localStationRef.current?.setCommunicationSettings(commSettings);
  }, [commSettings]);
  useEffect(() => {
    clearLegacyDirectoryStorage();
    // The stations the world generated come from a slower request: the first visit
    // waits for the naming model. Start it now, in parallel, but never wait for it.
    // The preset stations are listed as soon as they arrive, the generated ones
    // follow them when they are ready, and a failure here only means the directory
    // keeps the preset stations.
    const generatedCenters = fetchWorldCenters(wsURL).catch((): RegisteredCenter[] => []);
    fetchDirectory(wsURL).then(presetCenters => {
      if (!presetCenters.length) throw new Error('empty center directory');
      centersRef.current = presetCenters;
      directoryLoadStateRef.current = 'ready';
      setDirectoryCount(presetCenters.length);
      setDirectoryStatus(`センター情報読込完了 (${presetCenters.length}局)`);
      if (openDirectoryWhenReadyRef.current) {
        openDirectoryWhenReadyRef.current = false;
        directoryRef.current?.show();
        setDirectoryOpen(true);
      }
      void generatedCenters.then(generated => {
        if (!generated.length) return;
        const merged = mergeCenters(presetCenters, generated);
        centersRef.current = merged;
        setDirectoryCount(merged.length);
        setDirectoryStatus(`センター情報読込完了 (${merged.length}局)`);
      });
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
    modem.onTelemetry = setModemTelemetry;
    modem.onCallState = call => { const now = clock.now(); setWorldNow(now); setActiveCall(previous => { if (previous) setCompletedCost(value => value + tariff.chargeYen(previous.phone, previous.connectedAt, now)); return call ? { phone: call.phone, connectedAt: now, capabilities: call.capabilities } : null; }); };
    modem.setCommunicationSettings(commSettings);
    modem.setAutoRedial(autoRedial); modemRef.current = modem;
    const localStation = new LocalTestStation(terminal, { audio: { dial: playDialSequence, handshake: playHandshake } });
    localStation.onStatus = setStatus;
    localStation.onTelemetry = setModemTelemetry;
    localStation.onConnectionChange = connected => {
      setLocalTestConnected(connected);
      setLocalTestConnectedAt(connected ? clock.now() : null);
    };
    localStation.setCommunicationSettings(commSettings);
    localStationRef.current = localStation;
    directoryRef.current = new TerminalCenterDirectory(terminal, () => centersRef.current, center => { setDirectoryOpen(false); dialCenter(center); }, () => { setDirectoryOpen(false); showMainMenu(); });
    return () => { modem.onStatus = undefined; modem.onCallState = undefined; modem.onTelemetry = undefined; modem.dispose(); modemRef.current = null; localStation.onStatus = undefined; localStation.onConnectionChange = undefined; localStation.onTelemetry = undefined; localStation.dispose(); localStationRef.current = null; directoryRef.current = null; };
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
    terminal.write(' 例: ATDT0920000196\r\n');
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
  const generationTraceVisible = activeCall?.capabilities.generationTrace === true && !standaloneLine;
  const activeCenter = activeCall ? centersRef.current.find(center => center.phone === activeCall.phone) : undefined;
  const connectedMobileName = localTestConnected ? 'LOCAL TEST' : activeCall ? (activeCenter?.name ?? activeCall.phone) : null;
  const mobileConnectionStatus = resolveMobileConnectionStatus(modemTelemetry.phase, connectedMobileName);
  const mobileConnectedAt = localTestConnected ? localTestConnectedAt : activeCall?.connectedAt ?? null;
  const mobileElapsed = formatElapsed(mobileConnectedAt, worldNow);
  const mobileSessionCost = localTestConnected ? 0 : runningCost;
  const desktopHostName = localTestConnected ? 'LOCAL TEST' : activeCall ? (activeCenter?.name ?? activeCall.phone) : '';
  const desktopBaud = activeCall || localTestConnected ? modemTelemetry.baud : commSettings.dteBaud;
  const desktopElapsed = mobileConnectedAt ? mobileElapsed.slice(0, 5) : '00:00';
  const desktopLocation = `${callerLocation.maName}MA`;
  const buildInfoProps = {
    clientCommit: CLIENT_BUILD_COMMIT, clientRef: CLIENT_BUILD_REF, clientBuildTime: CLIENT_BUILD_TIME,
    server: serverBuildInfo, serverState: serverBuildState,
    onRetry: () => setVersionRefresh(value => value + 1),
  };
  return <main className="shell">
    <header className="desktop-topbar">
      {desktopHostName && <span className="desktop-host-name">{desktopHostName}</span>}
      <span className="desktop-world-clock">{desktopLocation}　・　{formatWorldDate(worldNow)}</span>
      <span className="desktop-modem-display" hidden={modemStatusMode === 'off'}>
        <ModemStatusDisplay mode={modemStatusMode} telemetry={modemTelemetry} dteBaud={commSettings.dteBaud} />
      </span>
      {generationTraceVisible && <button type="button" className={`generation-trace-link generation-trace-link--desktop${traceRunning ? ' generation-trace-link--running' : ''}`} onClick={() => setTraceOpen(true)}>{traceRunning ? '生成中…' : '生成ログ'}</button>}
      <button type="button" className="desktop-menu-toggle" aria-label="通信メニュー" aria-expanded={desktopMenuOpen} onClick={() => setDesktopMenuOpen(open => !open)}>
        <span /><span /><span />
      </button>
      {desktopMenuOpen && <nav className="desktop-menu" aria-label="通信メニュー">
        <div className="desktop-menu-row">
          <span>モデム表示</span>
          <div className="desktop-menu-toggle-group">
            <button type="button" aria-pressed={modemStatusMode === 'lamps'} onClick={() => setModemStatusMode('lamps')}>ランプ</button>
            <button type="button" aria-pressed={modemStatusMode === 'digital'} onClick={() => setModemStatusMode('digital')}>デジタル</button>
            <button type="button" aria-pressed={modemStatusMode === 'off'} onClick={() => setModemStatusMode('off')}>OFF</button>
          </div>
        </div>
        <div className="desktop-menu-row">
          <span>自動再接続</span>
          <button type="button" aria-pressed={autoRedial} onClick={() => setAutoRedial(value => !value)}>{autoRedial ? 'ON' : 'OFF'}</button>
        </div>
        <button type="button" className="desktop-menu-action" onClick={() => { setDesktopMenuOpen(false); routeCommand('ATH'); }}>電話を切る</button>
        <div className="desktop-menu-section">
          <span>画面サイズ</span>
          <button type="button" aria-pressed={terminalScreenMode === 'variable'} onClick={() => { setTerminalScreenMode('variable'); setDesktopMenuOpen(false); }}>80桁 × 可変行</button>
          <button type="button" aria-pressed={terminalScreenMode === 'fixed25'} onClick={() => { setTerminalScreenMode('fixed25'); setDesktopMenuOpen(false); }}>80桁 × 25行固定</button>
        </div>
        <BuildInfoPanel {...buildInfoProps} />
      </nav>}
    </header>
    <header className="titlebar"><span>ZUTTO COMMUNICATION TERMINAL Ver {APP_VERSION}</span><span>PC-9821 / 1996</span></header>
    <div className="mobile-statusbar" aria-label="接続状態">
      <span className="mobile-statusbar__connection">
        {mobileConnectionStatus.working && <span className="mobile-statusbar__activity" aria-hidden="true">
          <span /><span /><span />
        </span>}
        <span className={`mobile-statusbar__name${mobileConnectionStatus.working ? ' mobile-statusbar__name--working' : ''}`}>{mobileConnectionStatus.label}</span>
      </span>
      {generationTraceVisible && <button type="button" className={`generation-trace-link generation-trace-link--mobile${traceRunning ? ' generation-trace-link--running' : ''}`} onClick={() => setTraceOpen(true)} aria-label="生成ログを開く">{traceRunning ? '生成中…' : '生成ログ'}</button>}
      <span className="mobile-statusbar__stats">{mobileElapsed}&nbsp;&nbsp;¥{mobileSessionCost}</span>
    </div>
    <ModemStatusDisplay mode={modemStatusMode} telemetry={modemTelemetry} dteBaud={commSettings.dteBaud} />
    <section className="screen-wrap"><TerminalCanvas
      ref={terminalCanvasRef}
      terminal={terminal}
      keyboardActive={commandFocused}
      bottomControlsActive={directoryOpen}
      screenMode={terminalScreenMode}
      modemStatusMode={modemStatusMode}
      onModemStatusModeChange={setModemStatusMode}
      buildInfo={buildInfoProps}
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
    <footer className="desktop-statusbar" aria-label="通信状況">
      <span className="desktop-statusbar__separator" />
      <span className="desktop-statusbar__group">
        <span className={activeCall || localTestConnected ? 'desktop-statusbar__online' : 'desktop-statusbar__offline'}>{activeCall || localTestConnected ? '● 接続中' : status}</span>
        <span>{desktopBaud || commSettings.dteBaud} bps</span>
        <span>経過 {desktopElapsed}</span>
        <span>料金 ¥{cost}</span>
      </span>
    </footer>
    <footer className="statusbar"><span>{status}</span><span>{localTestConnected ? 'CALL LOCAL TEST / ¥0' : `CALL ¥${cost}`}</span><span>{localTestConnected ? 'LOCAL LOOP' : registeredCall ? 'TELEHODAI FIXED RATE' : teleho ? 'TELEHODAI TIME' : 'NORMAL TOLL'}</span><label><input type="checkbox" checked={autoRedial} onChange={e => setAutoRedial(e.target.checked)} disabled={localTestConnected} /> AUTO REDIAL</label></footer>
    <aside className="quick-help"><strong>発信地:</strong> {callerLocation.label}MA ({callerLocation.areaCode})<br /><strong>センター:</strong> {directoryStatus}<br /><strong>センターの呼び出し:</strong> メインメニューで <code>1</code>。現在 {directoryCount || '---'}局。<br /><strong>ターミナル・モード:</strong> メインメニューで <code>3</code>。電話番号を直接指定できます。<br /><strong>Local test station:</strong> <code>ATDT{LOCAL_TEST_NUMBER}</code>
      {!activeCall && !localTestConnected && <><details className="comm-panel"><summary>COMM SETTINGS / 通信設定</summary><div className="settings-summary">LINE {commSettings.lineBaud} / DTE {commSettings.dteBaud} / {framing} / {commSettings.flowControl.toUpperCase()}</div><div className="settings-grid"><label>MAX LINE SPEED<select value={commSettings.lineBaud} onChange={e => setting('lineBaud', Number(e.target.value) as CommSettings['lineBaud'])}><option value={2400}>2400 bps</option><option value={9600}>9600 bps</option><option value={14400}>14400 bps</option><option value={28800}>28800 bps</option></select></label></div></details><details className="debug-panel"><summary>DEBUG / MODEM AUDIO</summary><div className="audition-row">{([2400, 9600, 14400, 28800] as const).map(baud => <button key={baud} className="audition-btn" onClick={() => audition(baud)}>{baud}bps</button>)}</div><div className="audition-meta">AUDIO: {audioStatus}</div>{lastHandshake && <div className="audition-meta">RUN {lastHandshake.seed} / {lastHandshake.baud}bps</div>}</details></>}
    </aside>
    <GenerationInspector wsURL={wsURL} active={generationTraceVisible} open={traceOpen} onClose={() => setTraceOpen(false)} onRunningChange={setTraceRunning} />
  </main>;
}
