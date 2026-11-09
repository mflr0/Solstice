// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { timeZone } from '../../src/i18n/format.js';
import RelTime from '../../src/ui/RelTime.svelte';

let component: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
  vi.useRealTimers();
});

function rel(iso: string): HTMLElement {
  const target = document.createElement('div');
  document.body.append(target);
  component = mount(RelTime, { target, props: { iso } });
  flushSync();
  return target;
}

describe('RelTime', () => {
  const iso = '2026-10-27T13:32:00Z';

  it('has no native tooltip, and gives screen readers the exact date', () => {
    const target = rel(iso);
    const time = target.querySelector('time');
    expect(time?.hasAttribute('title')).toBe(false);
    expect(time?.getAttribute('datetime')).toBe(iso);
    expect(target.querySelector('.sr-only')?.textContent).toContain(timeZone());
    expect(target.querySelector('.reltime-tip')).toBeNull();
  });

  it('shows the exact date in a popup on hover, hidden from screen readers', () => {
    const target = rel(iso);
    const time = target.querySelector('time')!;
    time.dispatchEvent(new Event('pointerenter'));
    vi.advanceTimersByTime(300);
    flushSync();
    const tip = target.querySelector('.reltime-tip');
    expect(tip?.getAttribute('aria-hidden')).toBe('true');
    expect(tip?.getAttribute('popover')).toBe('manual');
    expect(tip?.querySelector('.reltime-day')?.textContent).toMatch(/2026/);
    expect(tip?.querySelector('.reltime-time')?.textContent).toContain(`· ${timeZone()}`);
    time.dispatchEvent(new Event('pointerleave'));
    flushSync();
    expect(target.querySelector('.reltime-tip')).toBeNull();
  });
});
