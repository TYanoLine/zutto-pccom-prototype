import { describe, expect, it, vi } from 'vitest';
import { fetchGenerationTrace, generationTraceEndpoint } from './GenerationInspector';

describe('generation trace API', () => {
  it('maps the configured WebSocket server to a read-only trace endpoint', () => {
    expect(generationTraceEndpoint('wss://bbs.example/ws')).toBe('https://bbs.example/api/debug/bbs/generation-trace');
    expect(generationTraceEndpoint('ws://localhost:8080/ws')).toBe('http://localhost:8080/api/debug/bbs/generation-trace');
    expect(generationTraceEndpoint('')).toBeNull();
  });

  it('polls without a debug credential and returns a snapshot', async () => {
    const fake = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      expect(init?.headers).toBeUndefined();
      expect(init?.method).toBe('GET');
      expect(init?.cache).toBe('no-store');
      return new Response(JSON.stringify({ runs: [], running: true }), { status: 200 });
    });
    const snapshot = await fetchGenerationTrace('wss://bbs.example/ws', fake as unknown as typeof fetch);
    expect(snapshot.running).toBe(true);
    expect(String(fake.mock.calls[0][0])).toBe('https://bbs.example/api/debug/bbs/generation-trace');
  });

  it('reports when capture is disabled by the server', async () => {
    const fake = vi.fn(async () => new Response('{"error":"disabled"}', { status: 403 }));
    await expect(fetchGenerationTrace('wss://bbs.example/ws', fake as unknown as typeof fetch)).rejects.toThrow('無効');
  });
});
