// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';

import { accountApi } from '../../src/lib/admin-api.js';

afterEach(() => vi.unstubAllGlobals());

const json = (body: unknown) => async () =>
  new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });

describe('mcpAddress', () => {
  it('takes the resource of the protected resource metadata', async () => {
    const fetch = vi.fn(json({ resource: 'https://solstice.example/base/mcp' }));
    vi.stubGlobal('fetch', fetch);
    expect(await accountApi.mcpAddress()).toBe('https://solstice.example/base/mcp');
    expect(fetch).toHaveBeenCalledWith('/.well-known/oauth-protected-resource', expect.anything());
  });

  const fallbacks: [string, () => Promise<Response>][] = [
    ['a rejected fetch', () => Promise.reject(new TypeError('Failed to fetch'))],
    ['a 500', async () => new Response(null, { status: 500 })],
    ['an HTML page', async () => new Response('<!doctype html>', { status: 200 })],
    ['no resource', json({ resource_name: 'Solstice' })],
  ];
  for (const [name, answer] of fallbacks) {
    it(`falls back to this origin's /mcp on ${name}`, async () => {
      vi.stubGlobal('fetch', vi.fn(answer));
      expect(await accountApi.mcpAddress()).toBe(`${location.origin}/mcp`);
    });
  }
});
