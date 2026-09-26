import type { ModemTelemetry } from './ModemTelemetry';
import { formatModemBaud, protocolIndicators } from './ModemTelemetry';

export type ModemStatusDisplayMode = 'lamps' | 'digital' | 'off';

const STORAGE_KEY = 'zutto.modemStatusDisplay.v1';

const sevenSegmentMap: Record<string, readonly string[]> = {
  '0': ['a', 'b', 'c', 'd', 'e', 'f'],
  '1': ['b', 'c'],
  '2': ['a', 'b', 'd', 'e', 'g'],
  '3': ['a', 'b', 'c', 'd', 'g'],
  '4': ['b', 'c', 'f', 'g'],
  '5': ['a', 'c', 'd', 'f', 'g'],
  '6': ['a', 'c', 'd', 'e', 'f', 'g'],
  '7': ['a', 'b', 'c'],
  '8': ['a', 'b', 'c', 'd', 'e', 'f', 'g'],
  '9': ['a', 'b', 'c', 'd', 'f', 'g'],
};

export function loadModemStatusDisplayMode(): ModemStatusDisplayMode {
  if (typeof window === 'undefined') return 'lamps';
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return value === 'digital' || value === 'off' || value === 'lamps' ? value : 'lamps';
  } catch {
    return 'lamps';
  }
}

export function saveModemStatusDisplayMode(mode: ModemStatusDisplayMode) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Optional preference persistence; the default remains the lamp display.
  }
}

type ModemStatusDisplayProps = {
  mode: ModemStatusDisplayMode;
  telemetry: ModemTelemetry;
  dteBaud: number;
};

const lampEntries = [
  ['MR', 'mr'],
  ['TR', 'tr'],
  ['SD', 'sd'],
  ['RD', 'rd'],
  ['OH', 'oh'],
  ['CD', 'cd'],
  ['AA', 'aa'],
  ['HS', 'hs'],
] as const;

function SevenSegmentDigit({ digit }: { digit: string }) {
  const active = sevenSegmentMap[digit] ?? [];
  return (
    <span className="modem-lcd__digit" aria-hidden="true">
      {(['a', 'b', 'c', 'd', 'e', 'f', 'g'] as const).map(segment => (
        <span
          key={segment}
          className={`modem-lcd__segment modem-lcd__segment--${segment}`}
          data-on={active.includes(segment) ? 'true' : 'false'}
        />
      ))}
    </span>
  );
}

function SegmentedSpeed({ value }: { value: string }) {
  return (
    <strong className="modem-lcd__speed" aria-label={value}>
      {Array.from(value).map((character, index) => {
        if (/\d/.test(character)) {
          return <SevenSegmentDigit key={`${character}-${index}`} digit={character} />;
        }
        if (character === '.') {
          return <span className="modem-lcd__decimal" key={`decimal-${index}`} aria-hidden="true" />;
        }
        return (
          <span className="modem-lcd__speed-unit" key={`${character}-${index}`} aria-hidden="true">
            {character}
          </span>
        );
      })}
    </strong>
  );
}

export function ModemStatusDisplay({ mode, telemetry, dteBaud }: ModemStatusDisplayProps) {
  if (mode === 'off') return null;

  if (mode === 'lamps') {
    return (
      <div className="mobile-modem-status mobile-modem-status--lamps" aria-label="モデム状態 ランプ表示">
        {lampEntries.map(([label, key]) => (
          <span className="modem-lamp" key={label}>
            <span className="modem-lamp__label">{label}</span>
            <span
              className="modem-lamp__led"
              data-on={telemetry[key] ? 'true' : 'false'}
              aria-label={`${label} ${telemetry[key] ? '点灯' : '消灯'}`}
            />
          </span>
        ))}
      </div>
    );
  }

  const protocol = protocolIndicators(telemetry);
  const speed = formatModemBaud(telemetry.baud, dteBaud);

  return (
    <div className="mobile-modem-status mobile-modem-status--digital" aria-label="モデム状態 デジタル表示">
      <div className="modem-lcd__protocols">
        <span data-on={protocol.v42bis ? 'true' : 'false'}>V.42bis</span>
        <span className="modem-lcd__mnp">
          <span data-on={protocol.mnp ? 'true' : 'false'}>MNP</span>
          <span data-on={protocol.mnp5 ? 'true' : 'false'}>5</span>
        </span>
      </div>
      <SegmentedSpeed value={speed} />
      <div className="modem-lcd__ofh">
        <span className="modem-lcd__dot" data-on={telemetry.oh ? 'true' : 'false'} />
        <span data-on={telemetry.oh ? 'true' : 'false'}>OFH</span>
      </div>
      <div className="modem-lcd__signals">
        <span data-on={telemetry.dsr ? 'true' : 'false'}>DSR</span>
        <span data-on={telemetry.cts ? 'true' : 'false'}>CTS</span>
      </div>
    </div>
  );
}
