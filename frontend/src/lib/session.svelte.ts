/**
 * Who is signed in, and the catalogues (statuses, priorities…) every page
 * needs. Loaded once, after sign-in.
 */

import type { Me, Meta, PriorityRef } from '../core/types.js';
import { api, ApiError, setUnauthorizedHandler } from './api.js';
import { router } from './router.svelte.js';
import { paths } from '../core/route.js';

class Session {
  me = $state<Me | null>(null);
  meta = $state<Meta | null>(null);
  ready = $state(false);

  async init(): Promise<void> {
    setUnauthorizedHandler(() => this.expired());
    try {
      this.me = await api.me();
      this.meta = await api.meta();
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 401) throw e;
      this.me = null;
    } finally {
      this.ready = true;
    }
  }

  async signedIn(me: Me): Promise<void> {
    this.me = me;
    this.meta = await api.meta();
  }

  async signOut(): Promise<void> {
    try {
      await api.logout();
    } finally {
      this.me = null;
      router.navigate(paths.login());
    }
  }

  private expired(): void {
    if (this.me === null && router.route.name === 'login') return;
    this.me = null;
    if (router.route.name !== 'login') {
      router.navigate(paths.login(router.here), { replace: true });
    }
  }

  /** Index of a priority in the ordered list (most urgent first). */
  priorityIndex(p: PriorityRef): number {
    return this.meta?.priorities.findIndex((x) => x.id === p.id) ?? -1;
  }
}

export const session = new Session();
