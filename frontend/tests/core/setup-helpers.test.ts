import { afterEach, describe, expect, it, vi } from 'vitest';

import { browserTimeZone, engineName, timeZones, wizardTimeZone } from '../../src/core/setup.js';

afterEach(() => {
  vi.restoreAllMocks();
});

function browserIn(zone: string) {
  vi.spyOn(Intl.DateTimeFormat.prototype, 'resolvedOptions').mockReturnValue({
    ...new Intl.DateTimeFormat().resolvedOptions(),
    timeZone: zone,
  });
}

describe('engineName', () => {
  it('names MySQL and PostgreSQL', () => {
    expect(engineName('mysql')).toBe('MySQL');
    expect(engineName('postgres')).toBe('PostgreSQL');
  });
});

describe('browserTimeZone', () => {
  it("is the browser's zone", () => {
    browserIn('Europe/Paris');
    expect(browserTimeZone()).toBe('Europe/Paris');
  });
});

describe('wizardTimeZone', () => {
  it("keeps the server's zone", () => {
    browserIn('Europe/Paris');
    expect(wizardTimeZone('Asia/Kolkata')).toBe('Asia/Kolkata');
  });

  it("falls back to the browser's when the server has none or only 'Local'", () => {
    browserIn('Europe/Paris');
    expect(wizardTimeZone(undefined)).toBe('Europe/Paris');
    expect(wizardTimeZone('')).toBe('Europe/Paris');
    expect(wizardTimeZone('Local')).toBe('Europe/Paris');
  });
});

describe('timeZones', () => {
  it('lists the IANA zones the browser knows', () => {
    expect(timeZones()).toContain('Europe/Berlin');
  });

  it('is empty when the browser cannot list them', () => {
    vi.spyOn(Intl, 'supportedValuesOf').mockImplementation(() => {
      throw new RangeError('unsupported');
    });
    expect(timeZones()).toEqual([]);
  });
});
