/**
 * Form logic of the personal settings (/settings): the profile form, the
 * subscriptions one owns, and the wording helpers for hours and weekdays
 * (built on Intl, so every language gets its own names). Problems are keys;
 * the pages word them (src/i18n, block `account`).
 */

import { isEmail, tooLong, type Problems } from './admin-forms.js';
import type { Subscription, SubscriptionInput } from './admin-types.js';
import type { Me } from './types.js';

/* Profile -------------------------------------------------------------------- */

export interface ProfileForm {
  name: string;
  email: string;
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export type ProfileProblem =
  | 'nameRequired'
  | 'nameTooLong'
  | 'emailInvalid'
  | 'currentRequired'
  | 'newRequired'
  | 'newShort'
  | 'confirmMismatch';

export type ProfileField = 'name' | 'email' | 'currentPassword' | 'newPassword' | 'confirmPassword';

export function profileForm(me: Me): ProfileForm {
  return {
    name: me.name,
    email: me.email,
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  };
}

/**
 * Whether the form asks for a new password: the new one or its confirmation
 * typed. The current one alone does not count, as password managers fill it.
 */
export function changesPassword(f: ProfileForm): boolean {
  return f.newPassword !== '' || f.confirmPassword !== '';
}

/** Whether the typed address differs from the stored one. */
export function changesEmail(f: ProfileForm, me: Me | null): boolean {
  return me !== null && f.email.trim() !== me.email;
}

export function profileProblems(
  f: ProfileForm,
  minPassword: number,
  me: Me | null,
): Problems<ProfileField, ProfileProblem> {
  const out: Problems<ProfileField, ProfileProblem> = {};
  const name = f.name.trim();
  if (!name) out.name = 'nameRequired';
  else if (tooLong(name)) out.name = 'nameTooLong';
  const email = f.email.trim();
  if (email && !isEmail(email)) out.email = 'emailInvalid';
  // Changing the address, like the password, takes the current one.
  const mailChanged = changesEmail(f, me);
  if (changesPassword(f) || mailChanged) {
    if (!f.currentPassword) out.currentPassword = 'currentRequired';
  }
  if (changesPassword(f)) {
    // A confirmation typed on its own means a new password is wanted, unless
    // the address change is what is saved.
    if (!f.newPassword && !mailChanged) out.newPassword = 'newRequired';
    else if (f.newPassword && [...f.newPassword].length < minPassword) out.newPassword = 'newShort';
    if (f.newPassword && f.confirmPassword !== f.newPassword)
      out.confirmPassword = 'confirmMismatch';
  }
  return out;
}

export interface ProfilePatch {
  name?: string;
  email?: string;
  currentPassword?: string;
  newPassword?: string;
}

/** What to send: only what changed, the password pair only when a new one is typed. */
export function profilePatch(f: ProfileForm, me: Me): ProfilePatch {
  const out: ProfilePatch = {};
  if (f.name.trim() !== me.name) out.name = f.name.trim();
  const mailChanged = changesEmail(f, me);
  if (mailChanged) out.email = f.email.trim();
  // The server asks for the current password when the address changes.
  if (mailChanged || f.newPassword) out.currentPassword = f.currentPassword;
  if (f.newPassword) out.newPassword = f.newPassword;
  return out;
}

/* Subscriptions ---------------------------------------------------------------- */

/** One's own subscriptions: to oneself, or made by oneself (a group's). */
export function ownSubscriptions(list: Subscription[], me: number): Subscription[] {
  return list.filter((s) => s.user?.id === me || s.owner?.id === me);
}

export function newSubscription(): SubscriptionInput {
  return { schedule: 'daily', days: [1, 2, 3, 4, 5], hour: 8, emptyToo: false };
}

export function subscriptionInput(s: Subscription): SubscriptionInput {
  return {
    schedule: s.schedule,
    days: s.days.length ? [...s.days] : [1, 2, 3, 4, 5],
    hour: s.hour,
    emptyToo: s.emptyToo,
  };
}

/** ISO weekdays, Monday first. */
export const WEEKDAYS = [1, 2, 3, 4, 5, 6, 7] as const;

/** A weekday's name in a language: 1 = Monday. */
export function weekdayName(day: number, locale: string, style: 'long' | 'short' = 'long'): string {
  // 1 January 2024 was a Monday.
  const date = new Date(Date.UTC(2024, 0, day, 12));
  return new Intl.DateTimeFormat(locale, { weekday: style, timeZone: 'UTC' }).format(date);
}

/** An hour of the day as the language writes it: "08:00", "8:00 AM". */
export function hourLabel(hour: number, locale: string): string {
  const date = new Date(Date.UTC(2024, 0, 1, hour, 0));
  return new Intl.DateTimeFormat(locale, {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'UTC',
  }).format(date);
}

/** The days of a weekly schedule, in week order, without repeats. */
export function sortedDays(days: readonly number[]): number[] {
  return [...new Set(days)].filter((d) => d >= 1 && d <= 7).sort((a, b) => a - b);
}

/** Whether a weekly schedule is every working day (Monday to Friday). */
export function isWorkingWeek(days: readonly number[]): boolean {
  return sortedDays(days).join() === '1,2,3,4,5';
}
