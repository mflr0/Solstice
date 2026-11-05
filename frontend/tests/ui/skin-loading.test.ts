// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { forgetSkin, isSkinLoaded, loadSkin, reloadSkin } from '../../src/lib/skins.svelte.js';
import { t } from '../../src/i18n/t.js';
import { appearance, applySkin, notes } from '../../src/lib/ui.svelte.js';

const root = document.documentElement;
const requests: string[] = [];
const caches: (RequestCache | undefined)[] = [];

beforeEach(() => {
  requests.length = 0;
  caches.length = 0;
  vi.stubGlobal('fetch', (url: string, init?: RequestInit) => {
    requests.push(url);
    caches.push(init?.cache);
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

describe('reloading a skin', () => {
  it('swaps the stylesheet for the new one', async () => {
    await loadSkin('three');
    const old = document.querySelector('style[data-skin-css="three"]');
    await reloadSkin('three');
    expect(requests).toEqual(['/api/themes/three/theme.css', '/api/themes/three/theme.css']);
    const styles = document.querySelectorAll('style[data-skin-css="three"]');
    expect(styles).toHaveLength(1);
    expect(styles[0]).not.toBe(old);
    expect(caches).toEqual([undefined, 'no-cache']);
  });

  it('fetches nothing for a skin never asked for', async () => {
    await reloadSkin('four');
    expect(requests).toEqual([]);
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

describe('a local theme', () => {
  it('that is not in this browser gives Carnet and the toast, without asking the server', async () => {
    notes.length = 0;
    applySkin('local-x');
    await vi.waitFor(() => expect(appearance.skin).toBe('carnet'));
    expect(requests.filter((url) => url.includes('local-x'))).toEqual([]);
    expect(notes.map((n) => n.text)).toEqual([t.account.themeFallback]);
    expect(isSkinLoaded('local-x')).toBe(false);
  });

  it('comes from the boot script when it read it first', async () => {
    window.solsticeBootSkin = {
      id: 'local-y',
      css: Promise.resolve("[data-skin='y'] { src: url(data:font/woff2;base64,AAAA); }"),
    };
    expect(await loadSkin('local-y')).toBe(true);
    const css = document.querySelector('style[data-skin-css="local-y"]')?.textContent;
    expect(css).toContain("[data-skin='local-y']");
    expect(css).toContain('url(data:font/woff2;base64,AAAA)');
    expect(requests).toEqual([]);
  });

  it('leaves the page when it is forgotten, and can come back', async () => {
    window.solsticeBootSkin = { id: 'local-z', css: Promise.resolve("[data-skin='z'] { }") };
    await loadSkin('local-z');
    forgetSkin('local-z');
    expect(isSkinLoaded('local-z')).toBe(false);
    expect(document.querySelector('style[data-skin-css="local-z"]')).toBeNull();
    window.solsticeBootSkin = { id: 'local-z', css: Promise.resolve("[data-skin='z'] { }") };
    expect(await loadSkin('local-z')).toBe(true);
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
