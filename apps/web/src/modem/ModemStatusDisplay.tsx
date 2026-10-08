import { useEffect, useState } from 'react';
import type { ModemTelemetry } from './ModemTelemetry';
import { formatModemBaud, protocolIndicators } from './ModemTelemetry';
import './ModemStatusDisplay.css';

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
  // True while the server is generating world data. Drives the AI indicator,
  // which replaces the never-lit AA (auto-answer) indicator.
  generating?: boolean;
};

// Random flicker for the AI indicator. A single lamp that stays lit looks like
// a stuck status, so each step picks a fresh state: lit phases are longer and
// more frequent than dark ones, so it reads as activity rather than a blink.
export function nextFlickerStep(random: () => number): { on: boolean; delayMs: number } {
  const on = random() < 0.7;
  const delayMs = on ? 80 + random() * 320 : 40 + random() * 140;
  return { on, delayMs: Math.round(delayMs) };
}

export function useGenerationFlicker(active: boolean): boolean {
  const [lit, setLit] = useState(false);
  useEffect(() => {
    if (!active) return;
    let timer = 0;
    const tick = () => {
      const step = nextFlickerStep(Math.random);
      setLit(step.on);
      timer = window.setTimeout(tick, step.delayMs);
    };
    tick();
    return () => window.clearTimeout(timer);
  }, [active]);
  return active && lit;
}

// ---------------------------------------------------------------------------
// Seven-segment glyph geometry.
//
// Each digit is drawn as inline SVG in a 30 x 62 cell. Segments are hexagons
// whose pointed ends stop short of the corners, so neighbouring segments are
// separated by a hairline gap (the old CSS-box version let them overlap and the
// digits clumped together). The whole glyph leans right like the photographed
// LCD, and unlit segments stay visible as a faint ghost.
// ---------------------------------------------------------------------------
const SEGMENT_ORDER = ['a', 'b', 'c', 'd', 'e', 'f', 'g'] as const;
type SegmentId = (typeof SEGMENT_ORDER)[number];

const THICKNESS = 6;
const GAP = 1.6;
const HALF = THICKNESS / 2;
const LEFT = 3;
const RIGHT = 23;
const TOP = 3;
const MIDDLE = 31;
const BOTTOM = 59;
const DIGIT_HEIGHT = 62;
const SKEW = Math.tan((7 * Math.PI) / 180);

const round = (value: number) => String(Math.round(value * 100) / 100);
const points = (coords: number[]) =>
  coords.reduce<string[]>((pairs, value, index) => {
    if (index % 2 === 0) pairs.push(`${round(value)},${round(coords[index + 1])}`);
    return pairs;
  }, []).join(' ');

function horizontalSegment(y: number) {
  const start = LEFT + GAP;
  const end = RIGHT - GAP;
  return points([
    start, y,
    start + HALF, y - HALF,
    end - HALF, y - HALF,
    end, y,
    end - HALF, y + HALF,
    start + HALF, y + HALF,
  ]);
}

function verticalSegment(x: number, from: number, to: number) {
  const start = from + GAP;
  const end = to - GAP;
  return points([
    x, start,
    x + HALF, start + HALF,
    x + HALF, end - HALF,
    x, end,
    x - HALF, end - HALF,
    x - HALF, start + HALF,
  ]);
}

const SEGMENT_POINTS: Record<SegmentId, string> = {
  a: horizontalSegment(TOP),
  b: verticalSegment(RIGHT, TOP, MIDDLE),
  c: verticalSegment(RIGHT, MIDDLE, BOTTOM),
  d: horizontalSegment(BOTTOM),
  e: verticalSegment(LEFT, MIDDLE, BOTTOM),
  f: verticalSegment(LEFT, TOP, MIDDLE),
  g: horizontalSegment(MIDDLE),
};

const skewTransform = (height: number) =>
  `matrix(1 0 ${Math.round(-SKEW * 10000) / 10000} 1 ${round(SKEW * height)} 0)`;

function SevenSegmentDigit({ digit }: { digit: string }) {
  const active = sevenSegmentMap[digit] ?? [];
  return (
    <svg
      className="modem-lcd__glyph"
      viewBox={`0 0 30 ${DIGIT_HEIGHT}`}
      aria-hidden="true"
      focusable="false"
    >
      <g transform={skewTransform(DIGIT_HEIGHT)}>
        {SEGMENT_ORDER.map(segment => (
          <polygon
            key={segment}
            className={`modem-lcd__seg modem-lcd__seg--${segment}`}
            data-on={active.includes(segment) ? 'true' : 'false'}
            points={SEGMENT_POINTS[segment]}
          />
        ))}
      </g>
    </svg>
  );
}

// "K" drawn with strokes of the same weight as the digit segments.
function KiloGlyph() {
  return (
    <svg className="modem-lcd__unit-glyph" viewBox="0 0 26 34" aria-hidden="true" focusable="false">
      <g
        transform={skewTransform(34)}
        fill="none"
        stroke="currentColor"
        strokeWidth="4.6"
        strokeLinejoin="miter"
      >
        <path d="M3 0V34" />
        <path d="M3 20L17 0" />
        <path d="M5 17.5L19 34" />
      </g>
    </svg>
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
            {character === 'K' ? <KiloGlyph /> : character}
          </span>
        );
      })}
    </strong>
  );
}

export function ModemStatusDisplay({ mode, telemetry, dteBaud, generating = false }: ModemStatusDisplayProps) {
  const aiLit = useGenerationFlicker(generating);
  if (mode === 'off') return null;

  if (mode === 'lamps') {
    const lamps: ReadonlyArray<readonly [string, boolean]> = [
      ['MR', telemetry.mr],
      ['TR', telemetry.tr],
      ['SD', telemetry.sd],
      ['RD', telemetry.rd],
      ['OH', telemetry.oh],
      ['CD', telemetry.cd],
      ['AI', aiLit],
      ['HS', telemetry.hs],
    ];
    return (
      <div className="mobile-modem-status mobile-modem-status--lamps" aria-label="モデム状態 ランプ表示">
        {lamps.map(([label, lit]) => (
          <span className="modem-lamp" key={label}>
            <span className="modem-lamp__label">{label}</span>
            <span
              className="modem-lamp__led"
              data-on={lit ? 'true' : 'false'}
              aria-label={`${label} ${lit ? '点灯' : '消灯'}`}
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
      <div className="modem-lcd__signals" aria-label="モデム信号">
        <span data-on={telemetry.tr ? 'true' : 'false'}>DTR</span>
        <span data-on={telemetry.dsr ? 'true' : 'false'}>DSR</span>
        <span data-on="false">RTS</span>
        <span data-on={telemetry.cts ? 'true' : 'false'}>CTS</span>
        <span data-on={aiLit ? 'true' : 'false'}>AI</span>
        <span data-on={telemetry.cd ? 'true' : 'false'}>DCD</span>
      </div>
    </div>
  );
}
