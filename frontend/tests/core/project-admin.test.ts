import { describe, expect, it } from 'vitest';

import {
  datesProblem,
  extraSignature,
  hasExtra,
  keyProblem,
  moveItem,
  nextTab,
  normalizeKey,
  otherComponents,
  otherVersions,
  parseTab,
  PROJECT_TABS,
  projectPayload,
  projectProblems,
  releaseTargets,
  suggestKey,
  toggleExtra,
  type ProjectForm,
} from '../../src/core/project-admin.js';

describe('project keys', () => {
  it.each([
    ['Site web', 'SW'],
    ['Marketing', 'MARK'],
    ['Équipe réseau et télécom', 'ERET'],
    ['A', 'AP'],
    ['', ''],
    ['  --  ', ''],
  ])('suggests a key from %j', (name, key) => {
    expect(suggestKey(name)).toBe(key);
  });

  it('always suggests a valid key for a non-empty name', () => {
    for (const name of ['2026 plan', 'x', 'Projet 42', 'été', 'Une très longue suite de mots pour voir']) {
      const key = suggestKey(name);
      expect(keyProblem(key), `${name} → ${key}`).toBeNull();
    }
  });

  it('normalizes what is typed', () => {
    expect(normalizeKey('web-2 été')).toBe('WEB2ETE');
    expect(normalizeKey('abcdefghijklm')).toBe('ABCDEFGHIJ');
  });

  it('checks the pattern', () => {
    expect(keyProblem('')).toBe('keyRequired');
    expect(keyProblem('W')).toBe('keyPattern');
    expect(keyProblem('2WEB')).toBe('keyPattern');
    expect(keyProblem('WEB')).toBeNull();
    expect(keyProblem('ABCDEFGHIJ')).toBeNull();
  });
});

describe('project creation', () => {
  const form = (patch: Partial<ProjectForm> = {}): ProjectForm => ({
    name: 'Site web',
    key: 'SW',
    template: 'software',
    lead: { id: 2, username: 'marie.durand', name: 'Marie Durand', active: true },
    category: null,
    access: 'lead',
    accessId: null,
    description: '',
    ...patch,
  });

  it('finds what is missing', () => {
    expect(projectProblems(form())).toEqual({});
    expect(projectProblems(form({ name: ' ', key: '' }))).toEqual({
      name: 'nameRequired',
      key: 'keyRequired',
    });
    expect(projectProblems(form({ access: 'group' }))).toEqual({ access: 'accessRequired' });
  });

  it('builds the request', () => {
    expect(projectPayload(form({ description: '  Le site  ' }))).toEqual({
      key: 'SW',
      name: 'Site web',
      template: 'software',
      description: 'Le site',
      lead: 2,
      category: null,
      access: null,
    });
    expect(projectPayload(form({ access: 'role', accessId: 3, category: 4 }))).toMatchObject({
      access: { holder: 'role', id: 3 },
      category: 4,
    });
    expect(projectPayload(form({ access: 'everyone' })).access).toEqual({ holder: 'everyone' });
  });
});

describe('settings tabs', () => {
  it('opens on the general tab for administrators, the workflows for others', () => {
    expect(parseTab(null, true)).toBe('general');
    expect(parseTab('versions', true)).toBe('versions');
    expect(parseTab('nope', true)).toBe('general');
    expect(parseTab('versions', false)).toBe('workflows');
  });

  it('cycles with the arrow keys', () => {
    expect(nextTab(PROJECT_TABS, 'notifications', 1)).toBe('general');
    expect(nextTab(PROJECT_TABS, 'general', -1)).toBe('notifications');
    expect(nextTab(PROJECT_TABS, 'components', 1)).toBe('versions');
  });
});

describe('versions and components', () => {
  const versions = [
    { id: 1, name: '1.0', released: true, archived: false },
    { id: 2, name: '1.1', released: false, archived: false },
    { id: 3, name: '2.0', released: false, archived: true },
    { id: 4, name: '1.2', released: false, archived: false },
  ];

  it('checks dates', () => {
    expect(datesProblem('2026-05-02', '2026-05-01')).toBe('startAfterRelease');
    expect(datesProblem('2026-05-01', '2026-05-01')).toBeNull();
    expect(datesProblem('', '2026-05-01')).toBeNull();
  });

  it('offers where issues may go', () => {
    expect(releaseTargets(versions, 2).map((v) => v.id)).toEqual([4]);
    expect(otherVersions(versions, 2).map((v) => v.id)).toEqual([1, 3, 4]);
    expect(
      otherComponents(
        [
          { id: 1, archived: false },
          { id: 2, archived: true },
          { id: 3, archived: false },
        ],
        1,
      ).map((c) => c.id),
    ).toEqual([3]);
  });

  it('reorders', () => {
    expect(moveItem(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a']);
    expect(moveItem(['a', 'b', 'c'], 2, 1)).toEqual(['a', 'c', 'b']);
    expect(moveItem(['a', 'b'], 0, 5)).toEqual(['a', 'b']);
  });
});

describe('project notifications', () => {
  it('adds and removes recipients', () => {
    const r = { event: 'created', kind: 'group' as const, id: 4, name: 'dev' };
    const on = toggleExtra([], r, true);
    expect(hasExtra(on, 'created', 'group', 4)).toBe(true);
    expect(toggleExtra(on, r, true)).toHaveLength(1);
    expect(toggleExtra(on, r, false)).toEqual([]);
    expect(extraSignature([r, { ...r, event: 'assigned' }])).toBe(
      extraSignature([{ ...r, event: 'assigned' }, r]),
    );
  });
});
