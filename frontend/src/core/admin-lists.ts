/**
 * The "show" filters of the administration's lists of accounts and
 * webhooks. Their siblings live with their pages' other rules
 * (`filterProjects` in project-admin.ts, `filterTokens` in admin-ops.ts).
 */

import { matches } from './admin-forms.js';
import { deliveryTone } from './admin-integrations.js';
import type { UserRow, Webhook } from './admin-types.js';

export type UserShow = 'all' | 'active' | 'inactive' | 'admins';

export const USER_SHOWS: readonly UserShow[] = ['all', 'active', 'inactive', 'admins'];

/** The accounts a "show" choice keeps (the search itself runs on the server). */
export function filterUsers<R extends Pick<UserRow, 'active' | 'admin'>>(
  rows: readonly R[],
  show: UserShow,
): R[] {
  return rows.filter((u) => {
    if (show === 'active' && !u.active) return false;
    if (show === 'inactive' && u.active) return false;
    if (show === 'admins' && !u.admin) return false;
    return true;
  });
}

export type WebhookShow = 'all' | 'enabled' | 'disabled' | 'failing';

export const WEBHOOK_SHOWS: readonly WebhookShow[] = ['all', 'enabled', 'disabled', 'failing'];

/** Whether a webhook's last call failed. */
export function webhookFailing(w: Pick<Webhook, 'lastDelivery'>): boolean {
  return !!w.lastDelivery && deliveryTone(w.lastDelivery) === 'failed';
}

/** The webhooks a search on name and address and a "show" choice keep. */
export function filterWebhooks<
  R extends Pick<Webhook, 'name' | 'url' | 'enabled' | 'lastDelivery'>,
>(rows: readonly R[], q: string, show: WebhookShow): R[] {
  return rows.filter((w) => {
    if (show === 'enabled' && !w.enabled) return false;
    if (show === 'disabled' && w.enabled) return false;
    if (show === 'failing' && !webhookFailing(w)) return false;
    return matches(q, w.name, w.url);
  });
}
