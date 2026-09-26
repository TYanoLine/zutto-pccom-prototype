import type { TariffDistanceClass } from './CallerLocation';

export type TariffTimeClass =
  | 'weekday-day'
  | 'weekday-evening'
  | 'deep-night'
  | 'holiday-day'
  | 'holiday-evening';

export type HistoricalTariffTable = {
  yenPerPulse: number;
  pulseSeconds: Record<TariffTimeClass, Record<TariffDistanceClass, number>>;
  telehodai: {
    startMinute: number;
    endMinute: number;
    maxRegisteredNumbers: number;
    eligibleDistanceClasses: TariffDistanceClass[];
  };
};

// NTT dial-call tariff effective from the March 1996 revision.
// Values are "seconds available per 10 yen" and are tax-exclusive, matching
// NTT East/West historical data books. The 1993 consolidation means some old
// distance columns share a rate; these semantic bands preserve the resulting
// tariff steps without vertically reproducing every legacy column.
export const ntt1996TariffTable: HistoricalTariffTable = {
  yenPerPulse: 10,
  pulseSeconds: {
    'weekday-day': {
      local: 180,
      'adjacent-or-20km': 90,
      'up-to-30km': 45,
      'up-to-60km': 36,
      'up-to-100km': 22.5,
      'up-to-160km': 13,
      'over-160km': 13,
    },
    'weekday-evening': {
      local: 180,
      'adjacent-or-20km': 90,
      'up-to-30km': 45,
      'up-to-60km': 36,
      'up-to-100km': 30,
      'up-to-160km': 22.5,
      'over-160km': 18,
    },
    'deep-night': {
      local: 240,
      'adjacent-or-20km': 120,
      'up-to-30km': 60,
      'up-to-60km': 60,
      'up-to-100km': 45,
      'up-to-160km': 30,
      'over-160km': 22.5,
    },
    'holiday-day': {
      local: 180,
      'adjacent-or-20km': 90,
      'up-to-30km': 45,
      'up-to-60km': 36,
      'up-to-100km': 30,
      'up-to-160km': 22.5,
      'over-160km': 18,
    },
    'holiday-evening': {
      local: 180,
      'adjacent-or-20km': 90,
      'up-to-30km': 45,
      'up-to-60km': 36,
      'up-to-100km': 30,
      'up-to-160km': 22.5,
      'over-160km': 18,
    },
  },
  telehodai: {
    startMinute: 23 * 60,
    endMinute: 8 * 60,
    maxRegisteredNumbers: 2,
    // 1995 Telehodai 1800 covered same-MA numbers and Telehodai 3600 covered
    // adjacent/<=20km numbers. We do not invent eligibility for farther calls.
    eligibleDistanceClasses: ['local', 'adjacent-or-20km'],
  },
};

// Exact 1996 national holidays/holidays from the National Astronomical
// Observatory calendar notice. World dates are encoded as UTC wall-clock dates.
export const JAPAN_1996_HOLIDAYS = new Set([
  '1996-01-01',
  '1996-01-15',
  '1996-02-11',
  '1996-02-12',
  '1996-03-20',
  '1996-04-29',
  '1996-05-03',
  '1996-05-04',
  '1996-05-05',
  '1996-05-06',
  '1996-07-20',
  '1996-09-15',
  '1996-09-16',
  '1996-09-23',
  '1996-10-10',
  '1996-11-03',
  '1996-11-04',
  '1996-11-23',
  '1996-12-23',
]);
