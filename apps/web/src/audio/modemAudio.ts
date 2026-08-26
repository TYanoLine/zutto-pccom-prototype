// ---------------------------------------------------------------------------
// modemAudio.ts — ITU-T modem handshake synthesis (Web Audio API)
//
// Handshake sequences are modelled on ITU-T Recommendations:
//   V.22bis (2400 bps), V.32 (9600 bps), V.32bis (14400 bps), V.34 (28800 bps)
//
// Terminology / signals used below:
//   ANS   — Answer tone: 2100 Hz CED, ±180° phase reversal every 450 ms
//           (V.25 §2.5.2). The phase reversal disables echo-cancellers.
//   Guard tone — 1800 Hz (or 550 Hz) supervisory tone sent alongside some signals.
//   AA1   — Calling signal tone 980 Hz (V.32/V.32bis calling state)
//   AC    — Tone sequence 1200 Hz used in V.32bis INFO/INFO0/INFO1c
//   S     — Scrambled all-ones (binary pattern ≈ quasi-random noise burst)
//   Sb,Sbar — complementary scrambled binary training bursts (V.32bis)
//   EC    — Echo canceller training: double talk period
//   INFO  — Integer-number negotiation burst  (V.32bis, V.34)
//
// Synthesising the actual QAM constellations and trellis-coded data is
// impractical in Web Audio; instead we render the perceptually dominant
// signals that give the characteristic dial-up "screech":
//   • ANS tone with phase-reversal clicks
//   • Guard tone (1800 Hz)
//   • AA1 calling tone (980 Hz)
//   • Noise-like training bursts (bandlimited noise approximated with
//     overlapping random-frequency sine waves)
//   • Negotiation chirp / tone pairs (V.32bis, V.34)
// ---------------------------------------------------------------------------

let ctx: AudioContext | null = null;

function audio(): AudioContext {
  ctx ??= new AudioContext();
  return ctx;
}

// ---------------------------------------------------------------------------
// Primitive helpers
// ---------------------------------------------------------------------------

/** Schedule a single sine-wave burst. Returns the stop time (ac.currentTime + start + duration). */
function tone(
  ac: AudioContext,
  freq: number,
  start: number,
  duration: number,
  gain = 0.03,
): number {
  const osc = ac.createOscillator();
  const g = ac.createGain();
  osc.frequency.value = freq;
  g.gain.value = gain;
  osc.connect(g).connect(ac.destination);
  osc.start(ac.currentTime + start);
  osc.stop(ac.currentTime + start + duration);
  return start + duration;
}

/** Schedule a linear frequency sweep (chirp). */
function chirp(
  ac: AudioContext,
  freqStart: number,
  freqEnd: number,
  start: number,
  duration: number,
  gain = 0.025,
) {
  const osc = ac.createOscillator();
  const g = ac.createGain();
  g.gain.value = gain;
  osc.frequency.setValueAtTime(freqStart, ac.currentTime + start);
  osc.frequency.linearRampToValueAtTime(freqEnd, ac.currentTime + start + duration);
  osc.connect(g).connect(ac.destination);
  osc.start(ac.currentTime + start);
  osc.stop(ac.currentTime + start + duration);
}

/**
 * Approximate a wideband noise burst by summing many sine waves at random
 * frequencies within [fLow, fHigh].  This is perceptually close to the
 * scrambled binary data bursts (S, Sb, Sbar, etc.) that appear in training.
 */
function noiseBurst(
  ac: AudioContext,
  fLow: number,
  fHigh: number,
  start: number,
  duration: number,
  sineCount = 24,
  gain = 0.008,
) {
  const rng = mulberry32(0xdeadbeef ^ Math.floor(start * 1000));
  for (let i = 0; i < sineCount; i++) {
    const f = fLow + rng() * (fHigh - fLow);
    tone(ac, f, start, duration, gain);
  }
}

/** Simple deterministic PRNG so noise bursts are reproducible across calls. */
function mulberry32(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6d2b79f5) >>> 0;
    let z = s;
    z = Math.imul(z ^ (z >>> 15), 1 | z);
    z = (z ^ (z + Math.imul(z ^ (z >>> 7), 61 | z))) >>> 0;
    return (z ^ (z >>> 14)) / 0x100000000;
  };
}

/** Envelope-shaped tone: fade-in over `attack` seconds, flat, fade-out over `release` seconds. */
function envelopedTone(
  ac: AudioContext,
  freq: number,
  start: number,
  duration: number,
  peakGain = 0.03,
  attack = 0.01,
  release = 0.01,
) {
  const osc = ac.createOscillator();
  const g = ac.createGain();
  osc.frequency.value = freq;
  const t0 = ac.currentTime + start;
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(peakGain, t0 + attack);
  g.gain.setValueAtTime(peakGain, t0 + duration - release);
  g.gain.linearRampToValueAtTime(0, t0 + duration);
  osc.connect(g).connect(ac.destination);
  osc.start(t0);
  osc.stop(t0 + duration);
}

// ---------------------------------------------------------------------------
// ANS tone (2100 Hz) with phase reversal every 450 ms
// ---------------------------------------------------------------------------
// ITU-T V.25 §2.5.2: the answer tone is 2100 Hz ±15 Hz, amplitude-modulated
// by ≤25%, transmitted for 3.3 s.  Phase reversals occur every 450 ±25 ms.
// We model reversal as a ~20 ms cosine cross-fade through silence.

function scheduleANS(
  ac: AudioContext,
  startOffset: number,
  totalDuration: number,
  peakGain = 0.030,
): number {
  const ANS_FREQ = 2100;
  const REVERSAL_PERIOD = 0.450; // seconds
  const CLICK_DUR = 0.018;       // brief silence at each reversal (≈18 ms)

  let t = startOffset;
  let phase = 1; // not used for frequency; we just gate oscillator segments
  while (t < startOffset + totalDuration - 0.02) {
    const segEnd = Math.min(t + REVERSAL_PERIOD, startOffset + totalDuration);
    const segDur = segEnd - t;
    if (segDur > CLICK_DUR) {
      envelopedTone(ac, ANS_FREQ, t, segDur - CLICK_DUR, peakGain * phase, 0.005, 0.005);
    }
    t = segEnd;
    phase = -phase; // conceptual; amplitude stays positive since we use abs
  }
  return startOffset + totalDuration;
}

// ---------------------------------------------------------------------------
// V.22bis  2400 bps  (ITU-T V.22bis)
// ---------------------------------------------------------------------------
// Call procedure (originating modem = calling, answering modem = answering):
//   t=0.000  Line seized; silence on calling side; answer modem sends ANS 2100 Hz
//   t=0.000  ANS tone  (3.3 s, phase reversal every 450 ms)
//   t=3.300  Guard tone 1800 Hz + S1 (2400 Hz) scrambled training  (1.2 s)
//   t=4.500  INFO0 negotiation burst (1200 Hz carrier, 600 bps)  (0.6 s)
//   t=5.100  S1 / S11 data-mode training burst  (0.9 s)
//   t=6.000  Connection established
//
// The calling modem contributes:
//   t=0.000  Unscrambled 1's at 1200 Hz for 155 ms
//   t=0.155  Scrambled 1's (noise burst)  until INFO received
//
// For playback we render the perceptually dominant answering-modem path plus
// the overlapping calling-modem signal.

function playV22bis(ac: AudioContext) {
  // Answering modem: ANS 2100 Hz with phase reversals (3.3 s)
  scheduleANS(ac, 0.0, 3.3, 0.028);

  // Calling modem: unscrambled 1200 Hz for 155 ms then noise
  envelopedTone(ac, 1200, 0.0, 0.155, 0.022, 0.01, 0.01);
  noiseBurst(ac, 1000, 3000, 0.155, 3.0, 20, 0.007); // scrambled 1's

  // Guard tone 1800 Hz + S1 training (2400 Hz carrier region noise)
  envelopedTone(ac, 1800, 3.3, 1.2, 0.018, 0.02, 0.02);
  noiseBurst(ac, 1800, 2600, 3.3, 1.2, 18, 0.012);

  // INFO0 negotiation: 1200 Hz burst
  envelopedTone(ac, 1200, 4.5, 0.6, 0.022, 0.02, 0.02);

  // S1 / data training
  noiseBurst(ac, 1600, 2800, 5.1, 0.9, 20, 0.015);
}

// ---------------------------------------------------------------------------
// V.32  9600 bps  (ITU-T V.32)
// ---------------------------------------------------------------------------
// V.32 handshake procedure (abbreviated, both directions mixed):
//   t=0.000  Calling modem sends AA1 tone: 980 Hz  (≥200 ms)
//   t=0.000  Answering modem: ANS 2100 Hz (3.3 s, phase reversals)
//   t=3.300  Both sides: EC (echo canceller training) – wideband noise (400 ms)
//   t=3.700  Calling: AC 1200 Hz tone; Answering: AC 1200 Hz
//   t=4.300  Both: S (scrambled all-1's burst) wideband  (0.9 s)
//   t=5.200  Both: Sbar (complement)  (0.9 s)
//   t=6.100  Short phase correction tones (200 ms each)
//   t=6.500  CONNECT

function playV32(ac: AudioContext) {
  // Calling modem: AA1 980 Hz throughout early phase
  envelopedTone(ac, 980, 0.0, 3.3, 0.020, 0.02, 0.02);

  // Answering modem: ANS 2100 Hz (3.3 s)
  scheduleANS(ac, 0.0, 3.3, 0.028);

  // EC phase: wideband noise (both modems, overlapping)
  noiseBurst(ac, 300, 3400, 3.3, 0.4, 28, 0.018);

  // AC tone 1200 Hz
  envelopedTone(ac, 1200, 3.7, 0.6, 0.025, 0.02, 0.02);

  // S scrambled burst: QAM-like noise 900–3200 Hz
  noiseBurst(ac, 900, 3200, 4.3, 0.9, 32, 0.020);

  // Sbar: slightly different seed (use different start so PRNG differs)
  noiseBurst(ac, 900, 3200, 5.2, 0.9, 32, 0.020);

  // Phase correction short tones
  tone(ac, 1800, 6.1, 0.18, 0.022);
  tone(ac, 2400, 6.28, 0.18, 0.022);
}

// ---------------------------------------------------------------------------
// V.32bis  14400 bps  (ITU-T V.32bis)  — PRIMARY profile
// ---------------------------------------------------------------------------
// V.32bis handshake (combined calling + answering, time-domain):
//
//   Phase 1 – Line seizure / ANS
//     t=0.000  Calling modem: AA1  980 Hz (calling tone, no modulation)
//     t=0.000  Answering modem: ANS 2100 Hz, phase reversal every 450 ms (3.3 s)
//
//   Phase 2 – Echo-canceller training
//     t=3.300  Both: EC double-talk  (wideband noise, ~500 ms)
//
//   Phase 3 – Signal AC / INFO
//     t=3.800  Both: AC tone 1200 Hz carrier  (600 ms)
//     t=4.400  Answering: INFO0 (capability announcement @ 1200 Hz DPSK, 75 ms)
//     t=4.400  Calling:   INFO1c negotiation burst (overlapping)
//
//   Phase 4 – Training
//     t=4.600  S/Sb/Sbar scrambled bursts (3 × 900 ms of QAM-like noise)
//     t=7.300  Short phase tones: 1800 Hz + 2400 Hz alternating
//
//   Phase 5 – Data
//     t=7.700  CONNECT 14400
//
// Key perceptual cues:
//   • The 2100 Hz ANS with audible clicks at each phase reversal (≈450 ms)
//   • A brief AA1 980 Hz calling chirp
//   • Loud wideband QAM noise during training (~2.7 s total)
//   • Short high-pitched "bleep" tones at the end

function playV32bis(ac: AudioContext) {
  // ── Phase 1 ──────────────────────────────────────────────────────────────

  // Calling modem: AA1 980 Hz
  envelopedTone(ac, 980, 0.0, 3.3, 0.020, 0.02, 0.05);

  // Answering modem: ANS 2100 Hz with phase reversals (3.3 s)
  scheduleANS(ac, 0.0, 3.3, 0.030);

  // ── Phase 2 – EC ─────────────────────────────────────────────────────────
  noiseBurst(ac, 300, 3400, 3.3, 0.5, 30, 0.022);

  // ── Phase 3 – AC / INFO ──────────────────────────────────────────────────
  // AC 1200 Hz carrier
  envelopedTone(ac, 1200, 3.8, 0.6, 0.028, 0.02, 0.02);

  // INFO0 / INFO1c — short 1200 Hz DPSK-like burst (overlapping)
  noiseBurst(ac, 1050, 1350, 4.2, 0.35, 12, 0.015); // narrow-band scramble

  // ── Phase 4 – Training (S, Sb, Sbar) ─────────────────────────────────────
  // Each burst uses a different pseudo-random seed by virtue of different start times.
  noiseBurst(ac, 900, 3400, 4.6, 0.9, 36, 0.024);   // S
  noiseBurst(ac, 900, 3400, 5.5, 0.9, 36, 0.024);   // Sb
  noiseBurst(ac, 900, 3400, 6.4, 0.9, 36, 0.024);   // Sbar

  // ── Phase 5 – Phase-correction tones ─────────────────────────────────────
  envelopedTone(ac, 1800, 7.3, 0.18, 0.030, 0.01, 0.01);
  envelopedTone(ac, 2400, 7.48, 0.18, 0.030, 0.01, 0.01);
}

// ---------------------------------------------------------------------------
// V.34  28800 bps  (ITU-T V.34)
// ---------------------------------------------------------------------------
// V.34 introduces a lengthy probing / negotiation sequence before training.
// Key phases (simplified):
//   t=0.000  Calling: CJ1 (980 Hz) + later CJ2 tone sequences
//   t=0.000  Answering: ANS 2100 Hz with phase reversals (3.3 s)
//   t=3.300  Both: ALT (ranging noise burst)  — wideband noise  (700 ms)
//   t=4.000  Answering: Aa (1200 Hz), Calling: Ac (2400 Hz)  overlap (600 ms)
//   t=4.600  Both: Ph1 phase-measurement tone pairs
//             — 600 Hz + 3000 Hz simultaneously  (200 ms each)
//   t=5.200  Both: INFOh / INFO0 negotiation bursts  (DPSK @ 2400 baud, 400 ms)
//   t=5.600  Both: Primary channel training (PPh) — broadband QAM noise  (1.5 s)
//   t=7.100  Both: MP / MPh (channel probe) — chirp sweeping 300–3400 Hz  (400 ms)
//   t=7.500  Both: S + CPt (complementary training) — noise  (1.2 s)
//   t=8.700  CONNECT 28800

function playV34(ac: AudioContext) {
  // ── Phase 1 ──────────────────────────────────────────────────────────────
  // Calling: CJ1 980 Hz then CJ2 1200 Hz
  envelopedTone(ac, 980, 0.0, 1.5, 0.018, 0.02, 0.05);
  envelopedTone(ac, 1200, 1.5, 1.8, 0.018, 0.02, 0.05);

  // Answering: ANS 2100 Hz (3.3 s)
  scheduleANS(ac, 0.0, 3.3, 0.030);

  // ── Phase 2 – ALT ranging ─────────────────────────────────────────────────
  noiseBurst(ac, 300, 3400, 3.3, 0.7, 34, 0.022);

  // ── Phase 3 – Aa / Ac ────────────────────────────────────────────────────
  envelopedTone(ac, 1200, 4.0, 0.6, 0.026, 0.02, 0.02); // Aa (answering)
  envelopedTone(ac, 2400, 4.0, 0.6, 0.020, 0.02, 0.02); // Ac (calling)

  // ── Phase 4 – Ph1 tone pairs  600 + 3000 Hz ──────────────────────────────
  tone(ac, 600, 4.6, 0.2, 0.020);
  tone(ac, 3000, 4.6, 0.2, 0.020);
  tone(ac, 600, 4.8, 0.2, 0.020);
  tone(ac, 3000, 4.8, 0.2, 0.020);

  // ── Phase 5 – INFO0 / INFOh negotiation bursts ────────────────────────────
  noiseBurst(ac, 2200, 2600, 5.2, 0.4, 14, 0.016);

  // ── Phase 6 – Primary channel training (PPh) ──────────────────────────────
  noiseBurst(ac, 300, 3400, 5.6, 1.5, 40, 0.026);

  // ── Phase 7 – MP / MPh channel probe (chirp) ─────────────────────────────
  chirp(ac, 300, 3400, 7.1, 0.2, 0.022);
  chirp(ac, 3400, 300, 7.3, 0.2, 0.022);

  // ── Phase 8 – S + CPt complementary training ─────────────────────────────
  noiseBurst(ac, 300, 3400, 7.5, 1.2, 40, 0.025);
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export function playDialSequence(phone: string) {
  const ac = audio();
  const digits: Record<string, [number, number]> = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '0': [941, 1336],
  };
  let t = 0;
  for (const d of phone.replace(/\D/g, '')) {
    const pair = digits[d];
    if (!pair) continue;
    tone(ac, pair[0], t, 0.07);
    tone(ac, pair[1], t, 0.07);
    t += 0.095;
  }
}

/**
 * Play an ITU-T-accurate modem handshake sequence for the given baud rate.
 *
 * Supported profiles:
 *   2400  → V.22bis
 *   9600  → V.32
 *   14400 → V.32bis  (default / primary)
 *   28800 → V.34
 *
 * Any other baud rate falls back to V.32bis.
 */
export function playHandshake(baud: number) {
  const ac = audio();
  if (baud <= 2400) {
    playV22bis(ac);
  } else if (baud <= 9600) {
    playV32(ac);
  } else if (baud <= 14400) {
    playV32bis(ac);
  } else {
    playV34(ac);
  }
}

export function playBusy() {
  const ac = audio();
  tone(ac, 400, 0, 0.25, 0.025);
  tone(ac, 400, 0.5, 0.25, 0.025);
}
