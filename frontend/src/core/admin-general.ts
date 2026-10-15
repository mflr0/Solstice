/**
 * The instance's general settings and banner as forms: checks before
 * sending, and the change to send (only what differs from what is stored).
 */

import type { Problems } from './admin-forms.js';
import type {
  BannerSettings,
  GeneralSettings,
  GeneralSettingsInput,
  SettingSource,
} from './admin-types.js';

export const NAME_MAX = 100;
export const BANNER_MAX = 500;
export const SESSION_DAYS = { min: 1, max: 365 } as const;
export const PASSWORD_LENGTH = { min: 8, max: 128 } as const;

export interface GeneralForm {
  instanceName: string;
  baseURL: string;
  timeZone: string;
  defaultLanguage: string;
  sessionDays: string;
  passwordMinLength: string;
}

export type GeneralProblem =
  | 'nameRequired'
  | 'nameTooLong'
  | 'baseUrlInvalid'
  | 'timeZoneUnknown'
  | 'sessionDaysRange'
  | 'passwordRange';

export function generalForm(s: GeneralSettings): GeneralForm {
  return {
    instanceName: s.instanceName,
    baseURL: s.baseURL,
    timeZone: s.timeZone,
    defaultLanguage: s.defaultLanguage,
    sessionDays: String(s.sessionDays),
    passwordMinLength: String(s.passwordMinLength),
  };
}

/** A public address: http(s)://host[:port], nothing after but an optional "/". */
export function isBaseURL(s: string): boolean {
  const v = s.trim();
  if (!/^https?:\/\/[^/?#\s]+\/?$/i.test(v)) return false;
  try {
    const u = new URL(v);
    return u.hostname !== '' && u.username === '' && u.password === '';
  } catch {
    return false;
  }
}

/** The address as stored: no trailing slash. */
export function cleanBaseURL(s: string): string {
  return s.trim().replace(/\/$/, '');
}

function inRange(s: string, r: { min: number; max: number }): boolean {
  const n = Number(s);
  return s.trim() !== '' && Number.isInteger(n) && n >= r.min && n <= r.max;
}

/**
 * Problems of the general form. `zones` are the time zones the browser
 * knows (null: it cannot list them, the server checks).
 */
export function generalProblems(
  f: GeneralForm,
  zones: readonly string[] | null,
): Problems<keyof GeneralForm, GeneralProblem> {
  const out: Problems<keyof GeneralForm, GeneralProblem> = {};
  const name = f.instanceName.trim();
  if (!name) out.instanceName = 'nameRequired';
  else if ([...name].length > NAME_MAX) out.instanceName = 'nameTooLong';
  if (f.baseURL.trim() && !isBaseURL(f.baseURL)) out.baseURL = 'baseUrlInvalid';
  const tz = f.timeZone.trim();
  if (tz && zones && zones.length > 0 && !zones.includes(tz) && tz !== 'UTC') {
    out.timeZone = 'timeZoneUnknown';
  }
  if (!inRange(f.sessionDays, SESSION_DAYS)) out.sessionDays = 'sessionDaysRange';
  if (!inRange(f.passwordMinLength, PASSWORD_LENGTH)) out.passwordMinLength = 'passwordRange';
  return out;
}

/** The fields that changed, ready to send. */
export function generalPatch(f: GeneralForm, s: GeneralSettings): GeneralSettingsInput {
  const out: GeneralSettingsInput = {};
  const name = f.instanceName.trim();
  if (name !== s.instanceName) out.instanceName = name;
  const url = cleanBaseURL(f.baseURL);
  if (url !== s.baseURL) out.baseURL = url;
  const tz = f.timeZone.trim();
  if (tz !== s.timeZone) out.timeZone = tz;
  if (f.defaultLanguage !== s.defaultLanguage) out.defaultLanguage = f.defaultLanguage;
  if (Number(f.sessionDays) !== s.sessionDays) out.sessionDays = Number(f.sessionDays);
  if (Number(f.passwordMinLength) !== s.passwordMinLength) {
    out.passwordMinLength = Number(f.passwordMinLength);
  }
  return out;
}

/** Whether a field's value can be put back to what it inherits (file, environment, default). */
export function canReset(source: SettingSource | undefined): boolean {
  return source === 'database';
}

/* Banner --------------------------------------------------------------------- */

export type BannerProblem = 'textRequired' | 'textTooLong';

export function bannerProblems(b: BannerSettings): Problems<'text', BannerProblem> {
  const n = [...b.text.trim()].length;
  if (n > BANNER_MAX) return { text: 'textTooLong' };
  if (b.enabled && n === 0) return { text: 'textRequired' };
  return {};
}

export function sameBanner(a: BannerSettings, b: BannerSettings): boolean {
  return a.enabled === b.enabled && a.level === b.level && a.text.trim() === b.text.trim();
}

/* Import --------------------------------------------------------------------- */

/** Whether the typed confirmation names the database (case and spaces matter not). */
export function importConfirmed(typed: string, expected: string): boolean {
  return expected !== '' && typed.trim().toLowerCase() === expected.trim().toLowerCase();
}

/** Progress of a running import, 0 to 1. */
export function importProgress(done: number, total: number): number {
  return total > 0 ? Math.min(1, Math.max(0, done / total)) : 0;
}
