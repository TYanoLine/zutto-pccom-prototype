import { describe, expect, it } from 'vitest';
import { noCapabilities, parseHostCapabilities } from './HostCapabilities';

describe('HostCapabilities', () => {
  describe('parseHostCapabilities', () => {
    it('returns false for undefined and null', () => {
      expect(parseHostCapabilities(undefined)).toEqual({ generationTrace: false });
      expect(parseHostCapabilities(null)).toEqual({ generationTrace: false });
    });

    it('returns false for non-object values', () => {
      expect(parseHostCapabilities('x')).toEqual({ generationTrace: false });
      expect(parseHostCapabilities(42)).toEqual({ generationTrace: false });
    });

    it('returns true when generation_trace is exactly true', () => {
      expect(parseHostCapabilities({ generation_trace: true })).toEqual({ generationTrace: true });
    });

    it('returns false for truthy but non-boolean values', () => {
      expect(parseHostCapabilities({ generation_trace: 'true' })).toEqual({ generationTrace: false });
      expect(parseHostCapabilities({ generation_trace: 1 })).toEqual({ generationTrace: false });
    });

    it('returns false for empty object', () => {
      expect(parseHostCapabilities({})).toEqual({ generationTrace: false });
    });
  });

  describe('noCapabilities', () => {
    it('returns a new object each time', () => {
      const cap1 = noCapabilities();
      const cap2 = noCapabilities();
      expect(cap1).not.toBe(cap2);
      expect(cap1).toEqual(cap2);
    });
  });
});
