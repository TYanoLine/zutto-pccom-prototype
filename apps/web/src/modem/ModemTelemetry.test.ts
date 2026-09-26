import { describe, expect, it } from 'vitest';
import { DEFAULT_COMM_SETTINGS } from './CommSettings';
import {
  createIdleModemTelemetry,
  formatModemBaud,
  protocolIndicators,
  protocolStateForSettings,
} from './ModemTelemetry';

describe('modem status reconstruction', () => {
  it('defaults to the configured preferred V.42bis indication while on-hook', () => {
    const telemetry = createIdleModemTelemetry(DEFAULT_COMM_SETTINGS);
    expect(protocolIndicators(telemetry)).toEqual({
      v42bis: true,
      mnp: false,
      mnp5: false,
    });
  });

  it('shows simultaneous V.42bis and MNP5 capability indications while negotiating in auto mode', () => {
    const telemetry = {
      ...createIdleModemTelemetry(DEFAULT_COMM_SETTINGS),
      phase: 'negotiating' as const,
      oh: true,
      protocol: protocolStateForSettings(DEFAULT_COMM_SETTINGS, 'negotiating'),
    };
    expect(protocolIndicators(telemetry)).toEqual({
      v42bis: true,
      mnp: true,
      mnp5: true,
    });
  });

  it('settles auto mode on the simulated V.42bis preference after connect', () => {
    const telemetry = {
      ...createIdleModemTelemetry(DEFAULT_COMM_SETTINGS),
      phase: 'online' as const,
      oh: true,
      cd: true,
      protocol: protocolStateForSettings(DEFAULT_COMM_SETTINGS, 'online'),
    };
    expect(protocolIndicators(telemetry)).toEqual({
      v42bis: true,
      mnp: false,
      mnp5: false,
    });
  });

  it('can show MNP without the class-5 digit for an MNP4 connection', () => {
    const settings = {
      ...DEFAULT_COMM_SETTINGS,
      errorCorrection: 'mnp4' as const,
      compression: 'off' as const,
    };
    const telemetry = {
      ...createIdleModemTelemetry(settings),
      phase: 'online' as const,
      oh: true,
      cd: true,
      protocol: protocolStateForSettings(settings, 'online'),
    };
    expect(protocolIndicators(telemetry)).toEqual({
      v42bis: false,
      mnp: true,
      mnp5: false,
    });
  });

  it('formats the modem LCD speed field in K units', () => {
    expect(formatModemBaud(28800, 38400)).toBe('28.8K');
    expect(formatModemBaud(14400, 38400)).toBe('14.4K');
    expect(formatModemBaud(null, 38400)).toBe('38.4K');
  });
});
