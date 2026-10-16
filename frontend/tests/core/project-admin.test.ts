import { describe, expect, it } from 'vitest';

import {
  categoryProblem,
  componentPayload,
  componentProblem,
  datesProblem,
  dayProblem,
  generalForm,
  generalPatch,
  generalProblems,
  nameProblem,
  orderAfterStep,
  toggleIssueType,
  validDay,
  versionLate,
  versionPayload,
  versionProblems,
  versionState,
  withMember,
  filterProjects,
  issueTypesProblem,
  parseCategoryFilter,
  parseShow,
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
  type GeneralForm,
  type ProjectForm,
  type ProjectSettings,
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
    for (const name of [
      '2026 plan',
      'x',
      'Projet 42',
      'été',
      'Une très longue suite de mots pour voir',
    ]) {
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

describe('creation: issue types and default assignee', () => {
  const form: ProjectForm = {
    name: 'Site web',
    key: 'SW',
    template: 'software',
    lead: null,
    category: null,
    access: 'lead',
    accessId: null,
    description: '',
  };
  const types = [
    { id: 'task', subtask: false },
    { id: 'bug', subtask: false },
    { id: 'sub', subtask: true },
  ];

  it('needs one type that is not a sub-task', () => {
    expect(issueTypesProblem(['task'], types)).toBeNull();
    expect(issueTypesProblem(['sub'], types)).toBe('typesRequired');
    expect(issueTypesProblem([], types)).toBe('typesRequired');
    expect(issueTypesProblem(['sub', 'bug'], types)).toBeNull();
  });

  it('sends them only when chosen', () => {
    expect(projectPayload(form)).not.toHaveProperty('issueTypes');
    expect(projectPayload(form)).not.toHaveProperty('defaultAssignee');
    expect(
      projectPayload({ ...form, issueTypes: ['task'], defaultAssignee: 'lead' }),
    ).toMatchObject({ issueTypes: ['task'], defaultAssignee: 'lead' });
  });
});

describe('administrators project list', () => {
  const dev = { id: 1, name: 'Development' };
  const rows = [
    {
      key: 'SW',
      name: 'Site web',
      archived: false,
      lead: { name: 'Marie Durand', username: 'marie.durand' },
      category: dev,
    },
    { key: 'OPS', name: 'Opérations', archived: true, lead: null, category: null },
    {
      key: 'MKT',
      name: 'Marketing',
      archived: false,
      lead: { name: 'Léa Martin', username: 'lea' },
      category: null,
    },
  ];
  const keys = (r: typeof rows) => r.map((x) => x.key);

  it('searches key, name, lead and category without accents or case', () => {
    expect(keys(filterProjects(rows, '', 'all', 'all'))).toEqual(['SW', 'OPS', 'MKT']);
    expect(keys(filterProjects(rows, 'operations', 'all', 'all'))).toEqual(['OPS']);
    expect(keys(filterProjects(rows, 'lea', 'all', 'all'))).toEqual(['MKT']);
    expect(keys(filterProjects(rows, 'durand dev', 'all', 'all'))).toEqual(['SW']);
    expect(keys(filterProjects(rows, 'mkt', 'all', 'all'))).toEqual(['MKT']);
  });

  it('filters by state and category', () => {
    expect(keys(filterProjects(rows, '', 'active', 'all'))).toEqual(['SW', 'MKT']);
    expect(keys(filterProjects(rows, '', 'archived', 'all'))).toEqual(['OPS']);
    expect(keys(filterProjects(rows, '', 'all', 'none'))).toEqual(['OPS', 'MKT']);
    expect(keys(filterProjects(rows, '', 'all', 1))).toEqual(['SW']);
    expect(keys(filterProjects(rows, '', 'archived', 1))).toEqual([]);
  });

  it('reads the filters from the address', () => {
    expect(parseShow('archived')).toBe('archived');
    expect(parseShow('x')).toBe('all');
    expect(parseShow(null)).toBe('all');
    expect(parseCategoryFilter('none')).toBe('none');
    expect(parseCategoryFilter('7')).toBe(7);
    expect(parseCategoryFilter('-2')).toBe('all');
    expect(parseCategoryFilter('abc')).toBe('all');
    expect(parseCategoryFilter(null)).toBe('all');
  });
});

describe('category names', () => {
  const cats = [
    { id: 1, name: 'Development' },
    { id: 2, name: 'Support' },
  ];

  it('checks the name', () => {
    expect(categoryProblem('  ', cats, null)).toBe('nameRequired');
    expect(categoryProblem('x'.repeat(256), cats, null)).toBe('nameTooLong');
    expect(categoryProblem('development', cats, null)).toBe('nameTaken');
    expect(categoryProblem('Development', cats, 1)).toBeNull();
    expect(categoryProblem('Sales', cats, null)).toBeNull();
  });
});

describe('general tab', () => {
  const saved: ProjectSettings = {
    id: 1,
    key: 'SW',
    name: 'Website',
    description: null,
    kind: 'software',
    lead: { id: 3, username: 'ada', name: 'Ada', active: true },
    category: { id: 2, name: 'Web' },
    defaultAssignee: 'unassigned',
    archived: false,
    issueTypes: [
      { id: 'task', name: 'Task', subtask: false },
      { id: 'sub', name: 'Sub-task', subtask: true },
    ],
    issues: 4,
  };
  const all = [
    { id: 'task', subtask: false },
    { id: 'bug', subtask: false },
    { id: 'sub', subtask: true },
  ];

  it('starts from the saved project and changes nothing', () => {
    expect(generalPatch(generalForm(saved), saved)).toEqual({});
  });

  it('sends only what changed', () => {
    const f: GeneralForm = {
      ...generalForm(saved),
      name: '  Site  ',
      description: ' About us ',
      category: null,
      defaultAssignee: 'lead',
    };
    expect(generalPatch(f, saved)).toEqual({
      name: 'Site',
      description: 'About us',
      category: null,
      defaultAssignee: 'lead',
    });
  });

  it('clears the description with null, and treats blank as unchanged when it was empty', () => {
    expect(generalPatch({ ...generalForm(saved), description: '   ' }, saved)).toEqual({});
    const withText = { ...saved, description: 'Old' };
    expect(generalPatch({ ...generalForm(withText), description: '' }, withText)).toEqual({
      description: null,
    });
  });

  it('sends the lead by id and the types as a set', () => {
    const f = {
      ...generalForm(saved),
      lead: { id: 9, username: 'bob', name: 'Bob', active: true },
      issueTypes: ['sub', 'task'],
    };
    expect(generalPatch(f, saved)).toEqual({ lead: 9 });
    expect(generalPatch({ ...f, issueTypes: ['task', 'bug'] }, saved).issueTypes).toEqual([
      'task',
      'bug',
    ]);
  });

  it('finds what is missing', () => {
    const f = generalForm(saved);
    expect(generalProblems(f, all)).toEqual({});
    expect(generalProblems({ ...f, name: ' ', lead: null, issueTypes: ['sub'] }, all)).toEqual({
      name: 'nameRequired',
      lead: 'leadRequired',
      types: 'typesRequired',
    });
    expect(generalProblems({ ...f, name: 'x'.repeat(256) }, all).name).toBe('nameTooLong');
  });

  it('keeps ticked types in the catalogue order', () => {
    expect(toggleIssueType(all, ['sub', 'task'], 'bug', true)).toEqual(['task', 'bug', 'sub']);
    expect(toggleIssueType(all, ['task', 'bug'], 'task', false)).toEqual(['bug']);
  });
});

describe('components', () => {
  const form = (o: Partial<Parameters<typeof componentPayload>[0]> = {}) => ({
    name: ' Api ',
    description: '',
    lead: null,
    assigneeType: 'project_default' as const,
    ...o,
  });

  it('checks the name like every name', () => {
    const others = [{ id: 1, name: 'Api' }];
    expect(nameProblem('api', others, null)).toBe('nameTaken');
    expect(nameProblem('api', others, 1)).toBeNull();
    expect(nameProblem(' ', others, null)).toBe('nameRequired');
  });

  it('needs a lead when the lead decides the assignee', () => {
    expect(componentProblem(form({ assigneeType: 'component_lead' }))).toBe('leadNeeded');
    expect(componentProblem(form())).toBeNull();
  });

  it('sends a cleared lead as null', () => {
    expect(componentPayload(form())).toEqual({
      name: 'Api',
      description: '',
      lead: null,
      assigneeType: 'project_default',
    });
    expect(
      componentPayload(form({ lead: { id: 5, username: 'u', name: 'U', active: true } })).lead,
    ).toBe(5);
  });
});

describe('versions', () => {
  it('reads real calendar days only', () => {
    expect(validDay('2026-02-28')).toBe(true);
    expect(validDay('2028-02-29')).toBe(true);
    expect(validDay('2026-02-30')).toBe(false);
    expect(validDay('2026-13-01')).toBe(false);
    expect(validDay('1/2/2026')).toBe(false);
    expect(dayProblem('')).toBeNull();
    expect(dayProblem('soon')).toBe('dayInvalid');
  });

  it('checks the dates of a version', () => {
    const f = { name: '1.0', description: '', startDate: '', releaseDate: '' };
    expect(versionProblems(f)).toEqual({});
    expect(versionProblems({ ...f, startDate: '2026-05-02', releaseDate: '2026-05-01' })).toEqual({
      dates: 'startAfterRelease',
    });
    expect(versionProblems({ ...f, startDate: 'x', releaseDate: '2026-05-01' })).toEqual({
      start: 'dayInvalid',
    });
  });

  it('sends empty days as null', () => {
    expect(
      versionPayload({ name: ' 1.1 ', description: ' ', startDate: '', releaseDate: '2026-06-01' }),
    ).toEqual({ name: '1.1', description: '', startDate: null, releaseDate: '2026-06-01' });
  });

  it('tells where a version stands', () => {
    expect(versionState({ released: true, archived: false })).toBe('released');
    expect(versionState({ released: true, archived: true })).toBe('archived');
    expect(versionState({ released: false, archived: false })).toBe('unreleased');
    const v = { released: false, archived: false, releaseDate: '2026-05-01' };
    expect(versionLate(v, '2026-05-02')).toBe(true);
    expect(versionLate(v, '2026-05-01')).toBe(false);
    expect(versionLate({ ...v, released: true }, '2026-06-01')).toBe(false);
    expect(versionLate({ ...v, releaseDate: null }, '2026-06-01')).toBe(false);
  });

  it('moves one place with the buttons, and stops at the ends', () => {
    expect(orderAfterStep([1, 2, 3], 2, -1)).toEqual([2, 1, 3]);
    expect(orderAfterStep([1, 2, 3], 2, 1)).toEqual([1, 3, 2]);
    expect(orderAfterStep([1, 2, 3], 1, -1)).toBeNull();
    expect(orderAfterStep([1, 2, 3], 3, 1)).toBeNull();
    expect(orderAfterStep([1, 2, 3], 9, 1)).toBeNull();
  });
});

describe('project roles', () => {
  const role = { users: [{ id: 1 }, { id: 2 }], groups: [{ id: 7 }] };

  it('adds and removes a user or a group without touching the other list', () => {
    expect(withMember(role, 'user', 3, true)).toEqual({ users: [1, 2, 3], groups: [7] });
    expect(withMember(role, 'user', 1, false)).toEqual({ users: [2], groups: [7] });
    expect(withMember(role, 'group', 8, true)).toEqual({ users: [1, 2], groups: [7, 8] });
    expect(withMember(role, 'group', 7, false)).toEqual({ users: [1, 2], groups: [] });
  });

  it('never lists someone twice', () => {
    expect(withMember(role, 'user', 2, true).users).toEqual([1, 2]);
  });
});
