import { it } from 'vitest';
import fc from 'fast-check';
import { flattenInline, ALL_EXIST } from '../../src/core/wiki-roundtrip.js';
import { parseWiki, serializeWiki } from '../../src/core/wiki-doc.js';
import { renderWiki } from '../../src/core/wiki.js';

const TOKENS = ['a', 'mot', ' ', '\n', '\n', '\\\\', '\\\\', '[x|http://y.fr]', '[http://y.fr]', 'http://e.fr/a', '*', '+', '_', '-', '{{', '}}', '[^f.pdf]', '[^f.pdf]', '* ', '** ', '# ', '#* ', 'bq. ', '{code}', '{code:java}', '{quote}', '{noformat}', '|', '||', '\t', '[', ']', '!', '.', 'DEMO-1', '~', '^'];

it('target', () => {
  const bad: string[] = [];
  let diff = 0, idem = 0;
  fc.assert(
    fc.property(fc.array(fc.constantFrom(...TOKENS), { maxLength: Number(process.env.LEN ?? 16) }), (parts) => {
      const src = parts.join('');
      const out = serializeWiki(parseWiki(src).doc);
      if (flattenInline(renderWiki(src, ALL_EXIST)) !== flattenInline(renderWiki(out, ALL_EXIST))) {
        diff++;
        if (bad.length < 25) bad.push(`DIFF ${JSON.stringify(src)}\n   -> ${JSON.stringify(out)}`);
      }
      const again = serializeWiki(parseWiki(out).doc);
      if (again !== out) { idem++; if (bad.length < 40) bad.push(`IDEM ${JSON.stringify(out)}\n   -> ${JSON.stringify(again)}`); }
      return true;
    }),
    { numRuns: Number(process.env.RUNS ?? 100000), seed: Number(process.env.SEED ?? 1) },
  );
  process.stdout.write(`diff=${diff} idem=${idem}\n${bad.join('\n')}\n`);
});
