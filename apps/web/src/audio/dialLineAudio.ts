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
  const attack = Math.min(0.004, duration / 4);
  const release = Math.min(0.006, duration / 4);

  osc.frequency.value = frequency;
  g.gain.setValueAtTime(0, t0);
  g.gain.linearRampToValueAtTime(gain, t0 + attack);
  g.gain.setValueAtTime(gain, Math.max(t0 + attack, t0 + duration - release));
  g.gain.linearRampToValueAtTime(0, t0 + duration);
  osc.connect(g).connect(ac.destination);
  osc.start(t0);
  osc.stop(t0 + duration);
}

function scheduleClick(ac: AudioContext, start: number, gain = 0.025): void {
  // A very short, slightly dirty line-break click. Two partials make it less
  // like a UI beep and more like a loop-disconnect pulse heard through a modem.
  scheduleTone(ac, 420, start, 0.010, gain);
  scheduleTone(ac, 1350, start, 0.004, gain * 0.55);
}

/**
 * Play dial tone followed by DTMF (ATDT) or loop-disconnect pulses (ATDP).
 * Returns the scheduled duration in seconds so standalone simulation can wait
 * until the number has actually finished dialling before returning BUSY.
 */
export function playDialSequence(phone: string, mode: DialMode = 'tone'): number {
  const ac = audio();
  void ac.resume();

  // Japanese analogue-line flavour: a modest 400 Hz dial tone before digits.
  scheduleTone(ac, 400, 0, 0.62, 0.020);
  let t = 0.78;

  if (mode === 'pulse') {
    for (const digit of phone) {
      if (!/\d/.test(digit)) continue;
      const pulses = digit === '0' ? 10 : Number(digit);
      for (let p = 0; p < pulses; p++) {
        const pulseAt = t + p * 0.100; // classic 10 pps loop-disconnect dialling
        scheduleClick(ac, pulseAt, 0.027);
        scheduleClick(ac, pulseAt + 0.061, 0.016);
      }
      t += pulses * 0.100 + 0.48; // inter-digit rotary return / pause
    }
    return t + 0.10;
  }

  const digits: Record<string, [number, number]> = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '0': [941, 1336], '*': [941, 1209], '#': [941, 1477],
  };

  for (const digit of phone) {
    const pair = digits[digit];
    if (!pair) continue;
    scheduleTone(ac, pair[0], t, 0.085, 0.022);
    scheduleTone(ac, pair[1], t, 0.085, 0.022);
    t += 0.135;
  }
  return t + 0.08;
}

/** Play a recognisable Japanese-style 400 Hz busy cadence. */
export function playBusy(): number {
  const ac = audio();
  void ac.resume();
  const cycles = 4;
  for (let i = 0; i < cycles; i++) {
    scheduleTone(ac, 400, i * 1.0, 0.50, 0.025);
  }
  return cycles;
}
