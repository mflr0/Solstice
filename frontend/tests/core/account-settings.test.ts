import { describe, expect, it } from 'vitest';

import {
  changesEmail,
  changesPassword,
  hourLabel,
  isWorkingWeek,
  newSubscription,
  ownSubscriptions,
  profileApplied,
  profileForm,
  profilePatch,
  profileProblems,
  sortedDays,
  subscriptionInput,
  weekdayName,
} from '../../src/core/account-settings.js';
import type {
  ProfileField,
  ProfileForm,
  ProfilePatch,
  ProfileProblem,
} from '../../src/core/account-settings.js';
import type { Problems } from '../../src/core/admin-forms.js';
import type { Subscription } from '../../src/core/admin-types.js';
import type { Me, UserRef } from '../../src/core/types.js';

const me: Me = {
  id: 7,
  username: 'marie',
  name: 'Marie Durand',
  email: 'marie@example.com',
  admin: false,
};

function user(id: number): UserRef {
  return { id, username: `u${id}`, name: `User ${id}`, active: true };
}

function sub(over: Partial<Subscription>): Subscription {
  return {
    id: 1,
    filterId: 1,
    owner: null,
    user: null,
    group: null,
    schedule: 'daily',
    days: [1, 2, 3, 4, 5],
    hour: 8,
    emptyToo: false,
    lastRun: null,
    nextRun: '2026-10-16T08:00:00Z',
    canEdit: true,
    ...over,
  };
}

describe('profileForm', () => {
  it('copies the name and the e-mail, leaves the passwords empty', () => {
    expect(profileForm(me)).toEqual({
      name: 'Marie Durand',
      email: 'marie@example.com',
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    });
  });
});

describe('changesPassword', () => {
  it('is false when the three password fields are empty', () => {
    expect(changesPassword(profileForm(me))).toBe(false);
  });

  it('is true when the current password is typed', () => {
    expect(changesPassword({ ...profileForm(me), currentPassword: 'old' })).toBe(true);
  });

  it('is true when the new password is typed', () => {
    expect(changesPassword({ ...profileForm(me), newPassword: 'abcdefgh' })).toBe(true);
  });

  it('is true when the confirmation is typed', () => {
    expect(changesPassword({ ...profileForm(me), confirmPassword: 'abcdefgh' })).toBe(true);
  });
});

describe('changesEmail', () => {
  it('is false when the address is the stored one', () => {
    expect(changesEmail(profileForm(me), me)).toBe(false);
  });

  it('ignores surrounding spaces', () => {
    expect(changesEmail({ ...profileForm(me), email: ' marie@example.com ' }, me)).toBe(false);
  });

  it('is true when another address is typed', () => {
    expect(changesEmail({ ...profileForm(me), email: 'other@example.com' }, me)).toBe(true);
  });

  it('is true when the address is cleared', () => {
    expect(changesEmail({ ...profileForm(me), email: '' }, me)).toBe(true);
  });

  it('is false without anyone signed in', () => {
    expect(changesEmail({ ...profileForm(me), email: 'other@example.com' }, null)).toBe(false);
  });
});

describe('profileProblems', () => {
  it.each<[ProfileForm, Problems<ProfileField, ProfileProblem>]>([
    [profileForm(me), {}],
    [{ ...profileForm(me), name: '   ' }, { name: 'nameRequired' }],
    [{ ...profileForm(me), name: 'a'.repeat(256) }, { name: 'nameTooLong' }],
    [{ ...profileForm(me), name: 'a'.repeat(255) }, {}],
    [
      { ...profileForm(me), email: 'not an email' },
      { email: 'emailInvalid', currentPassword: 'currentRequired' },
    ],
    [{ ...profileForm(me), email: '' }, { currentPassword: 'currentRequired' }],
    [{ ...profileForm(me), email: 'other@example.com', currentPassword: 'old' }, {}],
    [
      { ...profileForm(me), newPassword: 'abcdefgh' },
      { currentPassword: 'currentRequired', confirmPassword: 'confirmMismatch' },
    ],
    [
      {
        ...profileForm(me),
        currentPassword: 'old',
        newPassword: 'short',
        confirmPassword: 'short',
      },
      { newPassword: 'newShort' },
    ],
    [{ ...profileForm(me), currentPassword: 'old' }, { newPassword: 'newRequired' }],
    [
      {
        ...profileForm(me),
        currentPassword: 'old',
        newPassword: 'abcdefgh',
        confirmPassword: 'abcdefgh',
      },
      {},
    ],
  ])('%j gives %j', (form, want) => {
    expect(profileProblems(form, 8, me)).toEqual(want);
  });

  it('asks no current password when the address is the stored one', () => {
    expect(profileProblems(profileForm(me), 8, null)).toEqual({});
  });
});

describe('profilePatch', () => {
  it.each<[ProfileForm, ProfilePatch]>([
    [profileForm(me), {}],
    [{ ...profileForm(me), name: '  Marie D.  ' }, { name: 'Marie D.' }],
    [
      { ...profileForm(me), currentPassword: 'old', newPassword: 'abcdefgh' },
      { currentPassword: 'old', newPassword: 'abcdefgh' },
    ],
    [{ ...profileForm(me), currentPassword: 'old' }, {}],
  ])('%j gives %j', (form, want) => {
    expect(profilePatch(form, me)).toEqual(want);
  });
});

describe('profileApplied', () => {
  it.each<[ProfilePatch, boolean]>([
    [{}, true],
    [{ name: 'Marie Durand' }, true],
    [{ name: 'Other' }, false],
  ])('%j is %s', (patch, want) => {
    expect(profileApplied(patch, me)).toBe(want);
  });
});

describe('ownSubscriptions', () => {
  it('keeps subscriptions to oneself and made by oneself', () => {
    const list = [
      sub({ id: 1, user: user(7) }),
      sub({ id: 2, owner: user(7) }),
      sub({ id: 3, user: user(8) }),
      sub({ id: 4 }),
    ];
    expect(ownSubscriptions(list, 7).map((s) => s.id)).toEqual([1, 2]);
  });
});

describe('newSubscription', () => {
  it('starts as the working week, daily at 8', () => {
    expect(newSubscription()).toEqual({
      schedule: 'daily',
      days: [1, 2, 3, 4, 5],
      hour: 8,
      emptyToo: false,
    });
  });
});

describe('subscriptionInput', () => {
  it('copies the schedule of the subscription', () => {
    expect(
      subscriptionInput(sub({ schedule: 'weekly', days: [3], hour: 17, emptyToo: true })),
    ).toEqual({ schedule: 'weekly', days: [3], hour: 17, emptyToo: true });
  });

  it('takes the working week when the days are empty', () => {
    expect(subscriptionInput(sub({ days: [] })).days).toEqual([1, 2, 3, 4, 5]);
  });

  it('copies the days', () => {
    const s = sub({ days: [3] });
    subscriptionInput(s).days.push(7);
    expect(s.days).toEqual([3]);
  });
});

describe('sortedDays', () => {
  it.each([
    [
      [5, 1, 3, 1],
      [1, 3, 5],
    ],
    [[0, 8, 2], [2]],
    [[], []],
  ])('%j gives %j', (days, want) => {
    expect(sortedDays(days)).toEqual(want);
  });
});

describe('isWorkingWeek', () => {
  it.each<[number[], boolean]>([
    [[1, 2, 3, 4, 5], true],
    [[5, 4, 3, 2, 1, 1], true],
    [[1, 2, 3, 4], false],
    [[1, 2, 3, 4, 5, 6], false],
  ])('%j is %s', (days, want) => {
    expect(isWorkingWeek(days)).toBe(want);
  });
});

describe('weekdayName', () => {
  it.each<[number, string, string]>([
    [1, 'fr', 'lundi'],
    [7, 'fr', 'dimanche'],
    [1, 'en', 'Monday'],
  ])('day %i in %s is %s', (day, locale, want) => {
    expect(weekdayName(day, locale)).toBe(want);
  });

  it('writes the short name when asked', () => {
    expect(weekdayName(1, 'en', 'short')).toBe('Mon');
  });
});

describe('hourLabel', () => {
  it.each<[number, string, string]>([
    [8, 'fr', '08:00'],
    [17, 'de', '17:00'],
  ])('hour %i in %s is %s', (hour, locale, want) => {
    expect(hourLabel(hour, locale)).toBe(want);
  });

  it('writes the AM marker in English', () => {
    expect(hourLabel(8, 'en-US')).toMatch(/^08:00\sAM$/u);
  });
});
