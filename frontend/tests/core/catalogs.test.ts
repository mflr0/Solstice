import { describe, expect, it } from 'vitest';

import type { CatalogItem } from '../../src/core/admin-types.js';
import { catalogForm, catalogPayload } from '../../src/core/catalogs.js';

const item = (over: Partial<CatalogItem>): CatalogItem => ({
  id: '1',
  name: 'Fixed',
  position: 1,
  ...over,
});

describe('catalogPayload default', () => {
  it('sets a resolution as the default, and removes it', () => {
    const plain = item({ default: false });
    expect(catalogPayload('resolutions', { ...catalogForm(plain), default: true }, plain)).toEqual({
      default: true,
    });
    const def = item({ default: true });
    expect(catalogPayload('resolutions', { ...catalogForm(def), default: false }, def)).toEqual({
      default: false,
    });
    expect(catalogPayload('resolutions', catalogForm(def), def)).toEqual({});
  });

  it('only ever sets a priority as the default', () => {
    const def = item({ default: true, color: '' });
    expect(catalogPayload('priorities', { ...catalogForm(def), default: false }, def)).toEqual({});
    const plain = item({ default: false, color: '' });
    expect(catalogPayload('priorities', { ...catalogForm(plain), default: true }, plain)).toEqual({
      default: true,
    });
  });

  it('sends no default for other catalogues', () => {
    expect(
      catalogPayload('issueTypes', { ...catalogForm(null), name: 'Bug', default: true }, null),
    ).not.toHaveProperty('default');
  });
});
