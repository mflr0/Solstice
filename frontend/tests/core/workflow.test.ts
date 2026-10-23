import { describe, expect, it } from 'vitest';
import fc from 'fast-check';

import type { StatusRef, Workflow } from '../../src/core/types.js';
import {
  addStatus,
  byCategory,
  cell,
  isDirty,
  matrix,
  moveStatus,
  removeStatus,
  rename,
  searchStatuses,
  statusNamed,
  strandedOf,
  suggestDefaultRemap,
  suggestRemap,
  toDraft,
  toggleCell,
  toInput,
  type Draft,
  strandedDefaultOf,
} from '../../src/core/workflow.js';

const TODO: StatusRef = { id: '10000', name: 'To Do', category: 'todo' };
const DOING: StatusRef = { id: '3', name: 'In Progress', category: 'doing' };
const REVIEW: StatusRef = { id: '10002', name: 'In Review', category: 'doing' };
const DONE: StatusRef = { id: '10001', name: 'Done', category: 'done' };
const OPEN: StatusRef = { id: '1', name: 'Open', category: 'todo' };

const software: Workflow = {
  id: 1,
  name: 'Logiciel',
  default: true,
  initial: TODO,
  statuses: [TODO, DOING, REVIEW, DONE],
  transitions: [
    { id: 1, name: 'Démarrer', from: TODO, to: DOING },
    { id: 2, name: 'Demander une revue', from: DOING, to: REVIEW },
    { id: 3, name: 'Remettre à faire', from: DOING, to: TODO },
    { id: 4, name: 'Valider', from: REVIEW, to: DONE },
    { id: 7, name: 'Clore directement', from: null, to: DONE },
  ],
  issueTypes: [{ id: '1', name: 'Bug', subtask: false }],
  usage: { '10000': 50, '3': 20 },
};

const bugs: Workflow = {
  ...software,
  id: 9,
  name: 'Bogues',
  default: false,
  issueTypes: [
    { id: '1', name: 'Bug', subtask: false },
    { id: '5', name: 'Sub-task', subtask: true },
  ],
};

describe('toDraft', () => {
  it('keeps ids, names and the "any status" transitions', () => {
    const d = toDraft(software);
    expect(d.initial).toBe('10000');
    expect(d.statuses.map((s) => s.id)).toEqual(['10000', '3', '10002', '10001']);
    expect(d.transitions[0]).toEqual({
      key: 'id:1',
      id: 1,
      name: 'Démarrer',
      from: '10000',
      to: '3',
    });
    expect(d.transitions.at(-1)).toMatchObject({ id: 7, from: null, to: '10001' });
  });

  it('lists the issue types only for a workflow other than the default', () => {
    expect(toDraft(software).issueTypes).toEqual([]);
    expect(toDraft(bugs).issueTypes).toEqual(['1', '5']);
  });
});

describe('matrix', () => {
  it('has a row per status, then the "from any status" row', () => {
    const rows = matrix(software.statuses, toDraft(software).transitions);
    expect(rows.map((r) => r.from?.name ?? '*')).toEqual([
      'To Do',
      'In Progress',
      'In Review',
      'Done',
      '*',
    ]);
    expect(rows.every((r) => r.cells.length === 4)).toBe(true);
  });

  it('puts each transition in its cell and marks the diagonal', () => {
    const rows = matrix(software.statuses, toDraft(software).transitions);
    const names = rows.map((r) => r.cells.map((c) => c.transitions.map((t) => t.name).join()));
    expect(names).toEqual([
      ['', 'Démarrer', '', ''],
      ['Remettre à faire', '', 'Demander une revue', ''],
      ['', '', '', 'Valider'],
      ['', '', '', ''],
      ['', '', '', 'Clore directement'],
    ]);
    expect(rows.map((r) => r.cells.findIndex((c) => c.self))).toEqual([0, 1, 2, 3, -1]);
  });

  it('round-trips: checking every cell of a matrix gives back the same transitions', () => {
    const arb = fc.uniqueArray(
      fc.record({
        from: fc.constantFrom<string | null>(null, '10000', '3', '10002', '10001'),
        to: fc.constantFrom('10000', '3', '10002', '10001'),
      }),
      { selector: (t) => `${t.from}>${t.to}` },
    );
    fc.assert(
      fc.property(arb, (pairs) => {
        const wanted = pairs.filter((p) => p.from !== p.to);
        let list = toDraft({ ...software, transitions: [] }).transitions;
        for (const p of wanted) {
          const to = software.statuses.find((s) => s.id === p.to) as StatusRef;
          list = toggleCell(list, p.from, to, []);
        }
        const checked = matrix(software.statuses, list).flatMap((r) =>
          r.cells
            .filter((c) => c.transitions.length > 0)
            .map((c) => `${r.from?.id ?? null}>${c.to.id}`),
        );
        expect(checked.sort()).toEqual(wanted.map((p) => `${p.from}>${p.to}`).sort());
      }),
    );
  });
});

describe('toggleCell', () => {
  const saved = toDraft(software).transitions;

  it('adds a transition named after its target', () => {
    const list = toggleCell(saved, '10001', TODO, saved);
    expect(cell(list, '10001', '10000')).toEqual([
      { key: 'new:10001>10000', id: null, name: 'To Do', from: '10001', to: '10000' },
    ]);
  });

  it('adds "from any status" transitions', () => {
    const list = toggleCell(saved, null, TODO, saved);
    expect(cell(list, null, '10000')).toMatchObject([{ id: null, name: 'To Do' }]);
  });

  it('removes every transition of a checked cell', () => {
    const doubled = [...saved, { key: 'id:8', id: 8, name: 'Abandonner', from: null, to: '10001' }];
    const list = toggleCell(doubled, null, DONE, doubled);
    expect(cell(list, null, '10001')).toEqual([]);
    expect(list).toHaveLength(saved.length - 1);
  });

  it('brings a saved transition back with its id, name and place', () => {
    const renamed = rename(saved, 'id:2', 'Relire');
    const off = toggleCell(renamed, '3', REVIEW, saved);
    expect(cell(off, '3', '10002')).toEqual([]);
    const on = toggleCell(off, '3', REVIEW, saved);
    expect(on).toEqual(saved);
  });

  it('never links a status to itself', () => {
    expect(toggleCell(saved, '3', DOING, saved)).toBe(saved);
  });
});

describe('statuses', () => {
  const d = toDraft(software);

  it('removing one drops its transitions both ways and moves the start', () => {
    const out = removeStatus(d, '10000');
    expect(out.statuses.map((s) => s.id)).toEqual(['3', '10002', '10001']);
    expect(out.transitions.map((t) => t.id)).toEqual([2, 4, 7]);
    expect(out.initial).toBe('3');
    expect(removeStatus(d, '3').initial).toBe('10000');
  });

  it('adds a status once, at the end', () => {
    const out = addStatus(d, OPEN);
    expect(out.statuses.at(-1)).toBe(OPEN);
    expect(addStatus(out, OPEN)).toBe(out);
  });

  it('moves a status', () => {
    expect(moveStatus(d.statuses, 3, 0).map((s) => s.id)).toEqual(['10001', '10000', '3', '10002']);
    expect(moveStatus(d.statuses, 0, 1).map((s) => s.id)).toEqual(['3', '10000', '10002', '10001']);
    expect(moveStatus(d.statuses, 0, 9)).toBe(d.statuses);
  });

  it('groups by category in the workflow order', () => {
    expect(byCategory([DONE, REVIEW, TODO, DOING]).map((g) => g.statuses.map((s) => s.id))).toEqual(
      [['10000'], ['10002', '3'], ['10001']],
    );
    expect(byCategory([DONE]).map((g) => g.category)).toEqual(['done']);
  });

  it('searches the catalogue, accents and case aside, leaving out those in use', () => {
    const catalogue = [TODO, DOING, OPEN, { id: '20', name: 'Résolu', category: 'done' } as const];
    expect(searchStatuses(catalogue, [TODO], '').map((s) => s.id)).toEqual(['3', '1', '20']);
    expect(searchStatuses(catalogue, [], 'resolu').map((s) => s.id)).toEqual(['20']);
    expect(searchStatuses(catalogue, [], ' PROG ').map((s) => s.id)).toEqual(['3']);
  });

  it('finds a status by name the way the server does', () => {
    expect(statusNamed([TODO, OPEN], ' to do ')).toBe(TODO);
    expect(statusNamed([TODO, OPEN], 'Ouvert')).toBeUndefined();
  });
});

describe('isDirty', () => {
  const saved = toDraft(software);
  const edit = (f: (d: Draft) => Draft) => isDirty(f(structuredClone(saved)), saved, true);

  it('is false for an untouched draft', () => {
    expect(edit((d) => d)).toBe(false);
  });

  it('sees names, start, statuses, their order and transitions', () => {
    expect(edit((d) => ({ ...d, name: 'Autre' }))).toBe(true);
    expect(edit((d) => ({ ...d, initial: '3' }))).toBe(true);
    expect(edit((d) => ({ ...d, statuses: moveStatus(d.statuses, 0, 1) }))).toBe(true);
    expect(edit((d) => addStatus(d, OPEN))).toBe(true);
    expect(edit((d) => ({ ...d, transitions: rename(d.transitions, 'id:1', 'Go') }))).toBe(true);
    expect(edit((d) => ({ ...d, transitions: toggleCell(d.transitions, null, TODO, []) }))).toBe(
      true,
    );
  });

  it('ignores what does not change the saved workflow', () => {
    expect(edit((d) => ({ ...d, name: ` ${d.name}  ` }))).toBe(false);
    const t = saved.transitions;
    expect(
      edit((d) => ({
        ...d,
        transitions: toggleCell(toggleCell(t, '3', REVIEW, t), '3', REVIEW, t),
      })),
    ).toBe(false);
    expect(edit((d) => ({ ...d, transitions: [...d.transitions].reverse() }))).toBe(false);
    // The default workflow serves every type left: its list does not count.
    expect(edit((d) => ({ ...d, issueTypes: ['1'] }))).toBe(false);
  });

  it('sees issue types and the default switch on another workflow, in any order', () => {
    const other = toDraft(bugs);
    expect(isDirty({ ...other, issueTypes: ['5', '1'] }, other, false)).toBe(false);
    expect(isDirty({ ...other, issueTypes: ['1'] }, other, false)).toBe(true);
    expect(isDirty({ ...other, makeDefault: true }, other, false)).toBe(true);
  });

  it('treats a blank name as the target status name', () => {
    const added = { ...saved, transitions: toggleCell(saved.transitions, '10001', TODO, []) };
    const blank = { ...added, transitions: rename(added.transitions, 'new:10001>10000', '  ') };
    expect(isDirty(blank, added, true)).toBe(false);
  });
});

describe('toInput', () => {
  it('sends ids of saved transitions only, and names blank ones after their target', () => {
    let d = toDraft(software);
    d = { ...d, transitions: toggleCell(d.transitions, '10001', TODO, []) };
    d = { ...d, transitions: rename(d.transitions, 'new:10001>10000', ' ') };
    const input = toInput(d, true);
    expect(input).toEqual({
      name: 'Logiciel',
      initial: '10000',
      statuses: ['10000', '3', '10002', '10001'],
      transitions: [
        { id: 1, name: 'Démarrer', from: '10000', to: '3' },
        { id: 2, name: 'Demander une revue', from: '3', to: '10002' },
        { id: 3, name: 'Remettre à faire', from: '3', to: '10000' },
        { id: 4, name: 'Valider', from: '10002', to: '10001' },
        { id: 7, name: 'Clore directement', from: null, to: '10001' },
        { name: 'To Do', from: '10001', to: '10000' },
      ],
    });
  });

  it('sends the issue types of another workflow, or makes it the default', () => {
    const d = toDraft(bugs);
    expect(toInput(d, false).issueTypes).toEqual(['1', '5']);
    expect(toInput(d, false).default).toBeUndefined();
    const def = toInput({ ...d, makeDefault: true }, false);
    expect(def.default).toBe(true);
    expect(def.issueTypes).toBeUndefined();
  });

  it('adds the remap when there is one', () => {
    const d = toDraft(software);
    expect(toInput(d, true, {}).remap).toBeUndefined();
    expect(toInput(d, true, { '10002': '3' }).remap).toEqual({ '10002': '3' });
  });
});

describe('remap', () => {
  it('reads the stranded statuses of a 409', () => {
    const body = { error: 'x', stranded: [{ status: REVIEW, count: 9 }] };
    expect(strandedOf(body)).toEqual([{ status: REVIEW, count: 9 }]);
    expect(strandedOf({ error: 'x' })).toBeNull();
    expect(strandedOf(undefined)).toBeNull();
  });

  it('suggests a status of the same category, else the start', () => {
    const stranded = [
      { status: REVIEW, count: 9 },
      { status: OPEN, count: 2 },
    ];
    expect(suggestRemap(stranded, [DOING, DONE], '3')).toEqual({ '10002': '3', '1': '3' });
    expect(suggestRemap(stranded, [DONE, TODO, DOING], '10001')).toEqual({
      '10002': '3',
      '1': '10000',
    });
  });
});

describe('issue types going back to the default workflow', () => {
  const blocked = { id: '20', name: 'Bloqué', category: 'doing' as const };
  const todo = { id: '1', name: 'À faire', category: 'todo' as const };
  const doing = { id: '3', name: 'En cours', category: 'doing' as const };

  it('reads them out of a 409 body', () => {
    const body = {
      error: 'x',
      stranded: [],
      strandedDefault: [{ status: blocked, count: 2 }],
      defaultWorkflow: { id: 4, name: 'Logiciel', statuses: [todo, doing] },
    };
    expect(strandedDefaultOf(body)).toEqual({
      stranded: [{ status: blocked, count: 2 }],
      workflow: { id: 4, name: 'Logiciel', statuses: [todo, doing] },
    });
    expect(strandedDefaultOf({ stranded: [] })).toBeNull();
    expect(strandedDefaultOf({ strandedDefault: [], defaultWorkflow: {} })).toBeNull();
    expect(strandedDefaultOf(null)).toBeNull();
  });

  it('suggests their moves from the default workflow statuses', () => {
    const d = {
      stranded: [{ status: blocked, count: 2 }],
      workflow: { id: 4, name: 'Logiciel', statuses: [todo, doing] },
    };
    expect(suggestDefaultRemap(d)).toEqual({ '20': '3' });
    expect(suggestDefaultRemap({ ...d, workflow: { ...d.workflow, statuses: [todo] } })).toEqual({
      '20': '1',
    });
  });

  it('sends their moves as remapDefault, only when there are some', () => {
    const w = {
      id: 9,
      name: 'Bugs',
      default: false,
      initial: todo,
      statuses: [todo, doing],
      transitions: [],
      issueTypes: [],
      usage: {},
    };
    const d = toDraft(w);
    expect(toInput(d, false, undefined, { '20': '3' }).remapDefault).toEqual({ '20': '3' });
    expect('remapDefault' in toInput(d, false, undefined, {})).toBe(false);
  });
});
