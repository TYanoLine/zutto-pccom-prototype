import { useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent, WheelEvent as ReactWheelEvent } from 'react';
import type { TerminalCore } from './TerminalCore';

const PALETTE = ['#000000', '#aa0000', '#00aa00', '#aa5500', '#0000aa', '#aa00aa', '#00aaaa', '#aaaaaa'];

export function TerminalCanvas({ terminal }: { terminal: TerminalCore }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const scrollOffsetRef = useRef(0);
  const previousScrollbackLengthRef = useRef(terminal.scrollbackLength);
  const activePointerIdRef = useRef<number | null>(null);
  const pointerLastYRef = useRef(0);
  const pointerRemainderRef = useRef(0);

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
    draw();
  }), [terminal]);
  useEffect(() => { draw(); });

  function setScrollOffset(next: number) {
    const clamped = Math.max(0, Math.min(terminal.maxScrollOffset, next));
    if (clamped === scrollOffsetRef.current) return;
    scrollOffsetRef.current = clamped;
    draw();
  }

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
    activePointerIdRef.current = e.pointerId;
    pointerLastYRef.current = e.clientY;
    pointerRemainderRef.current = 0;
    e.currentTarget.setPointerCapture(e.pointerId);
    e.preventDefault();
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
    pointerLastYRef.current = e.clientY;
    pointerRemainderRef.current += dragPixels;

    const lines = Math.trunc(pointerRemainderRef.current / rowHeight);
    if (lines !== 0) {
      pointerRemainderRef.current -= lines * rowHeight;
      setScrollOffset(scrollOffsetRef.current + lines);
    }
    e.preventDefault();
  }

  function pointerEnd(e: ReactPointerEvent<HTMLCanvasElement>) {
    if (activePointerIdRef.current !== e.pointerId) return;
    activePointerIdRef.current = null;
    pointerRemainderRef.current = 0;
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
    e.preventDefault();
  }

  return (
    <canvas
      ref={ref}
      width={640}
      height={400}
      className="terminal-canvas"
      onWheel={wheel}
      onPointerDown={pointerDown}
      onPointerMove={pointerMove}
      onPointerUp={pointerEnd}
      onPointerCancel={pointerEnd}
    />
  );
}
