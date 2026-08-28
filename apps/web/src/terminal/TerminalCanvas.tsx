import { useEffect, useRef } from 'react';
import type { TerminalCore } from './TerminalCore';

const PALETTE = ['#000000', '#aa0000', '#00aa00', '#aa5500', '#0000aa', '#aa00aa', '#00aaaa', '#aaaaaa'];

export function TerminalCanvas({ terminal }: { terminal: TerminalCore }) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => terminal.subscribe(draw), [terminal]);
  useEffect(() => { draw(); });

  function draw() {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.fillStyle = '#000000';
    ctx.fillRect(0, 0, 640, 400);
    ctx.textBaseline = 'top';
    ctx.font = '16px monospace';

    // Paint every cell's background first, including the continuation cell of
    // a full-width character.  A continuation is still a real 8x16 terminal
    // cell; skipping it leaves a black stripe through ANSI reverse/highlight
    // regions containing Japanese text.
    for (let y = 0; y < terminal.height; y++) {
      for (let x = 0; x < terminal.width; x++) {
        const cell = terminal.cells[y][x];
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
        const cell = terminal.cells[y][x];
        if (cell.continuation) continue;
        const px = x * 8;
        const py = y * 16;
        ctx.fillStyle = PALETTE[cell.fg] ?? '#aaa';
        ctx.fillText(cell.ch, px, py);
        if (cell.bold && cell.ch !== ' ') ctx.fillText(cell.ch, px + 1, py);
      }
    }

    // Cursor block. The UI rerenders often enough for the prototype; blink comes later.
    ctx.globalAlpha = 0.65;
    ctx.fillStyle = '#aaaaaa';
    ctx.fillRect(terminal.cursorX * 8, terminal.cursorY * 16 + 14, 8, 2);
    ctx.globalAlpha = 1;
  }

  return <canvas ref={ref} width={640} height={400} className="terminal-canvas" />;
}
