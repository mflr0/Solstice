/**
 * What wiki text refers to — people's display names and which issue keys
 * exist — fetched in batches and kept for the session. Rendering reads this
 * state, so text re-renders with real names and links once they arrive.
 */

import type { WikiRefs, WikiResolver } from '../core/wiki.js';
import { api } from './api.js';

class RefCache {
  users = $state<Record<string, string | null>>({});
  issues = $state<Record<string, boolean>>({});
  private wantedUsers = new Set<string>();
  private wantedKeys = new Set<string>();
  private timer: ReturnType<typeof setTimeout> | undefined;

  /** Asks for whatever is not known yet; requests are grouped. */
  ensure(refs: WikiRefs): void {
    for (const u of refs.users) {
      const k = u.toLowerCase();
      if (!(k in this.users)) this.wantedUsers.add(k);
    }
    for (const key of refs.issueKeys) {
      const k = key.toUpperCase();
      if (!(k in this.issues)) this.wantedKeys.add(k);
    }
    if (this.wantedUsers.size + this.wantedKeys.size === 0 || this.timer) return;
    this.timer = setTimeout(() => void this.flush(), 20);
  }

  private async flush(): Promise<void> {
    this.timer = undefined;
    const users = [...this.wantedUsers];
    const keys = [...this.wantedKeys];
    this.wantedUsers.clear();
    this.wantedKeys.clear();
    try {
      const found = await api.refs(users, keys);
      for (const u of users) this.users[u] = found.users[u] ?? null;
      for (const k of keys) this.issues[k] = found.issues[k] === true;
    } catch {
      // Unresolved references simply stay as plain text.
    }
  }

  /** A resolver for one piece of text, attachments given by file name. */
  resolver(attachments: ReadonlyMap<string, string>): WikiResolver {
    return {
      user: (username) => this.users[username.toLowerCase()] ?? null,
      attachment: (filename) => attachments.get(filename) ?? null,
      issueExists: (key) => this.issues[key.toUpperCase()] === true,
    };
  }
}

export const refs = new RefCache();
