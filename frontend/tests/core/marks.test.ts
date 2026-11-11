import { describe, expect, it } from 'vitest';

import {
  fileExtension,
  initials,
  isImage,
  isIssueKey,
  priorityIcon,
  priorityLevel,
  projectOfIssueKey,
  typeLetter,
  typeTone,
} from '../../src/core/marks.js';

describe('priorityIcon', () => {
  it('shows the chosen icon wherever the priority sits', () => {
    expect(priorityIcon('blocker', 4, 5)).toBe('blocker');
    expect(priorityIcon('lowest', 0, 5)).toBe('lowest');
    expect(priorityIcon('trivial', -1, 5)).toBe('trivial');
  });

  it('falls back to the level of the position without one', () => {
    expect([0, 2, 4].map((i) => priorityIcon(undefined, i, 5))).toEqual([
      'highest',
      'medium',
      'lowest',
    ]);
    expect(priorityIcon(undefined, 1, 3)).toBe(priorityLevel(1, 3));
  });
});

describe('priorityLevel', () => {
  it("maps Jira's five priorities one to one", () => {
    expect([0, 1, 2, 3, 4].map((i) => priorityLevel(i, 5))).toEqual([
      'highest',
      'high',
      'medium',
      'low',
      'lowest',
    ]);
  });

  it('keeps extremes and the exact middle for other lists', () => {
    expect([0, 1, 2].map((i) => priorityLevel(i, 3))).toEqual(['high', 'medium', 'low']);
    expect([0, 1].map((i) => priorityLevel(i, 2))).toEqual(['high', 'low']);
    expect([0, 1, 2, 3].map((i) => priorityLevel(i, 4))).toEqual([
      'highest',
      'high',
      'low',
      'lowest',
    ]);
    expect([0, 3, 6].map((i) => priorityLevel(i, 7))).toEqual(['highest', 'medium', 'lowest']);
  });

  it('is medium when unknown', () => {
    expect(priorityLevel(-1, 5)).toBe('medium');
    expect(priorityLevel(0, 1)).toBe('medium');
  });
});

describe('issue types', () => {
  it('reads the family from the name, in English or French', () => {
    expect(typeTone('Bug', false)).toBe('bug');
    expect(typeTone('Anomalie', false)).toBe('bug');
    expect(typeTone('Epic', false)).toBe('epic');
    expect(typeTone('Story', false)).toBe('story');
    expect(typeTone('Tâche', false)).toBe('task');
    expect(typeTone('Sub-task', true)).toBe('subtask');
  });

  it('prints the first letter', () => {
    expect(typeLetter('Story')).toBe('S');
    expect(typeLetter('épopée')).toBe('É');
    expect(typeLetter('  ')).toBe('?');
  });
});

describe('initials', () => {
  it.each([
    ['Marie Durand', 'MD'],
    ['Jean-Pierre de la Fontaine', 'JF'],
    ['admin', 'AD'],
    ['marie.durand', 'MD'],
    ['', '?'],
  ])('%s → %s', (name, expected) => {
    expect(initials(name)).toBe(expected);
  });
});

describe('files and keys', () => {
  it('finds extensions and inline images', () => {
    expect(fileExtension('rapport.final.PDF')).toBe('PDF');
    expect(fileExtension('Makefile')).toBe('');
    expect(isImage('image/png')).toBe(true);
    expect(isImage('image/svg+xml')).toBe(false);
    expect(isImage('IMAGE/JPEG; charset=binary')).toBe(true);
  });

  it('extracts the project of an issue key', () => {
    expect(projectOfIssueKey('demo-12')).toBe('DEMO');
    expect(projectOfIssueKey(' WEB_2-3 ')).toBe('WEB_2');
    expect(projectOfIssueKey('DEMO')).toBeNull();
    expect(projectOfIssueKey('12-DEMO')).toBeNull();
  });

  it('tells an issue key from other text', () => {
    expect(isIssueKey('demo-12')).toBe(true);
    expect(isIssueKey(' WEB_2-3 ')).toBe(true);
    expect(isIssueKey('DEMO')).toBe(false);
    expect(isIssueKey('12-DEMO')).toBe(false);
  });
});
