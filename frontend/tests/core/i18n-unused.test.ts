import { readdirSync, readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

import { en } from '../../src/i18n/en.js';
import { LOCALES } from '../../src/i18n/locales.js';
import { en as setupEn } from '../../src/i18n/setup/en.js';

/**
 * Every text of the English catalogues is read somewhere in src outside the
 * catalogues. A text counts as read when the code names its whole path
 * (`t.adminArea.title`, `m.stepDone`), or when the code takes one of its
 * ancestors whole (`t.boards.kinds[b.kind]`, `const m = t.themes.mock`) and
 * the text's own name appears somewhere as a word.
 */

const src = new URL('../../src/', import.meta.url);

/**
 * Texts, or whole tables of them, that the test skips: mostly tables keyed by
 * values the server sends, which the interface never spells out.
 */
const DYNAMIC: Record<string, string> = {
  historyFields: 'the field names of the history the server returns',
  'hooksAdmin.eventNames': 'the server’s webhook events',
  'auditAdmin.actions': 'the audit actions the server writes (admin-ops.test.ts pins them)',
  'auditAdmin.detailKeys': 'the keys of the server’s audit details',
  'projects.kinds': 'the project types the server stores',
  'updates.badge': 'the account menu’s update notice, which #71 shows',
};

const SETUP_DYNAMIC: Record<string, string> = {
  'import.report': 'the importer’s steps (import-report-labels.test.ts pins them)',
};

const catalogueFile = new RegExp(`^i18n/(setup/)?(${LOCALES.join('|')})\\.ts$`);

/** The interface sources, catalogues left out. */
function sources(): string {
  return readdirSync(src, { recursive: true, encoding: 'utf8' })
    .map((f) => f.replaceAll('\\', '/'))
    .filter((f) => /\.(ts|js|svelte)$/.test(f) && !catalogueFile.test(f))
    .map((f) => readFileSync(new URL(f, src), 'utf8'))
    .join('\n');
}

/** The path of every leaf (a string, a function or a list). */
function leaves(node: object, path: string[] = []): string[][] {
  return Object.entries(node).flatMap(([k, v]) =>
    v !== null && typeof v === 'object' && !Array.isArray(v)
      ? leaves(v, [...path, k])
      : [[...path, k]],
  );
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** The code naming the key at `path` from the catalogue's root, as a regular expression. */
function access(root: string, path: string[]): string {
  return `\\b${root}${path.map((k) => `\\??\\.${escape(k)}`).join('')}\\b`;
}

/** The leaves of `catalogue` that nothing in `code` reads, `root` being the name the code reads it by. */
function unused(
  catalogue: object,
  root: string,
  dynamic: Record<string, string>,
  code: string,
): string[] {
  const named = (path: string[]) => new RegExp(access(root, path)).test(code);
  const whole = (path: string[]) => new RegExp(`${access(root, path)}(?!\\s*\\??\\.)`).test(code);
  return leaves(catalogue)
    .filter((path) => {
      const prefixes = path.map((_, i) => path.slice(0, i + 1));
      if (prefixes.some((a) => a.join('.') in dynamic) || named(path)) return false;
      const ancestors = prefixes.slice(0, -1);
      const word = new RegExp(`\\b${escape(path.at(-1)!)}\\b`);
      return !(ancestors.some(whole) && word.test(code));
    })
    .map((path) => path.join('.'));
}

describe('the interface texts', () => {
  const code = sources();

  it('reads the sources', () => {
    expect(code).toContain('t.adminArea.title');
    expect(code).not.toContain("tagline: 'Issue tracking'");
  });

  it('reads every text of the application catalogue', () => {
    expect(unused(en, '(?:t|en)', DYNAMIC, code)).toEqual([]);
  });

  it('reads every text of the setup catalogue', () => {
    expect(unused(setupEn, 'm', SETUP_DYNAMIC, code)).toEqual([]);
  });

  it('skips only keys that exist and, for a single text, that nothing reads yet', () => {
    const stale = (c: object, root: string, d: Record<string, string>) =>
      Object.keys(d).filter((p) => {
        const path = p.split('.');
        const node = path.reduce<unknown>((n, k) => (n as Record<string, unknown>)?.[k], c);
        if (node === undefined) return true;
        return typeof node !== 'object' && new RegExp(access(root, path)).test(code);
      });
    expect(stale(en, '(?:t|en)', DYNAMIC)).toEqual([]);
    expect(stale(setupEn, 'm', SETUP_DYNAMIC)).toEqual([]);
  });
});
