import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

import { AUDIT_FAMILIES } from '../../src/core/admin-integrations.js';
import {
  AUDIT_ACTIONS,
  AUDIT_MIN_RETENTION,
  TOKEN_SHOWS,
  auditActor,
  creatorsProblem,
  filterTokens,
  importReady,
  retentionProblem,
  sameCreators,
  splitUptime,
  toggleDay,
} from '../../src/core/admin-ops.js';
import type { AccessToken } from '../../src/core/admin-types.js';
import { de } from '../../src/i18n/de.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { hi } from '../../src/i18n/hi.js';
import { it as italian } from '../../src/i18n/it.js';

/** A Go source of the backend, from the repository root. */
function go(path: string): string {
  return readFileSync(new URL(`../../../backend/internal/${path}`, import.meta.url), 'utf8');
}

describe('the audit log actions', () => {
  it('lists every action the backend writes', () => {
    const written = new Set<string>();
    // audit.go: the constants of the package, and the catalogue kinds' own.
    for (const m of go('audit/audit.go').matchAll(/^\s+\w+\s+= "([a-z]+\.[a-z_]+)"$/gm)) {
      written.add(m[1]!);
    }
    for (const m of go('api/updates.go').matchAll(/^\s+auditUpdate\w+\s+= "(update\.[a-z]+)"$/gm)) {
      written.add(m[1]!);
    }
    for (const m of go('api/setup_steps.go').matchAll(
      /auditSetupComplete\s+= "(setup\.[a-z]+)"/g,
    )) {
      written.add(m[1]!);
    }
    const kinds = [...go('api/admin_catalogs.go').matchAll(/audit: "([a-z]+)"/g)].map((m) => m[1]!);
    expect(kinds.length).toBeGreaterThan(0);
    for (const k of kinds) {
      for (const v of ['create', 'update', 'delete', 'order']) written.add(`${k}.${v}`);
    }
    expect(written.size).toBeGreaterThan(70);
    expect([...written].filter((a) => !AUDIT_ACTIONS.includes(a))).toEqual([]);
    expect(AUDIT_ACTIONS.filter((a) => !written.has(a))).toEqual([]);
  });

  it('has no duplicate, and every family of the filter has actions', () => {
    expect(new Set(AUDIT_ACTIONS).size).toBe(AUDIT_ACTIONS.length);
    const families = new Set(AUDIT_ACTIONS.map((a) => a.slice(0, a.indexOf('.'))));
    expect([...families].sort()).toEqual([...AUDIT_FAMILIES].sort());
  });

  it('is named in every language', () => {
    for (const messages of [en, fr, de, italian, hi]) {
      const names = messages.auditAdmin.actions as Record<string, string>;
      expect(AUDIT_ACTIONS.filter((a) => !names[a])).toEqual([]);
      expect(Object.keys(names).sort()).toEqual([...AUDIT_ACTIONS].sort());
      const families = messages.auditAdmin.families as Record<string, string>;
      expect(Object.keys(families).sort()).toEqual([...AUDIT_FAMILIES].sort());
    }
  });
});

describe('audit retention', () => {
  it('takes whole days, 0 or at least the minimum', () => {
    expect(retentionProblem('0')).toBeNull();
    expect(retentionProblem(String(AUDIT_MIN_RETENTION))).toBeNull();
    expect(retentionProblem(' 365 ')).toBeNull();
    expect(retentionProblem('36500')).toBeNull();
    expect(retentionProblem('1')).toBe('tooShort');
    expect(retentionProblem(String(AUDIT_MIN_RETENTION - 1))).toBe('tooShort');
    expect(retentionProblem('36501')).toBe('invalid');
    expect(retentionProblem('')).toBe('invalid');
    expect(retentionProblem('-5')).toBe('invalid');
    expect(retentionProblem('12.5')).toBe('invalid');
    expect(retentionProblem('abc')).toBe('invalid');
  });

  it('matches the server minimum', () => {
    const m = /MinRetentionDays = (\d+)/.exec(go('audit/audit.go'));
    expect(Number(m?.[1])).toBe(AUDIT_MIN_RETENTION);
  });
});

describe('who an entry names', () => {
  it('prefers the account, then the name recorded, then nobody', () => {
    const user = { id: 1, username: 'paul.martin', name: 'Paul Martin', active: true };
    expect(auditActor({ user, username: 'paul.martin' })).toEqual({
      name: 'Paul Martin',
      username: 'paul.martin',
    });
    expect(auditActor({ user: null, username: 'inconnu' })).toEqual({
      name: 'inconnu',
      username: 'inconnu',
    });
    expect(auditActor({ user: null, username: null })).toBeNull();
  });
});

describe('uptime', () => {
  it('is cut into days, hours and minutes', () => {
    expect(splitUptime(0)).toEqual({ days: 0, hours: 0, minutes: 0 });
    expect(splitUptime(59)).toEqual({ days: 0, hours: 0, minutes: 0 });
    expect(splitUptime(3600 + 5 * 60 + 9)).toEqual({ days: 0, hours: 1, minutes: 5 });
    expect(splitUptime(3 * 86400 + 4 * 3600 + 30 * 60)).toEqual({ days: 3, hours: 4, minutes: 30 });
    expect(splitUptime(-10)).toEqual({ days: 0, hours: 0, minutes: 0 });
  });
});

function token(over: Partial<AccessToken>): AccessToken {
  return {
    id: 1,
    name: 'CI',
    created: '2026-01-01T00:00:00Z',
    expires: null,
    lastUsed: null,
    expired: false,
    user: { id: 1, username: 'paul.martin', name: 'Paul Martin', active: true },
    ...over,
  };
}

describe('the administrators’ token list', () => {
  const now = new Date('2026-10-16T12:00:00Z');
  const list = [
    token({ id: 1, name: 'Export de nuit' }),
    token({ id: 2, name: 'Plugin IDE', expires: '2026-10-20T00:00:00Z' }),
    token({
      id: 3,
      name: 'Ancien',
      expires: '2026-01-01T00:00:00Z',
      expired: true,
      user: { id: 2, username: 'hugo.robert', name: 'Hugo Robert', active: true },
    }),
  ];

  it('searches the owner and the token name', () => {
    expect(filterTokens(list, 'hugo', 'all', now).map((t) => t.id)).toEqual([3]);
    expect(filterTokens(list, 'PAUL', 'all', now).map((t) => t.id)).toEqual([1, 2]);
    expect(filterTokens(list, 'ide', 'all', now).map((t) => t.id)).toEqual([2]);
    expect(filterTokens(list, '  ', 'all', now)).toHaveLength(3);
  });

  it('filters by state, counting a token about to expire as working', () => {
    expect(filterTokens(list, '', 'active', now).map((t) => t.id)).toEqual([1, 2]);
    expect(filterTokens(list, '', 'expiring', now).map((t) => t.id)).toEqual([2]);
    expect(filterTokens(list, '', 'expired', now).map((t) => t.id)).toEqual([3]);
    expect(TOKEN_SHOWS).toContain('all');
  });
});

describe('who may create tokens', () => {
  it('needs a group when the choice is a group', () => {
    expect(creatorsProblem('group', null)).toBe('groupRequired');
    expect(creatorsProblem('group', 4)).toBeNull();
    expect(creatorsProblem('admins', null)).toBeNull();
    expect(creatorsProblem('everyone', null)).toBeNull();
  });

  it('tells a changed choice from the saved one', () => {
    const saved = { who: 'group' as const, group: { id: 4, name: 'devops' } };
    expect(sameCreators(saved, 'group', 4)).toBe(true);
    expect(sameCreators(saved, 'group', 5)).toBe(false);
    expect(sameCreators(saved, 'admins', null)).toBe(false);
    expect(sameCreators({ who: 'admins', group: null }, 'admins', 9)).toBe(true);
  });
});

describe('the import’s confirmation', () => {
  const ready = { tested: true, backup: true, typed: 'solstice', database: 'solstice' };

  it('needs a tested source, a backup and the database name typed', () => {
    expect(importReady(ready)).toBe(true);
    expect(importReady({ ...ready, tested: false })).toBe(false);
    expect(importReady({ ...ready, backup: false })).toBe(false);
    expect(importReady({ ...ready, typed: 'solstic' })).toBe(false);
    expect(importReady({ ...ready, typed: '  solstice ' })).toBe(true);
    expect(importReady({ ...ready, typed: '', database: '' })).toBe(false);
  });
});

describe('subscription schedules', () => {
  it('keeps the days in week order when one is ticked or unticked', () => {
    expect(toggleDay([4], 1, true)).toEqual([1, 4]);
    expect(toggleDay([1, 4], 4, false)).toEqual([1]);
    expect(toggleDay([1], 1, true)).toEqual([1]);
    expect(toggleDay([], 7, false)).toEqual([]);
  });
});

describe('the mail queue’s attempt count', () => {
  it('prints the number once in every language', () => {
    for (const messages of [en, fr, de, italian, hi]) {
      expect(messages.mailAdmin.attempts(3).match(/3/g)).toEqual(['3']);
    }
  });
});
