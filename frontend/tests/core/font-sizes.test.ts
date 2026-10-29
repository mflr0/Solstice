import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Font sizes are in rem so they follow the browser's font size and the
 * wide-screen steps of app.css; a px size other than 0 would stay put.
 */

const src = new URL('../../src/', import.meta.url);

function files(dir: URL, ext: string): URL[] {
  return readdirSync(dir, { withFileTypes: true, recursive: true })
    .filter((e) => e.isFile() && e.name.endsWith(ext))
    .map((e) => pathToFileURL(join(e.parentPath, e.name)));
}

function styles(file: URL): string {
  const text = readFileSync(file, 'utf8');
  if (!file.pathname.endsWith('.svelte')) return text;
  return [...text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]).join('\n');
}

/** The font-size values of a stylesheet that hold a px length other than 0. */
function pxFontSizes(css: string): string[] {
  return [...css.matchAll(/font-size\s*:\s*([^;}]*)/g)]
    .map((m) => m[1]!.trim())
    .filter((v) => [...v.matchAll(/(\d*\.?\d+)px/g)].some((n) => Number(n[1]) > 0));
}

describe('px font sizes', () => {
  it('are found, except 0', () => {
    expect(
      pxFontSizes('.a { font-size: 12px; } .b { font-size: 0; } .c { font-size: 0px }'),
    ).toEqual(['12px']);
    expect(pxFontSizes('.a { font-size: calc(1rem + 2px) } .b { font-size: .5px }')).toEqual([
      'calc(1rem + 2px)',
      '.5px',
    ]);
    expect(pxFontSizes('.a { font-size: 0.75rem; --gap: 4px; }')).toEqual([]);
  });

  const sheets = [
    new URL('app.css', src),
    ...files(new URL('themes/', src), '.css'),
    ...files(new URL('ui/', src), '.svelte'),
  ];

  it.each(sheets.map((f) => [f.pathname.slice(src.pathname.length), f]))(
    'are absent from %s',
    (_, f) => {
      expect(pxFontSizes(styles(f))).toEqual([]);
    },
  );
});
