// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { Webhook } from '../../src/core/admin-types.js';
import WebhooksEdit from '../../src/ui/admin/WebhooksEdit.svelte';

const hook: Webhook = {
  id: 1,
  name: 'Chat',
  url: 'https://example.com/hook',
  events: ['jira:issue_created'],
  jql: '',
  hasSecret: false,
  enabled: true,
  excludeBody: false,
  created: '2026-01-01T00:00:00Z',
  updated: '2026-01-01T00:00:00Z',
  lastDelivery: null,
};

const adminApi = vi.hoisted(() => ({
  webhookEvents: vi.fn(),
  webhook: vi.fn(),
  deliveries: vi.fn(),
}));

vi.mock('../../src/lib/admin-api.js', () => ({ adminApi }));

vi.mock('../../src/lib/api.js', () => ({
  api: { issues: () => Promise.resolve({ total: 0, issues: [] }) },
  errorMessage: String,
}));

vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    disconnect() {}
  },
);

let component: ReturnType<typeof mount> | undefined;

beforeEach(() => {
  adminApi.webhookEvents.mockResolvedValue([
    'jira:issue_created',
    'jira:issue_updated',
    'jira:issue_deleted',
  ]);
  adminApi.webhook.mockResolvedValue(hook);
  adminApi.deliveries.mockResolvedValue([]);
});

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  vi.clearAllMocks();
});

async function settle() {
  for (let i = 0; i < 10; i++) await tick();
  flushSync();
}

describe('WebhooksEdit', () => {
  it('loads a new webhook once, and typing does not reload it', async () => {
    component = mount(WebhooksEdit, { target: document.body, props: { id: null } });
    await settle();
    expect(adminApi.webhookEvents).toHaveBeenCalledTimes(1);

    const name = document.getElementById('hook-name') as HTMLInputElement;
    name.value = 'Chat';
    name.dispatchEvent(new Event('input', { bubbles: true }));
    const deleted = [...document.querySelectorAll<HTMLInputElement>('input[type=checkbox]')].find(
      (c) => c.closest('label')?.textContent?.includes('jira:issue_deleted'),
    );
    deleted?.click();
    await settle();

    expect(deleted?.checked).toBe(true);
    expect(adminApi.webhookEvents).toHaveBeenCalledTimes(1);
    expect((document.getElementById('hook-name') as HTMLInputElement).value).toBe('Chat');
  });

  it('loads an existing webhook once', async () => {
    component = mount(WebhooksEdit, { target: document.body, props: { id: 1 } });
    await settle();
    expect(adminApi.webhook).toHaveBeenCalledTimes(1);
    expect(adminApi.webhook).toHaveBeenCalledWith(1);
    expect(adminApi.webhookEvents).toHaveBeenCalledTimes(1);
  });
});
