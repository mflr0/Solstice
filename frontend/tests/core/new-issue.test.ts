import { describe, expect, it } from 'vitest';

import { newIssueProblems } from '../../src/core/new-issue.js';
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
