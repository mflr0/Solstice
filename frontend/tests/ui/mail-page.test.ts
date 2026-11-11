// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick, unmount } from 'svelte';

import type { MailSettings } from '../../src/core/admin-types.js';
import MailPage from '../../src/ui/admin/MailPage.svelte';

const settings: MailSettings = {
  enabled: true,
  host: 'smtp.example.org',
  port: 587,
  security: 'starttls',
  username: 'relay',
  from: 'solstice@example.org',
  fromName: '',
  passwordSet: true,
  baseUrl: null,
  fromConfig: ['host', 'port', 'security', 'username', 'password'],
};

const adminApi = vi.hoisted(() => ({
  mail: vi.fn(),
  saveMail: vi.fn(),
  mailQueue: vi.fn(),
}));

vi.mock('../../src/lib/admin-api.js', () => ({ adminApi }));

vi.mock('../../src/lib/api.js', () => ({ errorMessage: String }));

vi.mock('../../src/lib/public-settings.svelte.js', () => ({
  publicSettings: { load: () => Promise.resolve() },
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
  adminApi.mail.mockResolvedValue(settings);
  adminApi.saveMail.mockResolvedValue(settings);
  adminApi.mailQueue.mockResolvedValue({
    pending: 0,
    failed: 0,
    sentLastDay: 0,
    batching: 0,
    digests: 0,
    oldestPending: null,
    recent: [],
    errors: [],
  });
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

describe('MailPage', () => {
  it('shows the fields of the configuration file read-only and saves only the others', async () => {
    component = mount(MailPage, { target: document.body });
    await settle();

    const form = document.querySelector<HTMLFormElement>('form.settings-form')!;
    const values = [...form.querySelectorAll<HTMLInputElement>('input')].map((i) => i.value);
    expect(values).not.toContain('smtp.example.org');
    expect(values).not.toContain('relay');
    expect(form.querySelectorAll('input[type=radio], input[type=password]')).toHaveLength(0);
    expect(form.textContent).toContain('smtp.example.org');
    expect(form.textContent).toContain('SOLSTICE_MAIL_HOST');

    const from = form.querySelector<HTMLInputElement>('input[type=email]')!;
    from.value = 'tickets@example.org';
    from.dispatchEvent(new Event('input', { bubbles: true }));
    await settle();
    form.requestSubmit();
    await settle();

    expect(adminApi.saveMail).toHaveBeenCalledWith({
      enabled: true,
      from: 'tickets@example.org',
      fromName: '',
    });
  });
});
