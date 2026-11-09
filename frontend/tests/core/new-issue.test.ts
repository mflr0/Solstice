import { describe, expect, it } from 'vitest';

import {
  emptyMemory,
  memoryKey,
  newIssueProblems,
  notAttached,
  queueFiles,
  queueFor,
  readMemory,
  recall,
  remember,
  startProject,
  unqueueFile,
} from '../../src/core/new-issue.js';
import { fr } from '../../src/i18n/fr.js';

const filled = { project: 'DEMO', type: '10001', summary: 'Fix the login', estimate: '' };

describe('newIssueProblems', () => {
  it('finds nothing in a filled form', () => {
    expect(newIssueProblems(filled)).toEqual({});
  });

  it('asks for the project, the type and the summary', () => {
    expect(newIssueProblems({ project: '', type: '', summary: '  ', estimate: '' })).toEqual({
      project: 'required',
      type: 'required',
      summary: 'required',
    });
  });

  it('leaves the estimate optional, but must read it', () => {
    expect(newIssueProblems({ ...filled, estimate: '1d 4h' })).toEqual({});
    expect(newIssueProblems({ ...filled, estimate: 'soon' })).toEqual({ estimate: 'unreadable' });
  });

  it('reads the estimate in the viewer’s units', () => {
    expect(newIssueProblems({ ...filled, estimate: '1j 2h' }, fr.units.duration)).toEqual({});
  });
});

const ann = { id: 7, username: 'ann', name: 'Ann Lee', active: true };
const bob = { id: 8, username: 'bob', name: 'Bob Roy', active: true };
const comps = [
  { id: 1, name: 'API', archived: false },
  { id: 2, name: 'UI', archived: false },
  { id: 3, name: 'Old', archived: true },
];

describe('new-issue memory', () => {
  it('survives a round trip through storage', () => {
    const m = remember(emptyMemory, 'WEB', { assignee: ann, components: [1, 2] });
    expect(readMemory(JSON.stringify(m))).toEqual(m);
  });

  it('keeps each project’s fields apart', () => {
    let m = remember(emptyMemory, 'WEB', { assignee: ann, components: [1] });
    m = remember(m, 'MOB', { assignee: bob, components: [2] });
    m = readMemory(JSON.stringify(m));
    expect(m.project).toBe('MOB');
    expect(recall(m, 'WEB', comps)).toEqual({ assignee: ann, components: [1] });
    expect(recall(m, 'MOB', comps)).toEqual({ assignee: bob, components: [2] });
    expect(recall(m, 'OPS', comps)).toEqual({ assignee: null, components: [] });
  });

  it('drops archived and unknown components', () => {
    const m = remember(emptyMemory, 'WEB', { assignee: null, components: [1, 3, 99] });
    expect(recall(m, 'WEB', comps).components).toEqual([1]);
  });

  it('reads missing or malformed storage as empty', () => {
    expect(readMemory(null)).toEqual(emptyMemory);
    expect(readMemory('{oops')).toEqual(emptyMemory);
    expect(readMemory('"WEB"')).toEqual(emptyMemory);
    expect(
      readMemory(
        '{"project":"WEB","projects":{"WEB":{"assignee":{"id":"x"},"components":[1,"2"]}}}',
      ),
    ).toEqual({ project: 'WEB', projects: { WEB: { assignee: null, components: [1] } } });
  });

  it('keeps one memory per viewer', () => {
    expect(memoryKey(7)).not.toBe(memoryKey(8));
  });
});

describe('startProject', () => {
  const offered = ['MOB', 'OPS', 'WEB'];
  const m = remember(emptyMemory, 'OPS', { assignee: null, components: [] });

  it('lets an explicit project win over the remembered one', () => {
    expect(startProject('WEB', m, offered)).toBe('WEB');
  });

  it('opens on the last created issue’s project without an explicit one', () => {
    expect(startProject(undefined, m, offered)).toBe('OPS');
    expect(startProject(null, m, offered)).toBe('OPS');
  });

  it('skips projects no longer offered', () => {
    expect(startProject('GONE', m, offered)).toBe('OPS');
    expect(
      startProject(
        undefined,
        remember(emptyMemory, 'GONE', { assignee: null, components: [] }),
        offered,
      ),
    ).toBe('MOB');
    expect(startProject(undefined, emptyMemory, [])).toBe('');
  });
});

describe('file queue', () => {
  const file = (name: string, body = name) => new File([body], name);

  it('adds files in order', () => {
    const q = queueFiles(queueFiles([], [file('a.pdf')]), [file('b.png'), file('c.txt')]);
    expect(q.map((f) => f.name)).toEqual(['a.pdf', 'b.png', 'c.txt']);
  });

  it('keeps one file per name, the last one added, at the end', () => {
    const q = queueFiles([file('a.pdf', 'old'), file('b.png')], [file('a.pdf', 'newer')]);
    expect(q.map((f) => f.name)).toEqual(['b.png', 'a.pdf']);
    expect(q[1]!.size).toBe(5);
  });

  it('removes a file by name', () => {
    const q = unqueueFile([file('a.pdf'), file('b.png')], 'a.pdf');
    expect(q.map((f) => f.name)).toEqual(['b.png']);
  });

  it('is cleared for a project without attach, kept otherwise', () => {
    const q = [file('a.pdf')];
    expect(queueFor(q, false)).toEqual([]);
    expect(queueFor(q, true)).toEqual(q);
  });

  it('names the files the issue does not hold', () => {
    const q = [file('a.pdf'), file('b.png')];
    expect(notAttached(q, [{ filename: 'b.png' }])).toEqual(['a.pdf']);
    expect(notAttached(q, [])).toEqual(['a.pdf', 'b.png']);
  });
});
