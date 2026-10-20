import { readFileSync, readdirSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

import { setupCatalogues } from '../../src/i18n/setup/index.js';

const importer = new URL('../../../backend/internal/importer/', import.meta.url);

/** The step names the importer writes into its report, from its Go sources. */
function stepNames(): Set<string> {
  const names = new Set<string>();
  for (const file of readdirSync(importer)) {
    if (!file.endsWith('.go') || file.endsWith('_test.go')) continue;
    const src = readFileSync(new URL(file, importer), 'utf8');
    for (const m of src.matchAll(/rep\.step\("([^"]+)"/g)) names.add(m[1]!);
  }
  return names;
}

describe('the import report labels', () => {
  const steps = stepNames();

  it('finds the importer steps', () => {
    expect(steps.size).toBeGreaterThan(40);
  });

  for (const [locale, m] of Object.entries(setupCatalogues)) {
    it(`labels every importer step and nothing else in ${locale}`, () => {
      const keys = Object.keys(m.import.report);
      expect([...steps].filter((s) => !keys.includes(s))).toEqual([]);
      expect(keys.filter((k) => !steps.has(k))).toEqual([]);
    });
  }
});
