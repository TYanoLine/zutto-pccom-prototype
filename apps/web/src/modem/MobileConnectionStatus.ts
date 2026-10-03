import type { ModemPhase } from './ModemTelemetry';

export type MobileConnectionStatus = {
  label: string;
  working: boolean;
};

export function resolveMobileConnectionStatus(
  phase: ModemPhase,
  connectedName: string | null,
): MobileConnectionStatus {
  switch (phase) {
    case 'dialing':
      return { label: '発信中…', working: true };
    case 'ringing':
      return { label: '呼出中…', working: true };
    case 'negotiating':
      return { label: '接続処理中…', working: true };
    case 'recovering':
      return { label: '再接続中…', working: true };
    case 'online':
      return { label: connectedName ?? '接続中', working: false };
    case 'idle':
    default:
      return { label: 'オフライン', working: false };
  }
}
