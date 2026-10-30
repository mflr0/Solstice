/**
 * The new-issue form: what it needs before it can be sent. The server
 * requires a project, a type and a summary (backend/internal/api/issues.go,
 * planIssue). Problems are keys; the dialog words them (src/i18n, block create).
 * It also remembers, per viewer, the project, assignee and components of the
 * last created issues.
 */

import type { Problems } from './admin-forms.js';
import type { DurationUnits } from './dates.js';
import { readEstimate } from './estimates.js';
import type { Component, UserRef } from './types.js';

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

/** What the form keeps from a project's last created issue. */
export interface ProjectMemory {
  assignee: UserRef | null;
  components: number[];
}

/** What the new-issue form remembers from the viewer's last created issues. */
export interface NewIssueMemory {
  /** The project of the last created issue. */
  project: string;
  /** Per project key, since assignable users and components differ by project. */
  projects: Record<string, ProjectMemory>;
}

export const emptyMemory: NewIssueMemory = { project: '', projects: {} };

/** The localStorage key: one per viewer, so accounts sharing a browser don't mix. */
export function memoryKey(userId: number): string {
  return `solstice.newIssue.${userId}`;
}

function isUser(v: unknown): v is UserRef {
  const u = v as UserRef;
  return (
    typeof u === 'object' &&
    u !== null &&
    typeof u.id === 'number' &&
    typeof u.username === 'string' &&
    typeof u.name === 'string'
  );
}

/** Reads stored memory, dropping anything malformed. */
export function readMemory(raw: string | null): NewIssueMemory {
  let v: unknown;
  try {
    v = JSON.parse(raw ?? '');
  } catch {
    return emptyMemory;
  }
  if (typeof v !== 'object' || v === null) return emptyMemory;
  const o = v as Record<string, unknown>;
  const projects: Record<string, ProjectMemory> = {};
  if (typeof o.projects === 'object' && o.projects !== null) {
    for (const [key, p] of Object.entries(o.projects as Record<string, unknown>)) {
      if (typeof p !== 'object' || p === null) continue;
      const m = p as Record<string, unknown>;
      projects[key] = {
        assignee: isUser(m.assignee) ? m.assignee : null,
        components: Array.isArray(m.components)
          ? m.components.filter((c): c is number => Number.isInteger(c))
          : [],
      };
    }
  }
  return { project: typeof o.project === 'string' ? o.project : '', projects };
}

/** The memory once an issue was created in `project` with these fields. */
export function remember(
  m: NewIssueMemory,
  project: string,
  fields: ProjectMemory,
): NewIssueMemory {
  return {
    project,
    projects: {
      ...m.projects,
      [project]: { assignee: fields.assignee, components: [...fields.components] },
    },
  };
}

/**
 * The project the form opens on: the explicit one (the page's, the parent's,
 * the scoped one), else the last created issue's, else the first offered.
 */
export function startProject(
  explicit: string | null | undefined,
  m: NewIssueMemory,
  offered: readonly string[],
): string {
  for (const key of [explicit, m.project]) if (key && offered.includes(key)) return key;
  return offered[0] ?? '';
}

/** A project's remembered fields, without components that are archived or gone. */
export function recall(
  m: NewIssueMemory,
  project: string,
  components: readonly Component[],
): ProjectMemory {
  const p = m.projects[project];
  if (!p) return { assignee: null, components: [] };
  const live = new Set(components.filter((c) => !c.archived).map((c) => c.id));
  return { assignee: p.assignee, components: p.components.filter((id) => live.has(id)) };
}
