import { describe, expect, it, vi } from 'vitest';
import { fetchGenerationTrace, generationTraceEndpoint } from './GenerationInspector';

describe('HAKATA generation trace API', () => {
  it('maps the configured WebSocket server without putting tokens in URLs', () => {
    expect(generationTraceEndpoint('wss://bbs.example/ws')).toBe('https://bbs.example/api/debug/bbs/generation-trace');
    expect(generationTraceEndpoint('ws://localhost:8080/ws')).toBe('http://localhost:8080/api/debug/bbs/generation-trace');
    expect(generationTraceEndpoint('')).toBeNull();
  });

  it('sends the token as a request header and returns a polling snapshot', async () => {
    const fake = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      expect(init?.headers).toEqual({ 'X-Zutto-Debug-Token': 'operator-secret' });
      expect(init?.cache).toBe('no-store');
      return new Response(JSON.stringify({ runs: [], running: true }), { status: 200 });
    });
    const snapshot = await fetchGenerationTrace('wss://bbs.example/ws', 'operator-secret', fake as unknown as typeof fetch);
    expect(snapshot.running).toBe(true);
    expect(String(fake.mock.calls[0][0])).not.toContain('operator-secret');
  });

  it('never exposes unauthorized trace payloads', async () => {
    const fake = vi.fn(async () => new Response('{"error":"denied"}', { status: 403 }));
    await expect(fetchGenerationTrace('wss://bbs.example/ws', 'wrong', fake as unknown as typeof fetch)).rejects.toThrow('認証');
  });
});
