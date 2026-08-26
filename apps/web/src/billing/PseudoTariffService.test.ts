import { describe, expect, it } from 'vitest';
import { PseudoTariffService } from './PseudoTariffService';
import { pseudoTariffTable } from './pseudoTariffs';

const registered = '0451234567';
const service = new PseudoTariffService(pseudoTariffTable, [registered, '0450000001']);
const at = (value: string) => new Date(`1996-08-26T${value}:00Z`);

describe('Telehodai boundaries', () => {
  it.each([
    ['22:59', false],
    ['23:00', true],
    ['07:59', true],
    ['08:00', false],
  ])('%s has window=%s', (time, expected) => {
    expect(service.isTelehodaiWindow(at(time))).toBe(expected);
    expect(service.isTelehodaiCall(registered, at(time))).toBe(expected);
  });

  it('does not waive an unregistered destination', () => {
    expect(service.isTelehodaiCall('0459999999', at('23:00'))).toBe(false);
  });
});
describe('pseudo tariff charging', () => {
  it('charges by the configured pulse outside Telehodai', () => {
    expect(service.chargeYen(registered, at('22:00'), at('22:03'))).toBe(10);
    expect(service.chargeYen(registered, at('22:00'), at('22:04'))).toBe(20);
  });

  it('waives a registered call made inside Telehodai', () => {
    expect(service.chargeYen(registered, at('23:00'), at('23:30'))).toBe(0);
  });
});
