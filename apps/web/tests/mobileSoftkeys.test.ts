import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const appSource = readFileSync(new URL('../src/App.tsx', import.meta.url), 'utf8');
const terminalCanvasSource = readFileSync(new URL('../src/terminal/TerminalCanvas.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/styles.css', import.meta.url), 'utf8');

describe('mobile dedicated softkeys and hangup', () => {
  it('hides mobile-softkeys on desktop and displays on mobile layout', () => {
    const desktopStyles = styles.slice(0, styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));
    const mobileStyles = styles.slice(styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));

    expect(desktopStyles).toContain('.mobile-softkeys { display: none; }');

    const softkeysRule = mobileStyles.match(/\.mobile-softkeys \{([^}]*)\}/)?.[1]
      ?? mobileStyles.match(/\.directory-softkeys, \.mobile-softkeys \{([^}]*)\}/)?.[1];
    expect(softkeysRule).toBeDefined();
    expect(softkeysRule).toContain('position: fixed');
    expect(softkeysRule).toContain('bottom: env(safe-area-inset-bottom)');
    expect(softkeysRule).toContain('grid-template-columns: repeat(7, 1fr)');
  });

  it('renders all dedicated softkeys (ESC, ArrowUp, ArrowDown, ArrowLeft, ArrowRight, Enter/Call, HangUp)', () => {
    expect(appSource).toContain('className="mobile-softkeys directory-softkeys"');
    expect(appSource).toContain("softKey('Escape')");
    expect(appSource).toContain("softKey('ArrowUp')");
    expect(appSource).toContain("softKey('ArrowDown')");
    expect(appSource).toContain("softKey('ArrowLeft')");
    expect(appSource).toContain("softKey('ArrowRight')");
    expect(appSource).toContain("softKey('Enter')");
    expect(appSource).toContain("softKey('HangUp')");
  });

  it('provides hangup functionality and wires to modem/station hangup', () => {
    expect(appSource).toContain('function hangUp()');
    expect(appSource).toContain('localStationRef.current.hangup(true)');
    expect(appSource).toContain('modemRef.current?.hangup()');

    // Desktop menu hangup calls hangUp
    expect(appSource).toContain("onClick={() => { setDesktopMenuOpen(false); hangUp(); }}");

    // TerminalCanvas supports onHangUp in function menu
    expect(terminalCanvasSource).toContain('onHangUp');
    expect(terminalCanvasSource).toContain('terminal-function-hangup');
  });

  it('adjusts terminal row calculation using mobile-softkeys bounding rect', () => {
    expect(terminalCanvasSource).toContain("document.querySelector<HTMLElement>('.mobile-softkeys')");
  });
});
