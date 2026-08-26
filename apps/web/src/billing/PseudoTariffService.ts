import type { PseudoTariffTable, TimeBand } from './pseudoTariffs';

export class PseudoTariffService {
  private readonly registeredNumbers: Set<string>;

  constructor(
    private readonly table: PseudoTariffTable,
    registeredNumbers: string[],
  ) {
    const normalized = registeredNumbers.map(digitsOnly).filter(Boolean);
    if (normalized.length > table.telehodai.maxRegisteredNumbers) {
      throw new Error(`Telehodai accepts at most ${table.telehodai.maxRegisteredNumbers} numbers`);
    }
    this.registeredNumbers = new Set(normalized);
  }

  isTelehodaiWindow(at: Date): boolean {
    return containsMinute(
      minutesSinceMidnight(at),
      this.table.telehodai.startMinute,
      this.table.telehodai.endMinute,
    );
  }

  isTelehodaiCall(phone: string, at: Date): boolean {
    return this.registeredNumbers.has(digitsOnly(phone)) && this.isTelehodaiWindow(at);
  }

  chargeYen(phone: string, connectedAt: Date, at: Date): number {
    if (at <= connectedAt) return 0;

    const distance = this.table.distanceBands.find(band =>
      band.destinationPrefixes.some(prefix => digitsOnly(phone).startsWith(prefix)),
    ) ?? this.table.defaultDistanceBand;

    let charge = 0;
    let cursor = new Date(connectedAt);
    while (cursor < at) {
      if (this.isTelehodaiCall(phone, cursor)) {
        cursor = new Date(Math.min(at.getTime(), nextMinuteBoundary(cursor).getTime()));
        continue;
      }
      charge += distance.yenPerPulse;
      const band = this.timeBand(cursor);
      cursor = new Date(cursor.getTime() + band.pulseSeconds * 1000);
    }
    return charge;
  }

  private timeBand(at: Date): TimeBand {
    const minute = minutesSinceMidnight(at);
    const band = this.table.timeBands.find(candidate =>
      containsMinute(minute, candidate.startMinute, candidate.endMinute),
    );
    if (!band) throw new Error(`No tariff time band for minute ${minute}`);
    return band;
  }
}
function digitsOnly(value: string) {
  return value.replace(/\D/g, '');
}

function minutesSinceMidnight(at: Date) {
  return at.getUTCHours() * 60 + at.getUTCMinutes();
}

function containsMinute(minute: number, start: number, end: number) {
  return start < end
    ? minute >= start && minute < end
    : minute >= start || minute < end;
}

function nextMinuteBoundary(at: Date) {
  const next = new Date(at);
  next.setUTCSeconds(0, 0);
  next.setUTCMinutes(next.getUTCMinutes() + 1);
  return next;
}
