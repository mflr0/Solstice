// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { failureMessage, retrySeconds } from '../../src/core/failure.js';
import { en } from '../../src/i18n/en.js';
import { en as setupEn } from '../../src/i18n/setup/en.js';
import { ApiError, errorMessage, request, UserError } from '../../src/lib/api.js';
import {
  configureSetupApi,
  SetupError,
  setupApi,
  setupErrorMessage,
} from '../../src/lib/setup-api.js';

const html = '<!doctype html><html><body>Bad Gateway</body></html>';

/** The answers fetch gives, by case: a status, a body and headers, or a rejection. */
const cases: {
  name: string;
  answer: () => Promise<Response>;
  status: number;
  app: string;
  setup: string;
}[] = [
  {
    name: 'a rejected fetch',
    answer: () => Promise.reject(new TypeError('Failed to fetch')),
    status: 0,
    app: en.errors.network,
    setup: setupEn.errors.network,
  },
  {
    name: 'a 502 with an HTML body',
    answer: async () =>
      new Response(html, { status: 502, headers: { 'Content-Type': 'text/html' } }),
    status: 502,
    app: en.errors.network,
    setup: setupEn.errors.network,
  },
  {
    name: 'a 401',
    answer: async () => new Response(null, { status: 401 }),
    status: 401,
    app: en.errors.sessionEnded,
    setup: setupEn.errors.sessionEnded,
  },
  {
    name: 'a 403',
    answer: async () => new Response('Forbidden', { status: 403 }),
    status: 403,
    app: en.errors.forbidden,
    setup: setupEn.errors.forbidden,
  },
  {
    name: 'a 404',
    answer: async () => new Response(html, { status: 404 }),
    status: 404,
    app: en.errors.missing,
    setup: setupEn.errors.missing,
  },
  {
    name: 'a 413',
    answer: async () => new Response(html, { status: 413 }),
    status: 413,
    app: en.errors.tooLarge,
    setup: setupEn.errors.tooLarge,
  },
  {
    name: 'a 429 with Retry-After',
    answer: async () => new Response(null, { status: 429, headers: { 'Retry-After': '30' } }),
    status: 429,
    app: en.errors.tooMany(30),
    setup: setupEn.errors.tooMany(30),
  },
  {
    name: 'a 500 without a body',
    answer: async () => new Response(null, { status: 500 }),
    status: 500,
    app: en.errors.server,
    setup: setupEn.errors.server,
  },
  {
    name: 'a 400 with a JSON {error}',
    answer: async () =>
      new Response(JSON.stringify({ error: 'The summary is required.' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      }),
    status: 400,
    app: 'The summary is required.',
    setup: 'The summary is required.',
  },
  {
    name: 'a 503 with a JSON {error}',
    answer: async () =>
      new Response(JSON.stringify({ error: 'Down for maintenance.' }), { status: 503 }),
    status: 503,
    app: 'Down for maintenance.',
    setup: 'Down for maintenance.',
  },
  {
    name: 'a 200 with an HTML body',
    answer: async () =>
      new Response(html, { status: 200, headers: { 'Content-Type': 'text/html' } }),
    status: 200,
    app: en.errors.network,
    setup: setupEn.errors.network,
  },
];

async function failure(p: Promise<unknown>): Promise<unknown> {
  try {
    await p;
  } catch (e) {
    return e;
  }
  throw new Error('the request did not fail');
}

beforeAll(() => configureSetupApi('en', setupEn.errors));

afterEach(() => vi.unstubAllGlobals());

describe('request', () => {
  for (const c of cases) {
    it(`gives a readable message for ${c.name}`, async () => {
      vi.stubGlobal('fetch', vi.fn(c.answer));
      const e = await failure(request('GET', '/api/issues/DEMO-1'));
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).status).toBe(c.status);
      expect(errorMessage(e)).toBe(c.app);
    });
  }

  it('reads an empty success as no value', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response('', { status: 200 })),
    );
    expect(await request('POST', '/api/issues/DEMO-1/watch')).toBeUndefined();
  });
});

describe('the setup wizard’s call', () => {
  for (const c of cases) {
    it(`gives a readable message for ${c.name}`, async () => {
      vi.stubGlobal('fetch', vi.fn(c.answer));
      const e = await failure(setupApi.status());
      expect(e).toBeInstanceOf(SetupError);
      expect((e as SetupError).status).toBe(c.status);
      expect(setupErrorMessage(e)).toBe(c.setup);
    });
  }
});

describe('errorMessage', () => {
  it('hides a parser’s text behind the generic message', () => {
    let parsed: unknown;
    try {
      JSON.parse('<html>');
    } catch (e) {
      parsed = e;
    }
    expect(parsed).toBeInstanceOf(SyntaxError);
    expect(errorMessage(parsed)).toBe(en.errors.generic);
    expect(errorMessage(new TypeError('x is undefined'))).toBe(en.errors.generic);
    expect(setupErrorMessage(new TypeError('x is undefined'))).toBe(setupEn.errors.generic);
  });

  it('keeps a message written for the user', () => {
    expect(errorMessage(new UserError(en.catalogAdmin.chooseTarget))).toBe(
      en.catalogAdmin.chooseTarget,
    );
  });
});

describe('failureMessage', () => {
  it('says how long to wait, from seconds or a date', () => {
    const now = Date.parse('2026-11-10T12:00:00Z');
    expect(retrySeconds('120', now)).toBe(120);
    expect(retrySeconds('Tue, 10 Nov 2026 12:00:45 GMT', now)).toBe(45);
    expect(retrySeconds('Tue, 10 Nov 2026 11:00:00 GMT', now)).toBeNull();
    expect(retrySeconds('soon', now)).toBeNull();
    expect(retrySeconds(null, now)).toBeNull();
    expect(failureMessage(en.errors, 429, '120', now)).toBe(en.errors.tooMany(120));
    expect(en.errors.tooMany(120)).toContain('2 min');
    expect(failureMessage(en.errors, 429, null, now)).toBe(en.errors.tooMany(null));
  });

  it('tells the other 4xx and 5xx apart', () => {
    expect(failureMessage(en.errors, 409)).toBe(en.errors.generic);
    expect(failureMessage(en.errors, 501)).toBe(en.errors.server);
    expect(failureMessage(en.errors, 504)).toBe(en.errors.network);
  });
});
