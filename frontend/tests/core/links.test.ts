import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { LICENSES_PATH, SOURCE_URL } from '../../src/lib/links.js';

const src = new URL('../../src/', import.meta.url);

/** Every file under src/, as a path relative to it. */
function sources(dir = ''): string[] {
  return readdirSync(new URL(dir, src)).flatMap((name) => {
    const rel = dir + name;
    return statSync(new URL(rel, src)).isDirectory() ? sources(rel + '/') : [rel];
  });
}

describe('links', () => {
  it('points the licences link at a file the application ships', () => {
    expect(existsSync(new URL('../../public' + LICENSES_PATH, import.meta.url))).toBe(true);
  });

  it('writes the source address in one place only', () => {
    const files = sources().filter((f) =>
      readFileSync(new URL(f, src), 'utf8').includes(SOURCE_URL),
    );
    expect(files).toEqual(['lib/links.ts']);
  });

  it('writes the licences path in one place only', () => {
    const files = sources().filter((f) =>
      readFileSync(new URL(f, src), 'utf8').includes(LICENSES_PATH),
    );
    expect(files).toEqual(['lib/links.ts']);
  });
});
