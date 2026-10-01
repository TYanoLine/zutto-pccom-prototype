import { describe, expect, it, vi } from 'vitest';
import { fetchServerBuildInfo, serverVersionEndpoint, shortBuildCommit } from './ServerBuildInfo';

describe('server build metadata', () => {
  it('derives the metadata URL from the actual configured WebSocket backend', () => {
    expect(serverVersionEndpoint('wss://preview.example.com/ws?x=1')).toBe('https://preview.example.com/api/version');
    expect(serverVersionEndpoint('ws://localhost:8080/ws')).toBe('http://localhost:8080/api/version');
    expect(serverVersionEndpoint('')).toBeNull();
    expect(serverVersionEndpoint('https://not-a-websocket.invalid/ws')).toBeNull();
  });

  it('fetches independently without caching or changing the configured backend', async () => {
    const response = { commit: 'abcdef0123456789', branch: 'main', started_at: '2026-10-01T12:00:00Z' };
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue({
      ok: true,
      json: async () => response,
    } as Response);
    await expect(fetchServerBuildInfo('wss://example.onrender.com/ws', fetcher)).resolves.toEqual(response);
    expect(fetcher).toHaveBeenCalledWith('https://example.onrender.com/api/version', {
      method: 'GET',
      cache: 'no-store',
      signal: expect.any(AbortSignal),
    });
    expect(shortBuildCommit(response.commit)).toBe('abcdef012345');
  });

  it('keeps missing or invalid metadata unknown instead of guessing main', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue({
      ok: true, json: async () => ({ ok: true }),
    } as Response);
    await expect(fetchServerBuildInfo('ws://localhost:8080/ws', fetcher)).rejects.toThrow('invalid server version');
    expect(shortBuildCommit('unknown')).toBe('unknown');
  });

  it('does not request a backend in standalone mode', async () => {
    const fetcher = vi.fn<typeof fetch>();
    await expect(fetchServerBuildInfo('', fetcher)).rejects.toThrow('backend not configured');
    expect(fetcher).not.toHaveBeenCalled();
  });
});
