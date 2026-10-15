import { describe, expect, it } from 'vitest';

import { parseNotes, plainInline } from '../../src/core/release-notes.js';

describe('plainInline', () => {
  it.each([
    ['**Full Changelog**: https://github.com/x', 'Full Changelog: https://github.com/x'],
    ['see [the docs](https://example.org) and ![logo](a.png)', 'see the docs and logo'],
    ['`solstice health` *now* _checks_ ~~nothing~~', 'solstice health now checks nothing'],
    ['snake_case_name and 2*3*4 stay', 'snake_case_name and 2*3*4 stay'],
    ['<https://github.com/mflr0/Solstice>', 'https://github.com/mflr0/Solstice'],
    ['<b>bold</b> <img src=x onerror=alert(1)>', 'bold'],
    ['\\*not italic\\*', '*not italic*'],
  ])('%s', (input, want) => {
    expect(plainInline(input)).toBe(want);
  });
});

describe('parseNotes', () => {
  it('reads GitHub-generated notes', () => {
    const md = [
      "## What's Changed",
      '* feat: updates from the interface by @someone in https://github.com/mflr0/Solstice/pull/12',
      '* fix: boards',
      '',
      '**Full Changelog**: https://github.com/mflr0/Solstice/compare/v1.0.0...v1.1.0',
    ].join('\r\n');
    expect(parseNotes(md)).toEqual([
      { kind: 'heading', text: "What's Changed" },
      {
        kind: 'list',
        ordered: false,
        items: [
          'feat: updates from the interface by @someone in https://github.com/mflr0/Solstice/pull/12',
          'fix: boards',
        ],
      },
      {
        kind: 'paragraph',
        text: 'Full Changelog: https://github.com/mflr0/Solstice/compare/v1.0.0...v1.1.0',
      },
    ]);
  });

  it('keeps paragraphs, ordered lists, item continuations and code', () => {
    const md = [
      'First line',
      'same paragraph.',
      '',
      '1. one',
      '   continued',
      '2. two',
      '',
      '---',
      '```sh',
      'sudo systemctl status solstice-update',
      '```',
      '> quoted',
    ].join('\n');
    expect(parseNotes(md)).toEqual([
      { kind: 'paragraph', text: 'First line same paragraph.' },
      { kind: 'list', ordered: true, items: ['one continued', 'two'] },
      { kind: 'code', text: 'sudo systemctl status solstice-update' },
      { kind: 'paragraph', text: 'quoted' },
    ]);
  });

  it('turns markup into text, never HTML', () => {
    const blocks = parseNotes('<script>alert(1)</script>\n\n- <a href="javascript:x">click</a>');
    expect(blocks).toEqual([
      { kind: 'paragraph', text: 'alert(1)' },
      { kind: 'list', ordered: false, items: ['click'] },
    ]);
  });

  it('has nothing to show for empty notes', () => {
    expect(parseNotes('')).toEqual([]);
    expect(parseNotes('\n\n  \n')).toEqual([]);
  });
});
