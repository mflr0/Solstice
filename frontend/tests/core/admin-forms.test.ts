import { describe, expect, it } from 'vitest';

import {
  defaultPort,
  groupProblems,
  hasProblems,
  holderNeedsId,
  isEmail,
  LEVEL_HOLDERS,
  levelTargets,
  mailForm,
  mailPayload,
  mailProblems,
  matches,
  memberPayload,
  namedPatch,
  namedPayload,
  newUserForm,
  offeredScopes,
  readLocked,
  restrictionUsage,
  roleProblems,
  securityLevelProblem,
  suggestUsername,
  TOKEN_SCOPES,
  tokenProblems,
  toggleScope,
  tooLong,
  typedMatches,
  userProblems,
} from '../../src/core/admin-forms.js';
import {
  bannerProblems,
  canReset,
  cleanBaseURL,
  generalForm,
  generalPatch,
  generalProblems,
  importConfirmed,
  importProgress,
  isBaseURL,
} from '../../src/core/admin-general.js';
import type { GeneralSettings, MailSettings } from '../../src/core/admin-types.js';
import { ADMIN_NAV, ADMIN_SECTIONS, adminPath, adminSectionOf } from '../../src/core/areas.js';

const mail: MailSettings = {
  enabled: false,
  host: '',
  port: 587,
  security: 'starttls',
  username: '',
  from: '',
  fromName: '',
  passwordSet: true,
  baseUrl: null,
};

describe('areas', () => {
  it('lists every section once in the navigation, and finds it back from its address', () => {
    const listed = ADMIN_NAV.flatMap((g) => g.items);
    expect([...listed].sort()).toEqual([...ADMIN_SECTIONS].sort());
    for (const s of ADMIN_SECTIONS) {
      if (s === 'overview') continue;
      expect(adminSectionOf(adminPath(s).slice('/admin/'.length))).toBe(s);
    }
  });
});

describe('mail form', () => {
  it('asks for a server and a sender only when sending is on', () => {
    const f = mailForm(mail);
    expect(hasProblems(mailProblems(f))).toBe(false);
    f.enabled = true;
    expect(mailProblems(f)).toEqual({ host: 'hostRequired', from: 'fromRequired' });
    f.host = 'smtp.example.org';
    f.from = 'not an address';
    expect(mailProblems(f)).toEqual({ from: 'fromInvalid' });
    f.from = 'solstice@example.org';
    f.port = '70000';
    expect(mailProblems(f)).toEqual({ port: 'portInvalid' });
  });

  it('keeps, replaces or removes the stored password', () => {
    const f = mailForm(mail);
    expect('password' in mailPayload(f)).toBe(false);
    f.passwordAction = 'replace';
    f.password = 's3cret';
    expect(mailPayload(f).password).toBe('s3cret');
    f.passwordAction = 'remove';
    expect(mailPayload(f).password).toBeNull();
  });

  it('falls back to the usual port of the security mode', () => {
    const f = { ...mailForm(mail), port: ' ', security: 'tls' as const };
    expect(mailPayload(f).port).toBe(465);
    expect(defaultPort('none')).toBe(25);
  });

  it('accepts plain addresses only', () => {
    expect(isEmail('marie@example.org')).toBe(true);
    expect(isEmail('Marie <marie@example.org>')).toBe(false);
    expect(isEmail('marie@')).toBe(false);
  });
});

describe('user form', () => {
  it('checks the user name and password of a new account', () => {
    const f = newUserForm();
    expect(userProblems(f, true, 12)).toEqual({
      username: 'usernameRequired',
      password: 'passwordShort',
    });
    f.username = 'Marie Durand';
    f.password = 'x'.repeat(12);
    expect(userProblems(f, true, 12)).toEqual({ username: 'usernameInvalid' });
    f.username = 'marie.durand';
    expect(hasProblems(userProblems(f, true, 12))).toBe(false);
  });

  it('asks an invitation for an address instead of a password', () => {
    const f = { ...newUserForm(), username: 'marie.durand', invite: true };
    expect(userProblems(f, true, 12)).toEqual({ email: 'emailRequired' });
    f.email = 'marie@example.org';
    expect(hasProblems(userProblems(f, true, 12))).toBe(false);
    f.email = 'nope';
    expect(userProblems(f, true, 12)).toEqual({ email: 'emailInvalid' });
    expect(userProblems({ ...f, email: '', invite: false }, true, 12)).toEqual({
      password: 'passwordShort',
    });
  });

  it('only checks the e-mail of an existing account', () => {
    const f = { ...newUserForm(), email: 'nope' };
    expect(userProblems(f, false, 12)).toEqual({ email: 'emailInvalid' });
  });

  it('suggests a user name from a display name', () => {
    expect(suggestUsername('Élodie Le Gall')).toBe('elodie.le.gall');
    expect(suggestUsername('  --Jean-Paul  ')).toBe('jean.paul');
  });

  it('matches every typed word, accents ignored', () => {
    expect(matches('ren dur', 'René Durand', 'rene.durand')).toBe(true);
    expect(matches('paul', 'René Durand')).toBe(false);
    expect(matches('  ', 'anything')).toBe(true);
  });
});

describe('name length', () => {
  it('counts characters of the trimmed text, as the server does', () => {
    expect(tooLong('a'.repeat(255))).toBe(false);
    expect(tooLong('a'.repeat(256))).toBe(true);
    expect(tooLong(`  ${'a'.repeat(255)}  `)).toBe(false);
    expect(tooLong('😀'.repeat(255))).toBe(false);
    expect(tooLong('😀'.repeat(256))).toBe(true);
    expect(tooLong('a'.repeat(101), 100)).toBe(true);
  });
});

describe('group and role forms', () => {
  it('asks for a name, trimmed, of 255 characters at most', () => {
    expect(groupProblems({ name: '  ', description: '' })).toEqual({ name: 'nameRequired' });
    expect(roleProblems({ name: '', description: '' })).toEqual({ name: 'nameRequired' });
    expect(hasProblems(groupProblems({ name: ` ${'a'.repeat(255)} `, description: '' }))).toBe(
      false,
    );
    expect(roleProblems({ name: 'é'.repeat(256), description: '' })).toEqual({
      name: 'nameTooLong',
    });
  });

  it('limits the description of a group only', () => {
    const description = 'x'.repeat(256);
    expect(groupProblems({ name: 'devs', description })).toEqual({
      description: 'descriptionTooLong',
    });
    expect(hasProblems(roleProblems({ name: 'Developers', description }))).toBe(false);
  });

  it('sends trimmed texts, an empty description clearing it', () => {
    expect(namedPayload({ name: ' devs ', description: '  ' })).toEqual({
      name: 'devs',
      description: '',
    });
  });

  it('sends only the name or the description that changed', () => {
    const role = { name: 'Developers', description: 'Write code' };
    expect(namedPatch({ name: ' Developers ', description: 'Write code ' }, role)).toEqual({});
    expect(namedPatch({ name: 'Devs', description: 'Write code' }, role)).toEqual({ name: 'Devs' });
    expect(namedPatch({ name: 'Developers', description: '' }, role)).toEqual({ description: '' });
  });

  it('takes a missing description as empty', () => {
    const group = { name: 'devs', description: null };
    expect(namedPatch({ name: 'devs', description: '  ' }, group, true)).toEqual({});
    expect(namedPatch({ name: 'devs', description: 'Code' }, group, true)).toEqual({
      description: 'Code',
    });
  });

  it('does not take a group name changed by case alone for a change', () => {
    const group = { name: 'devs', description: '' };
    expect(namedPatch({ name: ' Devs', description: '' }, group, true)).toEqual({});
    expect(namedPatch({ name: 'Devs', description: '' }, group)).toEqual({ name: 'Devs' });
    expect(namedPatch({ name: 'Ops', description: '' }, group, true)).toEqual({ name: 'Ops' });
  });

  it('reads what restricts a deletion from the server answer', () => {
    expect(
      restrictionUsage({ error: 'x', usage: { comments: 3, worklogs: 1, grants: 2 } }),
    ).toEqual({
      comments: 3,
      worklogs: 1,
    });
    expect(restrictionUsage({ usage: { comments: 2 } })).toEqual({ comments: 2, worklogs: 0 });
    expect(restrictionUsage({ error: 'x' })).toBeNull();
    expect(restrictionUsage(undefined)).toBeNull();
    expect(restrictionUsage({ usage: null })).toBeNull();
  });
});

describe('general settings', () => {
  const settings: GeneralSettings = {
    instanceName: 'Solstice',
    baseURL: 'https://tickets.example.org',
    timeZone: 'Europe/Paris',
    defaultLanguage: 'fr',
    banner: { enabled: false, text: '', level: 'info' },
    sessionDays: 30,
    passwordMinLength: 12,
    sources: { baseURL: 'file', instanceName: 'database' },
    languages: ['fr', 'en'],
  };

  it('reads public addresses without a path', () => {
    expect(isBaseURL('https://tickets.example.org')).toBe(true);
    expect(isBaseURL('http://localhost:8080/')).toBe(true);
    expect(isBaseURL('https://example.org/jira')).toBe(false);
    expect(isBaseURL('ftp://example.org')).toBe(false);
    expect(cleanBaseURL(' http://localhost:8080/ ')).toBe('http://localhost:8080');
  });

  it('reports out-of-range values and unknown zones', () => {
    const f = generalForm(settings);
    expect(hasProblems(generalProblems(f, ['Europe/Paris']))).toBe(false);
    f.sessionDays = '0';
    f.passwordMinLength = '200';
    f.timeZone = 'Mars/Olympus';
    f.instanceName = ' ';
    expect(generalProblems(f, ['Europe/Paris'])).toEqual({
      sessionDays: 'sessionDaysRange',
      passwordMinLength: 'passwordRange',
      timeZone: 'timeZoneUnknown',
      instanceName: 'nameRequired',
    });
    expect(generalProblems(f, null).timeZone).toBeUndefined();
  });

  it('sends only what changed', () => {
    const f = generalForm(settings);
    expect(generalPatch(f, settings)).toEqual({});
    f.baseURL = 'https://tickets.example.org/';
    f.sessionDays = '7';
    expect(generalPatch(f, settings)).toEqual({ sessionDays: 7 });
  });

  it('only resets values set from the administration', () => {
    expect(canReset('database')).toBe(true);
    expect(canReset('file')).toBe(false);
    expect(canReset(undefined)).toBe(false);
  });

  it('wants a text for a banner that shows', () => {
    expect(bannerProblems({ enabled: true, text: '  ', level: 'info' })).toEqual({
      text: 'textRequired',
    });
    expect(bannerProblems({ enabled: false, text: 'x'.repeat(501), level: 'info' })).toEqual({
      text: 'textTooLong',
    });
    expect(bannerProblems({ enabled: false, text: '', level: 'warning' })).toEqual({});
  });
});

describe('import', () => {
  it('starts only once the database name is typed', () => {
    expect(importConfirmed(' Solstice ', 'solstice')).toBe(true);
    expect(importConfirmed('solstic', 'solstice')).toBe(false);
    expect(importConfirmed('', '')).toBe(false);
  });

  it('measures progress within bounds', () => {
    expect(importProgress(5, 10)).toBe(0.5);
    expect(importProgress(12, 10)).toBe(1);
    expect(importProgress(3, 0)).toBe(0);
  });
});

describe('mail form, password after a new destination', () => {
  const saved = {
    ...mail,
    passwordSet: true,
    host: 'smtp.example.org',
    port: 587,
    security: 'starttls' as const,
    username: 'relay',
  };

  it('asks for the password again only when it would follow a new destination', () => {
    const f = mailForm(saved);
    expect(mailProblems(f, saved)).toEqual({});
    f.fromName = 'Tickets';
    f.host = ' SMTP.example.org ';
    expect(mailProblems(f, saved)).toEqual({});
    for (const change of [
      { host: 'smtp.other.example' },
      { port: '25' },
      { security: 'tls' as const },
      { username: 'other' },
    ]) {
      const g = { ...mailForm(saved), ...change };
      expect(mailProblems(g, saved)).toEqual({ password: 'passwordRequired' });
    }
  });

  it('accepts a typed password, or a removal', () => {
    const f = { ...mailForm(saved), host: 'smtp.other.example' };
    f.passwordAction = 'replace';
    f.password = 's3cret';
    expect(mailProblems(f, saved)).toEqual({});
    f.password = '';
    expect(mailProblems(f, saved)).toEqual({ password: 'passwordRequired' });
    f.passwordAction = 'remove';
    expect(mailProblems(f, saved)).toEqual({});
  });

  it('has nothing to protect when no password is stored', () => {
    const none = { ...saved, passwordSet: false };
    expect(mailProblems({ ...mailForm(none), host: 'smtp.other.example' }, none)).toEqual({});
  });
});

describe('typed confirmation', () => {
  it('wants the exact phrase, spaces around ignored', () => {
    expect(typedMatches('SW', 'SW')).toBe(true);
    expect(typedMatches('  SW ', 'SW')).toBe(true);
    expect(typedMatches('sw', 'SW')).toBe(false);
    expect(typedMatches('S', 'SW')).toBe(false);
    expect(typedMatches('', '')).toBe(false);
  });
});

describe('security levels', () => {
  const levels = [
    { id: 1, name: 'Équipe' },
    { id: 2, name: 'Direction' },
  ];

  it('asks for a name no other level of the project has, whatever the case', () => {
    expect(securityLevelProblem('  ', levels, null)).toBe('nameRequired');
    expect(securityLevelProblem('x'.repeat(256), levels, null)).toBe('nameTooLong');
    expect(securityLevelProblem('x'.repeat(255), levels, null)).toBeNull();
    expect(securityLevelProblem(' direction ', levels, null)).toBe('nameTaken');
    expect(securityLevelProblem('Direction', levels, 2)).toBeNull();
    expect(securityLevelProblem('Direction', levels, 1)).toBe('nameTaken');
  });

  it('moves the issues of a deleted level to the others only', () => {
    expect(levelTargets(levels, levels[0]!)).toEqual([levels[1]]);
    expect(levelTargets([levels[0]!], levels[0]!)).toEqual([]);
  });

  it('offers exactly the holders of the permission checks', () => {
    expect([...LEVEL_HOLDERS].sort()).toEqual(
      ['assignee', 'everyone', 'group', 'lead', 'reporter', 'role', 'user'].sort(),
    );
    expect(LEVEL_HOLDERS.filter(holderNeedsId).sort()).toEqual(['group', 'role', 'user']);
  });

  it('sends the one chosen for a user, a group or a role, and nothing for the others', () => {
    expect(memberPayload('group', 4)).toEqual({ holder: 'group', id: 4 });
    expect(memberPayload('user', null)).toBeNull();
    expect(memberPayload('role', null)).toBeNull();
    expect(memberPayload('lead', null)).toEqual({ holder: 'lead' });
    expect(memberPayload('everyone', 7)).toEqual({ holder: 'everyone' });
  });
});

describe('token scopes', () => {
  const today = new Date(2026, 9, 29);

  it('ticks and locks read with any other scope', () => {
    const s = toggleScope([], 'comments:write', true);
    expect(s).toEqual(['read', 'comments:write']);
    expect(readLocked(s)).toBe(true);
    expect(toggleScope(s, 'read', false)).toEqual(['read', 'comments:write']);
    expect(toggleScope(['read'], 'admin', true)).toEqual(['read', 'admin']);
  });

  it('frees read once no other scope is left', () => {
    const s = toggleScope(['read', 'comments:write'], 'comments:write', false);
    expect(s).toEqual(['read']);
    expect(readLocked(s)).toBe(false);
    expect(toggleScope(s, 'read', false)).toEqual([]);
  });

  it("keeps the server's order", () => {
    expect(toggleScope(['read', 'profile:write'], 'issues:write', true)).toEqual([
      'read',
      'issues:write',
      'profile:write',
    ]);
  });

  it('cannot be sent without a scope', () => {
    expect(tokenProblems('ci', [], '90', '', today).scopes).toBe('scopesRequired');
    expect(tokenProblems('ci', ['read'], '90', '', today)).toEqual({});
  });

  it('offers admin to administrators only', () => {
    expect(offeredScopes(true)).toEqual(TOKEN_SCOPES);
    expect(offeredScopes(false)).not.toContain('admin');
    expect(offeredScopes(false)).toHaveLength(TOKEN_SCOPES.length - 1);
  });
});
