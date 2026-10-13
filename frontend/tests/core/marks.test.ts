import { describe, expect, it } from 'vitest';

import {
  fileExtension,
  initials,
  isImage,
  priorityLevel,
  projectOfKey,
  typeLetter,
  typeTone,
} from '../../src/core/marks.js';

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
    expect(projectOfKey('demo-12')).toBe('DEMO');
    expect(projectOfKey(' WEB_2-3 ')).toBe('WEB_2');
    expect(projectOfKey('DEMO')).toBeNull();
    expect(projectOfKey('12-DEMO')).toBeNull();
  });
});
