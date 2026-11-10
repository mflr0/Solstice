import { describe, expect, it } from 'vitest';

import { SHORTCUTS, type Shortcut } from '../../src/core/shortcuts.js';
import { startsTour, tourSteps, type TourOptions } from '../../src/core/tour.js';

const user: TourOptions = { admin: false, shortcuts: true, bindings: SHORTCUTS, mac: false };

const keysOf = (o: TourOptions, id: string) =>
  tourSteps(o)
    .find((s) => s.id === id)
    ?.keys.map((k) => `${k.action}:${k.binding}`);

describe('tourSteps', () => {
  it('walks the top bar from left to right', () => {
    expect(tourSteps(user).map((s) => s.id)).toEqual([
      'welcome',
      'scope',
      'work',
      'projects',
      'boards',
      'quickSearch',
      'create',
      'themes',
      'account',
    ]);
  });

  it('opens on a centered card and points every other step at an element', () => {
    const [welcome, ...rest] = tourSteps(user);
    expect(welcome?.target).toBeNull();
    expect(rest.every((s) => s.target)).toBe(true);
  });

  it('adds the Admin area step, on the account menu, for administrators only', () => {
    expect(tourSteps(user).some((s) => s.id === 'admin')).toBe(false);
    const steps = tourSteps({ ...user, admin: true });
    expect(steps.at(-1)).toEqual({ id: 'admin', target: 'account', keys: [] });
    expect(steps).toHaveLength(10);
  });

  it('names the search, create and help keys while shortcuts are on', () => {
    expect(keysOf(user, 'quickSearch')).toEqual(['search:/']);
    expect(keysOf(user, 'create')).toEqual(['create:c', 'help:?']);
  });

  it('names no single-character key while shortcuts are off', () => {
    const off = { ...user, shortcuts: false };
    expect(keysOf(off, 'quickSearch')).toEqual([]);
    expect(keysOf(off, 'create')).toEqual([]);
  });

  it('names a key with a modifier while shortcuts are off', () => {
    const bindings: Shortcut[] = SHORTCUTS.map((s) =>
      s.action === 'create' ? { ...s, keys: ['c', 'Alt+n'] } : s,
    );
    expect(keysOf({ ...user, shortcuts: false, bindings }, 'create')).toEqual(['create:Alt+n']);
  });

  it('names nothing for an action left without a key', () => {
    const bindings: Shortcut[] = SHORTCUTS.map((s) =>
      s.action === 'help' ? { ...s, keys: [] } : s,
    );
    expect(keysOf({ ...user, bindings }, 'create')).toEqual(['create:c']);
  });
});

describe('startsTour', () => {
  it('starts until the account has seen the tour', () => {
    expect(startsTour(undefined)).toBe(true);
    expect(startsTour({})).toBe(true);
    expect(startsTour({ tourSeen: false })).toBe(true);
    expect(startsTour({ tourSeen: true })).toBe(false);
  });
});
