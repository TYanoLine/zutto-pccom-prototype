import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent, WheelEvent as ReactWheelEvent } from 'react';
import type { TerminalCore } from './TerminalCore';
import { terminalCursorTargetScrollTop, terminalViewportHeight } from './terminalViewport';

const PALETTE = ['#000000', '#aa0000', '#00aa00', '#aa5500', '#0000aa', '#aa00aa', '#00aaaa', '#aaaaaa'];

export type TerminalCanvasHandle = {
  returnToLive: () => void;
  ensureCursorVisible: () => void;
};

type TerminalCanvasProps = {
  terminal: TerminalCore;
  onKeyboardRequest?: () => void;
  keyboardActive?: boolean;
};

export const TerminalCanvas = forwardRef<TerminalCanvasHandle, TerminalCanvasProps>(function TerminalCanvas(
  { terminal, onKeyboardRequest, keyboardActive = false },
  forwardedRef,
) {
  const ref = useRef<HTMLCanvasElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const [display, setDisplay] = useState<'readable' | 'fit'>('readable');
  const [historyOffset, setHistoryOffset] = useState(0);
  const scrollOffsetRef = useRef(0);
  const previousScrollbackLengthRef = useRef(terminal.scrollbackLength);
  const activePointerIdRef = useRef<number | null>(null);
  const pointerLastYRef = useRef(0);
  const pointerLastXRef = useRef(0);
  const pointerRemainderRef = useRef(0);
  const pointerTravelRef = useRef(0);

  useEffect(() => terminal.subscribe(() => {
    const previousLength = previousScrollbackLengthRef.current;
    const nextLength = terminal.scrollbackLength;
    const added = Math.max(0, nextLength - previousLength);

    // If the user is reading history, keep the same historical text under the
    // viewport while new live lines arrive. At the live bottom, keep following
    // output exactly as a normal communications terminal does.
    if (scrollOffsetRef.current > 0 && added > 0) {
      scrollOffsetRef.current = Math.min(terminal.maxScrollOffset, scrollOffsetRef.current + added);
    } else {
      scrollOffsetRef.current = Math.min(terminal.maxScrollOffset, scrollOffsetRef.current);
    }
    previousScrollbackLengthRef.current = nextLength;
    setHistoryOffset(scrollOffsetRef.current);
    draw();
    if (keyboardActive) window.requestAnimationFrame(ensureCursorVisible);
  }), [terminal, keyboardActive]);
  useEffect(() => { draw(); });

  function setScrollOffset(next: number) {
    const clamped = Math.max(0, Math.min(terminal.maxScrollOffset, next));
    if (clamped === scrollOffsetRef.current) return;
    scrollOffsetRef.current = clamped;
    setHistoryOffset(clamped);
    draw();
  }

  function isMobilePresentation() {
    return window.matchMedia('(max-width: 680px), (pointer: coarse)').matches;
  }

  function resetKeyboardViewport() {
    const viewport = viewportRef.current;
    if (!viewport) return;
    viewport.style.maxHeight = '';
    viewport.style.overflowY = '';
    viewport.scrollTop = 0;
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

    const rowHeight = Math.max(1, canvas.clientHeight / terminal.height);
    const cursorTop = terminal.cursorY * rowHeight;
    const targetScrollTop = terminalCursorTargetScrollTop(
      canvas.clientHeight,
      viewport.clientHeight,
      cursorTop,
      rowHeight,
    );
    if (Math.abs(viewport.scrollTop - targetScrollTop) > 1) viewport.scrollTop = targetScrollTop;
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
    update();

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
    const rows = terminal.viewportRows(scrollOffsetRef.current);
    ctx.fillStyle = '#000000';
    ctx.fillRect(0, 0, 640, 400);
    ctx.textBaseline = 'top';
    ctx.font = '16px monospace';

    // Paint every cell's background first, including the continuation cell of
    // a full-width character. A continuation is still a real 8x16 terminal
    // cell; skipping it leaves a black stripe through ANSI reverse/highlight
    // regions containing Japanese text.
    for (let y = 0; y < terminal.height; y++) {
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
    for (let y = 0; y < terminal.height; y++) {
      for (let x = 0; x < terminal.width; x++) {
        const cell = rows[y][x];
        if (cell.continuation) continue;
        const px = x * 8;
        const py = y * 16;
        ctx.fillStyle = PALETTE[cell.fg] ?? '#aaa';
        ctx.fillText(cell.ch, px, py);
        if (cell.bold && cell.ch !== ' ') ctx.fillText(cell.ch, px + 1, py);
      }
    }

    // The live cursor has no meaning while looking back through history.
    if (scrollOffsetRef.current === 0) {
      ctx.globalAlpha = 0.65;
      ctx.fillStyle = '#aaaaaa';
      ctx.fillRect(terminal.cursorX * 8, terminal.cursorY * 16 + 14, 8, 2);
      ctx.globalAlpha = 1;
    }
  }

  function wheel(e: ReactWheelEvent<HTMLCanvasElement>) {
    if (terminal.maxScrollOffset === 0 || e.deltaY === 0) return;
    e.preventDefault();
    const lines = Math.max(1, Math.min(8, Math.round(Math.abs(e.deltaY) / 32)));
    const delta = e.deltaY < 0 ? lines : -lines;
    setScrollOffset(scrollOffsetRef.current + delta);
  }

  function pointerDown(e: ReactPointerEvent<HTMLCanvasElement>) {
    if (e.pointerType === 'mouse') return;
    if (activePointerIdRef.current !== null) return;
    activePointerIdRef.current = e.pointerId;
    pointerLastYRef.current = e.clientY;
    pointerLastXRef.current = e.clientX;
    pointerRemainderRef.current = 0;
    pointerTravelRef.current = 0;
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function pointerMove(e: ReactPointerEvent<HTMLCanvasElement>) {
    if (activePointerIdRef.current !== e.pointerId || e.pointerType === 'mouse') return;
    const canvas = ref.current;
    if (!canvas) return;

    // Follow the direct-manipulation convention used by iOS scrolling:
    // pulling the content downward reveals older lines above, while pushing
    // upward moves back toward newer/live output. Scale by displayed row height
    // so the gesture feels the same on iPhone, iPad and desktop-sized canvases.
    const rowHeight = Math.max(1, canvas.clientHeight / terminal.height);
    const dragPixels = e.clientY - pointerLastYRef.current;
    const dragX = pointerLastXRef.current - e.clientX;
    pointerTravelRef.current += Math.abs(dragPixels) + Math.abs(dragX);
    if (viewportRef.current) viewportRef.current.scrollLeft += dragX;
    pointerLastXRef.current = e.clientX;
    pointerLastYRef.current = e.clientY;
    pointerRemainderRef.current += dragPixels;

    const lines = Math.trunc(pointerRemainderRef.current / rowHeight);
    if (lines !== 0) {
      pointerRemainderRef.current -= lines * rowHeight;
      setScrollOffset(scrollOffsetRef.current + lines);
    }
  }

  function finishPointer(e: ReactPointerEvent<HTMLCanvasElement>) {
    activePointerIdRef.current = null;
    pointerRemainderRef.current = 0;
    pointerTravelRef.current = 0;
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
  }

  function pointerEnd(e: ReactPointerEvent<HTMLCanvasElement>) {
    if (activePointerIdRef.current !== e.pointerId) return;
    const wasTap = e.pointerType !== 'mouse' && pointerTravelRef.current < 8;
    finishPointer(e);
    if (wasTap) onKeyboardRequest?.();
  }

  function pointerCancel(e: ReactPointerEvent<HTMLCanvasElement>) {
    if (activePointerIdRef.current !== e.pointerId) return;
    finishPointer(e);
  }

  return (
    <div className={`terminal-display terminal-display--${display}`}>
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
            width={640}
            height={400}
            className="terminal-canvas"
            aria-label="80桁25行の通信端末"
            onClick={e => { if (e.detail > 0 && window.matchMedia('(min-width: 681px) and (pointer: fine)').matches) onKeyboardRequest?.(); }}
            onWheel={wheel}
            onPointerDown={pointerDown}
            onPointerMove={pointerMove}
            onPointerUp={pointerEnd}
            onPointerCancel={pointerCancel}
          />
        </div>
        {historyOffset > 0 && <button type="button" className="terminal-live-return" onClick={returnToLive}>最新へ</button>}
      </div>
      <p className="terminal-hint">{historyOffset > 0 ? `履歴表示中（${historyOffset}行前） /「最新」で受信画面へ` : display === 'readable' ? '左右にスワイプで移動・上下で受信履歴' : '80桁全体表示・上下スワイプで受信履歴'}</p>
    </div>
  );
});
