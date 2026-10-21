import { describe, expect, it } from 'vitest';
import fc from 'fast-check';
import { Fragment, Slice, type Node as PMNode } from 'prosemirror-model';

import { renderWiki } from '../../src/core/wiki.js';
import {
  codeLanguage,
  parseWiki,
  parseWikiBlocks,
  schema,
  serializeWiki,
  withCodeLanguage,
} from '../../src/core/wiki-doc.js';
import { ALL_EXIST, flattenInline, NONE_EXIST, roundTrip } from '../../src/core/wiki-roundtrip.js';

/** Canonical markup: as if every block had been edited. */
const canon = (src: string): string => serializeWiki(parseWiki(src).doc);

const sameMeaning = (a: string, b: string): boolean =>
  [ALL_EXIST, NONE_EXIST].every(
    (r) => flattenInline(renderWiki(a, r)) === flattenInline(renderWiki(b, r)),
  );

/* ---------- building documents by hand ---------- */

const n = schema.nodes;
const m = schema.marks;
const text = (s: string, ...marks: string[]): PMNode =>
  schema.text(
    s,
    marks.map((k) => schema.marks[k]!.create()),
  );
const link = (s: string, href: string): PMNode => schema.text(s, [m.link.create({ href })]);
const p = (...content: PMNode[]): PMNode => n.paragraph.create(null, content);
const doc = (...blocks: PMNode[]): PMNode => n.doc.create(null, blocks);
const write = (...blocks: PMNode[]): string => serializeWiki(doc(...blocks));
const replaceBlock = (d: PMNode, from: number, to: number, block: PMNode): PMNode =>
  d.replace(from, to, new Slice(Fragment.from(block), 0, 0));

/* ---------- corpus ---------- */

/** Written back exactly as they are. */
const IDENTICAL = [
  'Un texte simple.',
  'a\nb\n\nc',
  'h1. Titre',
  'h3. *Gras* titre',
  'h6. Petit',
  'h1. A\n\nh2. B\n\nh3. C\n\nh4. D\n\nh5. E\n\nh6. F',
  '*gras* _italique_ -barré- +souligné+ ^exposant^ ~indice~ ??source??',
  '*_mot_*',
  'un *mot _très_ fort* ici',
  '(*mot*).',
  'snake_case_name, 2*3*4, a-b-c, file-name.txt, C++ et C++, 5 - 3 - 1',
  '{{a_b_c *x*}} et {{<b>}}',
  'a\\\\b',
  'C:\\Users\\x',
  '* un\n* deux',
  '# un\n# deux',
  '- un\n- deux',
  '* a\n** b\n* c',
  '# a\n#* b\n# c',
  '* a\n** b\n*** c\n* d',
  '*** a',
  '* a\n*# b\n** c',
  '* *gras* et [lien|https://example.com]',
  '||h1||h2||\n|a|b|',
  '|a|b|\n|c|d|',
  '||en-tête|cellule|',
  '|[x|http://y.fr]|[~marie.durand]|',
  '{code:java}\nint x = 1 < 2; // *x*\n{code}',
  '{code}\nplain\n{code}',
  '{code:title=Foo.java|borderStyle=solid}\nx\n{code}',
  '{noformat}\n*brut* <b>\n{noformat}',
  '{quote}\n*important*\n{quote}',
  '{quote}\na\n\n{code}\nb\n{code}\n{quote}',
  'bq. Une _citation_',
  'avant\n\n----\n\naprès',
  '[Solstice|https://example.com]',
  '[écrire|mailto:a@b.fr]',
  '[mailto:a@b.fr]',
  '[accueil|/projects]',
  '[#haut]',
  '[*gras*|https://x.fr]',
  '[voir DEMO-1|https://x.fr]',
  '[x|javascript:alert(1)]',
  '[une remarque] et [lien',
  'voir https://example.com/a_b_c.',
  '(https://example.com/x)',
  'https://fr.wikipedia.org/wiki/Rouille_(chimie)',
  '[~marie.durand] et [~inconnu]',
  '!capture.png!',
  '!capture.png|thumbnail!',
  '!capture.png|width=300!',
  '!schéma réseau.png!',
  '!http://example.com/x.png!',
  '[^spec.pdf] et [le cahier|^spec.pdf]',
  'Attention! Danger!',
  'voir DEMO-12. et [DEMO-1] et [le ticket|DEMO-1]',
  'demo-1 xDEMO-1 DEMO-12a',
  '(/) fait (x) non (!)(?)(i)(y)(n)',
  '{color:red}rouge{color} et {color:#FF0000}x{color}',
  '{panel:title=Note|borderStyle=dashed}\ntexte\n{panel}',
  'a{anchor:haut}b',
  '{toc}',
  '{foo} et {bar:x=1}',
  '« Guillemets », l’apostrophe — tiret … points',
  'Émoji 🎉 et 中文',
  'h1.5 est une version',
  '*gras',
  'un _début sans fin',
  '{{code',
  '{color:red}texte',
]; // prettier-ignore

/** Written back differently, with the same rendering. */
const EQUIVALENT = [
  'a\n\n\nb',
  '  * indenté',
  'h1.   Espaces',
  '* a\n# b',
  'texte {code}x{code} suite',
  '{color:red}rouge{color} et {panel:title=x}panneau{panel}',
  '| a | b |\n| c | d |',
  '|a|b|\n|c|',
  '{code}x{code}',
  'a\r\nb\r\n\r\nc',
  '[ voir |http://x.fr]',
  '[http://x.fr|http://x.fr]',
  '\\!pas une image\\!',
  '_*mot*_',
  '\\*pas gras\\*',
  '[https://example.com/a?b=1&c=2]',
  '{code}sans fin',
  '\\{code}x{code}',
]; // prettier-ignore

describe('round trip of imported markup', () => {
  it.each(IDENTICAL)('writes %j back as it was', (src) => {
    expect(canon(src)).toBe(src);
  });

  it.each(EQUIVALENT)('writes %j back with the same meaning', (src) => {
    const out = canon(src);
    expect(sameMeaning(src, out)).toBe(true);
    // And a second pass changes nothing.
    expect(canon(out)).toBe(out);
  });

  it.each([...IDENTICAL, ...EQUIVALENT])('classifies %j as safe', (src) => {
    expect(roundTrip(src).verdict).not.toBe('different');
  });

  it('gives back the exact source of an unchanged document', () => {
    for (const src of ['a\r\nb\r\n', '  h1.  x  \n\n\n', '{panel}x{panel}', '']) {
      const parsed = parseWiki(src);
      expect(serializeWiki(parsed.doc, parsed)).toBe(src);
    }
  });

  it('keeps untouched blocks as written when another one changes', () => {
    const src = 'h1.   Titre  \n\n|| a || b ||\n| 1 | 2 |\n\n*  une\n*  liste\n\nUn paragraphe.';
    const parsed = parseWiki(src);
    const last = parsed.doc.lastChild!;
    const end = parsed.doc.content.size;
    const edited = replaceBlock(
      parsed.doc,
      end - last.nodeSize,
      end,
      p(text('Un paragraphe modifié.')),
    );
    expect(serializeWiki(edited, parsed)).toBe(
      'h1.   Titre  \n\n|| a || b ||\n| 1 | 2 |\n\n*  une\n*  liste\n\nUn paragraphe modifié.',
    );
  });

  it('keeps the source of blocks rebuilt without change', () => {
    const src = 'h1.   Titre  \n|| a ||\n\nTexte \\*brut\\*';
    const parsed = parseWiki(src);
    // Splitting or joining blocks gives new node objects with the same content.
    const rebuilt = schema.nodeFromJSON(parsed.doc.toJSON());
    const end = rebuilt.content.size;
    const edited = replaceBlock(rebuilt, end, end, p(text('fin')));
    expect(serializeWiki(edited, parsed)).toBe(`${src}\n\nfin`);
  });

  it('does not reuse the source of a block that leaves a macro open', () => {
    const parsed = parseWiki('voir {code} ici\n\nfin');
    const end = parsed.doc.content.size;
    const code = n.code_block.create(null, schema.text('x'));
    const edited = replaceBlock(parsed.doc, end - parsed.doc.lastChild!.nodeSize, end, code);
    expect(serializeWiki(edited, parsed)).toBe('voir \\{code} ici\n\n{code}\nx\n{code}');
  });
});

describe('what the editor understands', () => {
  const types = (src: string): string[] => {
    const found: string[] = [];
    parseWiki(src).doc.descendants((node) => {
      found.push(node.type.name === 'raw_inline' || node.type.name === 'raw_block'
        ? `${node.type.name}:${node.attrs.kind as string}`
        : node.type.name);
    });
    return found;
  }; // prettier-ignore

  it('models the common markup as editable content', () => {
    expect(types('h2. T')).toEqual(['heading', 'text']);
    expect(types('* a\n** b')).toEqual([
      'bullet_list', 'list_item', 'paragraph', 'text',
      'bullet_list', 'list_item', 'paragraph', 'text',
    ]); // prettier-ignore
    expect(types('||h|\n|c|')).toEqual([
      'table', 'table_row', 'table_header', 'paragraph', 'text',
      'table_row', 'table_cell', 'paragraph', 'text',
    ]); // prettier-ignore
    expect(types('[~bob]')).toEqual(['paragraph', 'mention']);
    expect(types('{code:sql}\nSELECT 1\n{code}')).toEqual(['code_block', 'text']);
    expect(types('----')).toEqual(['horizontal_rule']);
  });

  it('keeps what it cannot edit as raw markup', () => {
    expect(types('{panel}x{panel}')).toEqual(['raw_block:panel']);
    expect(types('|a|b|\n|c|')).toEqual(['raw_block:table']);
    expect(types('{color:red}x{color}')).toEqual(['paragraph', 'raw_inline:color']);
    expect(types('(x)')).toEqual(['paragraph', 'raw_inline:emoticon']);
    expect(types('!a.png!')).toEqual(['paragraph', 'raw_inline:image']);
    expect(types('[^a.pdf]')).toEqual(['paragraph', 'raw_inline:file']);
    expect(types('[DEMO-1]')).toEqual(['paragraph', 'raw_inline:issue']);
    expect(types('{info}')).toEqual(['paragraph', 'raw_inline:macro']);
  });

  it('reads the language of a code block', () => {
    expect(codeLanguage('java')).toBe('java');
    expect(codeLanguage('title=A.java|language=java')).toBe('java');
    expect(codeLanguage('lang=sql')).toBe('sql');
    expect(codeLanguage(null)).toBe('');
    expect(withCodeLanguage(null, 'go')).toBe('go');
    expect(withCodeLanguage('java', 'go')).toBe('go');
    expect(withCodeLanguage('title=A|language=java', 'go')).toBe('title=A|language=go');
    expect(withCodeLanguage('java', '')).toBeNull();
    expect(withCodeLanguage('title=A', 'go')).toBe('go|title=A');
  });

  it('parses pasted markup as blocks', () => {
    expect(parseWikiBlocks('* a\n* b').firstChild?.type.name).toBe('bullet_list');
  });
});

describe('writing what was typed', () => {
  it('writes formatting', () => {
    expect(write(p(text('un '), text('mot', 'strong'), text(' fort')))).toBe('un *mot* fort');
    expect(write(p(text('a', 'em', 'strong')))).toBe('*_a_*');
    expect(write(p(text('x', 'underline'), text(' '), text('y', 'strike')))).toBe('+x+ -y-');
    expect(write(p(text('m'), text('2', 'sup'), text(' ou '), text('2', 'sup')))).toBe('m2 ou ^2^');
    expect(write(p(text('code', 'code')))).toBe('{{code}}');
  });

  it('moves spaces out of formatting', () => {
    expect(write(p(text('un'), text(' mot ', 'strong'), text('ici')))).toBe('un *mot* ici');
  });

  it('drops formatting the markup cannot hold inside a word', () => {
    expect(write(p(text('un'), text('mot', 'strong'), text('deux')))).toBe('unmotdeux');
  });

  it('escapes typed characters that would be read as markup', () => {
    for (const typed of [
      '*pas gras*', '_pas italique_', '[pas un lien|/x]', '[~pas.une.mention]',
      '!pas.png!', '{{pas mono}}', '(x) pas un smiley', '{color:red}x{color}', '{code}',
      '{info}', 'a -b- c', '??pas une source??', 'h1. pas un titre', '* pas une liste',
      '# pas une liste', '- pas une liste', '----', '|pas un tableau|', 'bq. pas une citation',
      
    ]) {
      const out = write(p(text(typed)));
      expect(renderWiki(out, ALL_EXIST), `${typed} → ${out}`).toBe(
        `<p>${typed.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}</p>`,
      );
    }
  }); // prettier-ignore

  it('escapes as little as possible', () => {
    expect(write(p(text('*pas gras*')))).toBe('\\*pas gras*');
    expect(write(p(text('h1. pas un titre')))).toBe('h1\\. pas un titre');
    expect(write(p(text('C++, a*b, snake_case, 5 - 3')))).toBe('C++, a*b, snake_case, 5 - 3');
  });

  it('writes line breaks', () => {
    const br = n.hard_break.create();
    expect(write(p(text('a'), br, text('b')))).toBe('a\nb');
    expect(write(p(text('a'), br, br, text('b')))).toBe('a\\\\\nb');
    expect(write(p(text('a'), br, text('* b')))).toBe('a\n\\* b');
    expect(write(n.heading.create({ level: 2 }, [text('a'), br, text('b')]))).toBe('h2. a\\\\b');
  });

  it('writes links, mentions and attachments', () => {
    expect(write(p(link('Solstice', 'https://x.fr')))).toBe('[Solstice|https://x.fr]');
    expect(write(p(text('voir '), link('https://x.fr', 'https://x.fr')))).toBe('voir https://x.fr');
    expect(write(p(link('https://x.fr', 'https://x.fr'), text('/a')))).toBe('https://x.fr\\/a');
    expect(write(p(text('voir'), link('https://x.fr', 'https://x.fr')))).toBe('voir[https://x.fr]');
    expect(write(p(link('a [b] | c', 'https://x.fr/[1]')))).toBe(
      '[a (b) / c|https://x.fr/%5B1%5D]',
    );
    expect(write(p(link('x', 'javascript:alert(1)')))).toBe('x');
    expect(write(p(n.mention.create({ username: 'marie.durand' })))).toBe('[~marie.durand]');
    expect(write(p(n.raw_inline.create({ wiki: '!a.png!', kind: 'image' })))).toBe('!a.png!');
  });

  it('writes nested and mixed lists', () => {
    const item = (s: string, ...sub: PMNode[]): PMNode =>
      n.list_item.create(null, [p(text(s)), ...sub]);
    const ul = (...items: PMNode[]): PMNode => n.bullet_list.create(null, items);
    const ol = (...items: PMNode[]): PMNode => n.ordered_list.create(null, items);
    expect(write(ul(item('a', ol(item('b'), item('c'))), item('d')))).toBe('* a\n*# b\n*# c\n* d');
    expect(write(ul(n.list_item.create(null, [p(), ul(item('x'))])))).toBe('** x');
    expect(write(ul(item('a'), n.list_item.create(null, [p(), ul(item('x'))])))).toBe(
      '* a\n* \n** x',
    );
  });

  it('writes tables', () => {
    const cell = (s: string, header = false): PMNode =>
      (header ? n.table_header : n.table_cell).create(null, p(...(s ? [text(s)] : [])));
    const row = (...cells: PMNode[]): PMNode => n.table_row.create(null, cells);
    const table = n.table.create(null, [
      row(cell('Nom', true), cell('Valeur', true)),
      row(cell('a|b'), cell('')),
      row(cell('[x'), cell('fin\\')),
    ]);
    const out = write(table);
    expect(out).toBe('||Nom||Valeur||\n|a\\|b| |\n|\\[x|fin\\ |');
    expect(renderWiki(out, NONE_EXIST)).toBe(
      '<table class="wiki-table"><tbody><tr><th>Nom</th><th>Valeur</th></tr>' +
        '<tr><td>a|b</td><td></td></tr><tr><td>[x</td><td>fin\\</td></tr></tbody></table>',
    );
  });

  it('writes code blocks, quotes and rules', () => {
    const code = n.code_block.create({ params: 'java' }, schema.text('a {code} b'));
    expect(write(code)).toBe('{code:java}\na {code\u200b} b\n{code}');
    const quote = n.blockquote.create(null, [p(text('a')), p(text('b'))]);
    expect(write(quote)).toBe('{quote}\na\n\nb\n{quote}');
    expect(write(n.blockquote.create({ form: 'bq' }, p(text('court'))))).toBe('bq. court');
    expect(write(p(text('a')), n.horizontal_rule.create(), p(text('b')))).toBe('a\n\n----\n\nb');
  });

  it('skips empty paragraphs', () => {
    expect(write(p(text('a')), p(), p(), p(text('b')))).toBe('a\n\nb');
    expect(write(p())).toBe('');
  });
});

describe('round trip of shapes found by fuzzing', () => {
  /** Rewritten canonically, they come back exactly as written. */
  const EXACT = [
    // Formatting inside a link label, the link stuck to a word.
    'a[+t+|/]', '[*t*|/]f', 'a[_a_|http://f]', '[-t-|http://r]q', '[ ~.~|http://r]t',
    '[,^a^|http://r]a', 'voir[*gras*|https://x.fr] et [_lien_|https://x.fr]s',
    // Formatting around a link, as the source nests them.
    '*[ é|http://r]*',
    // A line break inside a link label, inside formatting.
    '*b [\\\\|http://r]*', '\\\\^[a\\\\|/]^', '+[ \\\\|http://r]+',
    // Links keep their form: "[address]" next to an unresolved attachment
    // reads differently once bare.
    '[^f]^[http://r]', '[^f]^http://a[http://f]', '[\\\\[http://r]|http://f]',
    'voir [http://x.fr] et http://y.fr',
    // "[|address|title]": the label cannot hold what the address does.
    '[|/.\\-|]', '[|http://\\)|]', '[|http://x.fr|titre]',
    // An escaped character ends an address.
    '!g!http://\\!', 'http://\\-x',
    // Mentions keep their spacing.
    '[^f][~ ^]', '[~ bob] et [|~bob]',
    // A "-" list whose first item is empty.
    '- \n** \n- ',
    // Cells: a bar in mono or raw markup after an open "[" (last cell only).
    '|[{{|}}', '|a|[!a|[!', '|[{p:|}', '|[\\\\{{|}}', '|{{[|http://r}}][{{|\\-}}',
    '|[x !a|b! y]|c|',
  ]; // prettier-ignore

  it.each(EXACT)('writes %j back as it was', (src) => {
    expect(canon(src)).toBe(src);
  });

  it('closes a cell whose "[" in mono is closed by text', () => {
    const src = '|{{[}}]|q';
    const out = canon(src);
    expect(out).toBe('|{{[}}]|q|');
    expect(sameMeaning(src, out)).toBe(true);
    expect(canon(out)).toBe(out);
  });

  it('drops the mono of a "[" left open in a cell before the last', () => {
    const cell = (...c: PMNode[]): PMNode => n.table_cell.create(null, p(...c));
    const row = (...c: PMNode[]): PMNode => n.table.create(null, n.table_row.create(null, c));
    const out = write(row(cell(text('a[', 'code')), cell(text('x'))));
    expect(out).toBe('|a\\[|x|');
    expect(renderWiki(out, NONE_EXIST)).toBe(
      '<table class="wiki-table"><tbody><tr><td>a[</td><td>x</td></tr></tbody></table>',
    );
    // Closed in the mono, or in the last cell, the mono stays.
    expect(write(row(cell(text('a[b]', 'code')), cell(text('x'))))).toBe('|{{a[b]}}|x|');
    expect(write(row(cell(text('x')), cell(text('a[', 'code'))))).toBe('|x|{{a[}}');
  });

  it('keeps formatting inside a typed link stuck to a word', () => {
    const strong = m.strong.create();
    const href = m.link.create({ href: 'https://x.fr' });
    expect(write(p(text('voir'), schema.text('ceci', [href, strong])))).toBe(
      'voir[*ceci*|https://x.fr]',
    );
    // Still dropped when the markup cannot hold it.
    expect(write(p(text('un'), text('mot', 'strong'), text('deux')))).toBe('unmotdeux');
  });

  // Hard breaks (newline and \\) in and around links, formatting and mono,
  // in paragraphs, list items, quotes, code blocks and tables.
  const piece = fc.letrec((tie) => ({
    leaf: fc.oneof(
      { arbitrary: fc.constantFrom('a', 'mot', ' ', 'x.', ' b ', ',', ':', 'é'), weight: 5 },
      { arbitrary: fc.constantFrom('\n', '\\\\', '\\\\\n', '\n\\\\', ' \n', '\\\\ '), weight: 3 },
      {
        arbitrary: fc.constantFrom(
          '[^f.pdf]', '[doc|^f.pdf]', '[~bob]', '[http://y.fr]', 'http://e.fr/a', '!a.png!',
        ),
        weight: 2,
      },
      { arbitrary: tie('wrap'), weight: 3 },
    ), // prettier-ignore
    seq: fc.array(tie('leaf'), { minLength: 1, maxLength: 4 }).map((a) => a.join('')),
    wrap: fc
      .tuple(
        fc.constantFrom('*', '_', '+', '-', '{{', '[', '^'),
        tie('seq'),
        fc.constantFrom('', ' ', 'a', '\\\\', '\n'),
        fc.constantFrom('', ' ', 'a', '\\\\', '\n'),
      )
      .map(([k, s, before, after]) => {
        const core = k === '{{' ? `{{${s}}}` : k === '[' ? `[${s}|http://y.fr]` : `${k}${s}${k}`;
        return before + core + after;
      }),
  }));
  const item = fc
    .tuple(fc.constantFrom('*', '**', '#', '*#', '#*', '-'), piece.seq)
    .map(([marker, s]) => `${marker} ${s}`);
  const block = fc.oneof(
    piece.seq,
    fc.array(item, { minLength: 1, maxLength: 3 }).map((a) => a.join('\n')),
    piece.seq.map((s) => `bq. ${s}`),
    piece.seq.map((s) => `{quote}\n${s}\n{quote}`),
    piece.seq.map((s) => `{code}\n${s}\n{code}`),
    fc
      .array(fc.tuple(piece.seq, piece.seq), { minLength: 1, maxLength: 2 })
      .map((rows) => rows.map(([a, b]) => `|${a}|${b}|`).join('\n')),
  );

  it('reads back breaks next to links and formatting, stable on a second pass', () => {
    fc.assert(
      fc.property(
        fc.array(fc.tuple(block, fc.constantFrom('\n', '\n\n')), { minLength: 1, maxLength: 3 }),
        (blocks) => {
          const src = blocks.map(([b, sep], k) => (k === 0 ? b : sep + b)).join('');
          const { verdict, canonical } = roundTrip(src);
          expect(verdict, src).not.toBe('different');
          expect(canon(canonical), src).toBe(canonical);
        },
      ),
      { numRuns: 2000, seed: 20261015 },
    );
  });
});

describe('round trip of random markup', () => {
  const TOKENS = [
    'a', 'mot', ' ', '\n', '\n\n', '*', '_', '-', '+', '^', '~', '??', '{', '}', '{{', '}}',
    '[', ']', '|', '||', '!', '(', ')', '\\', '\\\\', '#', '* ', '# ', '** ', '*# ', 'h1. ',
    'bq. ', '.', ':', 'x.png', '{code}', '{code:java}', '{quote}', '{noformat}', '{panel}',
    '{color:red}', '{color}', '[~bob]', '[x|http://y.fr]', 'http://e.fr/a_b', 'DEMO-1', '(x)',
    '!a.png!', '{toc}', '[^f.pdf]', 'é', '«', '’', '\t',
  ]; // prettier-ignore

  it('writes back what it read, and the result is stable', () => {
    let different = 0;
    fc.assert(
      fc.property(fc.array(fc.constantFrom(...TOKENS), { maxLength: 24 }), (parts) => {
        const src = parts.join('');
        const out = canon(src);
        expect(canon(out)).toBe(out);
        const a = renderWiki(src, ALL_EXIST);
        const b = renderWiki(out, ALL_EXIST);
        if (flattenInline(a) !== flattenInline(b)) different++;
      }),
      { numRuns: 3000, seed: 20261014 },
    );
    // A handful of contrived mixes (formatting starting on a line break,
    // a link split by bold) lose their exact nesting.
    expect(different).toBeLessThanOrEqual(3);
  });
});
