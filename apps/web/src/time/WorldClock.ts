export interface WorldClock {
  now(): Date;
}
type DateSource = () => Date;

type DateParts = {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
};

const japanFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Tokyo',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  hourCycle: 'h23',
  minute: '2-digit',
  second: '2-digit',
});

function japanParts(date: Date): DateParts {
  const values = Object.fromEntries(
    japanFormatter.formatToParts(date)
      .filter(part => part.type !== 'literal')
      .map(part => [part.type, Number(part.value)]),
  );
  return values as DateParts;
}

function parseWorldDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) throw new Error(`Invalid world date: ${value}`);
  return { year: Number(match[1]), month: Number(match[2]), day: Number(match[3]) };
}

// Date values returned by this clock encode world wall-clock fields as UTC.
// Consumers should use getUTC* so their behavior is independent of browser TZ.
export class Japan1996WorldClock implements WorldClock {
  private readonly source: DateSource;
  private readonly realDayOrigin: number;
  private readonly worldDayOrigin: number;

  constructor(worldDate = '1996-08-26', source: DateSource = () => new Date()) {
    this.source = source;
    const real = japanParts(source());
    const world = parseWorldDate(worldDate);
    this.realDayOrigin = Date.UTC(real.year, real.month - 1, real.day);
    this.worldDayOrigin = Date.UTC(world.year, world.month - 1, world.day);
  }

  now(): Date {
    const sourceNow = this.source();
    const real = japanParts(sourceNow);
    const realDay = Date.UTC(real.year, real.month - 1, real.day);
    const dayOffset = realDay - this.realDayOrigin;
    return new Date(this.worldDayOrigin + dayOffset + (
      ((real.hour * 60 + real.minute) * 60 + real.second) * 1000 + sourceNow.getMilliseconds()
    ));
  }
}

export class FixedWorldClock implements WorldClock {
  constructor(private value: Date) {}

  now(): Date { return new Date(this.value); }

  set(value: Date) { this.value = value; }
}
