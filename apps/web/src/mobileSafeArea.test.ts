import { describe, expect, it } from 'vitest';
import indexHtml from '../index.html?raw';
import styles from './styles.css?raw';

describe('mobile safe-area layout', () => {
  it('keeps the viewport inside Safari\'s native safe area', () => {
    expect(indexHtml).toContain('viewport-fit=auto');
    expect(indexHtml).not.toContain('viewport-fit=cover');
  });

  it('keeps the connection row compact without duplicate top insets', () => {
    const mobileStyles = styles.slice(styles.indexOf('@media (max-width: 680px), (pointer: coarse)'));
    const shellRule = mobileStyles.match(/\.shell \{([^}]*)\}/)?.[1];
    const statusRule = mobileStyles.match(/\.mobile-statusbar \{([^}]*)\}/)?.[1];

    expect(shellRule).toBeDefined();
    expect(shellRule).not.toContain('safe-area-inset-top');
    expect(statusRule).toContain('flex: 0 0 24px');
    expect(statusRule).toContain('padding: 0 6px');
    expect(statusRule).not.toContain('safe-area-inset-top');
    expect(styles).not.toContain('--mobile-safe-area-top');
  });
});
