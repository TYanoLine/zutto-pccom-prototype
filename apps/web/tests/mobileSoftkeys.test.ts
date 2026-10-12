import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const appSource = readFileSync(new URL('../src/App.tsx', import.meta.url), 'utf8');
const terminalCanvasSource = readFileSync(new URL('../src/terminal/TerminalCanvas.tsx', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../src/styles.css', import.meta.url), 'utf8');

describe('mobile dedicated softkeys and menu hangup', () => {
  it('hides mobile-softkeys on desktop and displays on mobile layout with 6 columns', () => {
    const desktopStyles = styles.slice(0, styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));
    const mobileStyles = styles.slice(styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));

    expect(desktopStyles).toContain('.mobile-softkeys { display: none; }');

    const softkeysRule = mobileStyles.match(/\.mobile-softkeys \{([^}]*)\}/)?.[1]
      ?? mobileStyles.match(/\.directory-softkeys, \.mobile-softkeys \{([^}]*)\}/)?.[1];
    expect(softkeysRule).toBeDefined();
    expect(softkeysRule).toContain('position: fixed');
    expect(softkeysRule).toContain('bottom: env(safe-area-inset-bottom)');
    expect(softkeysRule).toContain('grid-template-columns: repeat(6, 1fr)');
  });

  it('renders 6 dedicated softkeys (ESC, ArrowUp, ArrowDown, ArrowLeft, ArrowRight, Enter/Call)', () => {
    expect(appSource).toContain('className="mobile-softkeys directory-softkeys"');
    expect(appSource).toContain("softKey('Escape')");
    expect(appSource).toContain("softKey('ArrowUp')");
    expect(appSource).toContain("softKey('ArrowDown')");
    expect(appSource).toContain("softKey('ArrowLeft')");
    expect(appSource).toContain("softKey('ArrowRight')");
    expect(appSource).toContain("softKey('Enter')");
    // Hangup button is in the function menu, not on the dedicated softkey bar
    expect(appSource).not.toContain("className=\"mobile-softkeys directory-softkeys\"\n      <button type=\"button\" onPointerDown={e => e.preventDefault()} onClick={() => softKey('HangUp')}");
  });

  it('provides hangup functionality in menus wired to modem/station hangup', () => {
    expect(appSource).toContain('function hangUp()');
    expect(appSource).toContain('localStationRef.current.hangup(true)');
    expect(appSource).toContain('modemRef.current?.hangup()');

    // Desktop menu hangup calls hangUp
    expect(appSource).toContain("onClick={() => { setDesktopMenuOpen(false); hangUp(); }}");

    // TerminalCanvas mobile function menu provides hangup button
    expect(terminalCanvasSource).toContain('onHangUp');
    expect(terminalCanvasSource).toContain('terminal-function-hangup');
  });

  it('adjusts terminal row calculation using mobile-softkeys bounding rect', () => {
    expect(terminalCanvasSource).toContain("document.querySelector<HTMLElement>('.mobile-softkeys')");
  });
});
