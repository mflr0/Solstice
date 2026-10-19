/**
 * JQL on the client. The lexer and parser follow the server's grammar
 * (backend/internal/jql/jql.go) so the search page can turn a query into
 * filters and back; the server stays the judge of what a query means.
 *
 * Also here: quoting for values and field names, and the reading the query
 * editor needs — what each token is (for colouring) and what may be typed
 * at the caret (for suggestions).
 */

export type TokenKind = 'word' | 'string' | 'op' | 'lparen' | 'rparen' | 'comma' | 'error';

export interface Token {
  kind: TokenKind;
  /** The token as written, quotes included. */
  raw: string;
  /** Its value: for a string, the content without quotes and escapes. */
  text: string;
  start: number;
  end: number;
  /** False for a string whose closing quote is missing. */
  closed: boolean;
}

/**
 * The syntax errors and the arguments each message takes. The codes are the
 * server's JQL* keys without the prefix (backend/internal/i18n/en.go).
 */
export interface JqlArgs {
  unclosedQuote: [];
  incompleteBang: [];
  unexpected: [token: string];
  orderBy: [];
  sortFieldExpected: [];
  parenExpected: [];
  fieldExpected: [];
  notIn: [];
  operatorExpected: [field: string];
  isEmpty: [];
  changedOnly: [keyword: string];
  duringDates: [];
  argumentInvalid: [name: string];
  valueExpected: [];
}

export type JqlCode = keyof JqlArgs;

/** What a catalogue supplies for the syntax errors (its `jql` section). */
export type JqlMessages = { readonly [C in JqlCode]: (...args: JqlArgs[C]) => string };

/** A syntax error: its code, the message's arguments and the offset it was found at. */
export class JqlError<C extends JqlCode = JqlCode> extends Error {
  readonly args: JqlArgs[C];

  constructor(
    readonly code: C,
    readonly pos: number,
    ...args: JqlArgs[C]
  ) {
    super(code);
    this.args = args;
  }
}

const isSpace = (c: string) => /\s/.test(c);
const WORD_STOP = '()",\'=!~<>';

/** Splits a query into tokens; never fails (a lone "!" becomes an error token). */
export function lex(src: string): Token[] {
  const out: Token[] = [];
  let i = 0;
  const push = (kind: TokenKind, start: number, end: number, text = src.slice(start, end)) =>
    out.push({ kind, raw: src.slice(start, end), text, start, end, closed: true });
  while (i < src.length) {
    const c = src.charAt(i);
    const start = i;
    if (isSpace(c)) {
      i++;
    } else if (c === '(') {
      push('lparen', i, ++i);
    } else if (c === ')') {
      push('rparen', i, ++i);
    } else if (c === ',') {
      push('comma', i, ++i);
    } else if (c === '"' || c === "'") {
      let text = '';
      i++;
      while (i < src.length && src.charAt(i) !== c) {
        if (src.charAt(i) === '\\' && i + 1 < src.length) i++;
        text += src.charAt(i);
        i++;
      }
      const closed = i < src.length;
      if (closed) i++;
      out.push({ kind: 'string', raw: src.slice(start, i), text, start, end: i, closed });
    } else if ('=!~<>'.includes(c)) {
      const n = src.charAt(i + 1);
      const len = n === '=' || (c === '!' && n === '~') ? 2 : 1;
      i += len;
      push(c === '!' && len === 1 ? 'error' : 'op', start, i);
    } else {
      while (i < src.length && !isSpace(src.charAt(i)) && !WORD_STOP.includes(src.charAt(i))) i++;
      push('word', start, i);
    }
  }
  return out;
}

/* Syntax tree ---------------------------------------------------------------- */

export type Value =
  | { kind: 'text'; text: string }
  | { kind: 'func'; name: string; args: string[] }
  | { kind: 'empty' };

export interface Predicate {
  /** after, before, on, during, by, from, to */
  kind: string;
  values: Value[];
}

export interface Clause {
  type: 'clause';
  field: string;
  /** = != ~ !~ > >= < <= in "not in" is "is not" was "was not" "was in" "was not in" changed */
  op: string;
  values: Value[];
  preds: Predicate[];
}

export type Node =
  | { type: 'and'; left: Node; right: Node }
  | { type: 'or'; left: Node; right: Node }
  | { type: 'not'; inner: Node }
  | Clause;

export interface SortTerm {
  field: string;
  desc: boolean;
}

export interface Query {
  where: Node | null;
  order: SortTerm[];
}

const EOF: Token = { kind: 'error', raw: '', text: '', start: 0, end: 0, closed: true };

class Parser {
  private i = 0;
  private readonly eof: Token;

  constructor(
    private readonly toks: Token[],
    length: number,
  ) {
    this.eof = { ...EOF, start: length, end: length };
  }

  peek(): Token {
    return this.toks[this.i] ?? this.eof;
  }

  atEnd(): boolean {
    return this.i >= this.toks.length;
  }

  next(): Token {
    const t = this.peek();
    if (!this.atEnd()) this.i++;
    return t;
  }

  keyword(...words: string[]): boolean {
    const t = this.peek();
    if (t.kind !== 'word') return false;
    const w = t.text.toLowerCase();
    return words.includes(w);
  }

  fail<C extends JqlCode>(code: C, ...args: JqlArgs[C]): never {
    throw new JqlError(code, this.peek().start, ...args);
  }

  query(): Query {
    const q: Query = { where: null, order: [] };
    if (!this.keyword('order') && !this.atEnd()) q.where = this.or();
    if (this.keyword('order')) {
      this.next();
      if (!this.keyword('by')) this.fail('orderBy');
      this.next();
      for (;;) {
        const t = this.next();
        if (t.kind !== 'word' && t.kind !== 'string') this.fail('sortFieldExpected');
        const s: SortTerm = { field: t.text, desc: false };
        if (this.keyword('asc', 'desc')) s.desc = this.next().text.toLowerCase() === 'desc';
        q.order.push(s);
        if (this.peek().kind !== 'comma') break;
        this.next();
      }
    }
    if (!this.atEnd()) this.fail('unexpected', this.peek().text);
    return q;
  }

  or(): Node {
    let left = this.and();
    while (this.keyword('or')) {
      this.next();
      left = { type: 'or', left, right: this.and() };
    }
    return left;
  }

  and(): Node {
    let left = this.not();
    while (this.keyword('and')) {
      this.next();
      left = { type: 'and', left, right: this.not() };
    }
    return left;
  }

  not(): Node {
    if (this.keyword('not')) {
      this.next();
      return { type: 'not', inner: this.not() };
    }
    if (this.peek().kind === 'lparen') {
      this.next();
      const inner = this.or();
      if (this.next().kind !== 'rparen') this.fail('parenExpected');
      return inner;
    }
    return this.clause();
  }

  clause(): Node {
    const f = this.next();
    if (f.kind !== 'word' && f.kind !== 'string') this.fail('fieldExpected');
    const c: Clause = { type: 'clause', field: f.text, op: '', values: [], preds: [] };
    const t = this.peek();
    if (t.kind === 'op') {
      c.op = this.next().text;
    } else if (this.keyword('in')) {
      this.next();
      c.op = 'in';
    } else if (this.keyword('not')) {
      this.next();
      if (!this.keyword('in')) this.fail('notIn');
      this.next();
      c.op = 'not in';
    } else if (this.keyword('is')) {
      this.next();
      c.op = 'is';
      if (this.keyword('not')) {
        this.next();
        c.op = 'is not';
      }
    } else if (this.keyword('was')) {
      this.next();
      c.op = 'was';
      if (this.keyword('not')) {
        this.next();
        c.op = 'was not';
      }
      if (this.keyword('in')) {
        this.next();
        c.op += ' in';
        c.values = this.list();
      } else {
        c.values = [this.value()];
      }
      return this.predicates(c);
    } else if (this.keyword('changed')) {
      this.next();
      c.op = 'changed';
      return this.predicates(c);
    } else {
      this.fail('operatorExpected', f.text);
    }

    if (c.op === 'in' || c.op === 'not in') {
      c.values = this.list();
      return c;
    }
    const v = this.value();
    if ((c.op === 'is' || c.op === 'is not') && v.kind !== 'empty') {
      this.fail('isEmpty');
    }
    c.values = [v];
    return c;
  }

  list(): Value[] {
    if (this.peek().kind !== 'lparen') return [this.value()];
    this.next();
    const vals: Value[] = [];
    for (;;) {
      vals.push(this.value());
      if (this.peek().kind === 'comma') {
        this.next();
        continue;
      }
      if (this.next().kind !== 'rparen') this.fail('parenExpected');
      return vals;
    }
  }

  predicates(c: Clause): Node {
    while (this.keyword('after', 'before', 'on', 'during', 'by', 'from', 'to')) {
      const kind = this.next().text.toLowerCase();
      if ((kind === 'from' || kind === 'to') && c.op !== 'changed') {
        this.fail('changedOnly', kind.toUpperCase());
      }
      let values: Value[];
      if (kind === 'during') {
        if (this.peek().kind !== 'lparen') {
          this.fail('duringDates');
        }
        values = this.list();
        if (values.length !== 2) this.fail('duringDates');
      } else if (kind === 'by' && this.peek().kind === 'lparen') {
        values = this.list();
      } else {
        values = [this.value()];
      }
      c.preds.push({ kind, values });
    }
    return c;
  }

  value(): Value {
    const t = this.next();
    if (t.kind === 'string') return { kind: 'text', text: t.text };
    if (t.kind === 'word') {
      const w = t.text.toLowerCase();
      if (w === 'empty' || w === 'null') return { kind: 'empty' };
      if (this.peek().kind === 'lparen') {
        this.next();
        const v: Value = { kind: 'func', name: t.text, args: [] };
        for (;;) {
          if (this.peek().kind === 'rparen') break;
          const a = this.next();
          if (a.kind !== 'word' && a.kind !== 'string') {
            throw new JqlError('argumentInvalid', a.start, t.text);
          }
          v.args.push(a.text);
          if (this.peek().kind === 'comma') this.next();
        }
        this.next();
        return v;
      }
      return { kind: 'text', text: t.text };
    }
    throw new JqlError('valueExpected', t.start);
  }
}

/** Reads a query; throws JqlError, with the server's error codes, on a syntax error. */
export function parse(src: string): Query {
  const toks = lex(src);
  const bad = toks.find((t) => t.kind === 'error' || (t.kind === 'string' && !t.closed));
  if (bad?.kind === 'error') throw new JqlError('incompleteBang', bad.start);
  if (bad) throw new JqlError('unclosedQuote', bad.start);
  return new Parser(toks, src.length).query();
}

/** The string index of the pos-th character (the server counts characters, not UTF-16 units). */
export function charIndex(src: string, pos: number): number {
  let index = 0;
  let n = 0;
  for (const ch of src) {
    if (n === pos) return index;
    index += ch.length;
    n++;
  }
  return src.length;
}

/* Writing JQL ---------------------------------------------------------------- */

const RESERVED = new Set([
  'and',
  'or',
  'not',
  'in',
  'is',
  'empty',
  'null',
  'order',
  'by',
  'asc',
  'desc',
  'was',
  'changed',
  'after',
  'before',
  'on',
  'during',
  'from',
  'to',
]);

/** A value as JQL: bare when it is a plain word, quoted (and escaped) otherwise. */
export function quote(value: string): string {
  if (/^[\p{L}\p{N}_.-]+$/u.test(value) && !RESERVED.has(value.toLowerCase())) return value;
  return quoteString(value);
}

/** A value always between quotes, as free text is written. */
export function quoteString(value: string): string {
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

/** A field name as JQL: "Epic Link" is quoted, cf[10100] is not. */
export function quoteField(name: string): string {
  if (/^[\p{L}\p{N}_.[\]]+$/u.test(name) && !RESERVED.has(name.toLowerCase())) return name;
  return quote(name);
}

/** A function the server knows, as the table below lists it. */
export interface FunctionInfo {
  /** The usual spelling. */
  name: string;
  /** The kind of field it is a value of. */
  kind: FieldKind;
  /** Stands for a whole list: "sprint in openSprints()", not "sprint = openSprints()". */
  list: boolean;
  /** What a suggestion puts between the parentheses, for the caret to fill in. */
  args?: string;
}

/**
 * The functions the server knows, with their usual spelling: the one table
 * every other list of them comes from (the suggestions, what the filters
 * accept, the special values).
 */
export const FUNCTIONS: readonly FunctionInfo[] = [
  { name: 'currentUser', kind: 'user', list: false },
  { name: 'membersOf', kind: 'user', list: true, args: '""' },
  { name: 'now', kind: 'date', list: false },
  { name: 'startOfDay', kind: 'date', list: false },
  { name: 'endOfDay', kind: 'date', list: false },
  { name: 'startOfWeek', kind: 'date', list: false },
  { name: 'endOfWeek', kind: 'date', list: false },
  { name: 'startOfMonth', kind: 'date', list: false },
  { name: 'endOfMonth', kind: 'date', list: false },
  { name: 'startOfYear', kind: 'date', list: false },
  { name: 'endOfYear', kind: 'date', list: false },
  { name: 'unreleasedVersions', kind: 'version', list: true },
  { name: 'releasedVersions', kind: 'version', list: true },
  { name: 'earliestUnreleasedVersion', kind: 'version', list: false },
  { name: 'latestReleasedVersion', kind: 'version', list: false },
  { name: 'standardIssueTypes', kind: 'type', list: true },
  { name: 'subTaskIssueTypes', kind: 'type', list: true },
  { name: 'openSprints', kind: 'sprint', list: true },
  { name: 'futureSprints', kind: 'sprint', list: true },
  { name: 'closedSprints', kind: 'sprint', list: true },
];

const FUNCTION_NAMES = new Map(FUNCTIONS.map((f) => [f.name.toLowerCase(), f.name]));

/** A function's usual spelling ("CURRENTUSER" → "currentUser"). */
export function functionName(name: string): string {
  return FUNCTION_NAMES.get(name.toLowerCase()) ?? name;
}

/** A function as a suggestion offers it: "openSprints()", 'membersOf("")'. */
export function functionCall(f: FunctionInfo): string {
  return `${f.name}(${f.args ?? ''})`;
}

/** A value as JQL, in its canonical spelling. */
export function formatValue(v: Value): string {
  switch (v.kind) {
    case 'empty':
      return 'EMPTY';
    case 'func':
      return `${functionName(v.name)}(${v.args.map(quote).join(', ')})`;
    case 'text':
      return quote(v.text);
  }
}

function formatClause(c: Clause): string {
  const field = quoteField(c.field);
  const listOp = c.op === 'in' || c.op === 'not in' || c.op.endsWith(' in');
  // "in membersOf(x)": a function standing for the whole list keeps no parentheses.
  const values =
    listOp && !(c.values.length === 1 && c.values[0]?.kind === 'func')
      ? `(${c.values.map(formatValue).join(', ')})`
      : c.values.map(formatValue).join(', ');
  let out = c.op === 'changed' ? `${field} changed` : `${field} ${c.op} ${values}`;
  for (const p of c.preds) {
    const vals =
      p.kind === 'during' || (p.kind === 'by' && p.values.length > 1)
        ? `(${p.values.map(formatValue).join(', ')})`
        : p.values.map(formatValue).join(', ');
    out += ` ${p.kind.toUpperCase()} ${vals}`;
  }
  return out;
}

function formatNode(n: Node, parent: 'and' | 'or' | 'not' | null): string {
  switch (n.type) {
    case 'clause':
      return formatClause(n);
    case 'not':
      return `NOT ${formatNode(n.inner, 'not')}`;
    case 'and': {
      const s = `${formatNode(n.left, 'and')} AND ${formatNode(n.right, 'and')}`;
      return parent === 'not' ? `(${s})` : s;
    }
    case 'or': {
      const s = `${formatNode(n.left, 'or')} OR ${formatNode(n.right, 'or')}`;
      return parent === 'and' || parent === 'not' ? `(${s})` : s;
    }
  }
}

export function formatOrder(order: SortTerm[]): string {
  if (order.length === 0) return '';
  return `ORDER BY ${order.map((s) => `${quoteField(s.field)} ${s.desc ? 'DESC' : 'ASC'}`).join(', ')}`;
}

/** Writes a parsed query back as text. */
export function format(q: Query): string {
  return [q.where ? formatNode(q.where, null) : '', formatOrder(q.order)]
    .filter((s) => s !== '')
    .join(' ');
}

/** The clauses of a query made only of ANDs, in order; null when it has OR or NOT. */
export function conjunction(where: Node | null): Clause[] | null {
  if (!where) return [];
  switch (where.type) {
    case 'clause':
      return [where];
    case 'and': {
      const l = conjunction(where.left);
      const r = conjunction(where.right);
      return l && r ? [...l, ...r] : null;
    }
    default:
      return null;
  }
}

/* Telling JQL from the simple syntax ------------------------------------------ */

const LOOKS_LIKE =
  /(^|[\s(])(project|status|statuscategory|assignee|reporter|creator|type|issuetype|priority|resolution|labels?|component|fixversion|affectedversion|created|updated|resolved|due|duedate|text|summary|description|comment|key|issue|issuekey|parent|watcher|voter|commenter|participants?|involves|cf\[\d+\]|"[^"]+")\s*(=|!=|~|!~|>=?|<=?|\s+in\s*\(|\s+not\s+in\s|\s+is\s|\s+was\s|\s+changed\b)|\border\s+by\b/i;

/** Whether the server will read a search string as JQL (same rule as jql.LooksLike). */
export function looksLikeJql(s: string): boolean {
  return LOOKS_LIKE.test(s);
}

/* Fields ---------------------------------------------------------------------- */

export type FieldKind =
  | 'project'
  | 'type'
  | 'status'
  | 'category'
  | 'priority'
  | 'resolution'
  | 'user'
  | 'label'
  | 'component'
  | 'version'
  | 'sprint'
  | 'epic'
  | 'issue'
  | 'text'
  | 'date'
  | 'duration'
  | 'level'
  /** Custom fields: numbers, and values picked from a list. */
  | 'number'
  | 'option';

export interface FieldInfo {
  name: string;
  kind: FieldKind;
  /** WAS and CHANGED work on it (the change log records it). */
  history?: true;
  aliases?: string[];
}

/** The fields the server understands, as suggested in the editor. */
export const FIELDS: FieldInfo[] = [
  { name: 'project', kind: 'project' },
  { name: 'type', kind: 'type', aliases: ['issuetype'] },
  { name: 'status', kind: 'status', history: true },
  { name: 'statusCategory', kind: 'category' },
  { name: 'assignee', kind: 'user', history: true },
  { name: 'reporter', kind: 'user', history: true },
  { name: 'creator', kind: 'user' },
  { name: 'priority', kind: 'priority', history: true },
  { name: 'resolution', kind: 'resolution', history: true },
  { name: 'labels', kind: 'label', aliases: ['label'] },
  { name: 'component', kind: 'component' },
  { name: 'fixVersion', kind: 'version', history: true },
  { name: 'affectedVersion', kind: 'version' },
  { name: 'sprint', kind: 'sprint' },
  { name: 'Epic Link', kind: 'epic', aliases: ['epiclink'] },
  { name: 'parent', kind: 'issue' },
  { name: 'key', kind: 'issue', aliases: ['issue', 'issuekey', 'id'] },
  { name: 'text', kind: 'text' },
  { name: 'summary', kind: 'text' },
  { name: 'description', kind: 'text' },
  { name: 'comment', kind: 'text' },
  { name: 'environment', kind: 'text' },
  { name: 'created', kind: 'date', aliases: ['createddate'] },
  { name: 'updated', kind: 'date', aliases: ['updateddate'] },
  { name: 'resolved', kind: 'date', aliases: ['resolutiondate'] },
  { name: 'due', kind: 'date', aliases: ['duedate'] },
  { name: 'watcher', kind: 'user' },
  { name: 'voter', kind: 'user' },
  // Solstice extensions: anyone involved, and authors of public comments.
  { name: 'participant', kind: 'user', aliases: ['participants', 'involves'] },
  { name: 'commenter', kind: 'user' },
  { name: 'level', kind: 'level' },
  { name: 'timespent', kind: 'duration' },
  { name: 'originalEstimate', kind: 'duration', aliases: ['timeoriginalestimate'] },
  { name: 'remainingEstimate', kind: 'duration', aliases: ['timeestimate'] },
];

const FIELD_INDEX = new Map<string, FieldInfo>();
for (const f of FIELDS) {
  FIELD_INDEX.set(f.name.toLowerCase(), f);
  for (const a of f.aliases ?? []) FIELD_INDEX.set(a, f);
}

/** A field by any of its names, case-insensitive; undefined for custom fields. */
export function fieldInfo(name: string): FieldInfo | undefined {
  return FIELD_INDEX.get(name.toLowerCase());
}

/** The kind of a custom field, from its type (GET /api/fields); undefined when unknown. */
export function customKind(type: string): FieldKind | undefined {
  switch (type.toLowerCase()) {
    case 'float':
    case 'number':
      return 'number';
    case 'datepicker':
    case 'datetime':
      return 'date';
    case 'textfield':
    case 'textarea':
    case 'url':
      return 'text';
    case 'userpicker':
    case 'multiuserpicker':
      return 'user';
    case 'labels':
      return 'label';
    case 'select':
    case 'multiselect':
    case 'radiobuttons':
    case 'multicheckboxes':
    case 'cascadingselect':
      return 'option';
    case 'gh-epic-link':
      return 'epic';
    default:
      return undefined;
  }
}

/** The fields ORDER BY accepts. */
export const SORT_FIELDS = [
  'updated',
  'created',
  'priority',
  'key',
  'status',
  'due',
  'resolved',
  'summary',
  'assignee',
  'reporter',
  'project',
  'type',
  'resolution',
  'fixVersion',
  'component',
  'sprint',
  'rank',
] as const;

/** Operators, as inserted in the editor, for a kind of field. */
export function operatorsFor(field: string, kind = fieldInfo(field)?.kind): string[] {
  const info = fieldInfo(field);
  const listOps = ['=', '!=', 'in', 'not in', 'is EMPTY', 'is not EMPTY'];
  const history = info?.history ? ['was', 'was in', 'was not', 'was not in', 'changed'] : [];
  // Everyone takes part in something: no EMPTY for participant.
  if (info?.name === 'participant') return ['=', '!=', 'in', 'not in'];
  switch (kind) {
    case 'text':
      return ['~', '!~', 'is EMPTY', 'is not EMPTY'];
    case 'date':
    case 'duration':
    case 'number':
      return ['>=', '>', '<=', '<', '=', '!=', 'is EMPTY', 'is not EMPTY'];
    case 'issue':
      return info?.name === 'key' ? ['=', '!=', 'in', 'not in', '>', '>=', '<', '<='] : listOps;
    case 'category':
      return ['=', '!=', 'in', 'not in'];
    case undefined:
      return ['=', '!=', '~', 'in', 'not in', '>', '<', 'is EMPTY', 'is not EMPTY'];
    default:
      return [...listOps, ...history];
  }
}

/** Functions that make sense as a value of a kind of field. */
export function functionsFor(field: string, kind = fieldInfo(field)?.kind): string[] {
  return FUNCTIONS.filter((f) => f.kind === kind).map(functionCall);
}

/* Reading as one types: roles and completion ---------------------------------- */

export type Role =
  'field' | 'operator' | 'keyword' | 'value' | 'string' | 'function' | 'punct' | 'error';

export interface Styled extends Token {
  role: Role;
}

/** What is expected next. */
type State =
  | 'field' // a field, NOT or "("
  | 'op' // an operator
  | 'notIn' // IN after "field NOT"
  | 'is' // NOT or EMPTY after IS
  | 'isNot' // EMPTY after IS NOT
  | 'was' // NOT, IN or a value after WAS
  | 'value' // one value
  | 'list' // "(" or one value (a function) after IN
  | 'item' // a value inside a list
  | 'sep' // "," or ")" inside a list
  | 'after' // AND, OR, ")", ORDER BY (or history predicates)
  | 'by' // BY after ORDER
  | 'sort' // a field to sort by
  | 'dir' // ASC, DESC or ","
  | 'sortNext' // "," after a direction
  | 'args'; // function arguments, until ")"

interface Machine {
  state: State;
  field: string;
  /** The clause reads the change log: predicates may follow. */
  history: boolean;
  /** Where to go once the current value (or list) is read. */
  then: State;
}

const PREDICATES = ['after', 'before', 'on', 'during', 'by', 'from', 'to'];

function start(): Machine {
  return { state: 'field', field: '', history: false, then: 'after' };
}

/** Feeds one token; returns its role. next is the token after it, for function calls. */
function step(m: Machine, t: Token, next: Token | undefined): Role {
  if (t.kind === 'error') return 'error';
  const w = t.kind === 'word' ? t.text.toLowerCase() : '';
  /** Reads a value token; a function call goes on with its arguments. */
  const readValue = (after: State): Role => {
    m.state = after;
    if (t.kind === 'string') return 'string';
    if (w === 'empty' || w === 'null') return 'keyword';
    if (next?.kind !== 'lparen') return 'value';
    m.state = 'args';
    m.then = after;
    return 'function';
  };

  switch (m.state) {
    case 'field':
      if (w === 'not') return 'keyword';
      if (t.kind === 'lparen') return 'punct';
      if (w === 'order') {
        m.state = 'by';
        return 'keyword';
      }
      if (t.kind === 'word' || t.kind === 'string') {
        m.field = t.text;
        m.history = false;
        m.state = 'op';
        return 'field';
      }
      return 'error';
    case 'op':
      if (t.kind === 'op') {
        m.state = 'value';
        m.then = 'after';
        return 'operator';
      }
      if (w === 'in') {
        m.state = 'list';
        m.then = 'after';
        return 'operator';
      }
      if (w === 'not') {
        m.state = 'notIn';
        return 'operator';
      }
      if (w === 'is') {
        m.state = 'is';
        return 'operator';
      }
      if (w === 'was') {
        m.history = true;
        m.state = 'was';
        return 'operator';
      }
      if (w === 'changed') {
        m.history = true;
        m.state = 'after';
        return 'operator';
      }
      return 'error';
    case 'notIn':
      if (w === 'in') {
        m.state = 'list';
        m.then = 'after';
        return 'operator';
      }
      return 'error';
    case 'is':
      if (w === 'not') {
        m.state = 'isNot';
        return 'operator';
      }
      if (w === 'empty' || w === 'null') {
        m.state = 'after';
        return 'keyword';
      }
      return 'error';
    case 'isNot':
      if (w === 'empty' || w === 'null') {
        m.state = 'after';
        return 'keyword';
      }
      return 'error';
    case 'was':
      if (w === 'not') return 'operator';
      if (w === 'in') {
        m.state = 'list';
        m.then = 'after';
        return 'operator';
      }
      if (t.kind === 'word' || t.kind === 'string') return readValue('after');
      return 'error';
    case 'value':
      if (t.kind === 'word' || t.kind === 'string') return readValue(m.then);
      return 'error';
    case 'list':
      if (t.kind === 'lparen') {
        m.state = 'item';
        return 'punct';
      }
      if (t.kind === 'word' || t.kind === 'string') return readValue(m.then);
      return 'error';
    case 'item':
      if (t.kind === 'word' || t.kind === 'string') return readValue('sep');
      if (t.kind === 'rparen') {
        m.state = 'after';
        return 'punct';
      }
      return 'error';
    case 'sep':
      if (t.kind === 'comma') {
        m.state = 'item';
        return 'punct';
      }
      if (t.kind === 'rparen') {
        m.state = 'after';
        return 'punct';
      }
      return 'error';
    case 'args':
      // The call's "(", its arguments and commas; its ")" ends it.
      if (t.kind === 'rparen') {
        m.state = m.then;
        return 'punct';
      }
      if (t.kind === 'lparen' || t.kind === 'comma') return 'punct';
      if (t.kind === 'string') return 'string';
      return 'value';
    case 'after':
      if (w === 'and' || w === 'or') {
        m.state = 'field';
        return 'keyword';
      }
      if (t.kind === 'rparen') return 'punct';
      if (w === 'order') {
        m.state = 'by';
        return 'keyword';
      }
      if (m.history && PREDICATES.includes(w)) {
        m.state = w === 'during' || w === 'by' ? 'list' : 'value';
        m.then = 'after';
        return 'keyword';
      }
      return 'error';
    case 'by':
      if (w === 'by') {
        m.state = 'sort';
        return 'keyword';
      }
      return 'error';
    case 'sort':
      if (t.kind === 'word' || t.kind === 'string') {
        m.state = 'dir';
        return 'field';
      }
      return 'error';
    case 'dir':
      if (w === 'asc' || w === 'desc') {
        m.state = 'sortNext';
        return 'keyword';
      }
      if (t.kind === 'comma') {
        m.state = 'sort';
        return 'punct';
      }
      return 'error';
    case 'sortNext':
      if (t.kind === 'comma') {
        m.state = 'sort';
        return 'punct';
      }
      return 'error';
  }
}

/** Each token with its role, for colouring. */
export function highlight(src: string): Styled[] {
  const toks = lex(src);
  const m = start();
  return toks.map((t, i) => ({ ...t, role: step(m, t, toks[i + 1]) }));
}

export type CompletionKind = 'field' | 'operator' | 'value' | 'keyword' | 'sort' | 'none';

export interface Completion {
  kind: CompletionKind;
  /** The field whose operator or value is being typed. */
  field: string;
  /** Keywords that fit here (AND, OR, ORDER BY, EMPTY, ASC…). */
  keywords: string[];
  /** What is already typed of the current word, without its opening quote. */
  prefix: string;
  /** The range a suggestion replaces. */
  from: number;
  to: number;
  /** Inside "in (…)": a value is followed by "," or ")", not a space. */
  inList: boolean;
  /**
   * Words typed after a bare value may continue it ("status = En c" for
   * « En cours »): the value so far and where it starts. null otherwise.
   */
  phrase: { prefix: string; from: number; inList: boolean } | null;
}

/**
 * The word being typed: a word, string or operator that touches the caret,
 * and how many tokens sit before it.
 */
function locateWord(toks: Token[], caret: number): { cur: Token | undefined; upto: number } {
  let cur: Token | undefined;
  let upto = toks.length;
  for (let i = 0; i < toks.length; i++) {
    const t = toks[i] as Token;
    if (t.start >= caret) {
      upto = i;
      break;
    }
    if (
      t.end >= caret &&
      (t.kind === 'word' || t.kind === 'string' || t.kind === 'op' || t.kind === 'error')
    ) {
      cur = t;
      upto = i;
      break;
    }
    upto = i + 1;
  }
  return { cur, upto };
}

/** Feeds the tokens before the caret; returns the role of the last one. */
function replay(m: Machine, toks: Token[], upto: number): Role | undefined {
  let lastRole: Role | undefined;
  for (let i = 0; i < upto; i++) lastRole = step(m, toks[i] as Token, toks[i + 1]);
  return lastRole;
}

/**
 * The bare value the caret is in, when the last token is a value that may go
 * on over several words ("status = En c" for « En cours »); null otherwise.
 */
function phraseAt(
  src: string,
  caret: number,
  last: Token | undefined,
  lastRole: Role | undefined,
  m: Machine,
  cur: Token | undefined,
): Completion['phrase'] {
  if (
    last?.kind === 'word' &&
    lastRole === 'value' &&
    (m.state === 'sep' || m.state === 'after') &&
    (!cur || cur.kind === 'word') &&
    caret > last.end
  ) {
    return { prefix: src.slice(last.start, caret), from: last.start, inList: m.state === 'sep' };
  }
  return null;
}

/** What may be typed in the state the machine ended in. */
function kindOf(m: Machine, base: Omit<Completion, 'kind' | 'keywords'>): Completion {
  switch (m.state) {
    case 'field':
      return { ...base, kind: 'field', keywords: ['NOT', 'ORDER BY'] };
    case 'op':
      return { ...base, kind: 'operator', keywords: [] };
    case 'notIn':
      return { ...base, kind: 'keyword', keywords: ['IN'] };
    case 'is':
      return { ...base, kind: 'keyword', keywords: ['EMPTY', 'NOT EMPTY'] };
    case 'isNot':
      return { ...base, kind: 'keyword', keywords: ['EMPTY'] };
    case 'was':
    case 'value':
      return { ...base, kind: 'value', keywords: ['EMPTY'] };
    case 'list':
      return { ...base, kind: 'value', keywords: ['('] };
    case 'item':
      return { ...base, kind: 'value', keywords: ['EMPTY'], inList: true };
    case 'after': {
      const kw = ['AND', 'OR', 'ORDER BY'];
      if (m.history) kw.push('AFTER', 'BEFORE', 'ON', 'DURING', 'BY');
      return { ...base, kind: 'keyword', keywords: kw };
    }
    case 'by':
      return { ...base, kind: 'keyword', keywords: ['BY'] };
    case 'sort':
      return { ...base, kind: 'sort', keywords: [] };
    case 'dir':
      return { ...base, kind: 'keyword', keywords: ['ASC', 'DESC'] };
    default:
      return { ...base, kind: 'none', keywords: [] };
  }
}

/** What may be typed at the caret, and the text a suggestion would replace. */
export function completionAt(src: string, caret: number): Completion {
  const toks = lex(src);
  const { cur, upto } = locateWord(toks, caret);
  const m = start();
  const lastRole = replay(m, toks, upto);
  const last = upto > 0 ? toks[upto - 1] : undefined;
  const phrase = phraseAt(src, caret, last, lastRole, m, cur);
  const from = cur ? cur.start : caret;
  const to = cur ? cur.end : caret;
  let prefix = '';
  if (cur?.kind === 'string') {
    prefix = src.slice(cur.start + 1, Math.min(caret, cur.closed ? cur.end - 1 : cur.end));
  } else if (cur) {
    prefix = src.slice(cur.start, caret);
  }
  return kindOf(m, { field: m.field, prefix, from, to, inList: false, phrase });
}
