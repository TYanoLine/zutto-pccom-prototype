let ctx: AudioContext | null = null;

function audio() {
  ctx ??= new AudioContext();
  return ctx;
}

function tone(freq: number, start: number, duration: number, gain = 0.03) {
  const ac = audio();
  const osc = ac.createOscillator();
  const g = ac.createGain();
  osc.frequency.value = freq;
  g.gain.value = gain;
  osc.connect(g).connect(ac.destination);
  osc.start(ac.currentTime + start);
  osc.stop(ac.currentTime + start + duration);
}

export function playDialSequence(phone: string) {
  const digits: Record<string, [number, number]> = {
    '1':[697,1209], '2':[697,1336], '3':[697,1477],
    '4':[770,1209], '5':[770,1336], '6':[770,1477],
    '7':[852,1209], '8':[852,1336], '9':[852,1477],
    '0':[941,1336],
  };
  let t = 0;
  for (const d of phone.replace(/\D/g, '')) {
    const pair = digits[d];
    if (!pair) continue;
    tone(pair[0], t, 0.07); tone(pair[1], t, 0.07);
    t += 0.095;
  }
}

export function playHandshake(baud: number) {
  // Stylized, not protocol-accurate: replace with captured/synthesized modem profiles later.
  const base = baud >= 28800 ? 1800 : baud >= 14400 ? 1650 : 1500;
  for (let i = 0; i < 18; i++) tone(base + ((i * 373) % 1400), i * 0.055, 0.045, 0.02);
  tone(2100, 1.03, 0.22, 0.025);
}

export function playBusy() {
  tone(400, 0, 0.25, 0.025); tone(400, 0.5, 0.25, 0.025);
}
