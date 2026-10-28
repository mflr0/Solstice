/**
 * Who is signed in, and the catalogues (statuses, priorities…) every page
 * needs. Loaded once, after sign-in.
 */

import type { Me, Meta, PriorityRef } from '../core/types.js';
import { accountApi } from './admin-api.js';
import { api, ApiError, setUnauthorizedHandler } from './api.js';
import { adoptAccountLocale } from './locale.js';
import { router } from './router.svelte.js';
import { isPublicRoute, paths } from '../core/route.js';

class Session {
  me = $state<Me | null>(null);
  meta = $state<Meta | null>(null);
  ready = $state(false);

  async init(): Promise<void> {
    setUnauthorizedHandler(() => this.expired());
    // The account speaks another language: the page reloads in it, still booting.
    let reloading = false;
    try {
      this.me = await api.me();
      reloading = adoptAccountLocale(this.me.locale);
      if (!reloading) this.meta = await api.meta();
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 401) throw e;
      this.me = null;
    } finally {
      if (!reloading) this.ready = true;
    }
  }

  async signedIn(me: Me): Promise<void> {
    if (adoptAccountLocale(me.locale)) return;
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
    if (this.me === null && isPublicRoute(router.route)) return;
    this.me = null;
    if (!isPublicRoute(router.route)) {
      router.navigate(paths.login(router.here), { replace: true });
    }
  }

  /** The key of the project the pages are scoped to, null for every project. */
  get scope(): string | null {
    return this.me?.project?.key ?? null;
  }

  /** Scopes the pages to a project (its key), or to every project (null). */
  async setScope(key: string | null): Promise<void> {
    this.me = await accountApi.updateProfile({ project: key });
  }

  /** Index of a priority in the ordered list (most urgent first). */
  priorityIndex(p: PriorityRef): number {
    return this.meta?.priorities.findIndex((x) => x.id === p.id) ?? -1;
  }
}

export const session = new Session();
