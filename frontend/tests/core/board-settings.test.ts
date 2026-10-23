import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

import { de } from '../../src/i18n/de.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { hi } from '../../src/i18n/hi.js';
import { it as italian } from '../../src/i18n/it.js';
import { LOCALES } from '../../src/i18n/locales.js';

const catalogs = { en, fr, de, it: italian, hi };

describe('the board settings’ sub-filter', () => {
  // The page flags the field when the server's error starts with the label.
  it.each(LOCALES)('carries the server’s word in %s', (l) => {
    const source = readFileSync(
      new URL(`../../../backend/internal/i18n/${l}.go`, import.meta.url),
      'utf8',
    );
    const server = /^\s+BoardSubFilter:\s+"([^"]+)",$/m.exec(source)?.[1];
    expect(server).toBeDefined();
    expect(catalogs[l].boardSettings.subQuery).toBe(server);
  });
});
