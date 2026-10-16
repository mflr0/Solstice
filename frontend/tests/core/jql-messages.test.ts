import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import type { JqlCode, JqlMessages } from '../../src/core/jql.js';
import { de } from '../../src/i18n/de.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { hi } from '../../src/i18n/hi.js';
import { it as itMessages } from '../../src/i18n/it.js';
import type { Messages } from '../../src/i18n/types.js';

// The client's JQL errors copy the server's (backend/internal/i18n/*.go):
// this keeps the two copies saying the same thing.
const catalogues: Record<string, Messages> = { en, fr, de, it: itMessages, hi };

/** The JQL* entries of a server catalogue, by client code. */
function serverMessages(lang: string): Map<string, string> {
  const go = readFileSync(
    new URL(`../../../backend/internal/i18n/${lang}.go`, import.meta.url),
    'utf8',
  );
  const out = new Map<string, string>();
  for (const [, key, literal] of go.matchAll(/^\s*JQL(\w+):\s*("(?:[^"\\]|\\.)*"),/gm)) {
    out.set(key![0]!.toLowerCase() + key!.slice(1), JSON.parse(literal!) as string);
  }
  return out;
}

describe('JQL error messages', () => {
  for (const [lang, messages] of Object.entries(catalogues)) {
    it(`match the server's in ${lang}`, () => {
      const server = serverMessages(lang);
      const jql: JqlMessages = messages.jql;
      for (const code of Object.keys(jql) as JqlCode[]) {
        const template = server.get(code);
        expect(template, `${lang} ${code}`).toBeDefined();
        const args = (template!.match(/%s/g) ?? []).map((_, i) => `arg${i}`);
        let i = 0;
        const expected = template!.replace(/%s/g, () => args[i++]!);
        const render = jql[code] as (...a: string[]) => string;
        expect(render(...args), `${lang} ${code}`).toBe(expected);
      }
    });
  }
});
