import { unlockModemAudio } from './modemAudio';

// Telephone-line sounds used before a modem carrier exists.
// The entire sequence is rendered to one PCM buffer before playback.  This is
// intentionally friendlier to iOS Safari than creating many scheduled
// oscillators while an AudioContext is still resuming from a user gesture.

let ctx: AudioContext | null = null;

export type DialMode = 'tone' | 'pulse';

type ToneEvent = {
  start: number;
  duration: number;
  frequency: number;
  gain: number;
  modulationHz?: number;
};

type LinePlan = {
  events: ToneEvent[];
  duration: number;
};

const TAU = Math.PI * 2;

function audio(): AudioContext {
  ctx ??= new AudioContext();
  return ctx;
}

/**
 * Call this from the ATD/CALL user gesture.  Both the telephone-line context
 * and the later modem-handshake context are resumed now, before iOS loses the
 * user activation while dial/ringback sounds are playing.
 */
export function unlockLineAudio(): void {
  try {
    const ac = audio();
    if (ac.state !== 'running') void ac.resume();
  } catch {
    // Audio is atmospheric; callers must continue even without Web Audio.
  }

  unlockModemAudio();
}

function addToneToPcm(pcm: Float32Array, sampleRate: number, event: ToneEvent): void {
  const start = Math.max(0, Math.floor(event.start * sampleRate));
  const length = Math.max(1, Math.floor(event.duration * sampleRate));
  const end = Math.min(pcm.length, start + length);
  const attack = Math.min(0.006, event.duration / 4);
  const release = Math.min(0.010, event.duration / 4);

  for (let i = start; i < end; i++) {
    const local = (i - start) / sampleRate;
    let envelope = 1;
    if (attack > 0 && local < attack) envelope = local / attack;
    const remaining = event.duration - local;
    if (release > 0 && remaining < release) envelope *= Math.max(0, remaining / release);

    const modulation = event.modulationHz
      ? 0.04 + 0.96 * (0.5 + 0.5 * Math.sin(TAU * event.modulationHz * local))
      : 1;
    pcm[i] += Math.sin(TAU * event.frequency * local) * event.gain * envelope * modulation;
  }
}

function playPlan(plan: LinePlan): number {
  try {
    const ac = audio();
    const sampleRate = ac.sampleRate;
    const frameCount = Math.max(1, Math.ceil(plan.duration * sampleRate));
    const pcm = new Float32Array(frameCount);
    for (const event of plan.events) addToneToPcm(pcm, sampleRate, event);

    // Mild line/speaker saturation, mostly to keep summed DTMF partials tidy.
    for (let i = 0; i < pcm.length; i++) pcm[i] = Math.tanh(pcm[i] * 1.18) * 0.92;

    const buffer = ac.createBuffer(1, pcm.length, sampleRate);
    buffer.getChannelData(0).set(pcm);
    const start = () => {
      if (ac.state === 'closed') return;
      const source = ac.createBufferSource();
      source.buffer = buffer;
      source.connect(ac.destination);
      source.start();
    };

    if (ac.state === 'running') {
      start();
    } else {
      void ac.resume().then(start).catch(() => undefined);
    }
  } catch {
    // Keep the telephone state machine working without Web Audio.
  }
  return plan.duration;
}

function dialPlan(phone: string, mode: DialMode): LinePlan {
  const events: ToneEvent[] = [];

  // Japanese analogue PSTN flavour: steady ~400 Hz 「ツー」 before digits.
  events.push({ start: 0, duration: 0.82, frequency: 400, gain: 0.17 });
  let t = 1.02;

  if (mode === 'pulse') {
    for (const digit of phone) {
      if (!/\d/.test(digit)) continue;
      const pulses = digit === '0' ? 10 : Number(digit);
      for (let p = 0; p < pulses; p++) {
        const pulseAt = t + p * 0.100;
        events.push({ start: pulseAt, duration: 0.014, frequency: 420, gain: 0.18 });
        events.push({ start: pulseAt, duration: 0.006, frequency: 1350, gain: 0.085 });
        events.push({ start: pulseAt + 0.061, duration: 0.012, frequency: 420, gain: 0.10 });
      }
      t += pulses * 0.100 + 0.48;
    }
    return { events, duration: t + 0.14 };
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
    events.push({ start: t, duration: 0.115, frequency: pair[0], gain: 0.115 });
    events.push({ start: t, duration: 0.115, frequency: pair[1], gain: 0.115 });
    t += 0.170;
  }
  return { events, duration: t + 0.12 };
}

function busyEvents(start: number, cycles: number): ToneEvent[] {
  const events: ToneEvent[] = [];
  for (let i = 0; i < cycles; i++) {
    events.push({ start: start + i * 1.0, duration: 0.50, frequency: 400, gain: 0.18 });
  }
  return events;
}

function ringbackPlan(cycles: number): LinePlan {
  const count = Math.max(1, cycles);
  const events: ToneEvent[] = [];
  for (let i = 0; i < count; i++) {
    // Japanese analogue ringback: a deeply amplitude-modulated 400 Hz tone.
    // Near-total modulation is intentional; shallow AM sounds like a plain
    // wavering beep on phone speakers rather than the remembered 「プルルル」.
    events.push({
      start: i * 3.0,
      duration: 1.0,
      frequency: 400,
      gain: 0.21,
      modulationHz: 16,
    });
  }
  return {
    events,
    // A BBS normally auto-answers after the first ring, so one cycle does not
    // need the entire 2-second inter-ring silence before the modem answers.
    duration: count === 1 ? 1.22 : (count - 1) * 3.0 + 1.22,
  };
}

/** 発信音 -> DTMF / pulse dialing. */
export function playDialSequence(phone: string, mode: DialMode = 'tone'): number {
  unlockLineAudio();
  return playPlan(dialPlan(phone, mode));
}

/** Caller-side ringing tone heard before the remote modem answers. */
export function playRingback(cycles = 1): number {
  unlockLineAudio();
  return playPlan(ringbackPlan(cycles));
}

/**
 * Complete no-server telephone attempt:
 * ツー -> ピポポ… -> silence -> ツー、ツー、ツー -> BUSY(result text)
 */
export function playStandaloneBusySequence(phone: string, mode: DialMode = 'tone'): number {
  unlockLineAudio();
  const plan = dialPlan(phone, mode);
  const busyStart = plan.duration + 0.85;
  const cycles = 3;
  return playPlan({
    events: plan.events.concat(busyEvents(busyStart, cycles)),
    duration: busyStart + cycles,
  });
}

/** Recognisable Japanese-style 400 Hz busy cadence. */
export function playBusy(): number {
  unlockLineAudio();
  const cycles = 3;
  return playPlan({ events: busyEvents(0, cycles), duration: cycles });
}
