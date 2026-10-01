import { describe, expect, it } from 'vitest';
import type { ModemPhase } from './ModemTelemetry';
import { resolveMobileConnectionStatus } from './MobileConnectionStatus';

describe('mobile connection status', () => {
  it.each([
    ['idle', null, 'オフライン', false],
    ['dialing', null, '発信中…', true],
    ['ringing', null, '呼出中…', true],
    ['negotiating', null, '接続処理中…', true],
    ['recovering', 'MAPLE TOWN', '再接続中…', true],
    ['online', 'MAPLE TOWN', 'MAPLE TOWN', false],
    ['online', null, '接続中', false],
  ] satisfies Array<[ModemPhase, string | null, string, boolean]>)(
    'maps %s to %s',
    (phase, connectedName, label, working) => {
      expect(resolveMobileConnectionStatus(phase, connectedName)).toEqual({
        label,
        working,
      });
    },
  );
});
