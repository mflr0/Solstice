/**
 * The setup wizard's API (/api/setup/*), typed. Like the application's
 * client it sends the X-Solstice header on writes and asks for messages in
 * the wizard's language; it stands alone so the wizard loads nothing of the
 * application.
 */

import type { Locale } from '../i18n/locales.js';

export interface ConfigSave {
  path: string;
  written: boolean;
  /** When the file could not be written: the lines to add by hand. */
  lines?: string;
  error?: string;
}

export interface SetupStatus {
  setup: boolean;
  unlocked: boolean;
  version: string;
  /** Absent once the instance is set up. */
  database?: { configured: boolean; fixed: boolean; server?: string };
  dataDir?: string;
  configPath?: string;
  baseURL?: string;
  baseURLFixed?: boolean;
  tz?: string;
  tzFixed?: boolean;
  imported?: boolean;
  import?: ImportState;
  config?: ConfigSave;
}

export interface DatabaseForm {
  host: string;
  port: number;
  name: string;
  user: string;
  password: string;
}

export interface DatabaseProbe {
  server: string;
  tables: number;
  accounts: number;
}

export interface DatabaseSaved {
  server: string;
  config: ConfigSave;
  /** The database held an instance already: setup is over. */
  existing: boolean;
}

export interface InstanceSaved {
  baseURL: string;
  tz: string;
  config: ConfigSave;
}

export interface SourceForm {
  url: string;
  jiraHome: string;
  tz: string;
}

export interface SourceProbe {
  engine: 'postgres' | 'mysql';
  projects: number;
  issues: number;
  users: number;
  attachments: boolean;
  warning?: string;
}

export interface ImportStep {
  name: string;
  imported: number;
  skipped: number;
}

export interface ImportState {
  state: 'running' | 'done' | 'failed';
  phase: string;
  done: number;
  total: number;
  rows: number;
  steps: ImportStep[];
  warnings: string[];
  error?: string;
  seconds: number;
}

export interface SetupAccount {
  username: string;
  name: string;
  email: string;
  /** Administrator in Jira already. */
  admin: boolean;
}

export type AdminForm =
  { username: string; name: string; email: string; password: string } | { existing: string };

export interface SetupFinished {
  username: string;
  baseURL: string;
  tz: string;
  imported: boolean;
  server: string;
  config: ConfigSave | null;
}

export class SetupError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

const settings = { locale: 'fr' as Locale, network: '', generic: '' };

/** The language answers come in, and the messages for failures without one. */
export function configureSetupApi(locale: Locale, network: string, generic: string): void {
  Object.assign(settings, { locale, network, generic });
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Accept-Language': settings.locale,
  };
  if (method !== 'GET') headers['X-Solstice'] = '1';
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: 'same-origin',
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
  } catch {
    throw new SetupError(settings.network, 0);
  }
  if (!res.ok) {
    let message = settings.generic;
    try {
      const data: unknown = await res.json();
      if (data && typeof data === 'object' && 'error' in data && typeof data.error === 'string') {
        message = data.error;
      }
    } catch {
      // Not JSON: keep the generic message.
    }
    throw new SetupError(message, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const setupApi = {
  status: () => call<SetupStatus>('GET', '/api/setup/status'),
  unlock: (code: string) => call<void>('POST', '/api/setup/unlock', { code }),
  testDatabase: (f: DatabaseForm) => call<DatabaseProbe>('POST', '/api/setup/database/test', f),
  saveDatabase: (f: DatabaseForm) => call<DatabaseSaved>('POST', '/api/setup/database', f),
  saveInstance: (baseURL: string, tz: string) =>
    call<InstanceSaved>('POST', '/api/setup/instance', { baseURL, tz }),
  testSource: (f: SourceForm) => call<SourceProbe>('POST', '/api/setup/source/test', f),
  startImport: (f: SourceForm) => call<ImportState>('POST', '/api/setup/import', f),
  importState: () => call<ImportState>('GET', '/api/setup/import'),
  accounts: (q: string) =>
    call<SetupAccount[]>('GET', `/api/setup/accounts?q=${encodeURIComponent(q)}`),
  finish: (f: AdminForm) => call<SetupFinished>('POST', '/api/setup/admin', f),
};

export function setupErrorMessage(e: unknown, generic: string): string {
  return e instanceof Error && e.message ? e.message : generic;
}
