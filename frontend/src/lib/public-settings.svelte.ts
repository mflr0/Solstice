/**
 * The instance's public settings (GET /api/settings/public, no sign-in
 * needed): its name, the announcement banner, the shortest password
 * allowed. Loaded once; the administration reloads them after a change.
 */

import { request } from './api.js';

export interface Banner {
  text: string;
  level: 'info' | 'warning';
}

export interface PublicSettings {
  instanceName: string;
  defaultLanguage: string;
  banner: Banner | null;
  passwordMinLength: number;
  /** Whether links can be sent by e-mail: reset ("forgot your password") and invitations. */
  emailLinks: boolean;
}

/** What an older server, without the endpoint, amounts to. */
const FALLBACK: PublicSettings = {
  instanceName: '',
  defaultLanguage: 'en',
  banner: null,
  passwordMinLength: 10,
  emailLinks: false,
};

class PublicSettingsStore {
  value = $state<PublicSettings>(FALLBACK);
  loaded = $state(false);

  async load(): Promise<void> {
    try {
      this.value = {
        ...FALLBACK,
        ...(await request<PublicSettings>('GET', '/api/settings/public')),
      };
    } catch {
      // An older server, or offline: no banner, the built-in name.
    } finally {
      this.loaded = true;
    }
  }
}

export const publicSettings = new PublicSettingsStore();
