/**
 * The new-issue form: what it needs before it can be sent. The server
 * requires a project, a type and a summary (backend/internal/api/issues.go,
 * planIssue). Problems are keys; the dialog words them (src/i18n, block create).
 */

import type { Problems } from './admin-forms.js';
import type { DurationUnits } from './dates.js';
import { readEstimate } from './estimates.js';

/** What the new-issue form checks before sending. */
export interface NewIssueForm {
  project: string;
  type: string;
  summary: string;
  estimate: string;
}

export type NewIssueField = keyof NewIssueForm;
export type NewIssueProblem = 'required' | 'unreadable';

const REQUIRED_FIELDS: readonly NewIssueField[] = ['project', 'type', 'summary'];

export function newIssueProblems(
  f: NewIssueForm,
  units?: DurationUnits,
): Problems<NewIssueField, NewIssueProblem> {
  const out: Problems<NewIssueField, NewIssueProblem> = {};
  for (const k of REQUIRED_FIELDS) if (!f[k].trim()) out[k] = 'required';
  if (!readEstimate(f.estimate, units).ok) out.estimate = 'unreadable';
  return out;
}
