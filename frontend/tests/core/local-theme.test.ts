import { describe, expect, it } from 'vitest';

import { inlineFiles, parseLocalManifest, readPackage } from '../../src/core/local-theme.js';

const bytes = (s: string) => new TextEncoder().encode(s);
const b64 = (s: string) => btoa(s);

const files = {
  'assets/a.svg': bytes('svg'),
  'assets/a.png': bytes('png'),
  'assets/a.webp': bytes('webp'),
  'assets/a.jpg': bytes('jpg'),
  'assets/B.PNG': bytes('upper'),
  'assets/a.bin': bytes('bin'),
  'fonts/a.woff2': bytes('font'),
};
const svg = `data:image/svg+xml;base64,${b64('svg')}`;
const png = `data:image/png;base64,${b64('png')}`;

describe('inlineFiles', () => {
  it.each([
    ['unquoted', 'a { b: url(assets/a.svg); }', `a { b: url(${svg}); }`],
    ['double quotes', 'a { b: url("assets/a.png"); }', `a { b: url("${png}"); }`],
    ['single quotes', "a { b: url('assets/a.png'); }", `a { b: url('${png}'); }`],
    ['spaces', 'a { b: url(  assets/a.png  ); }', `a { b: url(${png}); }`],
    ['upper-case call', 'a { b: URL(assets/a.png); }', `a { b: url(${png}); }`],
    ['a leading ./', 'a { b: url(./assets/a.png); }', `a { b: url(${png}); }`],
    ['a fragment', 'a { b: url(assets/a.svg#icon); }', `a { b: url(${svg}#icon); }`],
    [
      'webp',
      'a { b: url(assets/a.webp); }',
      `a { b: url(data:image/webp;base64,${b64('webp')}); }`,
    ],
    ['jpeg', 'a { b: url(assets/a.jpg); }', `a { b: url(data:image/jpeg;base64,${b64('jpg')}); }`],
    [
      'upper-case extension',
      'a { b: url(assets/B.PNG); }',
      `a { b: url(data:image/png;base64,${b64('upper')}); }`,
    ],
    [
      'unknown type',
      'a { b: url(assets/a.bin); }',
      `a { b: url(data:application/octet-stream;base64,${b64('bin')}); }`,
    ],
    [
      'a font',
      `@font-face { src: url(fonts/a.woff2) format('woff2'); }`,
      `@font-face { src: url(data:font/woff2;base64,${b64('font')}) format('woff2'); }`,
    ],
    [
      'image-set strings and urls',
      'a { b: image-set("assets/a.png" 1x, url(assets/a.svg) 2x); }',
      `a { b: image-set("${png}" 1x, url(${svg}) 2x); }`,
    ],
    [
      'a prefixed image-set, leaving type() alone',
      `a { b: -webkit-image-set('assets/a.png' type('assets/a.png')); }`,
      `a { b: -webkit-image-set('${png}' type('assets/a.png')); }`,
    ],
    [
      'the same file twice',
      'a { b: url(assets/a.png), url(assets/a.png); }',
      `a { b: url(${png}), url(${png}); }`,
    ],
  ])('inlines %s', (_, css, want) => {
    expect(inlineFiles(css, files)).toBe(want);
  });

  it.each([
    ['data: targets', 'a { b: url(data:image/png;base64,AAAA); }'],
    ['external targets', 'a { b: url(https://example.com/assets/a.png); }'],
    ['a file the package lacks', 'a { b: url(assets/none.png); }'],
    ['comments', '/* url(assets/a.png) */ a { }'],
    ['plain strings', 'a { content: "assets/a.png"; }'],
    ['a name that is no file of its own', 'a { b: url(constructor); }'],
  ])('leaves %s alone', (_, css) => {
    expect(inlineFiles(css, files)).toBe(css);
  });

  it('encodes a file larger than one chunk of base64', () => {
    const big = new Uint8Array(0x8000 * 2 + 7).fill(65);
    expect(inlineFiles('url(assets/big.png)', { 'assets/big.png': big })).toBe(
      `url(data:image/png;base64,${b64('A'.repeat(big.length))})`,
    );
  });
});

describe('parseLocalManifest', () => {
  it('needs an id to serve the theme under', () => {
    expect(parseLocalManifest('{')).toBeNull();
    expect(parseLocalManifest('[]')).toBeNull();
    expect(parseLocalManifest('{}')).toBeNull();
    expect(parseLocalManifest('{"id":"../x"}')).toBeNull();
    expect(parseLocalManifest(`{"id":"${'a'.repeat(33)}"}`)).toBeNull();
  });

  it('gives what a package leaves out a default', () => {
    expect(parseLocalManifest('{"id":"retro"}')).toEqual({
      id: 'retro',
      contract: 0,
      version: '',
      name: { en: 'retro' },
      description: {},
      author: '',
      schemes: ['light'],
      preview: {},
    });
  });

  it('drops what the interface could not draw', () => {
    const got = parseLocalManifest(
      JSON.stringify({
        id: 'retro',
        name: { en: 'Retro', fr: 3 },
        schemes: ['light', 'dark'],
        preview: {
          light: { bar: '#000000', paper: '#ffffff', ink: '#000000', accent: '#ff0000' },
          dark: {
            bar: 'red;background:url(x)',
            paper: '#ffffff',
            ink: '#000000',
            accent: '#ff0000',
          },
        },
      }),
    );
    expect(got?.name).toEqual({ en: 'Retro' });
    expect(got?.schemes).toEqual(['light', 'dark']);
    expect(got?.preview.light?.accent).toBe('#ff0000');
    expect(got?.preview.dark).toBeUndefined();
  });
});

describe('readPackage', () => {
  const manifest = bytes('{"id":"retro"}');

  it('needs a manifest and a stylesheet', () => {
    expect(readPackage({})).toBeNull();
    expect(readPackage({ 'theme.json': manifest })).toBeNull();
    expect(readPackage({ 'theme.css': bytes('') })).toBeNull();
    expect(readPackage({ 'theme.json': bytes('nope'), 'theme.css': bytes('') })).toBeNull();
  });

  it('inlines the stylesheet', () => {
    const got = readPackage({
      'theme.json': manifest,
      'theme.css': bytes('a { b: url(assets/a.png); }'),
      'assets/a.png': bytes('png'),
    });
    expect(got?.manifest.id).toBe('retro');
    expect(got?.css).toBe(`a { b: url(${png}); }`);
  });
});
