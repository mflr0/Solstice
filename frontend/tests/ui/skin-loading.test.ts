// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { isSkinLoaded, loadSkin } from '../../src/lib/skins.svelte.js';
import { t } from '../../src/i18n/t.js';
import { appearance, applySkin, notes } from '../../src/lib/ui.svelte.js';

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
    expect(notes.map((n) => n.text)).toContain(t.account.themeFallback);
  });

  it('shows Carnet once the timeout is over, and ignores a late answer', async () => {
    vi.useFakeTimers();
    try {
      notes.length = 0;
      let answer: (r: Response) => void = () => {};
      vi.stubGlobal('fetch', (url: string) =>
        url.includes('/slow/')
          ? new Promise<Response>((resolve) => (answer = resolve))
          : Promise.resolve(new Response("[data-skin='carnet'] { }")),
      );
      applySkin('slow');
      await vi.advanceTimersByTimeAsync(4999);
      expect(appearance.skin).toBe('slow');
      await vi.advanceTimersByTimeAsync(1);
      expect(appearance.skin).toBe('carnet');
      expect(root.hasAttribute('data-skin-loading')).toBe(false);
      expect(localStorage.getItem('solstice.skin')).toBe('slow');
      expect(notes.map((n) => n.text)).toEqual([t.account.themeFallback]);

      answer(new Response("[data-skin='slow'] { }"));
      await vi.advanceTimersByTimeAsync(0);
      expect(appearance.skin).toBe('carnet');
      expect(notes).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('a new page load', () => {
  /** The interface as a page load with this address and this stored skin gets it. */
  async function load(search: string, stored: string, listed: string[]) {
    history.replaceState(null, '', `/${search}`);
    localStorage.setItem('solstice.skin', stored);
    vi.stubGlobal('fetch', (url: string) =>
      Promise.resolve(
        url === '/api/themes'
          ? Response.json({ contract: 1, themes: listed.map((id) => ({ id })) })
          : new Response(`[data-skin='pkg'] { }`),
      ),
    );
    vi.resetModules();
    const ui = await import('../../src/lib/ui.svelte.js');
    const { themes } = await import('../../src/lib/skins.svelte.js');
    const stop = ui.initAppearance();
    await vi.waitFor(() => expect(themes.list).toHaveLength(listed.length));
    await vi.waitFor(() => expect(root.hasAttribute('data-skin-loading')).toBe(false));
    stop();
    history.replaceState(null, '', '/');
    return ui;
  }

  it('takes ?skin= and leaves the stored choice alone', async () => {
    const { appearance, notes } = await load('?skin=carnet', 'retro', ['carnet', 'retro']);
    expect(appearance.skin).toBe('carnet');
    expect(localStorage.getItem('solstice.skin')).toBe('retro');
    expect(notes).toHaveLength(0);
  });

  it('shows Carnet and a note when the stored skin is not listed', async () => {
    const { appearance, notes } = await load('', 'nope', ['carnet', 'retro']);
    expect(appearance.skin).toBe('carnet');
    expect(root.getAttribute('data-skin')).toBe('carnet');
    expect(localStorage.getItem('solstice.skin')).toBe('nope');
    expect(notes).toHaveLength(1);
  });

  it('keeps a listed skin', async () => {
    const { appearance, notes } = await load('', 'retro', ['carnet', 'retro']);
    expect(appearance.skin).toBe('retro');
    expect(notes).toHaveLength(0);
  });
});
