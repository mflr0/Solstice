import { describe, expect, it } from 'vitest';

import type { ComponentRow, VersionRow } from '../../src/core/admin-types.js';

import {
  ASSIGNEE_TYPES,
  categoryProblem,
  componentForm,
  componentPayload,
  datesProblem,
  filterProjects,
  issueTypesProblem,
  parseCategoryFilter,
  parseShow,
  extraSignature,
  generalChanged,
  generalForm,
  generalPatch,
  generalProblems,
  hasExtra,
  keyProblem,
  moveItem,
  notificationsChanged,
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
  toggleIssueType,
  typesInUse,
  uniqueNameProblem,
  versionForm,
  versionPayload,
  versionProblems,
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

  it('spells a capital sharp s as SS, like a small one', () => {
    expect(normalizeKey('GROẞ')).toBe('GROSS');
    expect(normalizeKey('Groß')).toBe('GROSS');
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

  it('tells unsaved changes from the saved ones', () => {
    const r = { event: 'created', kind: 'group' as const, id: 4, name: 'dev' };
    const other = { event: 'status', kind: 'role' as const, id: 2, name: 'qa' };
    const saved = { enabled: true, extra: [r, other] };
    expect(notificationsChanged(saved, true, [other, r])).toBe(false);
    expect(notificationsChanged(saved, false, [r, other])).toBe(true);
    expect(notificationsChanged(saved, true, [r])).toBe(true);
    expect(notificationsChanged(saved, true, [r, other, { ...r, event: 'deleted' }])).toBe(true);
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
  const lead = { id: 7, username: 'ada', name: 'Ada', active: true };
  const settings: ProjectSettings = {
    id: 1,
    key: 'WEB',
    name: 'Site web',
    description: 'The site',
    kind: 'software',
    lead,
    category: { id: 3, name: 'Internal' },
    defaultAssignee: 'unassigned',
    archived: false,
    issueTypes: [
      { id: '1', name: 'Task', subtask: false },
      { id: '2', name: 'Sub-task', subtask: true },
    ],
    issues: 4,
  };
  const all = settings.issueTypes;

  it('starts from the saved settings and sends nothing when nothing changed', () => {
    const f = generalForm(settings);
    expect(f.issueTypes).toEqual(['1', '2']);
    expect(generalPatch(settings, f)).toEqual({});
    expect(generalChanged(settings, f)).toBe(false);
    expect(generalChanged(settings, { ...f, name: '  Site web  ' })).toBe(false);
  });

  it('sends only the fields that changed', () => {
    const f = generalForm(settings);
    expect(generalPatch(settings, { ...f, name: ' Portal ' })).toEqual({ name: 'Portal' });
    expect(generalPatch(settings, { ...f, category: null })).toEqual({ category: null });
    expect(generalPatch(settings, { ...f, defaultAssignee: 'lead' })).toEqual({
      defaultAssignee: 'lead',
    });
    expect(generalPatch(settings, { ...f, lead: { ...lead, id: 9 } })).toEqual({ lead: 9 });
    expect(generalPatch(settings, { ...f, issueTypes: ['1'] })).toEqual({ issueTypes: ['1'] });
  });

  it('clears the description with null, and ignores an order-only change of types', () => {
    const f = generalForm(settings);
    expect(generalPatch(settings, { ...f, description: '  ' })).toEqual({ description: null });
    expect(generalPatch({ ...settings, description: null }, { ...f, description: '' })).toEqual({});
    expect(generalPatch(settings, { ...f, issueTypes: ['2', '1'] })).toEqual({});
  });

  it('checks the name and the types', () => {
    const f = generalForm(settings);
    expect(generalProblems(f, all)).toEqual({});
    expect(generalProblems({ ...f, name: '  ' }, all).name).toBe('nameRequired');
    expect(generalProblems({ ...f, name: 'x'.repeat(256) }, all).name).toBe('nameTooLong');
    expect(generalProblems({ ...f, issueTypes: ['2'] }, all).types).toBe('typesRequired');
  });

  it('adds and removes an issue type', () => {
    expect(toggleIssueType(['1'], '2', true)).toEqual(['1', '2']);
    expect(toggleIssueType(['1', '2'], '1', false)).toEqual(['2']);
    expect(toggleIssueType(['1', '2'], '2', true)).toEqual(['1', '2']);
  });

  it('reads the types still in use from a refusal', () => {
    expect(typesInUse({ inUse: [{ type: { id: '1', name: 'Bug' }, issues: 3 }] })).toEqual([
      { name: 'Bug', issues: 3 },
    ]);
    expect(typesInUse(undefined)).toEqual([]);
    expect(typesInUse({ inUse: 'x' })).toEqual([]);
    expect(typesInUse({ inUse: [{ issues: 2 }] })).toEqual([]);
  });
});

describe('names in a project', () => {
  const others = [
    { id: 1, name: 'Backend' },
    { id: 2, name: ' API ' },
  ];

  it('asks for a name, short and not taken by another', () => {
    expect(uniqueNameProblem('  ', others, null)).toBe('nameRequired');
    expect(uniqueNameProblem('x'.repeat(256), others, null)).toBe('nameTooLong');
    expect(uniqueNameProblem('backend', others, null)).toBe('nameTaken');
    expect(uniqueNameProblem('api', others, null)).toBe('nameTaken');
    expect(uniqueNameProblem('Backend', others, 1)).toBeNull();
    expect(uniqueNameProblem('Frontend', others, null)).toBeNull();
  });
});

describe('components', () => {
  const lead = { id: 7, username: 'ada', name: 'Ada', active: true };
  const row: ComponentRow = {
    id: 5,
    name: 'API',
    description: 'The public API',
    lead,
    assigneeType: 'component_lead',
    archived: false,
    issues: 2,
  };

  it('knows the four ways a new issue gets its assignee', () => {
    expect(ASSIGNEE_TYPES).toEqual([
      'project_default',
      'component_lead',
      'project_lead',
      'unassigned',
    ]);
  });

  it('starts blank, or from the component', () => {
    expect(componentForm(null)).toEqual({
      name: '',
      description: '',
      lead: null,
      assigneeType: 'project_default',
    });
    expect(componentForm(row)).toEqual({
      name: 'API',
      description: 'The public API',
      lead,
      assigneeType: 'component_lead',
    });
  });

  it('sends what is filled in to create', () => {
    expect(componentPayload({ ...componentForm(null), name: ' Web ' }, null)).toEqual({
      name: 'Web',
      assigneeType: 'project_default',
    });
    expect(componentPayload({ ...componentForm(row), name: 'Web' }, null)).toEqual({
      name: 'Web',
      description: 'The public API',
      lead: 7,
      assigneeType: 'component_lead',
    });
  });

  it('sends only the changes to update, and null to clear the lead', () => {
    const f = componentForm(row);
    expect(componentPayload(f, row)).toEqual({});
    expect(componentPayload({ ...f, name: ' Core ' }, row)).toEqual({ name: 'Core' });
    expect(componentPayload({ ...f, description: '' }, row)).toEqual({ description: '' });
    expect(componentPayload({ ...f, lead: null }, row)).toEqual({ lead: null });
    expect(componentPayload({ ...f, lead: { ...lead, id: 9 } }, row)).toEqual({ lead: 9 });
    expect(componentPayload({ ...f, assigneeType: 'unassigned' }, row)).toEqual({
      assigneeType: 'unassigned',
    });
  });
});

describe('versions', () => {
  const row: VersionRow = {
    id: 3,
    name: '1.0',
    description: 'First release',
    startDate: '2026-01-05',
    releaseDate: '2026-02-01',
    released: false,
    archived: false,
    position: 1,
    issues: { total: 4, unresolved: 1 },
  };

  it('starts blank, or from the version', () => {
    expect(versionForm(null)).toEqual({
      name: '',
      description: '',
      startDate: '',
      releaseDate: '',
    });
    expect(versionForm({ ...row, startDate: null, description: null })).toEqual({
      name: '1.0',
      description: '',
      startDate: '',
      releaseDate: '2026-02-01',
    });
  });

  it('checks the name and the dates', () => {
    const f = versionForm(row);
    const others = [
      { id: 3, name: '1.0' },
      { id: 4, name: '2.0' },
    ];
    expect(versionProblems(f, others, 3)).toEqual({});
    expect(versionProblems({ ...f, name: '2.0' }, others, 3)).toEqual({ name: 'nameTaken' });
    expect(versionProblems({ ...f, name: ' ' }, others, 3).name).toBe('nameRequired');
    expect(versionProblems({ ...f, startDate: '2026-03-01' }, others, 3)).toEqual({
      dates: 'startAfterRelease',
    });
    expect(versionProblems({ ...f, startDate: '' }, others, 3)).toEqual({});
  });

  it('sends what is filled in to create', () => {
    expect(versionPayload({ ...versionForm(null), name: ' 2.0 ' }, null)).toEqual({ name: '2.0' });
    expect(versionPayload(versionForm(row), null)).toEqual({
      name: '1.0',
      description: 'First release',
      startDate: '2026-01-05',
      releaseDate: '2026-02-01',
    });
  });

  it('sends only the changes to update, and null to clear a day', () => {
    const f = versionForm(row);
    expect(versionPayload(f, row)).toEqual({});
    expect(versionPayload({ ...f, name: ' 1.1 ' }, row)).toEqual({ name: '1.1' });
    expect(versionPayload({ ...f, description: '' }, row)).toEqual({ description: '' });
    expect(versionPayload({ ...f, startDate: '' }, row)).toEqual({ startDate: null });
    expect(versionPayload({ ...f, releaseDate: '2026-02-15' }, row)).toEqual({
      releaseDate: '2026-02-15',
    });
  });
});
