/**
 * The setup wizard's logic, kept pure for testing: its steps, the setup
 * code as typed, password strength, the database URL a form builds.
 */

export type SetupStep = 'language' | 'code' | 'database' | 'instance' | 'start' | 'done';

/** The steps in order; the database one only when the server has none yet. */
export function setupSteps(needDatabase: boolean): SetupStep[] {
  const all: SetupStep[] = ['language', 'code', 'database', 'instance', 'start', 'done'];
  return needDatabase ? all : all.filter((s) => s !== 'database');
}

/**
 * The setup code as the person types it: letters and digits only, upper
 * case, eight at most, shown as XXXX-XXXX.
 */
export function formatCode(input: string): string {
  const raw = input
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 8);
  return raw.length > 4 ? `${raw.slice(0, 4)}-${raw.slice(4)}` : raw;
}

/** Whether a typed code is complete. */
export function codeComplete(input: string): boolean {
  return formatCode(input).replace('-', '').length === 8;
}

/** The server's rule for the administrator's password. */
export const MIN_PASSWORD = 12;

export interface PasswordCheck {
  /** 0 (unusable) to 4 (strong). */
  score: 0 | 1 | 2 | 3 | 4;
  longEnough: boolean;
  notUsername: boolean;
  /** Whether the server will take it. */
  acceptable: boolean;
}

/**
 * How strong a password is: length counts most, then the kinds of
 * characters it mixes; repeated or sequential runs and the user name inside
 * it count against it. The server only enforces the length and the user name.
 */
export function checkPassword(password: string, username: string): PasswordCheck {
  const length = [...password].length;
  const lower = password.toLowerCase();
  const user = username.trim().toLowerCase();
  const longEnough = length >= MIN_PASSWORD;
  const notUsername = password.trim().toLowerCase() !== user || user === '';
  const acceptable = longEnough && notUsername;
  if (!acceptable) return { score: length === 0 ? 0 : 1, longEnough, notUsername, acceptable };

  const kinds = [/[a-z]/, /[A-Z]/, /[0-9]/, /[^A-Za-z0-9]/].filter((re) =>
    re.test(password),
  ).length;
  const distinct = new Set(lower).size;
  let points = length >= 20 ? 3 : length >= 16 ? 2 : 1;
  points += kinds >= 3 ? 1 : 0;
  // Spaces between words make long passphrases: as good as mixed kinds.
  if (kinds < 3 && /\S\s+\S.*\s+\S/.test(password)) points += 1;
  if (distinct < length / 3 || /(.)\1{3,}/.test(lower) || hasRun(lower)) points -= 1;
  if (user.length >= 3 && lower.includes(user)) points -= 1;
  const score = Math.max(1, Math.min(4, points + 1)) as 1 | 2 | 3 | 4;
  return { score, longEnough, notUsername, acceptable };
}

/** Whether the text holds four sequential characters, like "1234" or "abcd". */
function hasRun(s: string): boolean {
  let run = 1;
  for (let i = 1; i < s.length; i++) {
    const step = s.charCodeAt(i) - s.charCodeAt(i - 1);
    run = step === 1 || step === -1 ? run + 1 : 1;
    if (run >= 4) return true;
  }
  return false;
}

/** The importer's phases, in the order it runs them (internal/importer). */
export const IMPORT_PHASES = [
  'users',
  'groups',
  'catalogues',
  'projects',
  'roles',
  'project issue types',
  'workflows',
  'permissions',
  'security levels',
  'components and versions',
  'custom fields',
  'issues',
  'issue details',
  'comments and work logs',
  'attachments',
  'custom field values',
  'history',
  'filters',
  'boards and sprints',
] as const;

export type ImportPhase = (typeof IMPORT_PHASES)[number];

/** The kind of Jira database a URL names, or null when it is neither. */
export function sourceEngine(url: string): 'postgres' | 'mysql' | null {
  const scheme = /^([a-z]+):\/\//i.exec(url.trim())?.[1]?.toLowerCase();
  if (scheme === 'postgres' || scheme === 'postgresql') return 'postgres';
  if (scheme === 'mysql' || scheme === 'mariadb') return 'mysql';
  return null;
}

/** A public address guessed from the page's own: scheme and host, no path. */
export function guessBaseURL(origin: string): string {
  try {
    const u = new URL(origin);
    return `${u.protocol}//${u.host}`;
  } catch {
    return '';
  }
}

/** SQL an administrator runs to prepare an empty database for Solstice. */
export function databaseSQL(name: string, user: string): string {
  const db = name.trim() || 'solstice';
  const who = (user.trim() || 'solstice').replace(/'/g, "''");
  return [
    `CREATE DATABASE \`${db}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
    `CREATE USER '${who}'@'%' IDENTIFIED BY '…';`,
    `GRANT ALL ON \`${db}\`.* TO '${who}'@'%';`,
  ].join('\n');
}
