import { describe, expect, it } from 'vitest';

import {
  checkPassword,
  codeComplete,
  databaseSQL,
  formatCode,
  guessBaseURL,
  IMPORT_PHASES,
  setupSteps,
  sourceEngine,
} from '../../src/core/setup.js';

describe('setup steps', () => {
  it('asks for a database only when the server has none', () => {
    expect(setupSteps(true)).toEqual(['language', 'code', 'database', 'instance', 'start', 'done']);
    expect(setupSteps(false)).toEqual(['language', 'code', 'instance', 'start', 'done']);
  });
});

describe('setup code', () => {
  it.each([
    ['4f7k9qx2', '4F7K-9QX2'],
    ['4F7K-9QX2', '4F7K-9QX2'],
    [' 4f7k 9qx2 ', '4F7K-9QX2'],
    ['4F7', '4F7'],
    ['4F7K', '4F7K'],
    ['4F7K9', '4F7K-9'],
    ['4F7K-9QX2-EXTRA', '4F7K-9QX2'],
    ['é!?', ''],
  ])('%s → %s', (typed, shown) => {
    expect(formatCode(typed)).toBe(shown);
  });

  it('is complete at eight characters', () => {
    expect(codeComplete('4F7K-9QX')).toBe(false);
    expect(codeComplete('4f7k9qx2')).toBe(true);
  });
});

describe('administrator password', () => {
  it('follows the server: twelve characters, not the user name', () => {
    expect(checkPassword('', 'chef')).toMatchObject({ score: 0, acceptable: false });
    expect(checkPassword('onze-signes', 'chef')).toMatchObject({
      acceptable: false,
      longEnough: false,
    });
    expect(checkPassword('chef.de.projet', 'chef.de.projet')).toMatchObject({
      acceptable: false,
      notUsername: false,
    });
    expect(checkPassword('CHEF.DE.PROJET', 'chef.de.projet').acceptable).toBe(false);
    expect(checkPassword('douze-signes', 'chef').acceptable).toBe(true);
    // Twelve characters, counted as characters, not bytes.
    expect(checkPassword('éééééé€€€€€€', 'chef').longEnough).toBe(true);
  });

  it('rates length, variety and passphrases above patterns', () => {
    expect(checkPassword('aaaaaaaaaaaa', 'chef').score).toBe(1);
    expect(checkPassword('123456789012', 'chef').score).toBe(1);
    expect(checkPassword('abcdefghijkl', 'chef').score).toBe(1);
    expect(checkPassword('Tr0ub4dor&3x', 'chef').score).toBe(3);
    expect(checkPassword('une phrase de passe longue', 'chef').score).toBe(4);
    expect(checkPassword('V3rt-Pr@irie-Lointaine-42', 'chef').score).toBe(4);
    // The user name inside the password weakens it.
    expect(checkPassword('marie.durand-2026!', 'marie.durand').score).toBeLessThan(
      checkPassword('pivoine.durable-2026!', 'marie.durand').score,
    );
  });
});

describe('import', () => {
  it('lists the importer phases once each', () => {
    expect(new Set(IMPORT_PHASES).size).toBe(IMPORT_PHASES.length);
    expect(IMPORT_PHASES[0]).toBe('users');
  });

  it('recognises the source engines', () => {
    expect(sourceEngine('postgres://jira:x@db:5432/jira')).toBe('postgres');
    expect(sourceEngine('postgresql://db/jira')).toBe('postgres');
    expect(sourceEngine(' MySQL://jira@db/jira')).toBe('mysql');
    expect(sourceEngine('mariadb://db/jira')).toBe('mysql');
    expect(sourceEngine('jdbc:postgresql://db/jira')).toBe(null);
    expect(sourceEngine('')).toBe(null);
  });
});

describe('instance', () => {
  it('guesses the public address from the page', () => {
    expect(guessBaseURL('https://tickets.example.org')).toBe('https://tickets.example.org');
    expect(guessBaseURL('http://10.0.0.5:8095')).toBe('http://10.0.0.5:8095');
    expect(guessBaseURL('not a url')).toBe('');
  });

  it('writes the SQL that prepares a database', () => {
    expect(databaseSQL('tickets', "o'brien")).toBe(
      [
        'CREATE DATABASE `tickets` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;',
        "CREATE USER 'o''brien'@'%' IDENTIFIED BY '…';",
        "GRANT ALL ON `tickets`.* TO 'o''brien'@'%';",
      ].join('\n'),
    );
    expect(databaseSQL('', '')).toContain('CREATE DATABASE `solstice`');
  });
});
