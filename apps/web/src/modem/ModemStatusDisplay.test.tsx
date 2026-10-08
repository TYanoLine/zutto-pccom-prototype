import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { createIdleModemTelemetry } from './ModemTelemetry';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import { ModemStatusDisplay, nextFlickerStep } from './ModemStatusDisplay';

describe('PV-AF-style digital modem display', () => {
  it('renders the variable speed as seven-segment digits while keeping indicator legends as fixed text', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
      />,
    );

    expect(html.match(/modem-lcd__glyph/g)).toHaveLength(3);
    expect(html).toContain('modem-lcd__decimal');
    expect(html).toContain('modem-lcd__speed-unit');
    expect(html).toContain('V.42bis');
    expect(html).toContain('MNP');
    expect(html).toContain('OFH');
    expect(html).toContain('DTR');
    expect(html).toContain('DSR');
    expect(html).toContain('RTS');
    expect(html).toContain('CTS');
    expect(html).toContain('AI');
    expect(html).not.toContain('>AA<');
    expect(html).toContain('DCD');
  });

  it('keeps currently unmodeled RTS present but unlit', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
      />,
    );

    expect(html).toContain('<span data-on="false">RTS</span>');
  });

  it('renders lit and ghosted segments for an LCD-like fixed segment field', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
      />,
    );

    expect(html).toContain('modem-lcd__seg--a');
    expect(html).toContain('data-on="true"');
    expect(html).toContain('data-on="false"');
  });

  it('lights exactly the segments of each digit and ghosts the rest', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
      />,
    );

    // "38.4K": 3 lights 5 segments, 8 lights 7, 4 lights 4 -> 16 lit, 5 ghosted.
    expect(html.match(/modem-lcd__seg modem-lcd__seg--[a-g]" data-on="true"/g)).toHaveLength(16);
    expect(html.match(/modem-lcd__seg modem-lcd__seg--[a-g]" data-on="false"/g)).toHaveLength(5);
  });

  it('draws the kilo unit as a glyph instead of font text', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={14400}
      />,
    );

    expect(html).toContain('modem-lcd__unit-glyph');
    expect(html.match(/modem-lcd__glyph/g)).toHaveLength(3);
  });

  it('shows the AI lamp in the lamp row in place of AA', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="lamps"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
        generating
      />,
    );

    expect(html).toContain('<span class="modem-lamp__label">AI</span>');
    expect(html).not.toContain('AA');
    // Static markup runs no effects, so the flicker starts dark and only
    // the timer in the browser lights it.
    expect(html).toContain('aria-label="AI 消灯"');
  });
});

describe('nextFlickerStep', () => {
  it('keeps lit phases longer than dark ones', () => {
    const samples = Array.from({ length: 200 }, (_, index) => {
      const random = (() => { let seed = index + 1; return () => { seed = (seed * 9301 + 49297) % 233280; return seed / 233280; }; })();
      return nextFlickerStep(random);
    });
    const lit = samples.filter(step => step.on);
    const dark = samples.filter(step => !step.on);
    const average = (steps: { delayMs: number }[]) => steps.reduce((sum, step) => sum + step.delayMs, 0) / steps.length;

    expect(lit.length).toBeGreaterThan(dark.length);
    expect(average(lit)).toBeGreaterThan(average(dark));
  });

  it('keeps delays within the flicker range', () => {
    for (let index = 0; index < 50; index += 1) {
      const step = nextFlickerStep(() => index / 50);
      expect(step.delayMs).toBeGreaterThanOrEqual(40);
      expect(step.delayMs).toBeLessThanOrEqual(400);
    }
  });
});
