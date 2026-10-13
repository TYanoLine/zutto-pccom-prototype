import { describe, expect, it, vi } from 'vitest';
import { bbsResetEndpoint, requestBBSReset } from './BBSResetDialog';

describe('BBS reset API', () => {
  it('maps the configured WebSocket server to the reset endpoint', () => {
    expect(bbsResetEndpoint('wss://bbs.example/ws', '0920000196', 'generated')).toBe(
      'https://bbs.example/api/debug/bbs/reset?phone=0920000196',
    );
    expect(bbsResetEndpoint('ws://localhost:8080/ws', '0920000196', 'all')).toBe(
      'http://localhost:8080/api/debug/bbs/reset?phone=0920000196&mode=all',
    );
    expect(bbsResetEndpoint('', '0920000196', 'generated')).toBeNull();
  });

  it('sends POST request with token header and parses result', async () => {
    const fake = vi.fn(async (url: string | URL | Request, init?: RequestInit) => {
      expect(String(url)).toBe('https://bbs.example/api/debug/bbs/reset?phone=0920000196');
      expect(init?.method).toBe('POST');
      expect(init?.headers).toEqual({ 'X-Zutto-Debug-Token': 'test-token' });
      return new Response(
        JSON.stringify({ ok: true, phone: '0920000196', host_id: 'hakata-canal-net', removed_posts: 12, mode: 'generated' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      );
    });

    const result = await requestBBSReset(
      'wss://bbs.example/ws',
      '0920000196',
      'generated',
      'test-token',
      fake as unknown as typeof fetch,
    );

    expect(result.ok).toBe(true);
    expect(result.removed_posts).toBe(12);
    expect(result.mode).toBe('generated');
  });

  it('handles error response from server', async () => {
    const fake = vi.fn(async () =>
      new Response(JSON.stringify({ error: 'debug BBS reset requires valid DEBUG_RESET_TOKEN' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await expect(
      requestBBSReset('wss://bbs.example/ws', '0920000196', 'all', '', fake as unknown as typeof fetch),
    ).rejects.toThrow('DEBUG_RESET_TOKEN');
  });
});
