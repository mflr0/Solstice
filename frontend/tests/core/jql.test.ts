import { describe, expect, it } from 'vitest';

import {
  charIndex,
  completionAt,
  conjunction,
  customKind,
  functionsFor,
  format,
  formatValue,
  highlight,
  JqlError,
  type JqlCode,
  lex,
  looksLikeJql,
  operatorsFor,
  parse,
  quote,
  quoteField,
} from '../../src/core/jql.js';
import { renderJqlError } from '../../src/i18n/t.js';

describe('lex', () => {
  it('splits words, strings, operators and punctuation with positions', () => {
    const toks = lex('status in ("En cours", Open) ORDER BY key');
    expect(toks.map((t) => [t.kind, t.text])).toEqual([
      ['word', 'status'],
      ['word', 'in'],
      ['lparen', '('],
      ['string', 'En cours'],
      ['comma', ','],
      ['word', 'Open'],
      ['rparen', ')'],
      ['word', 'ORDER'],
      ['word', 'BY'],
      ['word', 'key'],
    ]);
    expect(toks[3]).toMatchObject({ start: 11, end: 21, raw: '"En cours"' });
  });

  it('reads two-character operators', () => {
    expect(
      lex('a!=b c!~d e>=f g<=h i~j')
        .filter((t) => t.kind === 'op')
        .map((t) => t.text),
    ).toEqual(['!=', '!~', '>=', '<=', '~']);
  });

  it('unescapes strings and keeps cf[…] brackets in words', () => {
    expect(lex(String.raw`summary ~ "a \"b\" \\ c"`)[2]?.text).toBe(String.raw`a "b" \ c`);
    expect(lex("cf[10100] = 'x'").map((t) => t.text)).toEqual(['cf[10100]', '=', 'x']);
  });

  it('marks unterminated strings and a lone "!"', () => {
    expect(lex('summary ~ "abc')[2]).toMatchObject({ kind: 'string', closed: false, text: 'abc' });
    expect(lex('a ! b')[1]?.kind).toBe('error');
  });
});

describe('parse', () => {
  it('builds clauses joined by AND and OR, with precedence', () => {
    const q = parse('a = 1 OR b = 2 AND NOT c = 3');
    expect(q.where?.type).toBe('or');
    expect(format(q)).toBe('a = 1 OR b = 2 AND NOT c = 3');
  });

  it('reads lists, functions, EMPTY and ORDER BY', () => {
    const q = parse(
      'assignee in (currentUser(), EMPTY, "jean dupont") AND sprint in openSprints() order by priority desc, key',
    );
    const clauses = conjunction(q.where);
    expect(clauses?.map((c) => [c.field, c.op, c.values.map(formatValue)])).toEqual([
      ['assignee', 'in', ['currentUser()', 'EMPTY', '"jean dupont"']],
      ['sprint', 'in', ['openSprints()']],
    ]);
    expect(q.order).toEqual([
      { field: 'priority', desc: true },
      { field: 'key', desc: false },
    ]);
  });

  it('reads IS, IS NOT, NOT IN, history clauses', () => {
    expect(format(parse('due is EMPTY AND labels is not null'))).toBe(
      'due is EMPTY AND labels is not EMPTY',
    );
    expect(format(parse('project not in (A,B)'))).toBe('project not in (A, B)');
    expect(format(parse('status was "In Progress" BY currentUser() AFTER -7d'))).toBe(
      'status was "In Progress" BY currentUser() AFTER -7d',
    );
    expect(format(parse('status changed FROM Open TO Done DURING (-2w, now())'))).toBe(
      'status changed FROM Open TO Done DURING (-2w, now())',
    );
    expect(format(parse('status was in (Open, Done)'))).toBe('status was in (Open, Done)');
  });

  it('keeps function arguments', () => {
    expect(format(parse('created >= startOfDay(-1d) AND assignee in membersOf("dév team")'))).toBe(
      'created >= startOfDay(-1d) AND assignee in membersOf("dév team")',
    );
  });

  it('reports errors with a code, arguments and a position', () => {
    const cases: [string, JqlCode, string[], number][] = [
      ['status', 'operatorExpected', ['status'], 6],
      ['status = ', 'valueExpected', [], 9],
      ['status in (Open', 'parenExpected', [], 15],
      ['summary ~ "abc', 'unclosedQuote', [], 10],
      ['status is Open', 'isEmpty', [], 14],
      ['a = 1 ORDER key', 'orderBy', [], 12],
      ['a = 1 b = 2', 'unexpected', ['b'], 6],
      ['a = 1 "b c"', 'unexpected', ['b c'], 6],
      ['a = 1 AND', 'fieldExpected', [], 9],
      ['status = Open FROM x', 'unexpected', ['FROM'], 14],
      ['status was Open from x', 'changedOnly', ['FROM'], 21],
    ];
    for (const [src, code, args, pos] of cases) {
      let err: unknown;
      try {
        parse(src);
      } catch (e) {
        err = e;
      }
      expect(err, src).toBeInstanceOf(JqlError);
      const e = err as JqlError;
      expect([e.code, e.args, e.pos], src).toEqual([code, args, pos]);
    }
  });

  it('renders an error in the language in use', () => {
    const at = (src: string) => {
      try {
        parse(src);
      } catch (e) {
        return renderJqlError(e as JqlError);
      }
      throw new Error(`${src} parsed`);
    };
    expect(at('status')).toBe('Operator expected after “status”.');
    expect(at('status in (Open')).toBe('Closing parenthesis expected.');
  });

  it('accepts an empty query and a query that only sorts', () => {
    expect(parse('')).toEqual({ where: null, order: [] });
    expect(parse('ORDER BY created DESC').order).toEqual([{ field: 'created', desc: true }]);
  });

  it('conjunction refuses OR and NOT', () => {
    expect(conjunction(parse('a = 1 AND (b = 2 AND c = 3)').where)?.length).toBe(3);
    expect(conjunction(parse('a = 1 AND (b = 2 OR c = 3)').where)).toBeNull();
    expect(conjunction(parse('NOT a = 1').where)).toBeNull();
  });
});

describe('charIndex', () => {
  it('turns a character offset into a string index', () => {
    expect(charIndex('été = x', 4)).toBe(4);
    expect(charIndex('labels = "🎉" AND foo = 1', 18)).toBe(19);
    expect(charIndex('abc', 9)).toBe(3);
  });
});

describe('quote', () => {
  it('leaves plain words bare and quotes the rest', () => {
    expect(quote('DEMO')).toBe('DEMO');
    expect(quote('2.4')).toBe('2.4');
    expect(quote('-7d')).toBe('-7d');
    expect(quote('Épopée')).toBe('Épopée');
    expect(quote('dette-technique')).toBe('dette-technique');
    expect(quote('En cours')).toBe('"En cours"');
    expect(quote("Won't Fix")).toBe('"Won\'t Fix"');
    expect(quote('a "b"')).toBe('"a \\"b\\""');
    expect(quote('back\\slash')).toBe('"back\\\\slash"');
    expect(quote('')).toBe('""');
  });

  it('quotes reserved words so they stay values', () => {
    for (const w of ['EMPTY', 'null', 'and', 'Or', 'in', 'order']) {
      expect(quote(w)).toBe(`"${w}"`);
    }
  });

  it('round-trips through the parser', () => {
    for (const v of ['En cours', 'a "b" c', 'x\\y', 'EMPTY', "l'été", 'a,b', '(x)']) {
      const c = conjunction(parse(`labels = ${quote(v)}`).where)?.[0];
      expect(c?.values[0]).toEqual({ kind: 'text', text: v });
    }
  });

  it('quotes field names with spaces but not cf[…]', () => {
    expect(quoteField('Epic Link')).toBe('"Epic Link"');
    expect(quoteField('cf[10100]')).toBe('cf[10100]');
    expect(quoteField('fixVersion')).toBe('fixVersion');
  });
});

describe('looksLikeJql', () => {
  it('follows the server rule', () => {
    expect(looksLikeJql('project = DEMO')).toBe(true);
    expect(looksLikeJql('status in (Open)')).toBe(true);
    expect(looksLikeJql('"Epic Link" = DEMO-1')).toBe(true);
    expect(looksLikeJql('ORDER BY created')).toBe(true);
    expect(looksLikeJql('participant = paul')).toBe(true);
    expect(looksLikeJql('commenter in (paul)')).toBe(true);
    expect(looksLikeJql('@moi est:ouvert')).toBe(false);
    expect(looksLikeJql('panier vide')).toBe(false);
    expect(looksLikeJql('projet:DEMO version:"2.4"')).toBe(false);
  });

  it('does not read U+0085 as a space', () => {
    expect(looksLikeJql('project\u0085= DEMO')).toBe(false);
    expect(looksLikeJql('\u0085project = DEMO')).toBe(false);
    expect(looksLikeJql('ORDER\u0085BY created')).toBe(false);
  });
});

describe('highlight', () => {
  const roles = (src: string) => highlight(src).map((t) => `${t.raw}:${t.role}`);

  it('tells fields, operators, values, functions and keywords apart', () => {
    expect(
      roles('project = DEMO AND assignee in (currentUser(), "Jean") ORDER BY key DESC'),
    ).toEqual([
      'project:field',
      '=:operator',
      'DEMO:value',
      'AND:keyword',
      'assignee:field',
      'in:operator',
      '(:punct',
      'currentUser:function',
      '(:punct',
      '):punct',
      ',:punct',
      '"Jean":string',
      '):punct',
      'ORDER:keyword',
      'BY:keyword',
      'key:field',
      'DESC:keyword',
    ]);
  });

  it('colours quoted field names, IS EMPTY and stray tokens', () => {
    expect(roles('"Epic Link" is not EMPTY')).toEqual([
      '"Epic Link":field',
      'is:operator',
      'not:operator',
      'EMPTY:keyword',
    ]);
    expect(roles('a = 1 b')).toEqual(['a:field', '=:operator', '1:value', 'b:error']);
    expect(roles('a ! b')[1]).toBe('!:error');
  });
});

describe('completionAt', () => {
  const at = (src: string, caret = src.length) => completionAt(src, caret);

  it('offers fields at the start and after AND', () => {
    expect(at('')).toMatchObject({ kind: 'field', prefix: '', from: 0, to: 0 });
    expect(at('project = DEMO AND sta')).toMatchObject({
      kind: 'field',
      prefix: 'sta',
      from: 19,
      to: 22,
    });
    expect(at('(st')).toMatchObject({ kind: 'field', prefix: 'st' });
  });

  it('offers operators after a field', () => {
    expect(at('status ')).toMatchObject({ kind: 'operator', field: 'status', prefix: '' });
    expect(at('status !')).toMatchObject({ kind: 'operator', prefix: '!' });
    expect(at('"Epic Link" ')).toMatchObject({ kind: 'operator', field: 'Epic Link' });
  });

  it('offers values for the field, in lists too', () => {
    expect(at('status = ')).toMatchObject({ kind: 'value', field: 'status', inList: false });
    expect(at('status = In')).toMatchObject({ kind: 'value', prefix: 'In', from: 9 });
    expect(at('status = "In Pr')).toMatchObject({
      kind: 'value',
      prefix: 'In Pr',
      from: 9,
      to: 15,
    });
    expect(at('assignee in (currentUser(), ma')).toMatchObject({
      kind: 'value',
      field: 'assignee',
      inList: true,
      prefix: 'ma',
    });
    expect(at('sprint in ')).toMatchObject({ kind: 'value', field: 'sprint', keywords: ['('] });
  });

  it('offers keywords after a clause, after IS and in ORDER BY', () => {
    expect(at('status = Open ')).toMatchObject({
      kind: 'keyword',
      keywords: ['AND', 'OR', 'ORDER BY'],
    });
    expect(at('status = Open A')).toMatchObject({ kind: 'keyword', prefix: 'A' });
    expect(at('due is ')).toMatchObject({ kind: 'keyword', keywords: ['EMPTY', 'NOT EMPTY'] });
    expect(at('a = 1 ORDER ')).toMatchObject({ kind: 'keyword', keywords: ['BY'] });
    expect(at('a = 1 ORDER BY pri')).toMatchObject({ kind: 'sort', prefix: 'pri' });
    expect(at('a = 1 ORDER BY priority ')).toMatchObject({
      kind: 'keyword',
      keywords: ['ASC', 'DESC'],
    });
    expect(at('status was Open ').keywords).toContain('BEFORE');
  });

  it('lets a bare value go on over several words', () => {
    expect(at('status in (In P').phrase).toEqual({ prefix: 'In P', from: 11, inList: true });
    expect(at('status = En ').phrase).toEqual({ prefix: 'En ', from: 9, inList: false });
    expect(at('status = "En" c').phrase).toBeNull();
    expect(at('status = Open').phrase).toBeNull();
    expect(at('status = currentUser() x').phrase).toBeNull();
  });

  it('reads only what is before the caret', () => {
    expect(at('status = Open AND type = Bug', 7)).toMatchObject({
      kind: 'operator',
      field: 'status',
    });
    expect(at('status = Open AND type = Bug', 3)).toMatchObject({
      kind: 'field',
      prefix: 'sta',
      to: 6,
    });
  });
});

describe('operatorsFor', () => {
  it('depends on the kind of field', () => {
    expect(operatorsFor('summary')).toEqual(['~', '!~', 'is EMPTY', 'is not EMPTY']);
    expect(operatorsFor('created')).toContain('>=');
    expect(operatorsFor('status')).toContain('was');
    expect(operatorsFor('labels')).not.toContain('was');
    expect(operatorsFor('issuetype')).toContain('in');
    expect(operatorsFor('participant')).toEqual(['=', '!=', 'in', 'not in']);
    expect(operatorsFor('involves')).toEqual(['=', '!=', 'in', 'not in']);
    expect(operatorsFor('commenter')).toContain('is EMPTY');
    expect(functionsFor('commenter')).toContain('currentUser()');
  });

  it('takes the kind of a custom field', () => {
    expect(operatorsFor('Story Points', customKind('float'))).toEqual([
      '>=',
      '>',
      '<=',
      '<',
      '=',
      '!=',
      'is EMPTY',
      'is not EMPTY',
    ]);
    expect(operatorsFor('Équipes concernées', customKind('multiselect'))).toEqual([
      '=',
      '!=',
      'in',
      'not in',
      'is EMPTY',
      'is not EMPTY',
    ]);
    expect(functionsFor('Référent métier', customKind('userpicker'))).toContain('currentUser()');
    expect(customKind('datepicker')).toBe('date');
    expect(customKind('something-else')).toBeUndefined();
  });
});
