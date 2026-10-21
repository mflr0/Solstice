import { describe, expect, it } from 'vitest';

import {
  canAskForLink,
  canResendInvitation,
  canSendResetLink,
  choosePasswordProblem,
  isLinkPurpose,
  passwordModes,
  passwordTooShort,
  type LinkAccount,
} from '../../src/core/account-links.js';

describe('choosing a password through a link', () => {
  it('asks for the length the server asks for, in characters', () => {
    expect(choosePasswordProblem('short', 'short', 10)).toBe('tooShort');
    expect(choosePasswordProblem('', '', 10)).toBe('tooShort');
    // Ten letters with accents are ten characters, not more than ten bytes.
    expect(choosePasswordProblem('éééééééééé', 'éééééééééé', 10)).toBeNull();
    expect(choosePasswordProblem('😀'.repeat(9), '😀'.repeat(9), 10)).toBe('tooShort');
    expect(choosePasswordProblem('😀'.repeat(10), '😀'.repeat(10), 10)).toBeNull();
  });

  it('wants the confirmation to match', () => {
    expect(choosePasswordProblem('a long enough one', 'a long enough ona', 10)).toBe('mismatch');
    expect(choosePasswordProblem('a long enough one', '', 10)).toBe('mismatch');
    expect(choosePasswordProblem('a long enough one', 'a long enough one', 10)).toBeNull();
  });

  it('reports the length before the mismatch', () => {
    expect(choosePasswordProblem('abc', 'abd', 10)).toBe('tooShort');
  });
});

describe('a password shorter than the minimum', () => {
  it('counts characters, not bytes or UTF-16 units', () => {
    expect(passwordTooShort('', 10)).toBe(true);
    expect(passwordTooShort('123456789', 10)).toBe(true);
    expect(passwordTooShort('1234567890', 10)).toBe(false);
    expect(passwordTooShort('éééééééééé', 10)).toBe(false);
    expect(passwordTooShort('😀'.repeat(9), 10)).toBe(true);
    expect(passwordTooShort('😀'.repeat(10), 10)).toBe(false);
  });
});

describe('asking for a link', () => {
  it('needs something typed', () => {
    expect(canAskForLink('')).toBe(false);
    expect(canAskForLink('   ')).toBe(false);
    expect(canAskForLink(' marie.durand ')).toBe(true);
    expect(canAskForLink('marie@example.org')).toBe(true);
  });

  it('knows what a link is for', () => {
    expect(isLinkPurpose('reset')).toBe(true);
    expect(isLinkPurpose('invite')).toBe(true);
    expect(isLinkPurpose('login')).toBe(false);
    expect(isLinkPurpose(undefined)).toBe(false);
  });
});

describe('what the administration offers', () => {
  const never: LinkAccount = { active: true, email: 'marie@example.org', lastLogin: null };
  const seen: LinkAccount = { ...never, lastLogin: '2026-10-01T09:00:00Z' };

  it('sends a reset link to an active account with an address, when e-mail links work', () => {
    expect(canSendResetLink(seen, true)).toBe(true);
    expect(canSendResetLink(seen, false)).toBe(false);
    expect(canSendResetLink({ ...seen, active: false }, true)).toBe(false);
    expect(canSendResetLink({ ...seen, email: '' }, true)).toBe(false);
    expect(canSendResetLink({ ...seen, email: '  ' }, true)).toBe(false);
    expect(canSendResetLink({ ...seen, directory: 'ldap' }, true)).toBe(false);
  });

  it('resends an invitation only to accounts that never signed in', () => {
    expect(canResendInvitation(never, true)).toBe(true);
    expect(canResendInvitation(seen, true)).toBe(false);
    expect(canResendInvitation(never, false)).toBe(false);
    expect(canResendInvitation({ ...never, active: false }, true)).toBe(false);
    expect(canResendInvitation({ ...never, email: '' }, true)).toBe(false);
    expect(canResendInvitation({ ...never, directory: 'ldap' }, true)).toBe(false);
    // An older server does not say: no guessing.
    expect(canResendInvitation({ active: true, email: 'a@example.org' }, true)).toBe(false);
  });

  it('offers the link among the ways to give a password only when it can work', () => {
    expect(passwordModes(seen, true)).toEqual(['generate', 'type', 'link']);
    expect(passwordModes(seen, false)).toEqual(['generate', 'type']);
    expect(passwordModes({ ...seen, email: '' }, true)).toEqual(['generate', 'type']);
  });
});
