export type TimeBand = {
  id: string;
  startMinute: number;
  endMinute: number;
  pulseSeconds: number;
};
export type DistanceBand = {
  id: string;
  destinationPrefixes: string[];
  yenPerPulse: number;
};

export type PseudoTariffTable = {
  timeBands: TimeBand[];
  distanceBands: DistanceBand[];
  defaultDistanceBand: Omit<DistanceBand, 'destinationPrefixes'>;
  telehodai: {
    startMinute: number;
    endMinute: number;
    maxRegisteredNumbers: number;
  };
};

// Atmospheric prototype values, intentionally not asserted as historical NTT
// tariffs. Keeping them as records makes researched replacements data-only.
export const pseudoTariffTable: PseudoTariffTable = {
  timeBands: [
    { id: 'day', startMinute: 8 * 60, endMinute: 19 * 60, pulseSeconds: 180 },
    { id: 'evening', startMinute: 19 * 60, endMinute: 23 * 60, pulseSeconds: 210 },
    { id: 'late-night', startMinute: 23 * 60, endMinute: 8 * 60, pulseSeconds: 240 },
  ],
  distanceBands: [
    { id: 'yokohama-local', destinationPrefixes: ['045'], yenPerPulse: 10 },
    { id: 'tokyo-nearby', destinationPrefixes: ['03'], yenPerPulse: 20 },
  ],
  defaultDistanceBand: { id: 'long-distance', yenPerPulse: 30 },
  telehodai: {
    startMinute: 23 * 60,
    endMinute: 8 * 60,
    maxRegisteredNumbers: 2,
  },
};
