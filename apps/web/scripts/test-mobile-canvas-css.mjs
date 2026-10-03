import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const stylesheet = readFileSync(new URL('../src/styles.css', import.meta.url), 'utf8');
const mobileStart = stylesheet.indexOf('@media (max-width: 680px), (pointer: coarse) {');
const mobileEnd = stylesheet.lastIndexOf('}');

assert.notEqual(mobileStart, -1, 'mobile presentation media query must exist');
assert.ok(mobileEnd > mobileStart, 'mobile media query must have a closing brace');

const mobileStyles = stylesheet.slice(mobileStart, mobileEnd);
assert.match(
  mobileStyles,
  /\.terminal-canvas\s*\{\s*max-height:\s*none;\s*\}/,
  'mobile terminal canvas must not inherit the desktop max-height constraint',
);
assert.match(
  mobileStyles,
  /\.terminal-display--fit \.terminal-canvas\s*\{[^}]*height:\s*auto;/,
  'fit-mode canvas must keep its natural height when the visual viewport shrinks',
);
assert.match(
  mobileStyles,
  /\.terminal-display--readable \.terminal-canvas\s*\{[^}]*height:\s*auto;/,
  'readable-mode canvas must keep its natural height when the visual viewport shrinks',
);

console.log('Mobile terminal canvas CSS regression checks passed.');
