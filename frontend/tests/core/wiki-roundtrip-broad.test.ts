import { it } from 'vitest';
import fc from 'fast-check';
import { flattenInline, ALL_EXIST, NONE_EXIST } from '../../src/core/wiki-roundtrip.js';
import { parseWiki, serializeWiki } from '../../src/core/wiki-doc.js';
import { renderWiki } from '../../src/core/wiki.js';

const TOKENS = [
  'a',
  'b',
  'mot',
  ' ',
  ' ',
  '\n',
  '\n\n',
  '*',
  '_',
  '-',
  '+',
  '^',
  '~',
  '??',
  '{',
  '}',
  '{{',
  '}}',
  '[',
  ']',
  '|',
  '||',
  '!',
  '(',
  ')',
  '\\',
  '\\\\',
  '#',
  '* ',
  '# ',
  '** ',
  '*# ',
  'h1. ',
  'bq. ',
  '----',
  '.',
  ':',
  'x.png',
  '{code}',
  '{code:java}',
  '{quote}',
  '{noformat}',
  '{panel}',
  '{color:red}',
  '{color}',
  '[~bob]',
  '[x|http://y.fr]',
  'http://e.fr/a_b',
  'DEMO-1',
  '(x)',
  '(/)',
  '!a.png!',
  '{toc}',
  '[^f.pdf]',
  'é',
  '«',
  '’',
  '—',
  '…',
  '\t',
  '\r\n',
];

it('broad', () => {
  const bad: string[] = [];
  let all = 0,
    none = 0,
    idem = 0;
  fc.assert(
    fc.property(fc.array(fc.constantFrom(...TOKENS), { maxLength: 30 }), (parts) => {
      const src = parts.join('');
      const out = serializeWiki(parseWiki(src).doc);
      const a =
        flattenInline(renderWiki(src, ALL_EXIST)) === flattenInline(renderWiki(out, ALL_EXIST));
      const n =
        flattenInline(renderWiki(src, NONE_EXIST)) === flattenInline(renderWiki(out, NONE_EXIST));
      if (!a) {
        all++;
        if (bad.length < 20) bad.push(`ALL ${JSON.stringify(src)}\n   -> ${JSON.stringify(out)}`);
      } else if (!n) {
        none++;
        if (bad.length < 30) bad.push(`NONE ${JSON.stringify(src)}\n   -> ${JSON.stringify(out)}`);
      }
      const again = serializeWiki(parseWiki(out).doc);
      if (again !== out) {
        idem++;
        if (bad.length < 40)
          bad.push(`IDEM ${JSON.stringify(out)}\n   -> ${JSON.stringify(again)}`);
      }
      return true;
    }),
    { numRuns: 100000, seed: Number(process.env.SEED ?? 5) },
  );
  process.stdout.write(`all=${all} noneOnly=${none} idem=${idem}\n${bad.join('\n')}\n`);
}, 60_000);
