import { describe, expect, it } from 'vitest';

import {
  defaultPort,
  hasProblems,
  isEmail,
  mailForm,
  mailPayload,
  mailProblems,
  matches,
  newUserForm,
  suggestUsername,
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
    const listed = ADMIN_NAV.flatMap((g) => g.items).filter((i) => i !== 'updates');
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

describe('general settings', () => {
  const settings: GeneralSettings = {
    instanceName: 'Solstice',
    baseURL: 'https://tickets.example.org',
    timeZone: 'Europe/Paris',
    defaultLanguage: 'fr',
    banner: { enabled: false, text: '', level: 'info' },
    sessionDays: 30,
    passwordMinLength: 12,
    sources: { baseURL: 'env', instanceName: 'database' },
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
    // A browser that cannot list zones leaves the check to the server.
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
    expect(canReset('env')).toBe(false);
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
