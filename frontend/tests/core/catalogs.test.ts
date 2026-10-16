import { describe, expect, it } from 'vitest';

import type { CatalogItem, FieldOption, FieldRow, StatusRow } from '../../src/core/admin-types.js';
import {
  catalogForm,
  catalogPayload,
  fieldChanges,
  fieldContextPayload,
  fieldFormOf,
  fieldProblems,
  moveItem,
  newFieldForm,
  optionProblem,
  optionsEditable,
  optionTargets,
  sameOrder,
  statusPayload,
  statusProblems,
  statusUnused,
  toggleIn,
  typeKind,
} from '../../src/core/catalogs.js';

const item = (over: Partial<CatalogItem>): CatalogItem => ({
  id: '1',
  name: 'Fixed',
  position: 1,
  ...over,
});

describe('catalogPayload default', () => {
  it('sets a resolution as the default, and removes it', () => {
    const plain = item({ default: false });
    expect(catalogPayload('resolutions', { ...catalogForm(plain), default: true }, plain)).toEqual({
      default: true,
    });
    const def = item({ default: true });
    expect(catalogPayload('resolutions', { ...catalogForm(def), default: false }, def)).toEqual({
      default: false,
    });
    expect(catalogPayload('resolutions', catalogForm(def), def)).toEqual({});
  });

  it('only ever sets a priority as the default', () => {
    const def = item({ default: true, color: '' });
    expect(catalogPayload('priorities', { ...catalogForm(def), default: false }, def)).toEqual({});
    const plain = item({ default: false, color: '' });
    expect(catalogPayload('priorities', { ...catalogForm(plain), default: true }, plain)).toEqual({
      default: true,
    });
  });

  it('sends no default for other catalogues', () => {
    expect(
      catalogPayload('issueTypes', { ...catalogForm(null), name: 'Bug', default: true }, null),
    ).not.toHaveProperty('default');
  });
});

describe('reordering', () => {
  const rows = ['a', 'b', 'c', 'd'].map((id) => ({ id }));
  const ids = (l: { id: string }[]) => l.map((x) => x.id).join('');

  it('moves a row down and up by one', () => {
    expect(ids(moveItem(rows, 1, 2))).toBe('acbd');
    expect(ids(moveItem(rows, 2, 1))).toBe('acbd');
    expect(ids(moveItem(rows, 3, 0))).toBe('dabc');
  });

  it('keeps the list when the move goes nowhere', () => {
    expect(ids(moveItem(rows, 0, -1))).toBe('abcd');
    expect(ids(moveItem(rows, 3, 9))).toBe('abcd');
    expect(ids(moveItem(rows, 7, 0))).toBe('abcd');
  });

  it('does not change the list it is given', () => {
    moveItem(rows, 0, 3);
    expect(ids(rows)).toBe('abcd');
  });

  it('tells whether two lists have the same order', () => {
    expect(sameOrder(rows, moveItem(rows, 1, 1))).toBe(true);
    expect(sameOrder(rows, moveItem(rows, 1, 2))).toBe(false);
    expect(sameOrder([{ id: 1 }], [{ id: '1' }])).toBe(true);
    expect(sameOrder(rows, rows.slice(1))).toBe(false);
  });
});

const status = (over: Partial<StatusRow>): StatusRow => ({
  id: '1',
  name: 'To Do',
  category: 'todo',
  position: 1,
  usage: { issues: 0, workflows: 0, boardColumns: 0 },
  ...over,
});

describe('statuses', () => {
  const all = [status({ id: '1', name: 'To Do' }), status({ id: '2', name: 'Done' })];

  it('checks the name against the other statuses, ignoring case', () => {
    expect(statusProblems('  ', all, null)).toEqual({ name: 'nameRequired' });
    expect(statusProblems('x'.repeat(256), all, null)).toEqual({ name: 'nameTooLong' });
    expect(statusProblems('done', all, null)).toEqual({ name: 'nameTaken' });
    expect(statusProblems('done', all, '2')).toEqual({});
    expect(statusProblems('In review', all, null)).toEqual({});
  });

  it('sends only what changed', () => {
    const s = status({ name: 'Blocked', category: 'doing' });
    expect(statusPayload(s, 'Blocked', 'doing')).toEqual({});
    expect(statusPayload(s, ' Waiting ', 'doing')).toEqual({ name: 'Waiting' });
    expect(statusPayload(s, 'Blocked', 'done')).toEqual({ category: 'done' });
    expect(statusPayload(s, 'Waiting', 'todo')).toEqual({ name: 'Waiting', category: 'todo' });
  });

  it('allows deletion only when nothing uses the status', () => {
    expect(statusUnused(status({}))).toBe(true);
    expect(statusUnused(status({ usage: { issues: 1, workflows: 0, boardColumns: 0 } }))).toBe(
      false,
    );
    expect(statusUnused(status({ usage: { issues: 0, workflows: 1, boardColumns: 0 } }))).toBe(
      false,
    );
    expect(statusUnused(status({ usage: { issues: 0, workflows: 0, boardColumns: 2 } }))).toBe(
      false,
    );
  });
});

const field = (over: Partial<FieldRow>): FieldRow => ({
  id: 7,
  name: 'Customer',
  description: null,
  type: 'select',
  typeKey: 'com.atlassian.jira.plugin.system.customfieldtypes:select',
  options: 2,
  issues: 0,
  context: { projects: [], issueTypes: [] },
  ...over,
});

describe('field context editing', () => {
  it('adds and removes a value once', () => {
    expect(toggleIn(['A'], 'B', true)).toEqual(['A', 'B']);
    expect(toggleIn(['A', 'B'], 'B', true)).toEqual(['A', 'B']);
    expect(toggleIn(['A', 'B'], 'A', false)).toEqual(['B']);
    expect(toggleIn(['A'], 'Z', false)).toEqual(['A']);
  });

  it('reads a global field as "all", a restricted one as its lists', () => {
    const global = fieldFormOf(field({}));
    expect(global.allProjects).toBe(true);
    expect(global.allTypes).toBe(true);
    const some = fieldFormOf(
      field({
        context: {
          projects: [{ id: 1, key: 'WEB', name: 'Web' }],
          issueTypes: [{ id: '10', name: 'Bug', subtask: false }],
        },
      }),
    );
    expect(some).toMatchObject({
      allProjects: false,
      projects: ['WEB'],
      allTypes: false,
      issueTypes: ['10'],
    });
  });

  it('sends an empty list for "all" even when boxes stay ticked', () => {
    const f = { ...newFieldForm(), projects: ['WEB'], issueTypes: ['10'] };
    expect(fieldContextPayload(f)).toEqual({ projects: [], issueTypes: [] });
    expect(fieldContextPayload({ ...f, allProjects: false })).toEqual({
      projects: ['WEB'],
      issueTypes: [],
    });
  });

  it('refuses "only these" with nothing ticked', () => {
    const f = { ...newFieldForm(), name: 'X', allProjects: false, allTypes: false };
    expect(fieldProblems(f, [], null)).toEqual({
      projects: 'projectsRequired',
      issueTypes: 'typesRequired',
    });
    expect(fieldProblems({ ...f, projects: ['WEB'], issueTypes: ['10'] }, [], null)).toEqual({});
  });

  it('sends nothing for an untouched field, and only the context when only it changed', () => {
    const f = field({
      description: 'Who asked',
      context: { projects: [{ id: 1, key: 'WEB', name: 'Web' }], issueTypes: [] },
    });
    expect(fieldChanges(f, fieldFormOf(f))).toEqual({});
    const widened = { ...fieldFormOf(f), allProjects: true };
    expect(fieldChanges(f, widened)).toEqual({ context: { projects: [], issueTypes: [] } });
    const swapped = { ...fieldFormOf(f), projects: ['OPS'] };
    expect(fieldChanges(f, swapped)).toEqual({ context: { projects: ['OPS'], issueTypes: [] } });
  });

  it('does not see a reordered context as a change', () => {
    const f = field({
      context: {
        projects: [
          { id: 1, key: 'A', name: 'A' },
          { id: 2, key: 'B', name: 'B' },
        ],
        issueTypes: [],
      },
    });
    expect(fieldChanges(f, { ...fieldFormOf(f), projects: ['B', 'A'] })).toEqual({});
  });

  it('sends a cleared description as null, a changed name trimmed', () => {
    const f = field({ description: 'Who asked' });
    expect(fieldChanges(f, { ...fieldFormOf(f), description: '  ' })).toEqual({
      description: null,
    });
    expect(fieldChanges(f, { ...fieldFormOf(f), name: ' Client ' })).toEqual({ name: 'Client' });
  });

  it('checks a renamed field against the other fields', () => {
    const others = [field({ id: 7, name: 'Customer' }), field({ id: 8, name: 'Severity' })];
    expect(fieldProblems({ ...fieldFormOf(others[0]!), name: 'severity' }, others, 7)).toEqual({
      name: 'nameTaken',
    });
    expect(fieldProblems(fieldFormOf(others[0]!), others, 7)).toEqual({});
  });
});

describe('field options', () => {
  const opt = (id: number, value: string, disabled = false): FieldOption => ({
    id,
    value,
    disabled,
    position: id,
    issues: 0,
    children: 0,
  });
  const options = [opt(1, 'Red'), opt(2, 'Green', true), opt(3, 'Blue')];

  it('checks an option against the others, ignoring case', () => {
    expect(optionProblem(' ', options, null)).toBe('optionRequired');
    expect(optionProblem('x'.repeat(256), options, null)).toBe('optionTooLong');
    expect(optionProblem('red', options, null)).toBe('optionTaken');
    expect(optionProblem('red', options, 1)).toBeNull();
    expect(optionProblem('Yellow', options, null)).toBeNull();
  });

  it('offers the other enabled options as the place values move to', () => {
    expect(optionTargets(options, options[0]!).map((o) => o.id)).toEqual([3]);
    expect(optionTargets([options[0]!], options[0]!)).toEqual([]);
  });

  it('reorders options through their ids', () => {
    const next = moveItem(options, 2, 0).map((o) => o.id);
    expect(next).toEqual([3, 1, 2]);
  });

  it('manages options of choice types and imported cascading fields only', () => {
    expect(optionsEditable(field({}))).toBe(true);
    expect(optionsEditable(field({ type: 'text', typeKey: 'x:textfield' }))).toBe(false);
    expect(optionsEditable(field({ type: null, typeKey: 'x:cascadingselect' }))).toBe(true);
    expect(optionsEditable(field({ type: null, typeKey: 'x:gh-sprint' }))).toBe(false);
    expect(typeKind('com.atlassian:multiselect')).toBe('multiselect');
    expect(typeKind('plain')).toBe('plain');
  });
});
