import type { CallerLocation, TariffDistanceClass } from './CallerLocation';
import { resolve1996DistanceClass } from './CallerLocation';
import type { HistoricalTariffTable, TariffTimeClass } from './pseudoTariffs';
import { JAPAN_1996_HOLIDAYS } from './pseudoTariffs';

export class PseudoTariffService {
  private readonly registeredNumbers: Set<string>;

  constructor(
    private readonly table: HistoricalTariffTable,
    registeredNumbers: string[],
    private readonly callerLocation: CallerLocation,
  ) {
    const normalized = registeredNumbers.map(digitsOnly).filter(Boolean);
    if (normalized.length > table.telehodai.maxRegisteredNumbers) {
      throw new Error(`Telehodai accepts at most ${table.telehodai.maxRegisteredNumbers} numbers`);
    }
    this.registeredNumbers = new Set(normalized);
  }

  distanceClass(phone: string): TariffDistanceClass {
    return resolve1996DistanceClass(this.callerLocation, phone);
  }

  isTelehodaiWindow(at: Date): boolean {
    return containsMinute(
      minutesSinceMidnight(at),
      this.table.telehodai.startMinute,
      this.table.telehodai.endMinute,
    );
  }

  isTelehodaiCall(phone: string, at: Date): boolean {
    const distance = this.distanceClass(phone);
    return this.registeredNumbers.has(digitsOnly(phone))
      && this.table.telehodai.eligibleDistanceClasses.includes(distance)
      && this.isTelehodaiWindow(at);
  }

  chargeYen(phone: string, connectedAt: Date, at: Date): number {
    if (at <= connectedAt) return 0;

    const distance = this.distanceClass(phone);
    let charge = 0;
    let cursor = new Date(connectedAt);

    // NTT's table is expressed as "10 yen per N seconds or fraction".
    // Therefore the first non-zero fraction of a pulse is already one 10-yen
    // unit; e.g. a 28-second same-MA daytime call is 10 yen, not 0 yen.
    while (cursor < at) {
      if (this.isTelehodaiCall(phone, cursor)) {
        cursor = new Date(Math.min(at.getTime(), nextMinuteBoundary(cursor).getTime()));
        continue;
      }

      charge += this.table.yenPerPulse;
      const seconds = this.table.pulseSeconds[this.timeClass(cursor)][distance];
      cursor = new Date(cursor.getTime() + seconds * 1000);
    }

    return charge;
  }

  private timeClass(at: Date): TariffTimeClass {
    const minute = minutesSinceMidnight(at);
    if (containsMinute(minute, 23 * 60, 8 * 60)) return 'deep-night';

    const holiday = is1996HolidayOrWeekend(at);
    if (holiday) return minute < 19 * 60 ? 'holiday-day' : 'holiday-evening';
    return minute < 19 * 60 ? 'weekday-day' : 'weekday-evening';
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

function worldDateKey(at: Date) {
  const y = at.getUTCFullYear();
  const m = String(at.getUTCMonth() + 1).padStart(2, '0');
  const d = String(at.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function is1996HolidayOrWeekend(at: Date) {
  const day = at.getUTCDay();
  return day === 0 || day === 6 || JAPAN_1996_HOLIDAYS.has(worldDateKey(at));
}

function nextMinuteBoundary(at: Date) {
  const next = new Date(at);
  next.setUTCSeconds(0, 0);
  next.setUTCMinutes(next.getUTCMinutes() + 1);
  return next;
}
