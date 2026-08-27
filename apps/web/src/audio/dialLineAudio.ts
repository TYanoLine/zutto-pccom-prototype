// Telephone-line sounds used before a modem carrier exists.
// These are deliberately separate from modemAudio.ts: ATD can work while the
// application is completely standalone and before any WebSocket is created.

let ctx: AudioContext | null = null;

export type DialMode = 'tone' | 'pulse';

function audio(): AudioContext {
  ctx ??= new AudioContext();
  return ctx;
}

function scheduleTone(
  ac: AudioContext,
  frequency: number,
  start: number,
  duration: number,
  gain: number,
): void {
  const osc = ac.createOscillator();
  const g = ac.createGain();
  const t0 = ac.currentTime + start;
  const attack = Math.min(0.006, duration / 4);
  const release = Math.min(0.010, duration / 4);

  osc.frequency.value = frequency;
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(gain, t0 + attack);
  g.gain.setValueAtTime(gain, Math.max(t0 + attack, t0 + duration - release));
  g.gain.linearRampToValueAtTime(0, t0 + duration);
  osc.connect(g).connect(ac.destination);
  osc.start(t0);
  osc.stop(t0 + duration);
}

function scheduleClick(ac: AudioContext, start: number, gain = 0.095): void {
  // A very short, slightly dirty line-break click. Two partials make it less
  // like a UI beep and more like a loop-disconnect pulse heard through a modem.
  scheduleTone(ac, 420, start, 0.014, gain);
  scheduleTone(ac, 1350, start, 0.006, gain * 0.55);
}

/**
 * Play Japanese-flavoured dial tone followed by DTMF (ATDT) or
 * loop-disconnect pulses (ATDP).
 *
 * Returns the scheduled duration in seconds so the standalone telephone-line
 * simulation can wait until the number has actually finished dialling before
 * starting its post-dial silence / busy cadence.
 */
export function playDialSequence(phone: string, mode: DialMode = 'tone'): number {
  const ac = audio();
  void ac.resume();

  // The old implementation used gains around 0.02, which was almost inaudible
  // through an iPhone speaker even though modem handshakes were loud. Keep the
  // line sounds comfortably below clipping but intentionally speaker-audible.
  // Japanese analogue PSTN flavour: steady ~400 Hz "ツー" before digits.
  scheduleTone(ac, 400, 0, 0.82, 0.105);
  let t = 1.02;

  if (mode === 'pulse') {
    for (const digit of phone) {
      if (!/\d/.test(digit)) continue;
      const pulses = digit === '0' ? 10 : Number(digit);
      for (let p = 0; p < pulses; p++) {
        const pulseAt = t + p * 0.100; // 10 pps loop-disconnect dialling
        scheduleClick(ac, pulseAt, 0.105);
        scheduleClick(ac, pulseAt + 0.061, 0.060);
      }
      t += pulses * 0.100 + 0.48;
    }
    return t + 0.14;
  }

  const digits: Record<string, [number, number]> = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '0': [941, 1336], '*': [941, 1209], '#': [941, 1477],
  };

  // Deliberately a little slower and louder than a modern phone UI: this is
  // meant to sound like digits monitored through a 1990s modem speaker.
  for (const digit of phone) {
    const pair = digits[digit];
    if (!pair) continue;
    scheduleTone(ac, pair[0], t, 0.105, 0.074);
    scheduleTone(ac, pair[1], t, 0.105, 0.074);
    t += 0.155;
  }
  return t + 0.10;
}

/**
 * Play a recognisable Japanese-style 400 Hz busy cadence:
 *   ツー (0.5 s) / silent (0.5 s), three times.
 * Returns the full audible cadence duration.
 */
export function playBusy(): number {
  const ac = audio();
  void ac.resume();
  const cycles = 3;
  for (let i = 0; i < cycles; i++) {
    scheduleTone(ac, 400, i * 1.0, 0.50, 0.115);
  }
  return cycles;
}
