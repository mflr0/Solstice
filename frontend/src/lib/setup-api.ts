/**
 * The setup wizard's API (/api/setup/*), typed. Like the application's
 * client it sends the X-Solstice header on writes and asks for messages in
 * the wizard's language; it stands alone so the wizard loads nothing of the
 * application.
 */

import type { ImportCheck, ImportSource, ImportView } from '../core/admin-types.js';
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
  database?: { configured: boolean; server?: string };
  dataDir?: string;
  configPath?: string;
  baseURL?: string;
  tz?: string;
  imported?: boolean;
  import?: ImportView;
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

const settings = { locale: 'en' as Locale, network: '', generic: '' };

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
  testSource: (f: ImportSource) => call<ImportCheck>('POST', '/api/setup/source/test', f),
  startImport: (f: ImportSource) => call<ImportView>('POST', '/api/setup/import', f),
  importState: () => call<ImportView>('GET', '/api/setup/import'),
  accounts: (q: string) =>
    call<SetupAccount[]>('GET', `/api/setup/accounts?q=${encodeURIComponent(q)}`),
  finish: (f: AdminForm) => call<SetupFinished>('POST', '/api/setup/admin', f),
};

/** The message of a failure, or the generic one configureSetupApi stored. */
export function setupErrorMessage(e: unknown, generic = settings.generic): string {
  return e instanceof Error && e.message ? e.message : generic;
}
