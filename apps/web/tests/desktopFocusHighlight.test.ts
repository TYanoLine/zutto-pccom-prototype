import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const styles = readFileSync(new URL('../src/styles.css', import.meta.url), 'utf8');

describe('desktop terminal focus highlight', () => {
  it('highlights canvas and viewport frame when keyboard focus is active on desktop', () => {
    const desktopFocusCanvasRule = styles.match(
      /\.terminal-viewport-shell:focus-within \.terminal-canvas,\s*\.terminal-display--focused \.terminal-canvas \{([^}]*)\}/,
    )?.[1];
    const desktopFocusViewportRule = styles.match(
      /\.terminal-viewport-shell:focus-within \.terminal-viewport,\s*\.terminal-display--focused \.terminal-viewport \{([^}]*)\}/,
    )?.[1];

    expect(desktopFocusCanvasRule).toBeDefined();
    expect(desktopFocusCanvasRule).toContain('box-shadow: 0 0 0 1px #b3832b');
    expect(desktopFocusCanvasRule).toContain('rgba(226, 173, 76, 0.24)');

    expect(desktopFocusViewportRule).toBeDefined();
    expect(desktopFocusViewportRule).toContain('box-shadow: 0 0 0 1px #383838');
  });

  it('provides subtle resting borders and text cursor for desktop terminal', () => {
    const viewportRule = styles.match(/^\.terminal-viewport \{([^}]*)\}/m)?.[1];
    const canvasRule = styles.match(/^\.terminal-canvas \{([^}]*)\}/m)?.[1];

    expect(viewportRule).toContain('cursor: text');
    expect(viewportRule).toContain('box-shadow: 0 0 0 1px #1a1a1a');

    expect(canvasRule).toContain('cursor: text');
    expect(canvasRule).toContain('box-shadow: 0 0 0 1px #252525');
  });

  it('resets desktop focus shadow and cursor in mobile query', () => {
    const mobileStyles = styles.slice(styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));
    const mobileFocusCanvasRule = mobileStyles.match(
      /\.terminal-viewport-shell:focus-within \.terminal-canvas,\s*\.terminal-display--focused \.terminal-canvas \{([^}]*)\}/,
    )?.[1];
    const mobileViewportRule = mobileStyles.match(/\n\s*\.terminal-viewport \{([^}]*)\}/)?.[1];

    expect(mobileFocusCanvasRule).toBeDefined();
    expect(mobileFocusCanvasRule).toContain('box-shadow: none');
    expect(mobileViewportRule).toContain('cursor: default');
  });
});
