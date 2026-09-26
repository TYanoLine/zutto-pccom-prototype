import { describe, expect, it } from 'vitest';
import { DEFAULT_CALLER_LOCATION } from './CallerLocation';
import { PseudoTariffService } from './PseudoTariffService';
import { ntt1996TariffTable } from './pseudoTariffs';

const hakata = '0920000196';
const yokohama = '0451234567';
const service = new PseudoTariffService(
  ntt1996TariffTable,
  [hakata, yokohama],
  DEFAULT_CALLER_LOCATION,
);
const at = (value: string) => new Date(`1996-08-26T${value}Z`);

describe('March 1996 NTT dial-call tariff', () => {
  it('treats the Fukuoka fixture as same-MA and charges 10 yen for a 28-second daytime call', () => {
    expect(service.distanceClass(hakata)).toBe('local');
    expect(service.chargeYen(hakata, at('12:00:00'), at('12:00:00'))).toBe(0);
    expect(service.chargeYen(hakata, at('12:00:00'), at('12:00:28'))).toBe(10);
    expect(service.chargeYen(hakata, at('12:00:00'), at('12:03:00'))).toBe(10);
    expect(service.chargeYen(hakata, at('12:00:00'), at('12:03:01'))).toBe(20);
  });

  it('uses the far-distance weekday daytime band for the Yokohama fixture from Fukuoka', () => {
    expect(service.distanceClass(yokohama)).toBe('over-160km');
    expect(service.chargeYen(yokohama, at('12:00:00'), at('12:00:28'))).toBe(30);
  });

  it('uses the 240-second same-MA deep-night pulse outside Telehodai', () => {
    const noTelehodai = new PseudoTariffService(
      ntt1996TariffTable,
      [],
      DEFAULT_CALLER_LOCATION,
    );
    expect(noTelehodai.chargeYen(hakata, at('23:00:00'), at('23:03:59'))).toBe(10);
    expect(noTelehodai.chargeYen(hakata, at('23:00:00'), at('23:04:01'))).toBe(20);
  });

  it('uses the Saturday/Sunday/holiday discount through 23:00', () => {
    // 1996-08-25 was Sunday. Far-distance holiday daytime/evening: 18 sec / 10 yen.
    const holidayService = new PseudoTariffService(
      ntt1996TariffTable,
      [],
      DEFAULT_CALLER_LOCATION,
    );
    const sunday = (value: string) => new Date(`1996-08-25T${value}Z`);
    expect(holidayService.chargeYen(yokohama, sunday('20:00:00'), sunday('20:00:18'))).toBe(10);
    expect(holidayService.chargeYen(yokohama, sunday('20:00:00'), sunday('20:00:19'))).toBe(20);
  });
});

describe('Telehodai boundaries and eligible distance', () => {
  it.each([
    ['22:59:00', false],
    ['23:00:00', true],
    ['07:59:00', true],
    ['08:00:00', false],
  ])('%s has window=%s', (time, expected) => {
    expect(service.isTelehodaiWindow(at(time))).toBe(expected);
  });

  it('waives a registered same-MA call inside Telehodai', () => {
    expect(service.isTelehodaiCall(hakata, at('23:00:00'))).toBe(true);
    expect(service.chargeYen(hakata, at('23:00:00'), at('23:30:00'))).toBe(0);
  });

  it('does not waive a registered far-distance destination', () => {
    expect(service.isTelehodaiCall(yokohama, at('23:00:00'))).toBe(false);
    expect(service.chargeYen(yokohama, at('23:00:00'), at('23:00:23'))).toBe(20);
  });
});
