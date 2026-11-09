import { describe, expect, it } from 'vitest';

import { issueLinkKey, modalIssue, withModalIssue } from '../../src/core/route.js';

describe('modalIssue', () => {
  it.each([
    ['', null],
    ['?quick=1', null],
    ['?issue=', null],
    ['?issue=%20', null],
    ['?issue=DEMO-12', 'DEMO-12'],
    ['?quick=1&issue=demo-12', 'DEMO-12'],
    ['?issue=DEMO-12&sprint=3', 'DEMO-12'],
  ])('%s gives %s', (search, key) => {
    expect(modalIssue(search)).toBe(key);
  });
});

describe('withModalIssue', () => {
  it('adds the issue after the other parameters', () => {
    expect(withModalIssue('?quick=1&quick=2&sprint=3', 'DEMO-12')).toBe(
      '?quick=1&quick=2&sprint=3&issue=DEMO-12',
    );
  });

  it('adds it to a page without parameters', () => {
    expect(withModalIssue('', 'DEMO-12')).toBe('?issue=DEMO-12');
  });

  it('replaces the issue in place', () => {
    expect(withModalIssue('?issue=DEMO-12&sprint=3', 'DEMO-13')).toBe('?issue=DEMO-13&sprint=3');
  });

  it('removes it and keeps the rest', () => {
    expect(withModalIssue('?quick=1&issue=DEMO-12&sprint=3', null)).toBe('?quick=1&sprint=3');
    expect(withModalIssue('?issue=DEMO-12', null)).toBe('');
    expect(withModalIssue('', null)).toBe('');
  });

  it('round-trips with modalIssue', () => {
    expect(modalIssue(withModalIssue('?sprint=3', 'DEMO-12'))).toBe('DEMO-12');
    expect(modalIssue(withModalIssue('?issue=DEMO-12', null))).toBeNull();
  });
});

describe('issueLinkKey', () => {
  const origin = 'https://solstice.example';
  const key = (href: string) => issueLinkKey(new URL(href, origin), origin);

  it('reads the key of an issue link', () => {
    expect(key('/browse/DEMO-12')).toBe('DEMO-12');
    expect(key('/browse/demo-12')).toBe('DEMO-12');
  });

  it('leaves other links alone', () => {
    expect(key('/')).toBeNull();
    expect(key('/projects/DEMO')).toBeNull();
    expect(key('/browse')).toBeNull();
    expect(key('/browse/DEMO-12/extra')).toBeNull();
    expect(key('/people/marie')).toBeNull();
    expect(key('/boards/3?issue=DEMO-12')).toBeNull();
    expect(key('https://elsewhere.example/browse/DEMO-12')).toBeNull();
  });

  it('leaves a link to a place in an issue to the full page', () => {
    expect(key('/browse/DEMO-12#comment-4')).toBeNull();
    expect(key('/browse/DEMO-12?focusedCommentId=4')).toBeNull();
  });
});
