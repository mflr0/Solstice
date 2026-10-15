import { describe, expect, it } from 'vitest';

import {
  addDays,
  categoryClause,
  currentSort,
  dayRange,
  emptyFilters,
  facetJql,
  filtersToJql,
  isBlank,
  jqlToFilters,
  listClause,
  literal,
  matchPreset,
  rangeDays,
  sortBy,
  toggle,
  type FilterState,
} from '../../src/core/filters.js';
import { simpleToJql } from '../../src/core/jql-simple.js';

const state = (patch: Partial<FilterState>): FilterState => ({ ...emptyFilters(), ...patch });

function read(jql: string): FilterState {
  const r = jqlToFilters(jql);
  if (!r.ok) throw new Error(`not readable: ${jql} (${r.reason})`);
  return r.state;
}

describe('filtersToJql', () => {
  it('writes nothing for no filter', () => {
    expect(filtersToJql(emptyFilters())).toBe('');
  });

  it('writes one value with =, several with in, and quotes when needed', () => {
    expect(
      filtersToJql(
        state({
          values: {
            project: ['DEMO'],
            status: ['"In Progress"', '"In Review"'],
            labels: ['dette-technique'],
          },
        }),
      ),
    ).toBe(
      'project = DEMO AND status in ("In Progress", "In Review") AND labels = dette-technique',
    );
  });

  it('writes EMPTY and functions', () => {
    expect(filtersToJql(state({ values: { assignee: ['currentUser()', 'EMPTY'] } }))).toBe(
      'assignee in (currentUser(), EMPTY)',
    );
    expect(filtersToJql(state({ values: { assignee: ['EMPTY'] } }))).toBe('assignee is EMPTY');
    expect(filtersToJql(state({ values: { sprint: ['openSprints()'] } }))).toBe(
      'sprint in openSprints()',
    );
    expect(filtersToJql(state({ values: { assignee: ['currentUser()'] } }))).toBe(
      'assignee = currentUser()',
    );
    expect(filtersToJql(state({ values: { epic: ['DEMO-114'] } }))).toBe('"Epic Link" = DEMO-114');
  });

  it('writes status categories the short way', () => {
    expect(categoryClause(['todo', 'doing'])).toBe('statusCategory != Done');
    expect(categoryClause(['done'])).toBe('statusCategory = Done');
    expect(categoryClause(['doing'])).toBe('statusCategory = "In Progress"');
    expect(categoryClause(['done', 'todo'])).toBe('statusCategory in ("To Do", Done)');
    expect(categoryClause([])).toBe('');
    expect(categoryClause(['todo', 'doing', 'done'])).toBe('');
  });

  it('writes dates, people, text and sort', () => {
    expect(
      filtersToJql(
        state({
          text: '  panier "vide"  ',
          values: { people: ['currentUser()'] },
          peopleRole: 'watcher',
          dates: {
            created: { from: { op: '>=', value: '-7d' }, to: null, empty: false },
            due: { from: null, to: null, empty: true },
            updated: {
              from: { op: '>=', value: '2026-10-01' },
              to: { op: '<', value: '2026-10-13' },
              empty: false,
            },
          },
          order: [{ field: 'priority', desc: true }],
        }),
      ),
    ).toBe(
      'watcher = currentUser() AND created >= -7d AND updated >= 2026-10-01 AND updated < 2026-10-13 AND due is EMPTY AND text ~ "panier \\"vide\\"" ORDER BY priority DESC',
    );
  });

  it('writes a sort alone', () => {
    expect(filtersToJql(state({ order: [{ field: 'key', desc: false }] }))).toBe(
      'ORDER BY key ASC',
    );
  });

  it('writes list clauses for any field name', () => {
    expect(listClause('Epic Link', ['EMPTY'])).toBe('"Epic Link" is EMPTY');
    expect(listClause('fixVersion', ['unreleasedVersions()', '"2.4"'])).toBe(
      'fixVersion in (unreleasedVersions(), "2.4")',
    );
  });
});

describe('jqlToFilters', () => {
  it('reads what it writes', () => {
    const samples: FilterState[] = [
      state({ values: { project: ['DEMO', 'WEB'], type: ['Bug'] } }),
      state({
        values: { assignee: ['currentUser()', 'EMPTY', 'paul.martin'] },
        categories: ['todo', 'doing'],
      }),
      state({ values: { resolution: ['EMPTY'], sprint: ['openSprints()', '12'] } }),
      state({ values: { fixVersion: ['"2.4 beta"', 'unreleasedVersions()'], component: ['api'] } }),
      state({
        text: 'tunnel de "commande"',
        values: { people: ['currentUser()', 'paul.martin'] },
        dates: {
          resolved: { from: { op: '>', value: 'startOfMonth(-1M)' }, to: null, empty: false },
        },
        order: [
          { field: 'priority', desc: true },
          { field: 'updated', desc: true },
        ],
      }),
      state({ categories: ['done'], values: { labels: ['"EMPTY"'] } }),
    ];
    for (const s of samples) {
      const jql = filtersToJql(s);
      expect(read(jql), jql).toEqual(s);
      expect(filtersToJql(read(jql))).toBe(jql);
    }
  });

  it('reads JQL written by hand', () => {
    expect(
      read(
        'project in (demo) and issuetype = "Bug" AND assignee = currentuser() AND resolution = Unresolved order by created desc',
      ),
    ).toEqual(
      state({
        values: {
          project: ['demo'],
          type: ['Bug'],
          assignee: ['currentUser()'],
          resolution: ['EMPTY'],
        },
        order: [{ field: 'created', desc: true }],
      }),
    );
    expect(read('statusCategory not in (Done)').categories).toEqual(['todo', 'doing']);
    expect(read('statusCategory = "en cours"').categories).toEqual(['doing']);
    expect(read('due < startOfDay() AND watcher in (currentUser())')).toMatchObject({
      values: { people: ['currentUser()'] },
      peopleRole: 'watcher',
      dates: { due: { from: null, to: { op: '<', value: 'startOfDay()' }, empty: false } },
    });
    expect(read('created >= "2026-10-01"').dates.created?.from).toEqual({
      op: '>=',
      value: '2026-10-01',
    });
    expect(read('"epic link" = DEMO-114').values.epic).toEqual(['DEMO-114']);
    expect(read('').values).toEqual({});
  });

  it('reads the saved filters of the demo data', () => {
    expect(
      read(
        'assignee = currentUser() AND resolution = Unresolved ORDER BY priority DESC, updated DESC',
      ),
    ).toMatchObject({ values: { assignee: ['currentUser()'], resolution: ['EMPTY'] } });
    expect(
      read(
        'project = DEMO AND issuetype = Bug AND status in ("In Progress", "In Review") ORDER BY created DESC',
      ).values.status,
    ).toEqual(['"In Progress"', '"In Review"']);
  });

  it('refuses what the filters cannot show', () => {
    const complex = [
      'status = Open OR status = Done',
      'NOT status = Open',
      'status != Open',
      'project not in (DEMO)',
      'status = Open AND status = Done',
      'summary ~ panier',
      'text ~ a AND text ~ b',
      'status was Open',
      'assignee changed',
      'created = -1d',
      'created >= -1d AND created > -2d',
      'due is EMPTY AND due < now()',
      'due is not EMPTY',
      'project is EMPTY',
      'assignee = foo()',
      'statusCategory = Nope',
      'cf[10100] = 3',
      'key = DEMO-1',
      'status = Done AND statusCategory != Done',
    ];
    for (const jql of complex)
      expect(jqlToFilters(jql), jql).toEqual({ ok: false, reason: 'complex' });
  });

  it('reports syntax errors', () => {
    const r = jqlToFilters('status in (Open');
    expect(r.ok).toBe(false);
    if (!r.ok && r.reason === 'syntax')
      expect(r.error.message).toBe('Parenthèse fermante attendue.');
    else throw new Error('expected a syntax error');
  });
});

describe('people', () => {
  it('writes the field of the chosen role', () => {
    const people = { people: ['currentUser()', 'paul.martin'] };
    expect(filtersToJql(state({ values: people }))).toBe(
      'participant in (currentUser(), paul.martin)',
    );
    expect(filtersToJql(state({ values: people, peopleRole: 'creator' }))).toBe(
      'creator in (currentUser(), paul.martin)',
    );
    expect(filtersToJql(state({ values: { people: ['EMPTY'] }, peopleRole: 'commenter' }))).toBe(
      'commenter is EMPTY',
    );
    expect(
      filtersToJql(
        state({ values: { project: ['DEMO'], people: ['"jean dupont"'] }, peopleRole: 'watcher' }),
      ),
    ).toBe('project = DEMO AND watcher = "jean dupont"');
  });

  it('reads every role and its aliases back', () => {
    for (const [jql, role, values] of [
      ['participant = currentUser()', 'participant', ['currentUser()']],
      ['involves in (marie.durand, paul.martin)', 'participant', ['marie.durand', 'paul.martin']],
      ['participants = x', 'participant', ['x']],
      ['creator = marie.durand', 'creator', ['marie.durand']],
      ['watcher in (currentUser(), EMPTY)', 'watcher', ['currentUser()', 'EMPTY']],
      ['commenter is EMPTY', 'commenter', ['EMPTY']],
      ['commenter in membersOf("devs")', 'commenter', ['membersOf(devs)']],
    ] as const) {
      const s = read(jql);
      expect(s.peopleRole, jql).toBe(role);
      expect(s.values.people, jql).toEqual(values);
      expect(read(filtersToJql(s)), jql).toEqual(s);
    }
  });

  it('keeps the assignee and the reporter apart from it', () => {
    const s = read('participant = currentUser() AND assignee = paul.martin AND reporter = EMPTY');
    expect(s.values).toEqual({
      people: ['currentUser()'],
      assignee: ['paul.martin'],
      reporter: ['EMPTY'],
    });
  });

  it('leaves two roles, or no one taking part, to the editor', () => {
    for (const jql of [
      'participant = a AND creator = b',
      'watcher = a AND watcher = b',
      'participant is EMPTY',
      'creator is EMPTY',
      'participant != a',
      'commenter not in (a)',
    ]) {
      expect(jqlToFilters(jql), jql).toEqual({ ok: false, reason: 'complex' });
    }
  });
});

describe('facetJql', () => {
  it('leaves out the field counted, and the sort', () => {
    const s = state({
      values: { project: ['DEMO'], assignee: ['currentUser()'] },
      categories: ['todo', 'doing'],
      text: 'panier',
      order: [{ field: 'key', desc: false }],
    });
    expect(facetJql(s, 'assignee')).toBe(
      'project = DEMO AND statusCategory != Done AND text ~ "panier"',
    );
    expect(facetJql(s, 'status')).toBe(
      'project = DEMO AND assignee = currentUser() AND text ~ "panier"',
    );
    expect(facetJql(state({ values: { labels: ['ux'] } }), 'labels')).toBe('');
  });
});

describe('sorting', () => {
  it('knows the offered sorts', () => {
    expect(currentSort([])).toEqual({ field: 'updated', desc: true });
    expect(currentSort([{ field: 'createdDate', desc: false }])).toEqual({
      field: 'created',
      desc: false,
    });
    expect(currentSort([{ field: 'rank', desc: false }])).toBeNull();
    expect(sortBy('updated', true)).toEqual([]);
    expect(sortBy('updated', false)).toEqual([{ field: 'updated', desc: false }]);
  });
});

describe('dates', () => {
  it('adds days across months and years', () => {
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01');
    expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
    expect(addDays('2028-02-28', 1)).toBe('2028-02-29');
  });

  it('turns two days into a range including both, and back', () => {
    const r = dayRange('2026-10-01', '2026-10-12');
    expect(r).toEqual({
      from: { op: '>=', value: '2026-10-01' },
      to: { op: '<', value: '2026-10-13' },
      empty: false,
    });
    expect(rangeDays(r)).toEqual({ from: '2026-10-01', to: '2026-10-12' });
    expect(dayRange('', '2026-10-12').from).toBeNull();
    expect(rangeDays({ from: { op: '>=', value: '-7d' }, to: null, empty: false })).toEqual({
      from: '',
      to: '',
    });
  });

  it('recognises shortcuts', () => {
    expect(matchPreset('created', read('created >= -7d').dates.created!)).toBe('last7');
    expect(matchPreset('due', read('due < startOfDay()').dates.due!)).toBe('overdue');
    expect(matchPreset('due', read('due is EMPTY').dates.due!)).toBe('none');
    expect(matchPreset('updated', read('updated < -30d').dates.updated!)).toBe('older30');
    expect(matchPreset('created', read('created >= -8d').dates.created!)).toBeNull();
  });
});

describe('values', () => {
  it('reads the text of a value', () => {
    expect(literal('"In Progress"')).toBe('In Progress');
    expect(literal('DEMO')).toBe('DEMO');
    expect(literal('"a \\"b\\""')).toBe('a "b"');
    expect(literal('EMPTY')).toBeNull();
    expect(literal('currentUser()')).toBeNull();
  });

  it('toggles case-insensitively', () => {
    expect(toggle(['DEMO'], 'demo')).toEqual([]);
    expect(toggle(['DEMO'], 'WEB')).toEqual(['DEMO', 'WEB']);
  });

  it('isBlank ignores the sort', () => {
    expect(isBlank(state({ order: [{ field: 'key', desc: false }] }))).toBe(true);
    expect(isBlank(state({ categories: ['todo', 'doing', 'done'] }))).toBe(true);
    expect(isBlank(state({ values: { people: ['currentUser()'] } }))).toBe(false);
    expect(isBlank(state({ peopleRole: 'commenter' }))).toBe(true);
  });
});

describe('simpleToJql', () => {
  it.each([
    [
      '@moi est:ouvert tri:priorité',
      'assignee = currentUser() AND statusCategory != Done ORDER BY priority DESC',
    ],
    ['par:@moi est:ouvert', 'reporter = currentUser() AND statusCategory != Done'],
    ['tri:maj', 'ORDER BY updated DESC'],
    ['projet:DEMO version:"2.4"', 'project = DEMO AND fixVersion = 2.4'],
    [
      'projet:WEB composant:"Paiement en ligne"',
      'project = WEB AND component = "Paiement en ligne"',
    ],
    ['#urgent #ux', 'labels in (urgent, ux)'],
    ['@personne', 'assignee is EMPTY'],
    ['@marie @paul.martin', 'assignee in (marie, paul.martin)'],
    ['statut:"En cours" type:Bug', 'status = "En cours" AND type = Bug'],
    ['-est:fini', 'statusCategory != Done'],
    ['-#bug -@personne', 'labels != bug AND assignee is not EMPTY'],
    ['panier vide DEMO-12', 'key = DEMO-12 AND text ~ "panier vide"'],
    ['"panier vide" -lent', 'text ~ "panier vide" AND text !~ "lent"'],
    ['', ''],
  ])('%s', (q, jql) => {
    expect(simpleToJql(q)).toBe(jql);
  });

  it('gives JQL the filters can show for the usual links', () => {
    for (const q of ['@moi est:ouvert tri:priorité', 'projet:DEMO est:ouvert', '#urgent']) {
      expect(jqlToFilters(simpleToJql(q)).ok, q).toBe(true);
    }
  });
});
