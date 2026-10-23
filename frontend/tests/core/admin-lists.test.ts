import { describe, expect, it } from 'vitest';

import {
  filterUsers,
  filterWebhooks,
  USER_SHOWS,
  WEBHOOK_SHOWS,
  webhookFailing,
} from '../../src/core/admin-lists.js';
import type { WebhookDelivery } from '../../src/core/admin-types.js';

describe('the accounts list', () => {
  const users = [
    { id: 1, active: true, admin: true },
    { id: 2, active: true, admin: false },
    { id: 3, active: false, admin: false },
    { id: 4, active: false, admin: true },
  ];
  const ids = (show: (typeof USER_SHOWS)[number]) => filterUsers(users, show).map((u) => u.id);

  it('keeps the accounts a choice shows', () => {
    expect(ids('all')).toEqual([1, 2, 3, 4]);
    expect(ids('active')).toEqual([1, 2]);
    expect(ids('inactive')).toEqual([3, 4]);
    // A deactivated administrator still counts as one.
    expect(ids('admins')).toEqual([1, 4]);
    expect(USER_SHOWS).toContain('all');
  });
});

describe('the webhooks list', () => {
  const delivery = (over: Partial<WebhookDelivery>): WebhookDelivery => ({
    id: 1,
    event: 'issue_created',
    status: 'sent',
    statusCode: 200,
    durationMs: 10,
    attempts: 1,
    error: null,
    created: '2026-10-01T10:00:00Z',
    attempted: '2026-10-01T10:00:00Z',
    nextAttempt: null,
    ...over,
  });
  const hooks = [
    { id: 1, name: 'Chat', url: 'https://chat.example/hook', enabled: true, lastDelivery: null },
    {
      id: 2,
      name: 'Build',
      url: 'https://ci.example/trigger',
      enabled: true,
      lastDelivery: delivery({ statusCode: 500 }),
    },
    {
      id: 3,
      name: 'Archive',
      url: 'https://archive.example/in',
      enabled: false,
      lastDelivery: delivery({ status: 'failed', statusCode: null }),
    },
    {
      id: 4,
      name: 'Mirror',
      url: 'https://mirror.example/in',
      enabled: true,
      lastDelivery: delivery({ status: 'pending', statusCode: null }),
    },
  ];
  const ids = (q: string, show: (typeof WEBHOOK_SHOWS)[number]) =>
    filterWebhooks(hooks, q, show).map((w) => w.id);

  it('keeps the webhooks a choice shows', () => {
    expect(ids('', 'all')).toEqual([1, 2, 3, 4]);
    expect(ids('', 'enabled')).toEqual([1, 2, 4]);
    expect(ids('', 'disabled')).toEqual([3]);
    // A call answered with an error status failed; one still waiting did not.
    expect(ids('', 'failing')).toEqual([2, 3]);
    expect(WEBHOOK_SHOWS).toContain('all');
  });

  it('searches the name and the address, with the choice', () => {
    expect(ids('build', 'all')).toEqual([2]);
    expect(ids('EXAMPLE/IN', 'all')).toEqual([3, 4]);
    expect(ids('example/in', 'enabled')).toEqual([4]);
    expect(ids('nowhere', 'all')).toEqual([]);
  });

  it('counts a webhook never called as not failing', () => {
    expect(webhookFailing({ lastDelivery: null })).toBe(false);
  });
});
