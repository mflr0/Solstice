import { describe, expect, it } from 'vitest';

import { ADMIN_SECTIONS } from '../../src/core/areas.js';
import { ADMIN_ICONS } from '../../src/ui/admin/admin-icons.js';

describe('ADMIN_ICONS', () => {
  it.each(ADMIN_SECTIONS.filter((s) => s !== 'overview'))('draws %s', (section) => {
    expect(ADMIN_ICONS[section as keyof typeof ADMIN_ICONS]?.length).toBeGreaterThan(0);
  });
});
