/**
 * The former search syntax (@moi, est:ouvert, projet:DEMO, #label…), read
 * as JQL. The server still understands it (backend/internal/search); links
 * and bookmarks that use it open on the search page as filters.
 */

import { categoryClause, EMPTY, listClause, ME } from './filters.js';
import { formatOrder, quote, quoteString, type SortTerm } from './jql.js';
import { STATUS_CATEGORIES, type StatusCategory } from './types.js';

type Field =
  | 'assignee'
  | 'reporter'
  | 'status'
  | 'category'
  | 'type'
  | 'project'
  | 'priority'
  | 'labels'
  | 'component'
  | 'fixVersion';

const PREFIXES: Record<string, Field> = {
  par: 'reporter',
  auteur: 'reporter',
  rapporteur: 'reporter',
  reporter: 'reporter',
  assigné: 'assignee',
  assigne: 'assignee',
  assignee: 'assignee',
  statut: 'status',
  status: 'status',
  est: 'category',
  is: 'category',
  type: 'type',
  projet: 'project',
  project: 'project',
  priorité: 'priority',
  priorite: 'priority',
  priority: 'priority',
  prio: 'priority',
  étiquette: 'labels',
  etiquette: 'labels',
  label: 'labels',
  composant: 'component',
  component: 'component',
  version: 'fixVersion',
};

const CATEGORIES: Record<string, StatusCategory[]> = {
  ouvert: ['todo', 'doing'],
  open: ['todo', 'doing'],
  'à-faire': ['todo'],
  'a-faire': ['todo'],
  todo: ['todo'],
  'en-cours': ['doing'],
  doing: ['doing'],
  fini: ['done'],
  fermé: ['done'],
  ferme: ['done'],
  résolu: ['done'],
  resolu: ['done'],
  done: ['done'],
  closed: ['done'],
  resolved: ['done'],
};

const SORTS: Record<string, SortTerm> = {
  maj: { field: 'updated', desc: true },
  updated: { field: 'updated', desc: true },
  création: { field: 'created', desc: true },
  creation: { field: 'created', desc: true },
  created: { field: 'created', desc: true },
  priorité: { field: 'priority', desc: true },
  priorite: { field: 'priority', desc: true },
  priority: { field: 'priority', desc: true },
  clé: { field: 'key', desc: false },
  cle: { field: 'key', desc: false },
  key: { field: 'key', desc: false },
  échéance: { field: 'due', desc: false },
  echeance: { field: 'due', desc: false },
  due: { field: 'due', desc: false },
};

const ME_WORDS = ['moi', 'me', 'moi-même'];
const NOBODY_WORDS = ['personne', 'none', 'aucun', 'nobody'];
const KEY = /^([A-Za-z][A-Za-z0-9_]*)-(\d+)$/;

interface Tok {
  text: string;
  quoted: boolean;
}

/** Splits on spaces, keeping "phrases" and prefix:"values" together. */
function tokenize(s: string): Tok[] {
  const out: Tok[] = [];
  let cur = '';
  let quoted = false;
  let inQuote = false;
  const flush = () => {
    if (cur) out.push({ text: cur, quoted });
    cur = '';
    quoted = false;
  };
  for (const c of s) {
    if (c === '"') {
      if (inQuote) {
        inQuote = false;
        // A phrase on its own is text; prefix:"value" stays a filter.
        if (!cur.includes(':')) quoted = true;
        flush();
      } else {
        inQuote = true;
      }
    } else if (/\s/.test(c) && !inQuote) {
      flush();
    } else {
      cur += c;
    }
  }
  flush();
  return out;
}

function userValue(v: string): string {
  const w = v.toLowerCase();
  if (ME_WORDS.includes(w)) return ME;
  if (NOBODY_WORDS.includes(w)) return EMPTY;
  return quote(v);
}

/** The former search syntax as an equivalent JQL query. */
export function simpleToJql(q: string): string {
  const words: string[] = [];
  const excluded: string[] = [];
  const keys: string[] = [];
  const groups = new Map<string, { field: Field; not: boolean; values: string[] }>();
  let order: SortTerm[] = [];

  for (const tok of tokenize(q)) {
    let raw = tok.text;
    if (tok.quoted) {
      words.push(raw);
      continue;
    }
    let not = false;
    if (raw.length > 1 && raw.startsWith('-')) {
      not = true;
      raw = raw.slice(1);
    }
    let field: Field | undefined;
    let value = '';
    if (raw.startsWith('@') && raw.length > 1) {
      field = 'assignee';
      value = raw.slice(1);
    } else if (raw.startsWith('#') && raw.length > 1) {
      field = 'labels';
      value = raw.slice(1);
    } else if (KEY.test(raw) && !not) {
      keys.push(raw.toUpperCase());
      continue;
    } else {
      const i = raw.indexOf(':');
      if (i > 0 && i < raw.length - 1) {
        const name = raw.slice(0, i).toLowerCase();
        const v = raw.slice(i + 1);
        if (name === 'tri' || name === 'sort') {
          const s = SORTS[v.toLowerCase()];
          if (s) order = [s];
          continue;
        }
        field = PREFIXES[name];
        value = v.replace(/^@/, '');
      }
    }
    if (!field) {
      (not ? excluded : words).push(raw);
      continue;
    }
    const id = `${field}/${not}`;
    const g = groups.get(id) ?? { field, not, values: [] };
    g.values.push(value);
    groups.set(id, g);
  }

  const parts: string[] = [];
  for (const g of groups.values()) {
    if (g.field === 'category') {
      const cats = new Set(g.values.flatMap((v) => CATEGORIES[v.toLowerCase()] ?? []));
      const chosen = STATUS_CATEGORIES.filter((c) => cats.has(c) !== g.not);
      const clause = categoryClause(chosen);
      if (clause) parts.push(clause);
      continue;
    }
    const values = g.values.map((v) =>
      g.field === 'assignee' || g.field === 'reporter' ? userValue(v) : quote(v),
    );
    const unique = [...new Set(values)];
    const clause = listClause(g.field, unique);
    if (!g.not) {
      parts.push(clause);
    } else if (unique.every((v) => v === EMPTY)) {
      parts.push(`${g.field} is not EMPTY`);
    } else {
      parts.push(
        unique.length === 1
          ? `${g.field} != ${unique[0]}`
          : `${g.field} not in (${unique.join(', ')})`,
      );
    }
  }
  if (keys.length)
    parts.push(keys.length === 1 ? `key = ${keys[0]}` : `key in (${keys.join(', ')})`);
  if (words.length) parts.push(`text ~ ${quoteString(words.join(' '))}`);
  for (const w of excluded) parts.push(`text !~ ${quoteString(w)}`);
  return [parts.join(' AND '), formatOrder(order)].filter((p) => p !== '').join(' ');
}
