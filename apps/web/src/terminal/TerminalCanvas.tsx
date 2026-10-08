import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react';
import type {
  ChangeEventHandler,
  CompositionEventHandler,
  CSSProperties,
  FocusEventHandler,
  KeyboardEventHandler,
  PointerEvent as ReactPointerEvent,
  WheelEvent as ReactWheelEvent,
} from 'react';
import { isFullWidth, type Cell, type TerminalCore } from './TerminalCore';
import type { ModemStatusDisplayMode } from '../modem/ModemStatusDisplay';
import { BuildInfoPanel } from '../build/BuildInfoPanel';
import type { BuildInfoPanelProps } from '../build/BuildInfoPanel';
import { desktopTerminalRows, mobileTerminalRows, terminalBackingScale, terminalCursorTargetScrollTop, terminalViewportHeight } from './terminalViewport';
import './terminalFit.css';

/**
 * PC-98 text GDC (NEC uPD7220 Master GDC) cursor blink interval in milliseconds.
 * In historical PC-9801 standard text mode (24.83 kHz horizontal / ~56.42 Hz vertical refresh),
 * the default blink rate (BR = 16, resulting in 2 * BR = 32 video frames on / 32 frames off)
 * yields 32 frames / 56.422 Hz ~= 567 ms on and 567 ms off (~1.134 s full cycle, ~0.88 Hz).
 */
export const PC98_CURSOR_BLINK_INTERVAL_MS = 567;

export function terminalCursorWidth(cell?: Pick<Cell, 'ch'>): number {
  return cell && isFullWidth(cell.ch) ? 16 : 8;
}

const PALETTE = ['#000000', '#aa0000', '#00aa00', '#aa5500', '#0000aa', '#aa00aa', '#00aaaa', '#aaaaaa'];

const HALF_WIDTH_FONT = '13px ui-monospace, "SFMono-Regular", Menlo, Monaco, Consolas, "Hiragino Sans", "Yu Gothic", monospace';
const FULL_WIDTH_FONT = '16px "Hiragino Sans", "Hiragino Kaku Gothic ProN", "Yu Gothic", Meiryo, sans-serif';

export function terminalGlyphPaintStyle(ch: string) {
  const fullWidth = isFullWidth(ch);
  return {
    fullWidth,
    font: fullWidth ? FULL_WIDTH_FONT : HALF_WIDTH_FONT,
    glyphWidth: fullWidth ? 16 : 8,
    yOffset: fullWidth ? 0 : 1,
  } as const;
}

export type TerminalCanvasHandle = {
  returnToLive: () => void;
  ensureCursorVisible: () => void;
};

export type TerminalKeyboardInput = {
  value: string;
  readOnly: boolean;
  onChange: ChangeEventHandler<HTMLInputElement>;
  onKeyDown: KeyboardEventHandler<HTMLInputElement>;
  onCompositionStart: CompositionEventHandler<HTMLInputElement>;
  onCompositionEnd: CompositionEventHandler<HTMLInputElement>;
  onFocus: FocusEventHandler<HTMLInputElement>;
  onBlur: FocusEventHandler<HTMLInputElement>;
};

export type TerminalScreenMode = 'variable' | 'fixed25';

type TerminalCanvasProps = {
  screenMode: TerminalScreenMode;
  terminal: TerminalCore;
  keyboardInput: TerminalKeyboardInput;
  keyboardActive?: boolean;
  bottomControlsActive?: boolean;
  modemStatusMode: ModemStatusDisplayMode;
  onModemStatusModeChange: (mode: ModemStatusDisplayMode) => void;
  buildInfo: BuildInfoPanelProps;
};

export const TerminalCanvas = forwardRef<TerminalCanvasHandle, TerminalCanvasProps>(function TerminalCanvas(
  {
    terminal,
    screenMode,
    keyboardInput,
    keyboardActive = false,
    bottomControlsActive = false,
    modemStatusMode,
    onModemStatusModeChange,
    buildInfo,
  },
  forwardedRef,
) {
  const ref = useRef<HTMLCanvasElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const keyboardProxyRef = useRef<HTMLInputElement>(null);
  const [display, setDisplay] = useState<'readable' | 'fit'>(() =>
    typeof window !== 'undefined' && window.matchMedia('(max-width: 680px), (pointer: coarse)').matches
      ? 'fit'
      : 'readable',
  );
  const [historyOffset, setHistoryOffset] = useState(0);
  const [layoutRows, setLayoutRows] = useState(terminal.height);
  const [backingScale, setBackingScale] = useState(() =>
    typeof window === 'undefined' ? 1 : terminalBackingScale(window.devicePixelRatio),
  );
  const [functionMenuOpen, setFunctionMenuOpen] = useState(false);
  const [cursorVisible, setCursorVisible] = useState(true);
  const cursorBlinkTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const layoutRowsRef = useRef(terminal.height);
  const scrollOffsetRef = useRef(0);
  const previousScrollbackLengthRef = useRef(terminal.scrollbackLength);
  const activePointerIdRef = useRef<number | null>(null);
  const pointerLastYRef = useRef(0);
  const pointerLastXRef = useRef(0);
  const pointerRemainderRef = useRef(0);
  const pointerTravelRef = useRef(0);

  const clearCursorBlinkTimer = useCallback(() => {
    if (cursorBlinkTimerRef.current !== null) {
      clearTimeout(cursorBlinkTimerRef.current);
      cursorBlinkTimerRef.current = null;
    }
  }, []);

  const scheduleCursorBlink = useCallback((nextVisible: boolean) => {
    clearCursorBlinkTimer();

    // Respect reduced-motion preferences: keep cursor steadily visible without blinking.
    if (typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setCursorVisible(true);
      return;
    }

    // Pause blinking when page is hidden to conserve power and CPU cycles.
    if (typeof document !== 'undefined' && document.hidden) {
      setCursorVisible(true);
      return;
    }

    cursorBlinkTimerRef.current = setTimeout(() => {
      setCursorVisible(nextVisible);
      scheduleCursorBlink(!nextVisible);
    }, PC98_CURSOR_BLINK_INTERVAL_MS);
  }, [clearCursorBlinkTimer]);

  const resetCursorBlink = useCallback(() => {
    setCursorVisible(true);
    scheduleCursorBlink(false);
  }, [scheduleCursorBlink]);

  useEffect(() => {
    resetCursorBlink();

    const handleVisibilityChange = () => {
      if (document.hidden) {
        clearCursorBlinkTimer();
        setCursorVisible(true);
      } else {
        resetCursorBlink();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      clearCursorBlinkTimer();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [resetCursorBlink, clearCursorBlinkTimer]);

  useEffect(() => terminal.subscribe(() => {
    resetCursorBlink();
    const previousLength = previousScrollbackLengthRef.current;
    const nextLength = terminal.scrollbackLength;
    const added = Math.max(0, nextLength - previousLength);

    // If the user is reading history, keep the same historical text under the
    // viewport while new live lines arrive. At the live bottom, keep following
    // output exactly as a normal communications terminal does.
    const maxOffset = terminal.maxScrollOffsetForRows(layoutRowsRef.current);
    if (scrollOffsetRef.current > 0 && added > 0) {
      scrollOffsetRef.current = Math.min(maxOffset, scrollOffsetRef.current + added);
    } else {
      scrollOffsetRef.current = Math.min(maxOffset, scrollOffsetRef.current);
    }
    previousScrollbackLengthRef.current = nextLength;
    setHistoryOffset(scrollOffsetRef.current);
    draw();
    if (keyboardActive) window.requestAnimationFrame(ensureCursorVisible);
  }), [terminal, keyboardActive, resetCursorBlink]);
  useEffect(() => { draw(); });

  useEffect(() => {
    const updateBackingScale = () => setBackingScale(terminalBackingScale(window.devicePixelRatio));
    updateBackingScale();
    window.addEventListener('resize', updateBackingScale);
    return () => window.removeEventListener('resize', updateBackingScale);
  }, []);

  useEffect(() => {
    if (!isMobilePresentation()) {
      if (screenMode === 'fixed25') {
        updateLayoutRows(terminal.height);
        return;
      }
      const viewport = viewportRef.current;
      if (!viewport) return;
      const syncDesktopRows = () => {
        const canvas = ref.current;
        if (!canvas) return;
        const rows = desktopTerminalRows(viewport.clientWidth, viewport.clientHeight);
        updateLayoutRows(rows);
      };
      const observer = new ResizeObserver(() => window.requestAnimationFrame(syncDesktopRows));
      observer.observe(viewport);
      syncDesktopRows();
      return () => observer.disconnect();
    }

    const visualViewport = window.visualViewport;
    const update = () => window.requestAnimationFrame(syncMobileRows);
    update();
    if (!keyboardActive) {
      window.addEventListener('resize', update);
      visualViewport?.addEventListener('resize', update);
    }
    return () => {
      window.removeEventListener('resize', update);
      visualViewport?.removeEventListener('resize', update);
    };
    // Geometry is intentionally measured from the live DOM after each relevant mode change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [display, keyboardActive, bottomControlsActive, modemStatusMode, screenMode]);

  function setScrollOffset(next: number) {
    const maxOffset = terminal.maxScrollOffsetForRows(layoutRowsRef.current);
    const clamped = Math.max(0, Math.min(maxOffset, next));
    if (clamped === scrollOffsetRef.current) return;
    scrollOffsetRef.current = clamped;
    setHistoryOffset(clamped);
    draw();
  }

  function updateLayoutRows(next: number) {
    const rows = Math.max(terminal.height, Math.min(120, Math.trunc(next)));
    layoutRowsRef.current = rows;
    const maxOffset = terminal.maxScrollOffsetForRows(rows);
    if (scrollOffsetRef.current > maxOffset) {
      scrollOffsetRef.current = maxOffset;
      setHistoryOffset(maxOffset);
    }
    setLayoutRows(rows);
  }

  function isMobilePresentation() {
    return window.matchMedia('(max-width: 680px), (pointer: coarse)').matches;
  }

  function syncMobileRows() {
    if (!isMobilePresentation()) return;
    if (display !== 'fit') {
      updateLayoutRows(terminal.height);
      return;
    }
    if (keyboardActive) return;

    const viewport = viewportRef.current;
    const canvas = ref.current;
    if (!viewport || !canvas) return;
    const visualViewport = window.visualViewport;
    const visibleBottom = visualViewport
      ? visualViewport.offsetTop + visualViewport.height
      : window.innerHeight;
    const viewportTop = viewport.getBoundingClientRect().top;
    const bottomControls = bottomControlsActive
      ? document.querySelector<HTMLElement>('.directory-softkeys')
      : null;
    const controlsTop = bottomControls?.getBoundingClientRect().top ?? visibleBottom;
    const availableHeight = Math.max(0, Math.min(visibleBottom, controlsTop) - viewportTop);
    const rows = mobileTerminalRows(canvas.clientWidth || viewport.clientWidth, availableHeight);
    updateLayoutRows(rows);
  }

  function resetKeyboardViewport() {
    const viewport = viewportRef.current;
    if (!viewport) return;
    viewport.style.maxHeight = '';
    viewport.style.overflowY = '';
    viewport.scrollTop = 0;
  }

  function positionKeyboardProxy() {
    const proxy = keyboardProxyRef.current;
    const canvas = ref.current;
    const viewport = viewportRef.current;
    if (!proxy || !canvas || !viewport) return;

    if (isMobilePresentation()) {
      proxy.style.left = '';
      proxy.style.top = '';
      proxy.style.width = '';
      proxy.style.height = '';
      return;
    }

    // On desktop the canvas is aspect-locked and centred, so it can be
    // letterboxed or pillarboxed inside the viewport. Measure its actual offset
    // so the IME composition window follows the cursor cell.
    const canvasRect = canvas.getBoundingClientRect();
    const viewportRect = viewport.getBoundingClientRect();
    const offsetLeft = canvasRect.left - viewportRect.left;
    const offsetTop = canvasRect.top - viewportRect.top;

    const cellWidth = Math.max(1, canvas.clientWidth / terminal.width);
    const rowHeight = Math.max(1, canvas.clientHeight / layoutRowsRef.current);
    const proxyWidth = Math.max(8, cellWidth);
    const proxyHeight = Math.max(16, rowHeight);
    const left = Math.max(
      0,
      Math.min(viewport.clientWidth - proxyWidth, offsetLeft + terminal.cursorX * cellWidth),
    );
    const top = Math.max(
      0,
      Math.min(viewport.clientHeight - proxyHeight, offsetTop + terminal.cursorY * rowHeight),
    );

    proxy.style.left = `${left}px`;
    proxy.style.top = `${top}px`;
    proxy.style.width = `${proxyWidth}px`;
    proxy.style.height = `${proxyHeight}px`;
  }

  function focusKeyboardProxy() {
    if (keyboardInput.readOnly) return;
    const proxy = keyboardProxyRef.current;
    if (!proxy) return;
    positionKeyboardProxy();
    proxy.focus({ preventScroll: true });
  }

  function ensureCursorVisible() {
    if (!keyboardActive || !isMobilePresentation()) return;
    const viewport = viewportRef.current;
    const canvas = ref.current;
    if (!viewport || !canvas) return;

    const visualViewport = window.visualViewport;
    const visualBottom = visualViewport
      ? visualViewport.offsetTop + visualViewport.height
      : window.innerHeight;
    const viewportTop = viewport.getBoundingClientRect().top;
    const nextHeight = terminalViewportHeight(canvas.clientHeight, viewportTop, visualBottom);

    viewport.style.maxHeight = `${nextHeight}px`;
    viewport.style.overflowY = nextHeight < canvas.clientHeight ? 'auto' : 'hidden';

    const displayedRows = layoutRowsRef.current;
    const cellWidth = Math.max(1, canvas.clientWidth / terminal.width);
    const rowHeight = Math.max(1, canvas.clientHeight / displayedRows);
    const cursorLeft = terminal.cursorX * cellWidth;
    const cursorTop = terminal.viewportCursorY(displayedRows) * rowHeight;
    const targetScrollTop = terminalCursorTargetScrollTop(
      canvas.clientHeight,
      viewport.clientHeight,
      cursorTop,
      rowHeight,
    );
    const maxScrollLeft = Math.max(0, canvas.clientWidth - viewport.clientWidth);
    const targetScrollLeft = Math.max(
      0,
      Math.min(maxScrollLeft, cursorLeft - viewport.clientWidth + cellWidth * 2),
    );

    if (Math.abs(viewport.scrollTop - targetScrollTop) > 1) viewport.scrollTop = targetScrollTop;
    if (Math.abs(viewport.scrollLeft - targetScrollLeft) > 1) viewport.scrollLeft = targetScrollLeft;
    positionKeyboardProxy();
  }

  function returnToLive() {
    if (scrollOffsetRef.current !== 0) setScrollOffset(0);
  }

  useImperativeHandle(forwardedRef, () => ({ returnToLive, ensureCursorVisible }));

  useEffect(() => {
    if (!keyboardActive || !isMobilePresentation()) {
      resetKeyboardViewport();
      return;
    }

    returnToLive();
    const visualViewport = window.visualViewport;
    const update = () => window.requestAnimationFrame(ensureCursorVisible);
    window.addEventListener('resize', update);
    visualViewport?.addEventListener('resize', update);

    return () => {
      window.removeEventListener('resize', update);
      visualViewport?.removeEventListener('resize', update);
    };
    // The functions intentionally read the current refs/cursor on every viewport event.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keyboardActive]);

  function draw() {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const displayedRows = layoutRowsRef.current;
    const rows = terminal.viewportRows(scrollOffsetRef.current, displayedRows);
    const logicalWidth = terminal.width * 8;
    const logicalHeight = displayedRows * 16;

    // Keep the historical 8x16 logical cell grid, but rasterize it at up to
    // 2x backing resolution. Mobile fit mode then downsamples a higher-quality
    // source instead of magnifying a low-resolution canvas with pixelated CSS.
    ctx.setTransform(backingScale, 0, 0, backingScale, 0, 0);
    ctx.fillStyle = '#000000';
    ctx.fillRect(0, 0, logicalWidth, logicalHeight);
    ctx.textBaseline = 'top';
    ctx.textAlign = 'center';

    // Paint every cell's background first, including the continuation cell of
    // a full-width character. A continuation is still a real 8x16 terminal
    // cell; skipping it leaves a black stripe through ANSI reverse/highlight
    // regions containing Japanese text.
    for (let y = 0; y < displayedRows; y++) {
      for (let x = 0; x < terminal.width; x++) {
        const cell = rows[y][x];
        if (cell.bg === 0) continue;
        ctx.fillStyle = PALETTE[cell.bg] ?? '#000';
        ctx.fillRect(x * 8, y * 16, 8, 16);
      }
    }

    // Then draw glyphs only from their leading cells. Full-width glyphs span
    // the leading cell and its continuation cell, whose background is already
    // present from the pass above.
    for (let y = 0; y < displayedRows; y++) {
      for (let x = 0; x < terminal.width; x++) {
        const cell = rows[y][x];
        if (cell.continuation) continue;
        const px = x * 8;
        const py = y * 16;
        ctx.fillStyle = PALETTE[cell.fg] ?? '#aaa';
        const glyphStyle = terminalGlyphPaintStyle(cell.ch);
        const glyphCenter = px + glyphStyle.glyphWidth / 2;
        const glyphY = py + glyphStyle.yOffset;

        // Do not use fillText(..., maxWidth): Canvas implements that by
        // horizontally squeezing the glyph, which made Latin text unnaturally
        // narrow on iOS. Instead choose a font size that naturally fits the
        // historical 8px half-width cell and center it in the fixed grid.
        ctx.font = glyphStyle.font;
        ctx.fillText(cell.ch, glyphCenter, glyphY);
        if (cell.bold && cell.ch !== ' ') ctx.fillText(cell.ch, glyphCenter + 0.5, glyphY);
      }
    }

    // The live cursor has no meaning while looking back through history.
    if (scrollOffsetRef.current === 0 && cursorVisible) {
      ctx.globalAlpha = 0.65;
      ctx.fillStyle = '#aaaaaa';
      const cursorY = terminal.viewportCursorY(displayedRows);
      const currentCell = terminal.cells[terminal.cursorY]?.[terminal.cursorX];
      const cursorWidth = terminalCursorWidth(currentCell);
      ctx.fillRect(terminal.cursorX * 8, cursorY * 16 + 14, cursorWidth, 2);
      ctx.globalAlpha = 1;
    }
    positionKeyboardProxy();
  }

  function wheel(e: ReactWheelEvent<HTMLCanvasElement>) {
    if (terminal.maxScrollOffsetForRows(layoutRowsRef.current) === 0 || e.deltaY === 0) return;
    e.preventDefault();
    const lines = Math.max(1, Math.min(8, Math.round(Math.abs(e.deltaY) / 32)));
    const delta = e.deltaY < 0 ? lines : -lines;
    setScrollOffset(scrollOffsetRef.current + delta);
  }

  function pointerDown(e: ReactPointerEvent<HTMLElement>) {
    if (e.pointerType === 'mouse') return;
    if (activePointerIdRef.current !== null) return;
    activePointerIdRef.current = e.pointerId;
    pointerLastYRef.current = e.clientY;
    pointerLastXRef.current = e.clientX;
    pointerRemainderRef.current = 0;
    pointerTravelRef.current = 0;
  }

  function pointerMove(e: ReactPointerEvent<HTMLElement>) {
    if (activePointerIdRef.current !== e.pointerId || e.pointerType === 'mouse') return;
    const canvas = ref.current;
    if (!canvas) return;

    // Follow the direct-manipulation convention used by iOS scrolling:
    // pulling the content downward reveals older lines above, while pushing
    // upward moves back toward newer/live output. Scale by displayed row height
    // so the gesture feels the same on iPhone, iPad and desktop-sized canvases.
    const rowHeight = Math.max(1, canvas.clientHeight / layoutRowsRef.current);
    const dragPixels = e.clientY - pointerLastYRef.current;
    const dragX = pointerLastXRef.current - e.clientX;
    pointerTravelRef.current += Math.abs(dragPixels) + Math.abs(dragX);
    if (pointerTravelRef.current >= 8 && !e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.setPointerCapture(e.pointerId);
    }
    if (viewportRef.current) viewportRef.current.scrollLeft += dragX;
    pointerLastXRef.current = e.clientX;
    pointerLastYRef.current = e.clientY;
    pointerRemainderRef.current += dragPixels;

    const lines = Math.trunc(pointerRemainderRef.current / rowHeight);
    if (lines !== 0) {
      pointerRemainderRef.current -= lines * rowHeight;
      setScrollOffset(scrollOffsetRef.current + lines);
    }
    if (pointerTravelRef.current >= 8) e.preventDefault();
  }

  function finishPointer(e: ReactPointerEvent<HTMLElement>) {
    activePointerIdRef.current = null;
    pointerRemainderRef.current = 0;
    pointerTravelRef.current = 0;
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
  }

  function pointerEnd(e: ReactPointerEvent<HTMLElement>) {
    if (activePointerIdRef.current !== e.pointerId) return;
    finishPointer(e);
  }

  function pointerCancel(e: ReactPointerEvent<HTMLElement>) {
    if (activePointerIdRef.current !== e.pointerId) return;
    finishPointer(e);
  }

  return (
    <div className={`terminal-display terminal-display--${display} terminal-display--rows-${screenMode}`}>
      <nav className="terminal-tools" aria-label="端末表示">
        <button type="button" aria-pressed={display === 'readable'} onClick={() => setDisplay('readable')}>文字拡大</button>
        <button type="button" aria-pressed={display === 'fit'} onClick={() => setDisplay('fit')}>全体表示</button>
        <button type="button" onClick={() => setScrollOffset(scrollOffsetRef.current + 12)}>履歴↑</button>
        <button type="button" onClick={() => setScrollOffset(scrollOffsetRef.current - 12)}>履歴↓</button>
        <button type="button" onClick={() => setScrollOffset(0)} disabled={historyOffset === 0}>最新</button>
      </nav>
      <div className="terminal-viewport-shell">
        <div ref={viewportRef} className="terminal-viewport" tabIndex={0} aria-label="端末画面。左右にスクロールできます">
          <canvas
            ref={ref}
            width={640 * backingScale}
            height={layoutRows * 16 * backingScale}
            className="terminal-canvas"
            style={{ '--terminal-rows': layoutRows } as CSSProperties}
            aria-label={`80桁${layoutRows}行の通信端末`}
            onClick={e => { if (e.detail > 0 && window.matchMedia('(min-width: 681px) and (pointer: fine)').matches) focusKeyboardProxy(); }}
            onWheel={wheel}
            onPointerDown={pointerDown}
            onPointerMove={pointerMove}
            onPointerUp={pointerEnd}
            onPointerCancel={pointerCancel}
          />
        </div>
        <input
          ref={keyboardProxyRef}
          className="terminal-input-proxy"
          aria-label="端末入力"
          type="text"
          value={keyboardInput.value}
          readOnly={keyboardInput.readOnly}
          onChange={e => {
            resetCursorBlink();
            keyboardInput.onChange(e);
          }}
          onKeyDown={e => {
            resetCursorBlink();
            keyboardInput.onKeyDown(e);
          }}
          onCompositionStart={e => {
            resetCursorBlink();
            keyboardInput.onCompositionStart(e);
          }}
          onCompositionEnd={e => {
            resetCursorBlink();
            keyboardInput.onCompositionEnd(e);
          }}
          onFocus={event => { setFunctionMenuOpen(false); keyboardInput.onFocus(event); }}
          onBlur={keyboardInput.onBlur}
          onPointerDown={pointerDown}
          onPointerMove={pointerMove}
          onPointerUp={pointerEnd}
          onPointerCancel={pointerCancel}
          autoCapitalize="none"
          autoCorrect="off"
          autoComplete="off"
          inputMode="text"
          enterKeyHint="send"
          spellCheck={false}
        />
        <button
          type="button"
          className="terminal-function-toggle"
          aria-expanded={functionMenuOpen}
          aria-controls="terminal-function-menu"
          onClick={() => setFunctionMenuOpen(open => !open)}
        >
          機能
        </button>
        <nav id="terminal-function-menu" className="terminal-function-menu" aria-label="端末機能" hidden={!functionMenuOpen}>
          <button type="button" aria-pressed={display === 'readable'} onClick={() => { setDisplay('readable'); setFunctionMenuOpen(false); }}>文字拡大</button>
          <button type="button" aria-pressed={display === 'fit'} onClick={() => { setDisplay('fit'); setFunctionMenuOpen(false); }}>全体表示</button>
          <span className="terminal-function-section-label">モデム表示</span>
          <div className="terminal-function-mode-row">
            <button type="button" aria-pressed={modemStatusMode === 'lamps'} onClick={() => { onModemStatusModeChange('lamps'); setFunctionMenuOpen(false); }}>ランプ</button>
            <button type="button" aria-pressed={modemStatusMode === 'digital'} onClick={() => { onModemStatusModeChange('digital'); setFunctionMenuOpen(false); }}>デジタル</button>
            <button type="button" aria-pressed={modemStatusMode === 'off'} onClick={() => { onModemStatusModeChange('off'); setFunctionMenuOpen(false); }}>OFF</button>
          </div>
          <button type="button" onClick={() => { setScrollOffset(scrollOffsetRef.current + 12); setFunctionMenuOpen(false); }}>履歴↑</button>
          <button type="button" onClick={() => { setScrollOffset(scrollOffsetRef.current - 12); setFunctionMenuOpen(false); }}>履歴↓</button>
          <button type="button" onClick={() => { setScrollOffset(0); setFunctionMenuOpen(false); }} disabled={historyOffset === 0}>最新</button>
          <details className="terminal-build-info">
            <summary>バージョン情報</summary>
            <BuildInfoPanel {...buildInfo} />
          </details>
          <button type="button" onClick={() => setFunctionMenuOpen(false)}>閉じる</button>
        </nav>
        {historyOffset > 0 && <button type="button" className="terminal-live-return" onClick={returnToLive}>最新へ</button>}
      </div>
      <p className="terminal-hint">{historyOffset > 0 ? `履歴表示中（${historyOffset}行前） /「最新」で受信画面へ` : display === 'readable' ? '左右にスワイプで移動・上下で受信履歴' : '80桁全体表示・上下スワイプで受信履歴'}</p>
    </div>
  );
});
