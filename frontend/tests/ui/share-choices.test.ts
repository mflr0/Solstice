import { describe, expect, it } from 'vitest';

import type { Project, UserRef } from '../../src/core/types.js';
import { MAX_CHOICES, shareChoices } from '../../src/ui/dashboard/share-choices.js';

const marie: UserRef = { id: 7, username: 'marie', name: 'Marie Durand', active: true };
const groups = [
  { id: 1, name: 'SOL developers' },
  { id: 2, name: 'Support' },
];
const projects = [
  { id: 3, key: 'SOL', name: 'Solstice' },
  { id: 4, key: 'WEB', name: 'Website' },
] as Project[];

describe('shareChoices', () => {
  it('offers nothing before a search', () => {
    expect(shareChoices('  ', [marie], groups, projects, [])).toEqual([]);
  });

  it('lists the people found, then groups and projects by name or key', () => {
    const got = shareChoices('sol', [marie], groups, projects, []);
    expect(got.map((c) => `${c.share.kind} ${c.share.name}`)).toEqual([
      'user Marie Durand',
      'group SOL developers',
      'project SOL',
    ]);
    expect(got[0]?.username).toBe('marie');
    expect(shareChoices('websi', [], groups, projects, []).map((c) => c.share.id)).toEqual([4]);
  });

  it('leaves out what the view is shared with, but not a narrower share', () => {
    const got = shareChoices('s', [], groups, projects, [
      { kind: 'group', id: 1 },
      { kind: 'project', id: 4, group: 2 },
    ]);
    expect(got.map((c) => `${c.share.kind} ${c.share.id}`)).toEqual([
      'group 2',
      'project 3',
      'project 4',
    ]);
  });

  it('keeps the list short', () => {
    const many = Array.from({ length: 20 }, (_, i) => ({ id: i, name: `Team ${i}` }));
    expect(shareChoices('team', [], many, [], [])).toHaveLength(MAX_CHOICES);
  });
});
