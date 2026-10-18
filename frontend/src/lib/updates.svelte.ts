/**
 * Solstice updates, for global administrators: the latest release on
 * GitHub, and on a Linux (systemd) installation, updating in place through
 * the root helper. See backend/internal/api/updates.go.
 */

import { request } from './api.js';

export type InstallKind = 'systemd' | 'docker' | 'manual';
export type AttemptStatus = 'pending' | 'running' | 'ok' | 'rolled_back' | 'failed';
export type CheckError = 'unreachable' | 'refused' | 'invalid';

export interface UpdateAttempt {
  at: string;
  from: string;
  to: string;
  status: AttemptStatus;
  /** The helper's log, in English, as it wrote it. */
  log: string;
}

export interface UpdateState {
  current: string;
  latest: string | null;
  newer: boolean;
  /** Markdown, as written on GitHub: shown as text (core/release-notes.ts). */
  notes: string;
  publishedAt: string | null;
  releaseUrl: string;
  checkedAt: string | null;
  checkError: CheckError | null;
  checksEnabled: boolean;
  install: InstallKind;
  canApply: boolean;
  /** Docker and manual installations: the commands to run. */
  commands: string[];
  /** Manual installations: the archive for this machine. */
  download: string;
  lastAttempt: UpdateAttempt | null;
}

export const updatesApi = {
  state: () => request<UpdateState>('GET', '/api/admin/updates'),
  check: () => request<UpdateState>('POST', '/api/admin/updates/check'),
  setChecks: (checksEnabled: boolean) =>
    request<UpdateState>('PUT', '/api/admin/updates/settings', { checksEnabled }),
  apply: (version: string, password: string) =>
    request<UpdateState>('POST', '/api/admin/updates/apply', { version, password }),
};

/** Whether an update waits for the helper or is running. */
export function inProgress(a: UpdateAttempt | null): boolean {
  return a?.status === 'pending' || a?.status === 'running';
}

/** The account menu's notice: a newer release, for administrators. */
class UpdateNotice {
  latest = $state<string | null>(null);
  private loading: Promise<void> | null = null;

  /** Once per page load; the server asks GitHub at most every 12 hours. */
  load(): Promise<void> {
    this.loading ??= updatesApi.state().then(
      (s) => this.set(s),
      () => {
        // The notice is a convenience: no error to show for it.
      },
    );
    return this.loading;
  }

  set(s: UpdateState): void {
    this.latest = s.newer ? s.latest : null;
  }
}

export const updateNotice = new UpdateNotice();
