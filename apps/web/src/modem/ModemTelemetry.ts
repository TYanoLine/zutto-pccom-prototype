import type { CommSettings } from './CommSettings';

export type ModemPhase =
  | 'idle'
  | 'dialing'
  | 'ringing'
  | 'negotiating'
  | 'online'
  | 'recovering';

export type NegotiatedModemProtocol =
  | 'v42bis'
  | 'mnp5'
  | 'mnp4'
  | 'v42'
  | 'none'
  | null;

export type ModemProtocolState = {
  v42bisAvailable: boolean;
  mnpAvailable: boolean;
  mnp5Available: boolean;
  negotiated: NegotiatedModemProtocol;
};

export type ModemTelemetry = {
  phase: ModemPhase;
  baud: number | null;
  mr: boolean;
  tr: boolean;
  sd: boolean;
  rd: boolean;
  oh: boolean;
  cd: boolean;
  aa: boolean;
  hs: boolean;
  dsr: boolean;
  cts: boolean;
  protocol: ModemProtocolState;
};

export type ModemProtocolIndicators = {
  v42bis: boolean;
  mnp: boolean;
  mnp5: boolean;
};

export function protocolStateForSettings(
  settings: CommSettings,
  phase: ModemPhase,
): ModemProtocolState {
  const v42bisAvailable = settings.compression === 'auto' || settings.compression === 'v42bis';
  const mnp5Available = settings.compression === 'auto' || settings.compression === 'mnp5';
  const mnpAvailable = settings.errorCorrection === 'auto'
    || settings.errorCorrection === 'mnp4'
    || mnp5Available;

  let negotiated: NegotiatedModemProtocol = null;
  if (phase === 'online') {
    // The current fictional modem's AUTO policy prefers V.42/V.42bis and falls
    // back to MNP. This is a simulation policy, not a claim about a particular
    // commercial modem's complete negotiation algorithm.
    if (settings.compression === 'auto' || settings.compression === 'v42bis') negotiated = 'v42bis';
    else if (settings.compression === 'mnp5') negotiated = 'mnp5';
    else if (settings.errorCorrection === 'mnp4') negotiated = 'mnp4';
    else if (settings.errorCorrection === 'auto' || settings.errorCorrection === 'v42') negotiated = 'v42';
    else negotiated = 'none';
  }

  return {
    v42bisAvailable,
    mnpAvailable,
    mnp5Available,
    negotiated,
  };
}

export function protocolIndicators(telemetry: ModemTelemetry): ModemProtocolIndicators {
  const { phase, protocol } = telemetry;

  if (phase === 'dialing' || phase === 'ringing' || phase === 'negotiating') {
    return {
      v42bis: protocol.v42bisAvailable,
      mnp: protocol.mnpAvailable,
      mnp5: protocol.mnp5Available,
    };
  }

  if (phase === 'online') {
    return {
      v42bis: protocol.negotiated === 'v42bis',
      mnp: protocol.negotiated === 'mnp4' || protocol.negotiated === 'mnp5',
      mnp5: protocol.negotiated === 'mnp5',
    };
  }

  // With the line on-hook, show the configured/preferred mode rather than all
  // fallbacks at once. This matches the working reconstruction from the
  // photographed PV-AF288-family LCDs while keeping the negotiation-time
  // simultaneous capability indication separate.
  if (protocol.v42bisAvailable) {
    return { v42bis: true, mnp: false, mnp5: false };
  }
  if (protocol.mnp5Available) {
    return { v42bis: false, mnp: true, mnp5: true };
  }
  if (protocol.mnpAvailable) {
    return { v42bis: false, mnp: true, mnp5: false };
  }
  return { v42bis: false, mnp: false, mnp5: false };
}

export function createIdleModemTelemetry(settings: CommSettings): ModemTelemetry {
  return {
    phase: 'idle',
    baud: null,
    mr: true,
    tr: true,
    sd: false,
    rd: false,
    oh: false,
    cd: false,
    aa: false,
    hs: false,
    dsr: true,
    cts: true,
    protocol: protocolStateForSettings(settings, 'idle'),
  };
}

export function formatModemBaud(baud: number | null, dteBaud: number): string {
  const value = baud && baud > 0 ? baud : dteBaud;
  if (value >= 1000) {
    const kilo = value / 1000;
    return Number.isInteger(kilo) ? `${kilo.toFixed(1)}K` : `${kilo.toFixed(1)}K`;
  }
  return String(value);
}
