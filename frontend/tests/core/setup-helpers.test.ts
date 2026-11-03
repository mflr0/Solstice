import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  browserTimeZone,
  engineName,
  timeZones,
  wizardTimeZone,
  zoneChoices,
  zoneOffset,
} from '../../src/core/setup.js';

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

describe('zoneOffset', () => {
  const winter = new Date('2026-01-15T12:00:00Z');
  const summer = new Date('2026-07-15T12:00:00Z');

  it('writes the offset from UTC at the given time', () => {
    expect(zoneOffset('Europe/Paris', winter)).toBe('UTC+01:00');
    expect(zoneOffset('Europe/Paris', summer)).toBe('UTC+02:00');
    expect(zoneOffset('America/New_York', winter)).toBe('UTC-05:00');
    expect(zoneOffset('Asia/Kolkata', winter)).toBe('UTC+05:30');
    expect(zoneOffset('UTC', winter)).toBe('UTC');
  });

  it('is empty for a zone the browser does not know', () => {
    expect(zoneOffset('Mars/Olympus_Mons', winter)).toBe('');
  });
});

describe('zoneChoices', () => {
  const winter = new Date('2026-01-15T12:00:00Z');
  const choice = (zone: string) => zoneChoices('en-US', winter).find((c) => c.zone === zone);

  it('lists every zone the browser knows, west to east, UTC first at UTC', () => {
    const all = zoneChoices('en-US', winter);
    expect(all.map((c) => c.zone)).toEqual(expect.arrayContaining(timeZones()));
    expect(all.filter((c) => c.zone === 'UTC')).toHaveLength(1);
    const minutes = all.map((c) => c.minutes);
    expect(minutes).toEqual([...minutes].sort((a, b) => a - b));
    expect(all.find((c) => c.minutes === 0)?.zone).toBe('UTC');
    const ones = all.filter((c) => c.minutes === 60).map((c) => c.zone);
    expect(ones).toEqual([...ones].sort());
  });

  it('shows the offset short, with a minus sign', () => {
    expect(choice('Europe/Paris')).toMatchObject({ offset: 'UTC+1', minutes: 60 });
    expect(choice('America/New_York')).toMatchObject({ offset: 'UTC\u22125', minutes: -300 });
    expect(choice('Asia/Tehran')).toMatchObject({ offset: 'UTC+3:30', minutes: 210 });
    expect(choice('UTC')).toMatchObject({ offset: 'UTC', minutes: 0 });
  });

  it('finds a zone by its offset as UTC or GMT, short and long', () => {
    expect(choice('Europe/Paris')?.keywords).toEqual(
      expect.arrayContaining(['UTC+01:00', 'UTC+1', 'GMT+1', 'GMT+01:00']),
    );
    expect(choice('America/New_York')?.keywords).toEqual(expect.arrayContaining(['UTC-5']));
    expect(choice('Asia/Tehran')?.keywords).toEqual(expect.arrayContaining(['UTC+3:30']));
    expect(choice('UTC')?.keywords).toContain('GMT');
  });

  it('finds a zone by its name with spaces and its short name', () => {
    expect(choice('America/New_York')?.keywords).toEqual(
      expect.arrayContaining(['America/New York', 'EST']),
    );
  });

  it('is empty when the browser cannot list the zones', () => {
    vi.spyOn(Intl, 'supportedValuesOf').mockImplementation(() => {
      throw new RangeError('unsupported');
    });
    expect(zoneChoices('en-US', winter)).toEqual([]);
  });
});
