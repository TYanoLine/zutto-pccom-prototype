import { describe, expect, it } from 'vitest';
import { isVisualViewportZoomed, mobileTerminalRows } from './terminalViewport';

describe('mobile pinch zoom stability', () => {
  it('identifies unzoomed visual viewport', () => {
    expect(isVisualViewportZoomed(null)).toBe(false);
    expect(isVisualViewportZoomed(undefined)).toBe(false);
    expect(isVisualViewportZoomed({ scale: 1 })).toBe(false);
    expect(isVisualViewportZoomed({ scale: 1.004 })).toBe(false);
    expect(isVisualViewportZoomed({ scale: 0.995 })).toBe(false);
  });

  it('identifies zoomed visual viewport during pinch gestures', () => {
    expect(isVisualViewportZoomed({ scale: 1.05 })).toBe(true);
    expect(isVisualViewportZoomed({ scale: 1.5 })).toBe(true);
    expect(isVisualViewportZoomed({ scale: 2.0 })).toBe(true);
    expect(isVisualViewportZoomed({ scale: 0.9 })).toBe(true);
  });

  it('demonstrates why row recalculation must be skipped when zoomed in', () => {
    const unzoomedAvailableHeight = 600;
    const canvasWidth = 360;
    const initialRows = mobileTerminalRows(canvasWidth, unzoomedAvailableHeight);
    expect(initialRows).toBe(66);

    // If visualViewport.height is reduced by 2x zoom (300px), recalculating rows
    // would drastically shrink the terminal height to 33 rows, causing DOM jitter
    const zoomedHeight = unzoomedAvailableHeight / 2;
    const shrunkRows = mobileTerminalRows(canvasWidth, zoomedHeight);
    expect(shrunkRows).toBe(33);

    // By skipping recalculation when isVisualViewportZoomed is true,
    // the terminal maintains its stable 66-row layout.
    const preservedRows = isVisualViewportZoomed({ scale: 2.0 }) ? initialRows : shrunkRows;
    expect(preservedRows).toBe(66);
  });
});
