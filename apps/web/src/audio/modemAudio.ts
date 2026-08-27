// ---------------------------------------------------------------------------
// modemAudio.ts — procedural dial-up modem synthesis
//
// The handshake is generated as PCM on every run.  No recorded samples and
// no "random pile of sine waves" are used for training.  Protocol-significant
// tones remain stable; the analogue path changes slightly each time:
// response/detector delay, send/receive level, line noise, residual carrier,
// speaker resonance, tiny clock drift, and hybrid echo.
// ---------------------------------------------------------------------------

let ctx: AudioContext | null = null;

const SAMPLE_RATE = 48_000;
const TAU = Math.PI * 2;

export type HandshakeRun = {
  baud: number;
  seed: string;
  duration: number;
  responseJitterMs: number;
  speakerResonanceHz: number;
  lineLevelDb: number;
};

type Variation = {
  seed: number;
  rng: () => number;
  responseJitterMs: number;
  speakerResonanceHz: number;
  lineLevelDb: number;
  noise: number;
  echoMs: number;
  clockPpm: number;
};

function audio(): AudioContext {
  ctx ??= new AudioContext();
  return ctx;
}

function randomSeed(): number {
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    const value = new Uint32Array(1);
    crypto.getRandomValues(value);
    return value[0] >>> 0;
  }
  return (Date.now() ^ Math.floor(Math.random() * 0xffffffff)) >>> 0;
}

function mulberry32(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6d2b79f5) >>> 0;
    let z = s;
    z = Math.imul(z ^ (z >>> 15), 1 | z);
    z ^= z + Math.imul(z ^ (z >>> 7), 61 | z);
    return ((z ^ (z >>> 14)) >>> 0) / 0x100000000;
  };
}

function signed(rng: () => number): number {
  return rng() * 2 - 1;
}

function makeVariation(): Variation {
  const seed = randomSeed();
  const rng = mulberry32(seed);
  return {
    seed,
    rng,
    responseJitterMs: 18 + rng() * 42,
    speakerResonanceHz: 1840 + rng() * 170,
    lineLevelDb: -0.8 + signed(rng) * 0.9,
    noise: 0.00035 + rng() * 0.00045,
    echoMs: 2.4 + rng() * 1.0,
    clockPpm: signed(rng) * 85,
  };
}

function dbToGain(db: number): number {
  return 10 ** (db / 20);
}

function secondsToSamples(seconds: number): number {
  return Math.max(0, Math.round(seconds * SAMPLE_RATE));
}

function envelope(length: number, fadeMs = 1.5): Float32Array {
  const out = new Float32Array(length);
  out.fill(1);
  const edgeSamples = Math.min(Math.floor(length / 2), Math.max(1, secondsToSamples(fadeMs / 1000)));
  for (let i = 0; i < edgeSamples; i++) {
    const x = i / Math.max(1, edgeSamples - 1);
    const g = 0.5 - 0.5 * Math.cos(Math.PI * x);
    out[i] = g;
    out[length - 1 - i] = g;
  }
  return out;
}

function tone(freq: number, duration: number, amp = 1, phase = 0, fadeMs = 1.5, clockScale = 1): Float32Array {
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, fadeMs);
  let p = phase;
  const step = TAU * freq * clockScale / SAMPLE_RATE;
  for (let i = 0; i < n; i++) {
    out[i] = Math.sin(p) * amp * env[i];
    p += step;
  }
  return out;
}

function ans(duration: number, amp: number, clockScale: number, ansam: boolean): Float32Array {
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, 2.5);
  for (let i = 0; i < n; i++) {
    const t = i / SAMPLE_RATE;
    const reversal = Math.floor(t / 0.450) & 1;
    const modulation = ansam ? 1 + 0.17 * Math.sin(TAU * 15 * t) : 1;
    out[i] = Math.sin(TAU * 2100 * clockScale * t + reversal * Math.PI) * amp * modulation * env[i];
  }
  return out;
}

function bits(count: number, rng: () => number): Uint8Array {
  const out = new Uint8Array(count);
  for (let i = 0; i < count; i++) out[i] = rng() >= 0.5 ? 1 : 0;
  return out;
}

function cpfsk(mark: number, space: number, baud: number, sequence: Uint8Array, amp: number, clockScale: number): Float32Array {
  const duration = sequence.length / baud;
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, 0.7);
  let phase = 0;
  for (let i = 0; i < n; i++) {
    const bitIndex = Math.min(sequence.length - 1, Math.floor(i * baud / SAMPLE_RATE));
    const f = sequence[bitIndex] ? mark : space;
    phase += TAU * f * clockScale / SAMPLE_RATE;
    out[i] = Math.sin(phase) * amp * env[i];
  }
  return out;
}

function dbpsk(carrier: number, baud: number, sequence: Uint8Array, amp: number, clockScale: number): Float32Array {
  const duration = sequence.length / baud;
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, 0.7);
  let symbol = -1;
  let phaseOffset = 0;
  for (let i = 0; i < n; i++) {
    const nextSymbol = Math.min(sequence.length - 1, Math.floor(i * baud / SAMPLE_RATE));
    if (nextSymbol !== symbol) {
      symbol = nextSymbol;
      if (sequence[symbol]) phaseOffset += Math.PI;
    }
    const t = i / SAMPLE_RATE;
    out[i] = Math.sin(TAU * carrier * clockScale * t + phaseOffset) * amp * env[i];
  }
  return out;
}

type Point = { i: number; q: number };

function constellation(states: number): Point[] {
  const side = Math.ceil(Math.sqrt(states));
  const values: number[] = [];
  for (let v = -(side - 1); v <= side - 1; v += 2) values.push(v);
  const all: Point[] = [];
  for (const q of values) for (const i of values) all.push({ i, q });
  all.sort((a, b) => (a.i * a.i + a.q * a.q) - (b.i * b.i + b.q * b.q));
  const selected = all.slice(0, states);
  let energy = 0;
  for (const p of selected) energy += p.i * p.i + p.q * p.q;
  const scale = Math.sqrt(energy / selected.length) || 1;
  return selected.map(p => ({ i: p.i / scale, q: p.q / scale }));
}

function modulated(
  carrier: number,
  symbolRate: number,
  duration: number,
  states: number,
  amp: number,
  rng: () => number,
  clockScale: number,
  mode: 'scrambled' | 'cycle' | 'corners' = 'scrambled',
): Float32Array {
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, 1);
  const points = constellation(states);
  const symbolCount = Math.ceil(duration * symbolRate) + 2;
  const symbols: Point[] = new Array(symbolCount);

  const cornerOrder = [
    points.reduce((best, p) => (p.i + p.q > best.i + best.q ? p : best), points[0]),
    points.reduce((best, p) => (-p.i + p.q > -best.i + best.q ? p : best), points[0]),
    points.reduce((best, p) => (-p.i - p.q > -best.i - best.q ? p : best), points[0]),
    points.reduce((best, p) => (p.i - p.q > best.i - best.q ? p : best), points[0]),
  ];

  for (let s = 0; s < symbolCount; s++) {
    if (mode === 'cycle') symbols[s] = points[s % points.length];
    else if (mode === 'corners') symbols[s] = cornerOrder[s % cornerOrder.length];
    else symbols[s] = points[Math.floor(rng() * points.length)];
  }

  for (let i = 0; i < n; i++) {
    const symbolPos = i * symbolRate / SAMPLE_RATE;
    const s = Math.min(symbolCount - 2, Math.floor(symbolPos));
    const frac = symbolPos - s;
    const mix = 0.5 - 0.5 * Math.cos(Math.PI * frac);
    const a = symbols[s];
    const b = symbols[s + 1];
    const iv = a.i + (b.i - a.i) * mix;
    const qv = a.q + (b.q - a.q) * mix;
    const t = i / SAMPLE_RATE;
    const phase = TAU * carrier * clockScale * t;
    out[i] = (iv * Math.cos(phase) - qv * Math.sin(phase)) * amp * env[i];
  }
  return out;
}

const V34_PROBE_FREQS = [
  150, 300, 450, 600, 750, 1050, 1350, 1500, 1650, 1950, 2100,
  2250, 2550, 2700, 2850, 3000, 3150, 3300, 3450, 3600, 3750,
];
const V34_PROBE_PHASE = [
  0, 180, 0, 0, 0, 0, 0, 0, 180, 0, 0, 180, 0, 180, 0, 180, 180, 180, 180, 0, 0,
].map(v => v * Math.PI / 180);

function probe(duration: number, amp: number, clockScale: number): Float32Array {
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  const env = envelope(n, 1);
  const normalization = 1 / Math.sqrt(V34_PROBE_FREQS.length / 2);
  for (let i = 0; i < n; i++) {
    const t = i / SAMPLE_RATE;
    let value = 0;
    for (let k = 0; k < V34_PROBE_FREQS.length; k++) {
      value += Math.cos(TAU * V34_PROBE_FREQS[k] * clockScale * t + V34_PROBE_PHASE[k]);
    }
    out[i] = value * normalization * amp * env[i];
  }
  return out;
}

function lineFloor(duration: number, rng: () => number, residualFreq: number | null, residualAmp: number, clockScale: number): Float32Array {
  const n = secondsToSamples(duration);
  const out = new Float32Array(n);
  let lp = 0;
  let hp = 0;
  let prev = 0;
  const lpAlpha = 1 - Math.exp(-TAU * 3400 / SAMPLE_RATE);
  const hpAlpha = Math.exp(-TAU * 250 / SAMPLE_RATE);
  for (let i = 0; i < n; i++) {
    const white = signed(rng) * 0.003;
    lp += lpAlpha * (white - lp);
    hp = hpAlpha * (hp + lp - prev);
    prev = lp;
    let value = hp * 0.18;
    if (residualFreq !== null && residualAmp > 0) {
      const t = i / SAMPLE_RATE;
      const decay = Math.exp(-t / Math.max(0.012, duration * 0.45));
      value += residualAmp * decay * Math.sin(TAU * residualFreq * clockScale * t);
    }
    out[i] = value;
  }
  return out;
}

class Mixer {
  readonly data: Float32Array;

  constructor(duration: number) {
    this.data = new Float32Array(secondsToSamples(duration));
  }

  add(start: number, source: Float32Array, gain = 1): void {
    const offset = secondsToSamples(start);
    const count = Math.min(source.length, this.data.length - offset);
    if (count <= 0) return;
    for (let i = 0; i < count; i++) this.data[offset + i] += source[i] * gain;
  }
}

function pause(
  mixer: Mixer,
  start: number,
  mean: number,
  spread: number,
  v: Variation,
  residualFreq: number | null,
  residualAmp: number,
): number {
  const globalBias = v.responseJitterMs / 1000;
  const duration = Math.max(0.018, mean + signed(v.rng) * spread + globalBias * 0.12);
  mixer.add(start, lineFloor(duration, v.rng, residualFreq, residualAmp, 1 + v.clockPpm * 1e-6));
  return start + duration;
}

function onePoleBandLimit(input: Float32Array): Float32Array {
  const out = new Float32Array(input.length);
  let low = 0;
  let high = 0;
  let previousLowInput = 0;
  const lowAlpha = 1 - Math.exp(-TAU * 3900 / SAMPLE_RATE);
  const highAlpha = Math.exp(-TAU * 170 / SAMPLE_RATE);
  for (let i = 0; i < input.length; i++) {
    low += lowAlpha * (input[i] - low);
    high = highAlpha * (high + low - previousLowInput);
    previousLowInput = low;
    out[i] = high;
  }
  return out;
}

function bandpassResonance(input: Float32Array, frequency: number, q: number): Float32Array {
  const out = new Float32Array(input.length);
  const w0 = TAU * frequency / SAMPLE_RATE;
  const alpha = Math.sin(w0) / (2 * q);
  const cos = Math.cos(w0);
  const a0 = 1 + alpha;
  const b0 = alpha / a0;
  const b1 = 0;
  const b2 = -alpha / a0;
  const a1 = -2 * cos / a0;
  const a2 = (1 - alpha) / a0;
  let x1 = 0, x2 = 0, y1 = 0, y2 = 0;
  for (let i = 0; i < input.length; i++) {
    const x0 = input[i];
    const y0 = b0 * x0 + b1 * x1 + b2 * x2 - a1 * y1 - a2 * y2;
    out[i] = y0;
    x2 = x1; x1 = x0; y2 = y1; y1 = y0;
  }
  return out;
}

function speakerLine(input: Float32Array, v: Variation): Float32Array {
  const limited = onePoleBandLimit(input);
  const r1 = bandpassResonance(limited, v.speakerResonanceHz, 1.05);
  const r2 = bandpassResonance(limited, 2350 + signed(v.rng) * 65, 1.8);
  const out = new Float32Array(input.length);
  const echo1 = secondsToSamples(v.echoMs / 1000);
  const echo2 = secondsToSamples((v.echoMs + 4.7) / 1000);
  const lineGain = dbToGain(v.lineLevelDb);
  const driftPhase = v.rng() * TAU;

  let peak = 0;
  for (let i = 0; i < out.length; i++) {
    const t = i / SAMPLE_RATE;
    const slowDrift = 1 + 0.016 * Math.sin(TAU * 0.19 * t + driftPhase);
    let x = (0.80 * limited[i] + 0.15 * r1[i] + 0.05 * r2[i]) * lineGain * slowDrift;
    if (i >= echo1) x += out[i - echo1] * 0.016;
    if (i >= echo2) x += out[i - echo2] * 0.006;
    x += signed(v.rng) * v.noise;
    const drive = x >= 0 ? 1.20 : 1.05;
    x = Math.tanh(x * drive) / Math.tanh(drive);
    out[i] = x;
    peak = Math.max(peak, Math.abs(x));
  }

  if (peak > 0.94) {
    const scale = 0.94 / peak;
    for (let i = 0; i < out.length; i++) out[i] *= scale;
  }
  return out;
}

function buildV22bis(v: Variation): { pcm: Float32Array; duration: number } {
  const clock = 1 + v.clockPpm * 1e-6;
  const m = new Mixer(6.4);
  m.add(0.05, ans(2.05 + signed(v.rng) * 0.04, 0.47, clock, false));
  m.add(1.62, tone(1200, 0.52, 0.16, 0, 1.2, clock));
  m.add(2.13, tone(1200, 0.24, 0.20, 0, 1.0, clock));
  m.add(2.24, tone(2400, 0.52, 0.28, 0, 1.0, clock));
  m.add(2.24, tone(1800, 0.52, 0.075, 0, 1.0, clock));
  m.add(2.78, modulated(2400, 600, 0.74, 16, 0.23, v.rng, clock, 'cycle'));
  m.add(3.55, modulated(1200, 600, 0.74, 16, 0.22, v.rng, clock, 'corners'));
  m.add(4.31, modulated(1200, 600, 1.23, 16, 0.18, v.rng, clock));
  m.add(4.31, modulated(2400, 600, 1.23, 16, 0.18, v.rng, clock));
  return { pcm: speakerLine(m.data, v), duration: 5.58 };
}

function buildV32(v: Variation, bis: boolean): { pcm: Float32Array; duration: number } {
  const clock = 1 + v.clockPpm * 1e-6;
  const m = new Mixer(bis ? 9.7 : 8.8);
  m.add(0.05, ans(2.03 + signed(v.rng) * 0.04, 0.47, clock, false));
  m.add(1.18, tone(1800, 0.98, 0.27, 0, 1.1, clock));
  let t = 2.18;
  m.add(t, tone(600, 0.44, 0.08, 0, 1.0, clock));
  m.add(t, tone(3000, 0.44, 0.08, 0, 1.0, clock));
  m.add(t, tone(1800, 0.44, 0.035, 0, 1.0, clock));
  t += 0.46;
  t = pause(m, t, 0.055, 0.025, v, 1800, 0.006);

  const states = bis ? 128 : 32;
  m.add(t, modulated(1800, 2400, 1.12, states, 0.29, v.rng, clock));
  t += 1.12;
  t = pause(m, t, 0.055, 0.020, v, 1800, 0.006);
  m.add(t, modulated(1800, 2400, 0.16, bis ? 8 : 4, 0.18, v.rng, clock, 'corners'));
  t += 0.16;
  if (bis) {
    t = pause(m, t, 0.060, 0.025, v, 1800, 0.005);
    m.add(t, dbpsk(1800, 600, bits(78, v.rng), 0.13, clock));
    t += 0.13;
  }
  t = pause(m, t, 0.060, 0.025, v, 1800, 0.005);
  m.add(t, modulated(1800, 2400, bis ? 1.10 : 1.04, states, 0.28, v.rng, clock));
  t += bis ? 1.10 : 1.04;
  t = pause(m, t, 0.055, 0.020, v, 1800, 0.004);
  m.add(t, modulated(1800, 2400, bis ? 1.48 : 1.25, states, 0.25, v.rng, clock));
  t += bis ? 1.48 : 1.25;
  m.add(t + 0.03, modulated(1800, 2400, 0.52, states, 0.17, v.rng, clock));
  return { pcm: speakerLine(m.data, v), duration: t + 0.58 };
}

function buildV34(v: Variation): { pcm: Float32Array; duration: number } {
  const clock = 1 + v.clockPpm * 1e-6;
  const m = new Mixer(15.0);

  m.add(0.05, ans(2.08 + signed(v.rng) * 0.035, 0.47, clock, true));
  m.add(0.70 + signed(v.rng) * 0.012, cpfsk(980, 1180, 300, bits(208, v.rng), 0.145, clock));
  m.add(1.16 + signed(v.rng) * 0.015, cpfsk(1650, 1850, 300, bits(176, v.rng), 0.155, clock));
  m.add(1.61 + signed(v.rng) * 0.015, cpfsk(980, 1180, 300, bits(128, v.rng), 0.13, clock));

  let t = 2.06;
  t = pause(m, t, 0.115, 0.035, v, 1180, 0.012);

  const info = bits(120, v.rng);
  m.add(t, dbpsk(1200, 600, info, 0.13, clock));
  m.add(t + 0.05, dbpsk(2400, 600, bits(120, v.rng), 0.12, clock));
  t += 0.21;
  t = pause(m, t, 0.145, 0.045, v, 2400, 0.009);

  m.add(t, tone(1200, 0.16, 0.16, 0, 1.0, clock));
  m.add(t, tone(2400, 0.16, 0.15, 0, 1.0, clock));
  t += 0.18;
  t = pause(m, t, 0.085, 0.025, v, 2400, 0.007);

  const l2 = 0.245 * dbToGain(signed(v.rng) * 0.45);
  const l1 = l2 * 10 ** (6 / 20);
  m.add(t, probe(0.16, l1, clock));
  t += 0.16;
  t = pause(m, t, 0.055, 0.016, v, 2100, 0.011);
  m.add(t, probe(0.97 + signed(v.rng) * 0.035, l2, clock));
  t += 0.97;
  t = pause(m, t, 0.135, 0.045, v, 1950, 0.006);

  m.add(t, tone(2400, 0.13, 0.16, 0, 1.0, clock));
  m.add(t, tone(1200, 0.13, 0.12, 0, 1.0, clock));
  t += 0.14;
  t = pause(m, t, 0.050, 0.016, v, 1200, 0.006);
  m.add(t, dbpsk(2400, 600, bits(96, v.rng), 0.12, clock));
  t += 0.17;
  t = pause(m, t, 0.115, 0.035, v, 2400, 0.007);

  const l2b = 0.225 * dbToGain(signed(v.rng) * 0.45);
  const l1b = l2b * 10 ** (6 / 20);
  m.add(t, probe(0.16, l1b, clock));
  t += 0.16;
  t = pause(m, t, 0.050, 0.016, v, 2100, 0.010);
  m.add(t, probe(0.94 + signed(v.rng) * 0.035, l2b, clock));
  t += 0.94;
  t = pause(m, t, 0.205, 0.055, v, 1800, 0.004);

  m.add(t, modulated(1800, 3000, 0.10, 4, 0.20, v.rng, clock, 'cycle'));
  t += 0.10;
  t = pause(m, t, 0.040, 0.015, v, 1800, 0.006);
  m.add(t, modulated(1800, 3000, 0.26, 16, 0.25, v.rng, clock, 'cycle'));
  t += 0.26;
  t = pause(m, t, 0.060, 0.020, v, 1800, 0.005);
  m.add(t, modulated(1800, 3000, 1.16, 16, 0.29, v.rng, clock));
  t += 1.16;
  t = pause(m, t, 0.070, 0.025, v, 1800, 0.006);
  m.add(t, modulated(1800, 3000, 0.18, 4, 0.18, v.rng, clock, 'corners'));
  t += 0.18;
  t = pause(m, t, 0.055, 0.020, v, 1800, 0.005);
  m.add(t, modulated(1800, 3000, 1.09, 32, 0.27, v.rng, clock));
  t += 1.09;
  t = pause(m, t, 0.095, 0.030, v, 1800, 0.004);
  m.add(t, modulated(1800, 3000, 1.05, 256, 0.25, v.rng, clock));
  t += 1.05;
  t = pause(m, t, 0.060, 0.020, v, 1800, 0.004);
  m.add(t, modulated(1800, 3000, 0.83, 512, 0.22, v.rng, clock));
  t += 0.83;
  t = pause(m, t, 0.090, 0.025, v, 1800, 0.003);
  m.add(t, modulated(1800, 3000, 0.52, 512, 0.17, v.rng, clock));
  t += 0.54;

  const pcm = speakerLine(m.data, v);
  const gate = Math.min(pcm.length, secondsToSamples(t));
  const fade = secondsToSamples(0.018);
  for (let i = gate; i < pcm.length; i++) pcm[i] = 0;
  for (let i = 0; i < fade && gate - i - 1 >= 0; i++) {
    pcm[gate - i - 1] *= i / Math.max(1, fade - 1);
  }
  return { pcm, duration: t };
}

function renderHandshake(baud: number, v: Variation): { pcm: Float32Array; duration: number } {
  if (baud <= 2400) return buildV22bis(v);
  if (baud <= 9600) return buildV32(v, false);
  if (baud <= 14400) return buildV32(v, true);
  return buildV34(v);
}

function playPcm(ac: AudioContext, pcm: Float32Array): void {
  void ac.resume();
  const buffer = ac.createBuffer(1, pcm.length, SAMPLE_RATE);
  buffer.getChannelData(0).set(pcm);
  const source = ac.createBufferSource();
  source.buffer = buffer;
  source.connect(ac.destination);
  source.start();
}

export function playHandshake(baud: number): HandshakeRun {
  const v = makeVariation();
  const rendered = renderHandshake(baud, v);
  playPcm(audio(), rendered.pcm);
  return {
    baud,
    seed: v.seed.toString(16).padStart(8, '0').toUpperCase(),
    duration: rendered.duration,
    responseJitterMs: Math.round(v.responseJitterMs),
    speakerResonanceHz: Math.round(v.speakerResonanceHz),
    lineLevelDb: Math.round(v.lineLevelDb * 10) / 10,
  };
}

function scheduleTone(ac: AudioContext, freq: number, start: number, duration: number, gain: number): void {
  const osc = ac.createOscillator();
  const g = ac.createGain();
  const t0 = ac.currentTime + start;
  osc.frequency.value = freq;
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(gain, t0 + 0.004);
  g.gain.setValueAtTime(gain, Math.max(t0 + 0.004, t0 + duration - 0.006));
  g.gain.linearRampToValueAtTime(0, t0 + duration);
  osc.connect(g).connect(ac.destination);
  osc.start(t0);
  osc.stop(t0 + duration);
}

export function playDialSequence(phone: string): void {
  const ac = audio();
  void ac.resume();
  const digits: Record<string, [number, number]> = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '0': [941, 1336], '*': [941, 1209], '#': [941, 1477],
  };
  let t = 0;
  for (const digit of phone) {
    const pair = digits[digit];
    if (!pair) continue;
    scheduleTone(ac, pair[0], t, 0.085, 0.022);
    scheduleTone(ac, pair[1], t, 0.085, 0.022);
    t += 0.135;
  }
}

export function playBusy(): void {
  const ac = audio();
  void ac.resume();
  scheduleTone(ac, 400, 0, 0.25, 0.025);
  scheduleTone(ac, 400, 0.5, 0.25, 0.025);
}
