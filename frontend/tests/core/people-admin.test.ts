import { describe, expect, it } from 'vitest';

import {
  filterGroups,
  filterRoles,
  filterUsers,
  groupInUse,
  groupKey,
  groupNameProblem,
  isBlocked,
  namedPayload,
  parseGroupFilter,
  parseUserShow,
  restrictedUsage,
  roleInUse,
  roleNameProblem,
  usageLines,
} from '../../src/core/people-admin.js';

const groups = [
  { id: 1, name: 'developers' },
  { id: 2, name: 'support' },
];

describe('group names', () => {
  it('are folded to lower case, as the server stores them', () => {
    expect(groupKey('  Developers ')).toBe('developers');
  });

  it('are required, at most 255 characters', () => {
    expect(groupNameProblem('   ', groups, null)).toBe('nameRequired');
    expect(groupNameProblem('x'.repeat(255), groups, null)).toBeNull();
    expect(groupNameProblem('x'.repeat(256), groups, null)).toBe('nameTooLong');
  });

  it('may not repeat another group, whatever the case', () => {
    expect(groupNameProblem('Support', groups, null)).toBe('nameTaken');
    expect(groupNameProblem(' DEVELOPERS ', groups, 2)).toBe('nameTaken');
  });

  it('may keep a group’s own name when renaming it', () => {
    expect(groupNameProblem('Developers', groups, 1)).toBeNull();
  });
});

describe('role names', () => {
  const roles = [{ id: 5, name: 'Developer' }];

  it('are compared without regard to case', () => {
    expect(roleNameProblem('developer', roles, null)).toBe('nameTaken');
    expect(roleNameProblem('developer', roles, 5)).toBeNull();
    expect(roleNameProblem('Tester', roles, null)).toBeNull();
    expect(roleNameProblem('', roles, null)).toBe('nameRequired');
  });
});

describe('searching', () => {
  const rows = [
    { name: 'developers', description: 'Engineering' },
    { name: 'support', description: null },
  ];

  it('looks in the name and the description, without accents or case', () => {
    expect(filterGroups(rows, 'ENGINEER').map((g) => g.name)).toEqual(['developers']);
    expect(filterGroups(rows, 'supp').map((g) => g.name)).toEqual(['support']);
    expect(filterGroups(rows, '')).toHaveLength(2);
    expect(filterRoles(rows, 'zzz')).toEqual([]);
  });
});

describe('namedPayload', () => {
  it('trims both fields, an empty description clears it', () => {
    expect(namedPayload(' qa ', '  ')).toEqual({ name: 'qa', description: '' });
  });
});

describe('group usage', () => {
  const none = { grants: 0, roles: 0, securityLevels: 0, comments: 0, worklogs: 0 };

  it('lists only the uses that exist, in a fixed order', () => {
    expect(usageLines(none)).toEqual([]);
    expect(usageLines({ ...none, worklogs: 2, grants: 1 })).toEqual([
      { kind: 'grants', count: 1 },
      { kind: 'worklogs', count: 2 },
    ]);
  });

  it('blocks deletion while comments or work logs are restricted to the group', () => {
    expect(isBlocked(none)).toBe(false);
    expect(isBlocked({ ...none, comments: 1 })).toBe(true);
    expect(isBlocked({ ...none, worklogs: 1 })).toBe(true);
  });

  it('asks a typed confirmation when grants, roles or levels would go with the group', () => {
    expect(groupInUse(none)).toBe(false);
    expect(groupInUse({ ...none, comments: 3 })).toBe(false);
    expect(groupInUse({ ...none, grants: 1 })).toBe(true);
    expect(groupInUse({ ...none, roles: 1 })).toBe(true);
    expect(groupInUse({ ...none, securityLevels: 1 })).toBe(true);
  });

  it('asks it for a role held in a project or named by a grant', () => {
    expect(roleInUse({ projects: 0, grants: 0 })).toBe(false);
    expect(roleInUse({ projects: 1, grants: 0 })).toBe(true);
    expect(roleInUse({ projects: 0, grants: 2 })).toBe(true);
  });
});

describe('restrictedUsage', () => {
  it('reads the usage of a 409 answer', () => {
    expect(restrictedUsage({ error: 'x', usage: { comments: 2, worklogs: 1 } })).toEqual({
      comments: 2,
      worklogs: 1,
    });
    expect(restrictedUsage({ usage: { comments: 2 } })).toEqual({ comments: 2, worklogs: 0 });
  });

  it('is null when there is nothing usable', () => {
    expect(restrictedUsage(undefined)).toBeNull();
    expect(restrictedUsage({ error: 'x' })).toBeNull();
    expect(restrictedUsage({ usage: 'x' })).toBeNull();
    expect(restrictedUsage({ usage: { comments: 0, worklogs: 0 } })).toBeNull();
    expect(restrictedUsage({ usage: { comments: '3' } })).toBeNull();
  });
});

describe('account filters', () => {
  const users = [
    { id: 1, active: true, admin: true, groups: [{ id: 1 }, { id: 2 }] },
    { id: 2, active: true, admin: false, groups: [{ id: 2 }] },
    { id: 3, active: false, admin: false, groups: [] },
    { id: 4, active: true, admin: false },
  ];
  const ids = (rows: { id: number }[]) => rows.map((u) => u.id);

  it('keeps everyone by default', () => {
    expect(ids(filterUsers(users, 'all', 'all'))).toEqual([1, 2, 3, 4]);
  });

  it('filters by state', () => {
    expect(ids(filterUsers(users, 'active', 'all'))).toEqual([1, 2, 4]);
    expect(ids(filterUsers(users, 'inactive', 'all'))).toEqual([3]);
    expect(ids(filterUsers(users, 'admins', 'all'))).toEqual([1]);
  });

  it('filters by group, or by having none', () => {
    expect(ids(filterUsers(users, 'all', 2))).toEqual([1, 2]);
    expect(ids(filterUsers(users, 'all', 1))).toEqual([1]);
    expect(ids(filterUsers(users, 'all', 'none'))).toEqual([3, 4]);
  });

  it('combines state and group', () => {
    expect(ids(filterUsers(users, 'active', 'none'))).toEqual([4]);
    expect(ids(filterUsers(users, 'inactive', 2))).toEqual([]);
  });

  it('reads the address parameters, falling back to everyone', () => {
    expect(parseUserShow('admins')).toBe('admins');
    expect(parseUserShow('bogus')).toBe('all');
    expect(parseUserShow(null)).toBe('all');
    expect(parseGroupFilter('none')).toBe('none');
    expect(parseGroupFilter('12')).toBe(12);
    expect(parseGroupFilter('0')).toBe('all');
    expect(parseGroupFilter('x')).toBe('all');
    expect(parseGroupFilter(null)).toBe('all');
  });
});
