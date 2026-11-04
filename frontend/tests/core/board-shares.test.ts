import { describe, expect, it } from 'vitest';

import { addShare, removeShare, shareLabel, sharesLost } from '../../src/core/board-shares.js';
import type { FilterShare } from '../../src/core/types.js';
import { en } from '../../src/i18n/en.js';

const everyone: FilterShare = { kind: 'everyone' };
const group: FilterShare = { kind: 'group', id: 3, name: 'Team' };
const project: FilterShare = { kind: 'project', id: 3, name: 'SOL' };
const inProject: FilterShare = { kind: 'project', id: 3, group: 4, name: 'SOL · Team' };
const person: FilterShare = { kind: 'user', id: 3, name: 'Marie' };

describe('addShare', () => {
  it('keeps one of a share added twice, whatever its name says', () => {
    const list = addShare([group], { kind: 'group', id: 3 });
    expect(list).toEqual([group]);
  });

  it('tells shares apart by kind, id and group', () => {
    const list = [group, project, inProject, person, everyone].reduce<FilterShare[]>(addShare, []);
    expect(list).toHaveLength(5);
  });
});

describe('removeShare', () => {
  it('removes that share only', () => {
    expect(removeShare([project, inProject, group], project)).toEqual([inProject, group]);
    expect(removeShare([project], person)).toEqual([project]);
  });
});

describe('sharesLost', () => {
  it('sees a removal, not an addition', () => {
    expect(sharesLost([project], [project, group])).toBe(false);
    expect(sharesLost([project], [group])).toBe(true);
    expect(sharesLost([project], [])).toBe(true);
    expect(sharesLost([], [])).toBe(false);
  });
});

describe('shareLabel', () => {
  it('words each kind', () => {
    const { everyone: all, group: g, project: p, user: u } = en.boards.shareKinds;
    expect(shareLabel(everyone)).toBe(all);
    expect(shareLabel(group)).toBe(`${g}: Team`);
    expect(shareLabel(project)).toBe(`${p}: SOL`);
    expect(shareLabel(inProject)).toBe(`${p}: SOL · Team`);
    expect(shareLabel(person)).toBe(`${u}: Marie`);
  });
});
