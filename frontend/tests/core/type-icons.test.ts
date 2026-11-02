import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { TYPE_ICONS } from '../../src/ui/type-icons.js';

describe('type icons', () => {
  it('draws every icon name the server accepts, and no other', () => {
    const doc = readFileSync(
      new URL('../../../backend/internal/api/ADMIN_API.md', import.meta.url),
      'utf8',
    );
    const list = /`icon` one of([\s\S]*?)\. Without an icon/.exec(doc)?.[1] ?? '';
    const names = [...list.matchAll(/`([a-z]+)`/g)].map((m) => m[1]);
    expect(names.length).toBeGreaterThan(0);
    expect(Object.keys(TYPE_ICONS).sort()).toEqual(names.sort());
  });
});
