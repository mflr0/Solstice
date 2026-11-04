import { describe, expect, it } from 'vitest';

import {
  findingPlace,
  previewOpen,
  themeActions,
  themeFindings,
} from '../../src/core/admin-themes.js';

describe('themeFindings', () => {
  it('reads the findings of a refused import', () => {
    const finding = { rule: 'tokens', message: 'missing --ink', file: 'theme.css', line: 3 };
    expect(themeFindings({ error: 'refused', findings: [finding] })).toEqual([finding]);
  });

  it.each([[undefined], [null], ['text'], [{ error: 'other' }], [{ findings: 'x' }]])(
    'finds none in %j',
    (data) => {
      expect(themeFindings(data)).toEqual([]);
    },
  );
});

describe('findingPlace', () => {
  it('gives the file and line', () => {
    expect(findingPlace({ file: 'theme.css', line: 12 })).toBe('theme.css:12');
  });

  it('gives the file alone when the finding concerns all of it', () => {
    expect(findingPlace({ file: 'theme.json' })).toBe('theme.json');
  });
});

describe('previewOpen', () => {
  it('opens an enabled theme and keeps a disabled one collapsed', () => {
    expect(previewOpen({ enabled: true }, undefined)).toBe(true);
    expect(previewOpen({ enabled: false }, undefined)).toBe(false);
  });

  it('follows the administrator once they toggled it', () => {
    expect(previewOpen({ enabled: true }, false)).toBe(false);
    expect(previewOpen({ enabled: false }, true)).toBe(true);
  });
});

describe('themeActions', () => {
  it('keeps the default theme on', () => {
    expect(themeActions({ id: 'carnet', origin: 'builtin' })).toEqual({
      switchOff: false,
      replace: false,
      remove: false,
    });
  });

  it('lets a built-in theme be turned off, not replaced or deleted', () => {
    expect(themeActions({ id: 'plan', origin: 'builtin' })).toEqual({
      switchOff: true,
      replace: false,
      remove: false,
    });
  });

  it('allows everything on an imported theme', () => {
    expect(themeActions({ id: 'plan-2', origin: 'imported' })).toEqual({
      switchOff: true,
      replace: true,
      remove: true,
    });
  });
});
