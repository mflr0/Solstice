// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { isSkinLoaded, loadSkin } from '../../src/lib/skins.svelte.js';
import { appearance, applySkin } from '../../src/lib/ui.svelte.js';

const root = document.documentElement;
const requests: string[] = [];

beforeEach(() => {
  requests.length = 0;
  vi.stubGlobal('fetch', (url: string) => {
    requests.push(url);
    return Promise.resolve(
      url.includes('/gone/')
        ? new Response('', { status: 404 })
        : new Response(`[data-skin='pkg'] { background: url('assets/a.svg'); }`),
    );
  });
});

describe('loading a skin', () => {
  it('adds the stylesheet, fit to the served id', async () => {
    expect(await loadSkin('one')).toBe(true);
    expect(requests).toEqual(['/api/themes/one/theme.css']);
    const css = document.querySelector('style[data-skin-css="one"]')?.textContent;
    expect(css).toContain("[data-skin='one']");
    expect(css).toContain("url('/api/themes/one/assets/a.svg')");
    expect(isSkinLoaded('one')).toBe(true);
  });

  it('takes the fetch the boot script started', async () => {
    window.solsticeBootSkin = { id: 'two', css: Promise.resolve("[data-skin='two'] { }") };
    expect(await loadSkin('two')).toBe(true);
    expect(requests).toEqual([]);
    expect(window.solsticeBootSkin).toBeUndefined();
  });

  it('tries a failed skin again next time', async () => {
    expect(await loadSkin('gone')).toBe(false);
    expect(await loadSkin('gone')).toBe(false);
    expect(requests).toHaveLength(2);
  });
});

describe('a skin that can not be loaded', () => {
  it('shows Carnet and keeps the stored choice', async () => {
    applySkin('gone');
    expect(root.hasAttribute('data-skin-loading')).toBe(true);
    await vi.waitFor(() => expect(root.hasAttribute('data-skin-loading')).toBe(false));
    expect(appearance.skin).toBe('carnet');
    expect(root.getAttribute('data-skin')).toBe('carnet');
    expect(localStorage.getItem('solstice.skin')).toBe('gone');
    expect(isSkinLoaded('carnet')).toBe(true);
  });
});
