import { describe, expect, it } from 'vitest';
import fc from 'fast-check';

import {
  isSafeUrl,
  renderWiki,
  wikiReferences,
  wikiToText,
  type WikiResolver,
} from '../../src/core/wiki.js';

const USERS: Record<string, string> = { 'marie.durand': 'Marie Durand' };
const FILES = new Set(['capture.png', 'spec.pdf', 'schéma réseau.png']);
const ISSUES = new Set(['DEMO-1', 'DEMO-12', 'WEB-3']);

const resolver: WikiResolver = {
  user: (username) => USERS[username] ?? null,
  attachment: (filename) =>
    FILES.has(filename) ? `/api/attachments/by-name/${encodeURIComponent(filename)}` : null,
  issueExists: (key) => ISSUES.has(key),
};

const r = (src: string): string => renderWiki(src, resolver);

const EXT = 'rel="nofollow noopener noreferrer"';

/* ---------- HTML safety oracle ---------- */

const ALLOWED_TAGS = new Set([
  'p', 'br', 'strong', 'em', 'del', 'ins', 'sup', 'sub', 'cite', 'code', 'pre', 'a', 'img',
  'span', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'blockquote', 'hr', 'table',
  'tbody', 'tr', 'th', 'td', 'div',
]); // prettier-ignore
const ALLOWED_ATTRS = new Set([
  'class', 'href', 'rel', 'src', 'alt', 'width', 'height', 'loading', 'style', 'data-lang',
]); // prettier-ignore
const VOID_TAGS = new Set(['br', 'hr', 'img']);
const TAG_RE = /<(\/?)([a-z][a-z0-9]*)((?:\s+[a-z-]+="[^"<>]*")*)\s*>/g;
const ATTR_RE = /\s+([a-z-]+)="([^"]*)"/g;

/**
 * Describes the first problem in generated HTML, or returns null: a tag or
 * attribute outside the allowlist, a dangerous URL or style, a stray "<" or
 * ">" in text, or tags that are not properly nested.
 */
function htmlProblem(html: string): string | null {
  const stack: string[] = [];
  let last = 0;
  for (const m of html.matchAll(TAG_RE)) {
    const between = html.slice(last, m.index);
    if (/[<>]/.test(between)) return `raw angle bracket in text: ${JSON.stringify(between)}`;
    last = m.index + m[0].length;
    const [, closing = '', name = '', attrs = ''] = m;
    if (!ALLOWED_TAGS.has(name)) return `tag <${name}>`;
    if (closing) {
      const open = stack.pop();
      if (open !== name) return `</${name}> closes <${open ?? 'nothing'}>`;
      continue;
    }
    for (const a of attrs.matchAll(ATTR_RE)) {
      const [, attr = '', value = ''] = a;
      if (!ALLOWED_ATTRS.has(attr)) return `attribute ${attr} on <${name}>`;
      const decoded = value.replace(/&amp;/g, '&');
      if (
        (attr === 'href' || attr === 'src') &&
        !/^(?:https?:\/\/|ftp:\/\/|mailto:|\/(?![/\\])|#)/i.test(decoded)
      ) {
        return `unsafe ${attr}=${value}`;
      }
      if (attr === 'style' && !/^color: (?:#[0-9a-f]{3,6}|[a-z]+)$/.test(value)) {
        return `unsafe style=${value}`;
      }
    }
    if (!VOID_TAGS.has(name)) stack.push(name);
  }
  if (/[<>]/.test(html.slice(last))) return 'raw angle bracket at the end';
  if (stack.length > 0) return `unclosed ${stack.join(', ')}`;
  return null;
}

/* ---------- blocks ---------- */

describe('escaping', () => {
  it('escapes every HTML character of the source', () => {
    expect(r('Hello <script>alert(1)</script> & "x" \'y\'')).toBe(
      '<p>Hello &lt;script&gt;alert(1)&lt;/script&gt; &amp; &quot;x&quot; &#39;y&#39;</p>',
    );
  });

  it('renders nothing for an empty text', () => {
    expect(r('')).toBe('');
    expect(r('\n\n  \n')).toBe('');
  });

  it('cannot forge a block placeholder', () => {
    expect(r('\uE0000\uE001')).toBe('<p>0</p>');
  });
});

describe('paragraphs and line breaks', () => {
  it('separates paragraphs by blank lines and keeps single newlines as <br>', () => {
    expect(r('a\nb\n\nc')).toBe('<p>a<br>b</p><p>c</p>');
  });

  it('reads Windows and old Mac line endings', () => {
    expect(r('a\r\nb\r\n\r\nc')).toBe('<p>a<br>b</p><p>c</p>');
    expect(r('a\rb')).toBe('<p>a<br>b</p>');
  });

  it('turns \\\\ into a forced line break', () => {
    expect(r('a\\\\b')).toBe('<p>a<br>b</p>');
  });
});

describe('headings, quotes and rules', () => {
  it('renders h1. to h6.', () => {
    expect(r('h1. Titre')).toBe('<h1>Titre</h1>');
    expect(r('h6. Petit')).toBe('<h6>Petit</h6>');
    expect(r('h3. *Gras* titre')).toBe('<h3><strong>Gras</strong> titre</h3>');
  });

  it('does not take "h1.5" for a heading', () => {
    expect(r('h1.5 est une version')).toBe('<p>h1.5 est une version</p>');
  });

  it('renders bq. as a blockquote', () => {
    expect(r('bq. Une _citation_')).toBe('<blockquote>Une <em>citation</em></blockquote>');
  });

  it('renders ---- as a rule between paragraphs', () => {
    expect(r('avant\n----\naprès')).toBe('<p>avant</p><hr><p>après</p>');
  });
});

describe('text effects', () => {
  it.each([
    ['*gras*', '<strong>gras</strong>'],
    ['_italique_', '<em>italique</em>'],
    ['-barré-', '<del>barré</del>'],
    ['+souligné+', '<ins>souligné</ins>'],
    ['^exposant^', '<sup>exposant</sup>'],
    ['~indice~', '<sub>indice</sub>'],
    ['??source??', '<cite>source</cite>'],
  ])('renders %s', (src, html) => {
    expect(r(src)).toBe(`<p>${html}</p>`);
  });

  it('nests effects', () => {
    expect(r('*_mot_*')).toBe('<p><strong><em>mot</em></strong></p>');
    expect(r('un *mot _très_ fort* ici')).toBe(
      '<p>un <strong>mot <em>très</em> fort</strong> ici</p>',
    );
  });

  it('accepts punctuation around markers', () => {
    expect(r('(*mot*).')).toBe('<p>(<strong>mot</strong>).</p>');
  });

  it.each(['snake_case_name', '2*3*4', 'a-b-c', 'file-name.txt', 'C++ et C++', '5 - 3 - 1'])(
    'keeps %s literal (markers inside words or around spaces)',
    (src) => {
      expect(r(src)).toBe(`<p>${src}</p>`);
    },
  );

  it('keeps unterminated markers literal', () => {
    expect(r('*gras')).toBe('<p>*gras</p>');
    expect(r('un _début sans fin')).toBe('<p>un _début sans fin</p>');
  });

  it('does not let an effect span two lines', () => {
    expect(r('*a\nb*')).toBe('<p>*a<br>b*</p>');
  });

  it('honours backslash escapes', () => {
    expect(r('\\*pas gras\\*')).toBe('<p>*pas gras*</p>');
    expect(r('C:\\Users\\x')).toBe('<p>C:\\Users\\x</p>');
  });
});

describe('monospace and colour', () => {
  it('renders {{…}} literally inside <code>', () => {
    expect(r('{{a_b_c *x*}}')).toBe('<p><code>a_b_c *x*</code></p>');
    expect(r('{{<b>}}')).toBe('<p><code>&lt;b&gt;</code></p>');
  });

  it('keeps an unterminated {{ literal', () => {
    expect(r('{{code')).toBe('<p>{{code</p>');
  });

  it('accepts named and hex colours only', () => {
    expect(r('{color:red}rouge{color}')).toBe('<p><span style="color: red">rouge</span></p>');
    expect(r('{color:#FF0000}x{color}')).toBe('<p><span style="color: #ff0000">x</span></p>');
    expect(r('{color:notacolor}x{color}')).toBe('<p>x</p>');
  });

  it('refuses a CSS injection through {color}', () => {
    const html = r('{color:red;background:url(x)}texte{color}');
    expect(html).toBe('<p>texte</p>');
  });

  it('keeps an unterminated {color} literal', () => {
    expect(r('{color:red}texte')).toBe('<p>{color:red}texte</p>');
  });
});

describe('links', () => {
  it('renders [label|url] and [url]', () => {
    expect(r('[Solstice|https://example.com]')).toBe(
      `<p><a href="https://example.com" ${EXT}>Solstice</a></p>`,
    );
    expect(r('[https://example.com/a?b=1&c=2]')).toBe(
      `<p><a href="https://example.com/a?b=1&amp;c=2" ${EXT}>https://example.com/a?b=1&amp;c=2</a></p>`,
    );
  });

  it('renders mailto, same-site and anchor links', () => {
    expect(r('[écrire|mailto:a@b.fr]')).toBe(`<p><a href="mailto:a@b.fr" ${EXT}>écrire</a></p>`);
    expect(r('[mailto:a@b.fr]')).toBe(`<p><a href="mailto:a@b.fr" ${EXT}>a@b.fr</a></p>`);
    expect(r('[accueil|/projects]')).toBe('<p><a href="/projects">accueil</a></p>');
    expect(r('[#haut]')).toBe('<p><a href="#haut">haut</a></p>');
  });

  it('renders markup inside a label, without nested links', () => {
    expect(r('[*gras*|https://x.fr]')).toBe(
      `<p><a href="https://x.fr" ${EXT}><strong>gras</strong></a></p>`,
    );
    expect(r('[voir DEMO-1|https://x.fr]')).toBe(
      `<p><a href="https://x.fr" ${EXT}>voir DEMO-1</a></p>`,
    );
  });

  it.each([
    '[x|javascript:alert(1)]',
    '[x|JaVaScRiPt:alert(1)]',
    '[x|data:text/html,<b>]',
    '[x|vbscript:msgbox]',
    '[x|//evil.example]',
    '[x|/\\evil.example]',
    '[x|java\tscript:alert(1)]',
    '[x|file:///etc/passwd]',
  ])('renders the unsafe %s as text', (src) => {
    const html = r(src);
    expect(html).not.toContain('<a');
    expect(htmlProblem(html)).toBeNull();
  });

  it('cannot break out of the href attribute', () => {
    const html = r('[x|https://e.fr/" onmouseover="alert(1)]');
    expect(htmlProblem(html)).toBeNull();
    expect(html).toContain('href="https://e.fr/&quot;%20onmouseover=&quot;alert(1)"');
  });

  it('keeps a bracket without a valid target as text', () => {
    expect(r('[une remarque]')).toBe('<p>[une remarque]</p>');
    expect(r('[lien')).toBe('<p>[lien</p>');
    expect(r('[]')).toBe('<p>[]</p>');
  });

  it('autolinks bare URLs without trailing punctuation', () => {
    expect(r('voir https://example.com/a_b_c.')).toBe(
      `<p>voir <a href="https://example.com/a_b_c" ${EXT}>https://example.com/a_b_c</a>.</p>`,
    );
    expect(r('(https://example.com/x)')).toBe(
      `<p>(<a href="https://example.com/x" ${EXT}>https://example.com/x</a>)</p>`,
    );
    expect(r('https://fr.wikipedia.org/wiki/Rouille_(chimie)')).toContain(
      'href="https://fr.wikipedia.org/wiki/Rouille_(chimie)"',
    );
  });
});

describe('mentions', () => {
  it('shows the display name of a known user', () => {
    expect(r('[~marie.durand]')).toBe(
      '<p><a class="mention" href="/people/marie.durand">Marie Durand</a></p>',
    );
  });

  it('falls back to @username', () => {
    expect(r('[~inconnu]')).toBe('<p><a class="mention" href="/people/inconnu">@inconnu</a></p>');
  });

  it('encodes the username in the URL and escapes it in the text', () => {
    expect(r('[~jean dupont]')).toBe(
      '<p><a class="mention" href="/people/jean%20dupont">@jean dupont</a></p>',
    );
    expect(r('[~<b>]')).toBe('<p><a class="mention" href="/people/%3Cb%3E">@&lt;b&gt;</a></p>');
  });
});

describe('attachments and images', () => {
  const img = '/api/attachments/by-name/capture.png';
  const pdf = '/api/attachments/by-name/spec.pdf';

  it('embeds an attached image', () => {
    expect(r('!capture.png!')).toBe(`<p><img src="${img}" alt="capture.png" loading="lazy"></p>`);
  });

  it('reads thumbnail and size parameters', () => {
    expect(r('!capture.png|thumbnail!')).toBe(
      `<p><img class="thumbnail" src="${img}" alt="capture.png" loading="lazy"></p>`,
    );
    expect(r('!capture.png|width=300!')).toBe(
      `<p><img src="${img}" alt="capture.png" width="300" loading="lazy"></p>`,
    );
  });

  it('links to a file that is not an image', () => {
    expect(r('!spec.pdf!')).toBe(`<p><a class="attachment" href="${pdf}">spec.pdf</a></p>`);
    expect(r('[^spec.pdf]')).toBe(`<p><a class="attachment" href="${pdf}">spec.pdf</a></p>`);
    expect(r('[le cahier|^spec.pdf]')).toBe(
      `<p><a class="attachment" href="${pdf}">le cahier</a></p>`,
    );
  });

  it('handles file names with spaces and accents', () => {
    expect(r('!schéma réseau.png!')).toContain(
      'src="/api/attachments/by-name/sch%C3%A9ma%20r%C3%A9seau.png"',
    );
  });

  it('keeps an unknown attachment as written', () => {
    expect(r('!absent.png!')).toBe('<p>!absent.png!</p>');
    expect(r('[^absent.pdf]')).toBe('<p>[^absent.pdf]</p>');
  });

  it('links external images instead of loading them', () => {
    expect(r('!http://example.com/x.png!')).toBe(
      `<p><a href="http://example.com/x.png" ${EXT}>http://example.com/x.png</a></p>`,
    );
  });

  it('refuses script URLs as images', () => {
    const html = r('!javascript:alert(1)!');
    expect(html).not.toMatch(/<(?:img|a)\b/);
    expect(htmlProblem(html)).toBeNull();
  });

  it('refuses an unsafe URL returned by the resolver', () => {
    const evil: WikiResolver = { ...resolver, attachment: () => 'javascript:alert(1)' };
    expect(renderWiki('!capture.png! [^spec.pdf]', evil)).toBe('<p>!capture.png! [^spec.pdf]</p>');
  });

  it('leaves ordinary exclamation marks alone', () => {
    expect(r('Attention! Danger!')).toBe('<p>Attention! Danger!</p>');
  });
});

describe('issue keys', () => {
  it('links existing issue keys in text', () => {
    expect(r('voir DEMO-12.')).toBe(
      '<p>voir <a class="issue-key" href="/browse/DEMO-12">DEMO-12</a>.</p>',
    );
  });

  it('leaves unknown keys and look-alikes as text', () => {
    expect(r('DEMO-999')).toBe('<p>DEMO-999</p>');
    expect(r('demo-1 xDEMO-1 DEMO-12a')).toBe('<p>demo-1 xDEMO-1 DEMO-12a</p>');
  });

  it('links [KEY] and [label|KEY]', () => {
    expect(r('[DEMO-1]')).toBe('<p><a class="issue-key" href="/browse/DEMO-1">DEMO-1</a></p>');
    expect(r('[le ticket|DEMO-1]')).toBe(
      '<p><a class="issue-key" href="/browse/DEMO-1">le ticket</a></p>',
    );
    expect(r('[DEMO-999]')).toBe('<p>[DEMO-999]</p>');
  });
});

describe('emoticons', () => {
  it('renders the Jira status glyphs as text, not emoji', () => {
    expect(r('(/) fait (x) non')).toBe(
      '<p><span class="emo emo-ok">✓</span> fait <span class="emo emo-no">✗</span> non</p>',
    );
    expect(r('(!)(?)(i)(y)(n)')).toBe(
      '<p><span class="emo emo-warn">!</span><span class="emo emo-help">?</span>' +
        '<span class="emo emo-info">i</span><span class="emo emo-up">+</span>' +
        '<span class="emo emo-down">−</span></p>',
    );
  });
});

describe('code, noformat, quote and panel', () => {
  it('renders {code:lang} without inline parsing', () => {
    expect(r('{code:java}\nint x = 1 < 2; // *x*\n{code}')).toBe(
      '<pre class="code" data-lang="java"><code>int x = 1 &lt; 2; // *x*</code></pre>',
    );
    expect(r('{code}[a|http://b] _c_{code}')).toBe(
      '<pre class="code"><code>[a|http://b] _c_</code></pre>',
    );
  });

  it('cannot close the generated markup from inside {code}', () => {
    expect(r('{code}</code><script>{code}')).toBe(
      '<pre class="code"><code>&lt;/code&gt;&lt;script&gt;</code></pre>',
    );
  });

  it('reads named parameters and validates the language', () => {
    expect(r('{code:title=Foo.java|borderStyle=solid}x{code}')).toBe(
      '<div class="code-title">Foo.java</div><pre class="code"><code>x</code></pre>',
    );
    expect(r('{code:language=sql}SELECT 1{code}')).toBe(
      '<pre class="code" data-lang="sql"><code>SELECT 1</code></pre>',
    );
    expect(r('{code:java" onload="x}y{code}')).toBe('<pre class="code"><code>y</code></pre>');
  });

  it('splits a paragraph around a code block', () => {
    expect(r('avant {code}x{code} après')).toBe(
      '<p>avant </p><pre class="code"><code>x</code></pre><p> après</p>',
    );
  });

  it('keeps an unterminated {code} literal', () => {
    expect(r('{code}sans fin')).toBe('<p>{code}sans fin</p>');
    expect(r('\\{code}x{code}')).toBe('<p>{code}x{code}</p>');
  });

  it('renders {noformat} verbatim', () => {
    expect(r('{noformat}*brut* <b>{noformat}')).toBe(
      '<pre class="noformat">*brut* &lt;b&gt;</pre>',
    );
  });

  it('renders {quote} with markup and nested code', () => {
    expect(r('{quote}\n*important*\n{quote}')).toBe(
      '<blockquote><p><strong>important</strong></p></blockquote>',
    );
    expect(r('{quote}a\n{code}b{code}\n{quote}')).toBe(
      '<blockquote><p>a</p><pre class="code"><code>b</code></pre></blockquote>',
    );
  });

  it('renders {panel} with and without a title', () => {
    expect(r('{panel:title=Note|borderStyle=dashed}\ntexte\n{panel}')).toBe(
      '<div class="panel"><div class="panel-title">Note</div><p>texte</p></div>',
    );
    expect(r('{panel}texte{panel}')).toBe('<div class="panel"><p>texte</p></div>');
    expect(r('{panel:title=<i>}x{panel}')).toBe(
      '<div class="panel"><div class="panel-title">&lt;i&gt;</div><p>x</p></div>',
    );
  });

  it('drops {anchor} and {toc}, shows unknown macros as written', () => {
    expect(r('a{anchor:haut}b')).toBe('<p>ab</p>');
    expect(r('{toc}')).toBe('');
    expect(r('{toc:maxLevel=2}\nh1. A')).toBe('<h1>A</h1>');
    expect(r('{foo} et {bar:x=1}')).toBe('<p>{foo} et {bar:x=1}</p>');
  });
});

describe('lists', () => {
  it('renders bullet, numbered and dash lists', () => {
    expect(r('* un\n* deux')).toBe('<ul><li>un</li><li>deux</li></ul>');
    expect(r('# un\n# deux')).toBe('<ol><li>un</li><li>deux</li></ol>');
    expect(r('- un\n- deux')).toBe('<ul><li>un</li><li>deux</li></ul>');
  });

  it('nests and mixes list types', () => {
    expect(r('* a\n** b\n* c')).toBe('<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>');
    expect(r('# a\n#* b\n# c')).toBe('<ol><li>a<ul><li>b</li></ul></li><li>c</li></ol>');
    expect(r('* a\n** b\n*** c\n* d')).toBe(
      '<ul><li>a<ul><li>b<ul><li>c</li></ul></li></ul></li><li>d</li></ul>',
    );
  });

  it('starts a new list when the type changes', () => {
    expect(r('* a\n# b')).toBe('<ul><li>a</li></ul><ol><li>b</li></ol>');
  });

  it('opens intermediate levels on a depth jump', () => {
    expect(r('*** a')).toBe('<ul><li><ul><li><ul><li>a</li></ul></li></ul></li></ul>');
  });

  it('renders markup in items and ends the list on plain text', () => {
    expect(r('* *gras*\ntexte')).toBe('<ul><li><strong>gras</strong></li></ul><p>texte</p>');
  });

  it('does not take bold at the start of a line for a list', () => {
    expect(r('*gras* au début')).toBe('<p><strong>gras</strong> au début</p>');
  });

  it('reads CRLF lists', () => {
    expect(r('* a\r\n* b')).toBe('<ul><li>a</li><li>b</li></ul>');
  });
});

describe('tables', () => {
  it('renders header and data cells with markup', () => {
    expect(r('||Nom||Rôle||\n|Marie|_dev_|')).toBe(
      '<table class="wiki-table"><tbody><tr><th>Nom</th><th>Rôle</th></tr>' +
        '<tr><td>Marie</td><td><em>dev</em></td></tr></tbody></table>',
    );
  });

  it('does not split a cell on the bar of a link', () => {
    expect(r('|[site|https://x.fr]|b|')).toBe(
      `<table class="wiki-table"><tbody><tr><td><a href="https://x.fr" ${EXT}>site</a></td>` +
        '<td>b</td></tr></tbody></table>',
    );
  });

  it('accepts a row without the closing bar', () => {
    expect(r('|a|b')).toBe(
      '<table class="wiki-table"><tbody><tr><td>a</td><td>b</td></tr></tbody></table>',
    );
  });
});

/* ---------- references and excerpts ---------- */

describe('wikiReferences', () => {
  it('lists mentions, attachments and issue keys once, in order, outside code', () => {
    const src =
      '[~marie.durand] et [~paul] puis [~marie.durand] !capture.png! [^spec.pdf] ' +
      'DEMO-1 [WEB-3] DEMO-1\n{code}[~cache] DEMO-9 !x.png!{code}';
    expect(wikiReferences(src)).toEqual({
      users: ['marie.durand', 'paul'],
      attachments: ['capture.png', 'spec.pdf'],
      issueKeys: ['DEMO-1', 'WEB-3'],
    });
  });

  it('returns empty lists for plain text', () => {
    expect(wikiReferences('rien de spécial')).toEqual({
      users: [],
      attachments: [],
      issueKeys: [],
    });
  });
});

describe('wikiToText', () => {
  const src = 'h1. Titre\n\n*Gras* et [lien|https://x.fr]\n* un\n* deux\n{code}a < b{code}';

  it('strips the markup and collapses whitespace', () => {
    expect(wikiToText(src)).toBe('Titre Gras et lien un deux a < b');
    expect(wikiToText('[~marie.durand] & co')).toBe('@marie.durand & co');
  });

  it('cuts with an ellipsis within the requested length', () => {
    const text = wikiToText(src, 10);
    expect(text.length).toBeLessThanOrEqual(10);
    expect(text.endsWith('…')).toBe(true);
    expect(wikiToText('court', 10)).toBe('court');
  });

  it('prefers cutting at a word boundary', () => {
    expect(wikiToText('un deux trois quatre cinq', 16)).toBe('un deux trois…');
  });
});

describe('isSafeUrl', () => {
  it.each([
    'https://example.com/a',
    'http://example.com',
    'mailto:a@b.fr',
    '/projects/DEMO',
    '#haut',
  ])('accepts %s', (url) => {
    expect(isSafeUrl(url)).toBe(true);
  });

  it.each([
    'javascript:alert(1)',
    'JaVaScRiPt:alert(1)',
    'data:text/html,<b>',
    'vbscript:msgbox',
    'file:///etc/passwd',
    '//evil.example',
    '/\\evil.example',
    'java\tscript:alert(1)',
    'example.com',
  ])('refuses %s', (url) => {
    expect(isSafeUrl(url)).toBe(false);
  });
});

/* ---------- properties ---------- */

const WIKI_PIECES = [
  '*', '_', '-', '+', '^', '~', '??', '{', '}', '[', ']', '|', '!', '(', ')', '#', '\\', '\n',
  '\r\n', ' ', 'a', 'é', '0', 'h1. ', 'bq. ', '* ', '# ', '----', '||', '{code}', '{code:js}',
  '{noformat}', '{quote}', '{panel:title=t}', '{panel}', '{color:red}', '{color:x;y}',
  '{color}', '{{', '}}', '{anchor:a}', '<script>', '</code>', '"', "'", '&', '&lt;', 'onerror=',
  'javascript:', 'http://x.y/', '//evil', 'DEMO-1', '~marie.durand', '^capture.png',
  'capture.png', 'spec.pdf', '|thumbnail', '|width=9', '(/)', '\uE000', '\uE001',
]; // prettier-ignore

const wikiish = fc.string({ unit: fc.constantFrom(...WIKI_PIECES), maxLength: 40 });

describe('safety properties', () => {
  it('only produces allowed, well-nested markup for wiki-like input', () => {
    fc.assert(
      fc.property(wikiish, (src) => {
        expect(htmlProblem(r(src))).toBeNull();
      }),
      { numRuns: 3000 },
    );
  });

  it('only produces allowed, well-nested markup for arbitrary text', () => {
    fc.assert(
      fc.property(fc.string({ unit: 'binary', maxLength: 80 }), (src) => {
        expect(htmlProblem(r(src))).toBeNull();
      }),
      { numRuns: 1000 },
    );
  });

  it('never makes an excerpt longer than asked', () => {
    fc.assert(
      fc.property(wikiish, fc.integer({ min: 1, max: 30 }), (src, max) => {
        expect(wikiToText(src, max).length).toBeLessThanOrEqual(max);
      }),
    );
  });

  it('stays fast on long runs of unmatched markers', () => {
    const inputs = [
      '*a '.repeat(20000),
      '{color:red}'.repeat(10000),
      '{{'.repeat(20000),
      '{panel:x}'.repeat(10000),
      '['.repeat(50000),
      '!a '.repeat(20000),
    ];
    const start = performance.now();
    for (const src of inputs) expect(htmlProblem(r(src))).toBeNull();
    expect(performance.now() - start).toBeLessThan(2000);
  });
});
