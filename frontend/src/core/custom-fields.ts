// Custom field values as issue forms hold them, and as the REST API reads them.

import type { CustomValue, OfferedField, UserRef } from './types.js';

/**
 * What an editor holds: the text typed (text, number, URL, dates), the
 * chosen option ids, labels or people. A single choice is a list of at
 * most one.
 */
export type CustomDraft = string | number[] | string[] | UserRef[];

/** The draft of a field nothing was entered in. */
export function emptyDraft(field: OfferedField): CustomDraft {
  switch (field.type) {
    case 'select':
    case 'radio':
    case 'multiselect':
    case 'checkboxes':
    case 'labels':
    case 'user':
    case 'multiuser':
      return [];
    default:
      return '';
  }
}

/** A draft as the REST API reads it in "customFields"; undefined when empty. */
export function restValue(field: OfferedField, draft: CustomDraft): unknown {
  switch (field.type) {
    case 'select':
    case 'radio': {
      const [id] = draft as number[];
      return id === undefined ? undefined : { id: String(id) };
    }
    case 'multiselect':
    case 'checkboxes': {
      const ids = draft as number[];
      return ids.length ? ids.map((id) => ({ id: String(id) })) : undefined;
    }
    case 'labels': {
      const labels = draft as string[];
      return labels.length ? labels : undefined;
    }
    case 'user': {
      const [u] = draft as UserRef[];
      return u ? { name: u.username } : undefined;
    }
    case 'multiuser': {
      const users = draft as UserRef[];
      return users.length ? users.map((u) => ({ name: u.username })) : undefined;
    }
    case 'number': {
      const s = (draft as string).trim();
      if (!s) return undefined;
      // Not a number: sent as typed, so the server's error names the field.
      return Number.isFinite(Number(s)) ? Number(s) : s;
    }
    case 'datetime': {
      const s = draft as string;
      if (!s) return undefined;
      const d = new Date(s);
      return Number.isNaN(d.getTime()) ? s : d.toISOString();
    }
    case 'date':
      return (draft as string) || undefined;
    default: {
      const s = draft as string;
      return s.trim() ? s : undefined;
    }
  }
}

/**
 * The draft of a field's stored value, as the issue view shows it: options
 * by label, dates and times as the server writes them. Labels of options no
 * longer offered are left out.
 */
export function draftOf(field: OfferedField, value: CustomValue | undefined): CustomDraft {
  if (!value) return emptyDraft(field);
  switch (field.type) {
    case 'select':
    case 'radio':
    case 'multiselect':
    case 'checkboxes':
      return value.values.flatMap((v) => field.options.find((o) => o.value === v)?.id ?? []);
    case 'labels':
      return [...value.values];
    case 'user':
    case 'multiuser':
      return [...(value.users ?? [])];
    case 'datetime':
      return value.values[0] ? localDateTime(value.values[0]) : '';
    default:
      return value.values[0] ?? '';
  }
}

// An instant as a datetime-local input holds it, in the browser's zone.
function localDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}
