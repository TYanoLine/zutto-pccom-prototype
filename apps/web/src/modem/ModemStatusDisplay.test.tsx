import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { createIdleModemTelemetry } from './ModemTelemetry';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import { ModemStatusDisplay } from './ModemStatusDisplay';

describe('PV-AF-style digital modem display', () => {
  it('renders the variable speed as seven-segment digits while keeping indicator legends as fixed text', () => {
    const html = renderToStaticMarkup(
      <ModemStatusDisplay
        mode="digital"
        telemetry={createIdleModemTelemetry(DEFAULT_COMM_SETTINGS)}
        dteBaud={38400}
      />,
    );

    expect(html.match(/modem-lcd__digit/g)).toHaveLength(3);
    expect(html).toContain('modem-lcd__decimal');
    expect(html).toContain('modem-lcd__speed-unit');
    expect(html).toContain('V.42bis');
    expect(html).toContain('MNP');
    expect(html).toContain('OFH');
    expect(html).toContain('DTR');
    expect(html).toContain('DSR');
    expect(html).toContain('RTS');
    expect(html).toContain('CTS');
    expect(html).toContain('AA');
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

    expect(html).toContain('modem-lcd__segment--a');
    expect(html).toContain('data-on="true"');
    expect(html).toContain('data-on="false"');
  });
});
