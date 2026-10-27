/**
 * The instance's public settings (GET /api/settings/public, no sign-in
 * needed): its name, the announcement banner, the shortest password
 * allowed, the single sign-on button. Loaded once; the administration reloads them after a change.
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
  /** Single sign-on (OpenID Connect), null when off; absent on older servers. */
  sso?: { name: string } | null;
}

/** Used when the call fails: the endpoint exists in every server build. */
const FALLBACK: PublicSettings = {
  instanceName: '',
  defaultLanguage: 'en',
  banner: null,
  // The server's defaultPasswordMin owns this value.
  passwordMinLength: 12,
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
      // Failed call (offline, proxy error): keep the fallback.
    } finally {
      this.loaded = true;
    }
  }
}

export const publicSettings = new PublicSettingsStore();
