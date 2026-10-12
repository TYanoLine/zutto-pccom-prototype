import { useEffect, useMemo, useRef, useState } from 'react';
import { TerminalCore } from './terminal/TerminalCore';
import { TerminalCanvas, type TerminalCanvasHandle, type TerminalScreenMode } from './terminal/TerminalCanvas';
import { VirtualModem } from './modem/VirtualModem';
import type { HostCapabilities } from './modem/HostCapabilities';
import { LocalTestStation, LOCAL_TEST_NUMBER } from './modem/LocalTestStation';
import { clearLegacyDirectoryStorage, fetchDirectory, fetchWorldCenters, mergeCenters, resetWorldHosts } from './modem/CenterDirectory';
import type { RegisteredCenter } from './modem/CenterDirectory';
import { TerminalCenterDirectory } from './modem/TerminalCenterDirectory';
import { TerminalMainMenu } from './terminal/TerminalMainMenu';
import { DEFAULT_COMM_SETTINGS, normalizeCommSettings } from './modem/CommSettings';
import type { CommSettings } from './modem/CommSettings';
import { PseudoTariffService } from './billing/PseudoTariffService';
import { ntt1996TariffTable } from './billing/pseudoTariffs';
import { CALLER_LOCATION_PRESETS, loadCallerLocation, saveCallerLocation } from './billing/CallerLocation';
import { addMonthlyCharge, getMonthKey, loadMonthlyTotal } from './billing/MonthlyBilling';
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
type ScreenMode = 'main' | 'terminal' | 'regenerating';

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
  const [callerLocation, setCallerLocation] = useState(loadCallerLocation);
  const [feePopoverOpen, setFeePopoverOpen] = useState(false);
  const [editingLocation, setEditingLocation] = useState(false);
  const feePopoverRef = useRef<HTMLDivElement | null>(null);
  const [monthlyAccumulated, setMonthlyAccumulated] = useState(() => loadMonthlyTotal(getMonthKey(clock.now())));
  const tariff = useMemo(() => new PseudoTariffService(ntt1996TariffTable, telehodaiNumbers, callerLocation), [callerLocation]);
  const modemRef = useRef<VirtualModem | null>(null);
  const localStationRef = useRef<LocalTestStation | null>(null);
  const directoryRef = useRef<TerminalCenterDirectory | null>(null);
  const mainMenuRef = useRef<TerminalMainMenu | null>(null);
  const centersRef = useRef<RegisteredCenter[]>([]);
  const presetCentersRef = useRef<RegisteredCenter[]>([]);
  const directoryLoadStateRef = useRef<DirectoryLoadState>('loading');
  const openDirectoryWhenReadyRef = useRef(false);
  const screenModeRef = useRef<ScreenMode>('main');
  const lastDialWasLocalRef = useRef(false);
  const lastLocalDialModeRef = useRef<DialMode>('tone');
  const echoedInputRef = useRef('');
  const composingRef = useRef(false);
  const suppressEnterRef = useRef(false);
  const commandHistoryRef = useRef<string[]>([]);
  const commandHistoryIndexRef = useRef<number>(-1);
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
  const [directoryGenerating, setDirectoryGenerating] = useState(true);

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
  useEffect(() => {
    if (!feePopoverOpen) return;
    const handlePointerDown = (e: PointerEvent) => {
      const target = e.target as HTMLElement | null;
      if (feePopoverRef.current && !feePopoverRef.current.contains(target) && !target?.closest('.desktop-statusbar__fee-btn')) {
        setFeePopoverOpen(false);
        setEditingLocation(false);
      }
    };
    window.addEventListener('pointerdown', handlePointerDown);
    return () => window.removeEventListener('pointerdown', handlePointerDown);
  }, [feePopoverOpen]);
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
    let cancelled = false;
    setDirectoryGenerating(true);

    async function loadDirectory() {
      try {
        const [presets, generated] = await Promise.all([
          fetchDirectory(wsURL),
          fetchWorldCenters(wsURL).catch(error => {
            console.warn('Failed to fetch world centers, falling back to presets:', error);
            return [] as RegisteredCenter[];
          }),
        ]);
        if (cancelled) return;
        if (!presets.length) throw new Error('empty center directory');

        presetCentersRef.current = presets;
        const merged = mergeCenters(presets, generated);
        centersRef.current = merged;
        directoryLoadStateRef.current = 'ready';
        setDirectoryCount(merged.length);
        setDirectoryStatus(`センター情報読込完了 (${merged.length}局)`);
        setDirectoryGenerating(false);

        if (openDirectoryWhenReadyRef.current) {
          openDirectoryWhenReadyRef.current = false;
          directoryRef.current?.show();
          setDirectoryOpen(true);
        } else if (screenModeRef.current === 'main') {
          mainMenuRef.current?.render();
        }
      } catch (error) {
        if (cancelled) return;
        directoryLoadStateRef.current = 'error';
        setDirectoryGenerating(false);
        const message = error instanceof Error ? error.message : String(error);
        setDirectoryStatus(`CENTER API ERROR: ${message}`);
        if (openDirectoryWhenReadyRef.current) {
          openDirectoryWhenReadyRef.current = false;
          showDirectoryError();
        }
      }
    }

    void loadDirectory();

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    mainMenuRef.current = new TerminalMainMenu(
      terminal,
      APP_VERSION,
      () => (directoryLoadStateRef.current === 'ready' ? `${centersRef.current.length}局` : '---'),
      item => { executeMainMenuCommand(item.key); },
    );
    showMainMenu();
    const modem = new VirtualModem(terminal, wsURL, standaloneLine ? { offlineBusyExtraMs: 0, audio: { dial: playStandaloneBusySequence, busy: () => 0, handshake: playHandshake } } : {});
    modem.onStatus = setStatus;
    modem.onTelemetry = setModemTelemetry;
    modem.onCallState = call => {
      const now = clock.now();
      setWorldNow(now);
      setActiveCall(previous => {
        if (previous) {
          const yen = tariff.chargeYen(previous.phone, previous.connectedAt, now);
          setCompletedCost(value => value + yen);
          if (yen > 0) {
            const nextMonthly = addMonthlyCharge(getMonthKey(now), yen);
            setMonthlyAccumulated(nextMonthly);
          }
        }
        return call ? { phone: call.phone, connectedAt: now, capabilities: call.capabilities } : null;
      });
    };
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
    return () => { modem.onStatus = undefined; modem.onCallState = undefined; modem.onTelemetry = undefined; modem.dispose(); modemRef.current = null; localStation.onStatus = undefined; localStation.onConnectionChange = undefined; localStation.onTelemetry = undefined; localStation.dispose(); localStationRef.current = null; directoryRef.current = null; mainMenuRef.current = null; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clock, tariff, terminal]);
  useEffect(() => { modemRef.current?.setAutoRedial(autoRedial); }, [autoRedial]);
  useEffect(() => { const id = window.setInterval(() => setWorldNow(clock.now()), 1000); return () => window.clearInterval(id); }, [clock]);
  useEffect(() => {
    const resetScroll = () => {
      if (typeof window !== 'undefined' && (window.scrollY !== 0 || window.scrollX !== 0)) {
        window.scrollTo(0, 0);
      }
    };
    resetScroll();
    window.addEventListener('scroll', resetScroll, { passive: true });
    window.addEventListener('resize', resetScroll, { passive: true });
    window.addEventListener('orientationchange', resetScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', resetScroll);
      window.removeEventListener('resize', resetScroll);
      window.removeEventListener('orientationchange', resetScroll);
    };
  }, []);

  function resetInput() { setInput(''); echoedInputRef.current = ''; }
  function showMainMenu(index?: number) {
    screenModeRef.current = 'main';
    if (typeof index === 'number') {
      mainMenuRef.current?.setSelectedIndex(index);
    }
    mainMenuRef.current?.render();
    resetInput();
  }
  function executeMainMenuCommand(rawCommand: string) {
    const command = rawCommand.trim();
    const lower = command.toLowerCase();
    if (lower === '1') {
      if (directoryLoadStateRef.current === 'ready') { directoryRef.current?.show(); setDirectoryOpen(true); }
      else if (directoryLoadStateRef.current === 'loading') { openDirectoryWhenReadyRef.current = true; showDirectoryLoading(); }
      else showDirectoryError();
    } else if (lower === '1r') {
      startHostRegeneration();
    } else if (lower === '3') {
      showTerminalMode();
    } else if (command !== '') {
      mainMenuRef.current?.setSelectedByKey(command);
      terminal.write('\r\n 現在この項目は未実装です。\r\n\r\n');
      showMainMenu();
    } else {
      showMainMenu();
    }
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
    resetInput();
  }
  function showDirectoryError() {
    terminal.clear();
    terminal.write(`\x1b[37;44m ずっとパソコン通信　センター情報読み込み                         Ver ${APP_VERSION} \x1b[0m\r\n\r\n`);
    terminal.write('\r\n センター情報を読み込むことができませんでした。\r\n');
    terminal.write(' ESCキーでメイン・メニューに戻ってください。');
    resetInput();
  }
  function startHostRegeneration() {
    screenModeRef.current = 'regenerating';
    setDirectoryGenerating(true);
    showDirectoryLoading();
    resetWorldHosts(wsURL).then(generated => {
      const merged = mergeCenters(presetCentersRef.current, generated);
      centersRef.current = merged;
      setDirectoryCount(merged.length);
      setDirectoryStatus(`センター情報読込完了 (${merged.length}局)`);
      directoryRef.current?.setNotice('');
      setDirectoryGenerating(false);
      if (screenModeRef.current === 'regenerating') {
        screenModeRef.current = 'main';
        directoryRef.current?.show();
        setDirectoryOpen(true);
      } else {
        directoryRef.current?.refresh();
      }
    }).catch(() => {
      setDirectoryGenerating(false);
      if (screenModeRef.current === 'regenerating') {
        screenModeRef.current = 'main';
        showDirectoryError();
      }
    });
  }
  function syncTerminalInput(next: string) { const p = Array.from(echoedInputRef.current), n = Array.from(next); let c = 0; while (c < p.length && c < n.length && p[c] === n[c]) c++; for (let i = p.length; i > c; i--) terminal.backspace(); if (c < n.length) terminal.write(n.slice(c).join('')); echoedInputRef.current = next; }
  function followLiveInput() { terminalCanvasRef.current?.returnToLive(); }
  function change(e: React.ChangeEvent<HTMLInputElement>) { if (directoryRef.current?.isOpen()) return; followLiveInput(); const next = e.currentTarget.value; syncTerminalInput(next); setInput(next); }
  function compositionStart() { composingRef.current = true; followLiveInput(); }
  function compositionEnd(e: React.CompositionEvent<HTMLInputElement>) { composingRef.current = false; followLiveInput(); const next = e.currentTarget.value; syncTerminalInput(next); setInput(next); suppressEnterRef.current = true; window.setTimeout(() => { suppressEnterRef.current = false; }, 0); }
  function hangUp() {
    if (localStationRef.current?.isConnected() || localStationRef.current?.isDialing()) {
      localStationRef.current.hangup(true);
      return;
    }
    modemRef.current?.hangup();
  }
  function navigateHistory(direction: -1 | 1) {
    const history = commandHistoryRef.current;
    if (history.length === 0) return;
    if (direction === -1) {
      if (commandHistoryIndexRef.current === -1) {
        commandHistoryIndexRef.current = history.length - 1;
      } else if (commandHistoryIndexRef.current > 0) {
        commandHistoryIndexRef.current--;
      }
    } else {
      if (commandHistoryIndexRef.current !== -1) {
        if (commandHistoryIndexRef.current < history.length - 1) {
          commandHistoryIndexRef.current++;
        } else {
          commandHistoryIndexRef.current = -1;
          syncTerminalInput('');
          resetInput();
          return;
        }
      } else {
        return;
      }
    }
    const cmd = history[commandHistoryIndexRef.current];
    syncTerminalInput(cmd);
    setInput(cmd);
  }
  function routeCommand(raw: string) {
    const upper = raw.trim().toUpperCase(), station = localStationRef.current;
    if (upper === 'ATH') {
      hangUp();
      return;
    }
    if (station?.isConnected()) { station.submitLine(raw); return; }
    if (station?.isDialing()) { return; }
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
    if (upper.startsWith('ATDT') || upper.startsWith('ATDP') || /^ATD\d/.test(upper)) lastDialWasLocalRef.current = false;
    modemRef.current?.submitLine(raw);
  }
  function keyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (directoryRef.current?.isOpen()) {
      if (e.key === 'ArrowLeft') { directoryRef.current.handleKey('PageUp'); e.preventDefault(); return; }
      if (e.key === 'ArrowRight') { directoryRef.current.handleKey('PageDown'); e.preventDefault(); return; }
      directoryRef.current.handleKey(e.key);
      e.preventDefault();
      return;
    }
    const native = e.nativeEvent as KeyboardEvent;
    if (e.key === 'Escape') {
      if (!activeCall && !localTestConnected) {
        if (openDirectoryWhenReadyRef.current) openDirectoryWhenReadyRef.current = false;
        if (screenModeRef.current === 'terminal' || screenModeRef.current === 'regenerating' || openDirectoryWhenReadyRef.current === false) showMainMenu();
        e.preventDefault(); return;
      } else {
        if (input.length > 0) {
          syncTerminalInput('');
          resetInput();
          e.preventDefault();
          return;
        }
      }
    }
    if (!activeCall && !localTestConnected && screenModeRef.current === 'main') {
      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        e.preventDefault();
        resetInput();
        mainMenuRef.current?.handleKey(e.key);
        return;
      }
    } else if (screenModeRef.current === 'terminal' || activeCall || localTestConnected) {
      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        if (commandHistoryRef.current.length > 0) {
          e.preventDefault();
          navigateHistory(e.key === 'ArrowUp' ? -1 : 1);
          return;
        }
      }
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
    if (raw.length > 0) {
      commandHistoryRef.current.push(raw);
      commandHistoryIndexRef.current = -1;
    }
    resetInput();
    if (!activeCall && !localTestConnected && screenModeRef.current === 'main') {
      const selectedKey = mainMenuRef.current?.getSelectedItem().key ?? '1';
      const targetCommand = command !== '' ? command : selectedKey;
      executeMainMenuCommand(targetCommand);
    } else {
      routeCommand(raw);
    }
  }
  function softKey(key: string) {
    if (key === 'HangUp') {
      hangUp();
      return;
    }
    if (directoryRef.current?.isOpen()) {
      if (key === 'ArrowLeft') {
        directoryRef.current.handleKey('PageUp');
        return;
      }
      if (key === 'ArrowRight') {
        directoryRef.current.handleKey('PageDown');
        return;
      }
      directoryRef.current.handleKey(key);
      return;
    }
    if (key === 'Escape') {
      if (openDirectoryWhenReadyRef.current) openDirectoryWhenReadyRef.current = false;
      if (!activeCall && !localTestConnected) {
        if (screenModeRef.current === 'terminal' || screenModeRef.current === 'regenerating') {
          showMainMenu();
        } else {
          resetInput();
        }
      } else {
        if (input.length > 0) {
          syncTerminalInput('');
          resetInput();
        } else {
          routeCommand('\x1b');
        }
      }
      return;
    }
    if (key === 'Enter') {
      submitInput();
      return;
    }
    if (!activeCall && !localTestConnected && screenModeRef.current === 'main') {
      if (key === 'ArrowUp' || key === 'ArrowDown') {
        resetInput();
        mainMenuRef.current?.handleKey(key);
        return;
      }
    } else if (screenModeRef.current === 'terminal' || activeCall || localTestConnected) {
      if (key === 'ArrowUp' || key === 'ArrowDown') {
        navigateHistory(key === 'ArrowUp' ? -1 : 1);
        return;
      }
      if (key === 'ArrowLeft') {
        if (input.length > 0) {
          const next = input.slice(0, -1);
          syncTerminalInput(next);
          setInput(next);
        }
        return;
      }
    }
  }
  function dialCenter(center: RegisteredCenter) { screenModeRef.current = 'terminal'; const command = `${center.dialMode === 'pulse' ? 'ATDP' : 'ATDT'}${center.phone}`; terminal.write(`${command}\r\n`); routeCommand(command); resetInput(); }
  function audition(baud: number) { try { setAudioStatus(`SYNTHESIZING ${baud}bps...`); const run = playHandshake(baud); setLastHandshake(run); setAudioStatus(`PLAYING ${baud}bps`); } catch (error) { setAudioStatus(`AUDIO ERROR: ${error instanceof Error ? error.message : String(error)}`); } }
  function setting<K extends keyof CommSettings>(key: K, value: CommSettings[K]) { setCommSettings(current => ({ ...current, [key]: value })); }

  const isCallingOrConnected = Boolean(
    activeCall ||
    localTestConnected ||
    modemTelemetry.phase !== 'idle' ||
    modemTelemetry.oh ||
    modemTelemetry.cd
  );
  const runningCost = activeCall ? tariff.chargeYen(activeCall.phone, activeCall.connectedAt, worldNow) : 0, teleho = tariff.isTelehodaiWindow(worldNow), registeredCall = activeCall && tariff.isTelehodaiCall(activeCall.phone, worldNow), framing = `${commSettings.dataBits}${commSettings.parity === 'none' ? 'N' : commSettings.parity === 'even' ? 'E' : 'O'}${commSettings.stopBits}`;
  const isConnected = !!(activeCall || localTestConnected);
  const cost = localTestConnected ? 0 : runningCost;
  const desktopCostLabel = isConnected ? `料金 ¥${cost}` : '料金 ー円';
  const mobileSessionCost = localTestConnected ? 0 : runningCost;
  const mobileCostLabel = isConnected ? `¥${mobileSessionCost}` : 'ー円';
  const currentMonthLabel = `${worldNow.getMonth() + 1}月`;
  const monthlyTotalDisplay = monthlyAccumulated + (localTestConnected ? 0 : runningCost);
  const generationTraceVisible = activeCall?.capabilities.generationTrace === true && !standaloneLine;
  const activeCenter = activeCall ? centersRef.current.find(center => center.phone === activeCall.phone) : undefined;
  const connectedMobileName = localTestConnected ? 'LOCAL TEST' : activeCall ? (activeCenter?.name ?? activeCall.phone) : null;
  const mobileConnectionStatus = resolveMobileConnectionStatus(modemTelemetry.phase, connectedMobileName);
  const mobileConnectedAt = localTestConnected ? localTestConnectedAt : activeCall?.connectedAt ?? null;
  const mobileElapsed = formatElapsed(mobileConnectedAt, worldNow);
  const desktopHostName = localTestConnected ? 'LOCAL TEST' : activeCall ? (activeCenter?.name ?? activeCall.phone) : '';
  const desktopBaud = activeCall || localTestConnected ? modemTelemetry.baud : commSettings.dteBaud;
  const desktopElapsed = mobileConnectedAt ? mobileElapsed.slice(0, 5) : '00:00';
  const buildInfoProps = {
    clientCommit: CLIENT_BUILD_COMMIT, clientRef: CLIENT_BUILD_REF, clientBuildTime: CLIENT_BUILD_TIME,
    server: serverBuildInfo, serverState: serverBuildState,
    onRetry: () => setVersionRefresh(value => value + 1),
  };
  const feePopoverContent = feePopoverOpen && (
    <div className="fee-popover" ref={feePopoverRef} role="dialog" aria-label="発信地と通話料金">
      {!editingLocation ? (
        <div className="fee-popover__content">
          <div className="fee-popover__row">
            <span>発信地：{callerLocation.label}({callerLocation.areaCode})</span>
            <button
              type="button"
              className="fee-popover__change-btn"
              onClick={() => setEditingLocation(true)}
            >
              [変更]
            </button>
            <button
              type="button"
              className="fee-popover__close-btn"
              onClick={() => setFeePopoverOpen(false)}
              aria-label="閉じる"
            >
              ✕
            </button>
          </div>
          <div className="fee-popover__row fee-popover__row--sub">
            <span>当月({currentMonthLabel})の累積：¥{monthlyTotalDisplay}</span>
          </div>
        </div>
      ) : (
        <div className="fee-popover__edit">
          <label className="fee-popover__label">
            <span>発信地：</span>
            <select
              className="fee-popover__select"
              value={callerLocation.id}
              onChange={e => {
                const next = CALLER_LOCATION_PRESETS.find(p => p.id === e.target.value);
                if (next) {
                  setCallerLocation(next);
                  setEditingLocation(false);
                }
              }}
            >
              {CALLER_LOCATION_PRESETS.map(preset => (
                <option key={preset.id} value={preset.id}>
                  {preset.label}({preset.areaCode})
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className="fee-popover__change-btn"
            onClick={() => setEditingLocation(false)}
          >
            [完了]
          </button>
        </div>
      )}
    </div>
  );
  return <main className="shell">
    <header className="desktop-topbar">
      {desktopHostName && <span className="desktop-host-name" title={desktopHostName}>{desktopHostName}</span>}
      <span className="desktop-world-clock">{formatWorldDate(worldNow)}</span>
      <span className="desktop-modem-display" hidden={modemStatusMode === 'off'}>
        <ModemStatusDisplay mode={modemStatusMode} telemetry={modemTelemetry} dteBaud={commSettings.dteBaud} generating={traceRunning || directoryGenerating} />
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
        <button type="button" className="desktop-menu-action" onClick={() => { setDesktopMenuOpen(false); hangUp(); }}>電話を切る</button>
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
      <span className="mobile-statusbar__stats">
        {mobileElapsed}&nbsp;&nbsp;
        <span className="desktop-statusbar__fee-wrap">
          <button
            type="button"
            className="desktop-statusbar__fee-btn"
            onClick={() => { setFeePopoverOpen(open => !open); setEditingLocation(false); }}
            aria-expanded={feePopoverOpen}
            aria-label="通話料金と発信地"
          >
            {mobileCostLabel}
          </button>
          {feePopoverContent}
        </span>
      </span>
    </div>
    <ModemStatusDisplay mode={modemStatusMode} telemetry={modemTelemetry} dteBaud={commSettings.dteBaud} generating={traceRunning || directoryGenerating} />
    <section className="screen-wrap"><TerminalCanvas
      ref={terminalCanvasRef}
      terminal={terminal}
      keyboardActive={commandFocused}
      bottomControlsActive={true}
      screenMode={terminalScreenMode}
      modemStatusMode={modemStatusMode}
      onModemStatusModeChange={setModemStatusMode}
      buildInfo={buildInfoProps}
      onHangUp={hangUp}
      isCallingOrConnected={isCallingOrConnected}
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
    <nav className="mobile-softkeys directory-softkeys" aria-label="専用ソフトキー">
      <button type="button" onPointerDown={e => e.preventDefault()} onClick={() => softKey('Escape')}>
        ESC<small>戻る</small>
      </button>
      <button type="button" onPointerDown={e => e.preventDefault()} onClick={() => softKey('ArrowUp')}>
        ▲<small>上</small>
      </button>
      <button type="button" onPointerDown={e => e.preventDefault()} onClick={() => softKey('ArrowDown')}>
        ▼<small>下</small>
      </button>
      <button type="button" onPointerDown={e => e.preventDefault()} onClick={() => softKey('ArrowLeft')}>
        ◀<small>{directoryOpen ? '前頁' : '左'}</small>
      </button>
      <button type="button" onPointerDown={e => e.preventDefault()} onClick={() => softKey('ArrowRight')}>
        ▶<small>{directoryOpen ? '次頁' : '右'}</small>
      </button>
      <button
        type="button"
        className={directoryOpen ? 'softkey-call' : 'softkey-enter'}
        onPointerDown={e => e.preventDefault()}
        onClick={() => softKey('Enter')}
      >
        {directoryOpen ? 'CALL' : 'RET'}<small>{directoryOpen ? '呼出' : '送出'}</small>
      </button>
    </nav>
    <footer className="desktop-statusbar" aria-label="通信状況">
      <span className="desktop-statusbar__separator" />
      <span className="desktop-statusbar__group">
        <span className={activeCall || localTestConnected ? 'desktop-statusbar__online' : 'desktop-statusbar__offline'}>{activeCall || localTestConnected ? '● 接続中' : status}</span>
        <span>{desktopBaud || commSettings.dteBaud} bps</span>
        <span>経過 {desktopElapsed}</span>
        <span className="desktop-statusbar__fee-wrap">
          <button
            type="button"
            className="desktop-statusbar__fee-btn"
            onClick={() => { setFeePopoverOpen(open => !open); setEditingLocation(false); }}
            aria-expanded={feePopoverOpen}
            aria-label="通話料金と発信地"
          >
            {desktopCostLabel}
          </button>
          {feePopoverContent}
        </span>
      </span>
    </footer>
    <footer className="statusbar"><span>{status}</span><span>{localTestConnected ? 'CALL LOCAL TEST / ¥0' : `CALL ¥${cost}`}</span><span>{localTestConnected ? 'LOCAL LOOP' : registeredCall ? 'TELEHODAI FIXED RATE' : teleho ? 'TELEHODAI TIME' : 'NORMAL TOLL'}</span><label><input type="checkbox" checked={autoRedial} onChange={e => setAutoRedial(e.target.checked)} disabled={localTestConnected} /> AUTO REDIAL</label></footer>
    <aside className="quick-help"><strong>発信地:</strong> {callerLocation.label} ({callerLocation.areaCode})<br /><strong>センター:</strong> {directoryStatus}<br /><strong>センターの呼び出し:</strong> メインメニューで <code>1</code>。現在 {directoryCount || '---'}局。<br /><strong>ターミナル・モード:</strong> メインメニューで <code>3</code>。電話番号を直接指定できます。<br /><strong>Local test station:</strong> <code>ATDT{LOCAL_TEST_NUMBER}</code>
      {!activeCall && !localTestConnected && <><details className="comm-panel"><summary>COMM SETTINGS / 通信設定</summary><div className="settings-summary">LINE {commSettings.lineBaud} / DTE {commSettings.dteBaud} / {framing} / {commSettings.flowControl.toUpperCase()}</div><div className="settings-grid"><label>MAX LINE SPEED<select value={commSettings.lineBaud} onChange={e => setting('lineBaud', Number(e.target.value) as CommSettings['lineBaud'])}><option value={2400}>2400 bps</option><option value={9600}>9600 bps</option><option value={14400}>14400 bps</option><option value={28800}>28800 bps</option></select></label></div></details><details className="debug-panel"><summary>DEBUG / MODEM AUDIO</summary><div className="audition-row">{([2400, 9600, 14400, 28800] as const).map(baud => <button key={baud} className="audition-btn" onClick={() => audition(baud)}>{baud}bps</button>)}</div><div className="audition-meta">AUDIO: {audioStatus}</div>{lastHandshake && <div className="audition-meta">RUN {lastHandshake.seed} / {lastHandshake.baud}bps</div>}</details></>}
    </aside>
    <GenerationInspector wsURL={wsURL} active={generationTraceVisible} open={traceOpen} onClose={() => setTraceOpen(false)} onRunningChange={setTraceRunning} />
  </main>;
}
