import { readdirSync, readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

/**
 * Third-party themes hook onto the class names of ANATOMY.md, section 3. Each
 * must still be set by a component, or a theme breaks with no contract bump.
 */

const frontend = new URL('../../', import.meta.url);
const anatomy = readFileSync(
  new URL('../../../backend/internal/themes/ANATOMY.md', import.meta.url),
  'utf8',
);

/** Hooks the markup can't show: `file` (from frontend/) holds `code`, which sets `names`. */
const BUILT: { names: string[]; file: string; code: string }[] = [
  { names: ['error', 'info'], file: 'src/ui/Notes.svelte', code: 'toast {n.kind}' },
  {
    names: ['forward', 'backward', 'to-todo', 'to-doing', 'to-done'],
    file: 'src/ui/workflow/WorkflowDiagram.svelte',
    code: 'wire {a.kind} to-{category',
  },
  {
    names: [
      'quick-issue',
      'quick-project',
      'quick-person',
      'quick-all',
      'quick-assigned',
      'quick-reported',
      'quick-profile',
    ],
    file: 'src/ui/search/QuickSearch.svelte',
    code: 'item quick-{row.kind}',
  },
  {
    names: [
      'jql-token-field',
      'jql-token-operator',
      'jql-token-keyword',
      'jql-token-value',
      'jql-token-string',
      'jql-token-function',
      'jql-token-punct',
      'jql-token-error',
    ],
    file: 'src/ui/search/JqlEditor.svelte',
    code: '`jql-token-${',
  },
  { names: ['is-stacked'], file: 'src/lib/actions.ts', code: "toggle('is-stacked'" },
  { names: ['touch-ghost'], file: 'src/lib/touch-drag.ts', code: "add('touch-ghost')" },
  { names: ['panel-title'], file: 'src/core/wiki.ts', code: 'class="panel-title"' },
  { names: ['wiki-table'], file: 'src/core/wiki.ts', code: 'class="wiki-table"' },
  { names: ['issue-key'], file: 'src/core/wiki.ts', code: 'class="issue-key"' },
  {
    names: ['mention', 'editor-mention'],
    file: 'src/core/wiki-doc.ts',
    code: "'mention editor-mention'",
  },
  { names: ['editor-raw'], file: 'src/core/wiki-doc.ts', code: "class: 'editor-raw'" },
  {
    names: ['editor-content'],
    file: 'src/ui/editor/RichEditor.svelte',
    code: "'editor-content wiki'",
  },
  { names: ['is-empty'], file: 'src/ui/editor/plugins.ts', code: "class: 'is-empty'" },
  {
    names: ['selectedCell'],
    file: 'node_modules/prosemirror-tables/dist/index.js',
    code: 'selectedCell',
  },
  {
    names: ['ProseMirror-selectednode'],
    file: 'node_modules/prosemirror-view/dist/index.js',
    code: 'ProseMirror-selectednode',
  },
];

/** The class names in the inline code of the section on hooks. */
function hookNames(doc: string): string[] {
  const section = /^## 3\. Hooks$([\s\S]*?)^## /m.exec(doc)?.[1] ?? '';
  const names = [...section.matchAll(/`([^`]+)`/g)]
    .map((m) => m[1]!)
    .filter((code) => !/^[\w/.-]+\.(css|svelte|ts)$/.test(code))
    .map((code) => code.replace(/'[^']*'|\[[^\]]*\]/g, '').replace(/(^|[^\w-])\d*\.\d+\w*/g, '$1'))
    .flatMap((code) => [...code.matchAll(/\.([A-Za-z_][\w-]*)/g)].map((m) => m[1]!));
  return [...new Set(names)];
}

/** The text of the `{…}` opening at `open`, without its braces. */
function braces(text: string, open: number): string {
  let depth = 0;
  for (let i = open; i < text.length; i++) {
    if (text[i] === '{') depth++;
    else if (text[i] === '}' && --depth === 0) return text.slice(open + 1, i);
  }
  return '';
}

/** The string literals of an expression, which are all it can add to a class. */
function literals(expr: string): string {
  return [...expr.matchAll(/'([^']*)'|`([^`]*)`/g)]
    .map((m) => (m[1] ?? m[2]!).replace(/\$\{[^}]*\}/g, ' '))
    .join(' ');
}

/** The class names a component's markup spells out: `class`, `*Class` props, `class:` directives. */
function classNames(svelte: string): string[] {
  const markup = svelte.replace(/<(script|style)[\s>][\s\S]*?<\/\1>/g, '');
  const values = [...markup.matchAll(/\bclass:([\w-]+)/g)].map((m) => m[1]!);
  for (const m of markup.matchAll(/\b\w*class=("[^"]*"|\{)/gi)) {
    values.push(
      m[1] === '{'
        ? literals(braces(markup, m.index + m[0].length - 1))
        : m[1]!.slice(1, -1).replace(/\{[^{}]*\}/g, literals),
    );
  }
  return values.flatMap((v) => v.split(/\s+/)).filter(Boolean);
}

function markupClasses(): Set<string> {
  const src = new URL('src/', frontend);
  return new Set(
    readdirSync(src, { recursive: true, encoding: 'utf8' })
      .filter((f) => f.endsWith('.svelte'))
      .flatMap((f) => classNames(readFileSync(new URL(f, src), 'utf8'))),
  );
}

describe('hookNames', () => {
  it('takes class names from inline code, in that section only', () => {
    const doc = `## 2. Tokens
\`.before\`
## 3. Hooks
\`button.item[role=menuitem]\`, \`.a > .b\`, \`h1.page-title\`, \`.c[data-x='.no']\`
\`url('assets/logo.svg')\`, \`app.css\`, \`src/ui/x-y.ts\`, \`1.55em\`, \`--paper\`
## 4. Next
\`.after\``;
    expect(hookNames(doc)).toEqual(['item', 'a', 'b', 'page-title', 'c']);
  });

  it('finds the hooks of ANATOMY.md', () => {
    expect(hookNames(anatomy)).toEqual(
      expect.arrayContaining(['topbar-inner', 'ledger', 'c-summary', 'is-stacked']),
    );
  });
});

describe('classNames', () => {
  it('reads classes, class props and directives, not scripts, styles or other attributes', () => {
    const svelte = `<script>const s = { class: 'script' };</script>
<p class="a {on ? 'b' : ''} c-{kind}" class:d={on} title="title"></p>
<div class={\`e \${f}\`}></div>
<Menu listClass={big ? 'g' : 'h'} class={\`i\`}></Menu>
<style>.style { color: red }</style>`;
    expect(classNames(svelte).sort()).toEqual(['a', 'b', 'c-', 'd', 'e', 'g', 'h', 'i']);
  });
});

describe('theme hooks', () => {
  const hooks = hookNames(anatomy);
  const inMarkup = markupClasses();
  const built = BUILT.flatMap((b) => b.names);

  it('all still appear in the interface', () => {
    const lost = hooks.filter((h) => !inMarkup.has(h) && !built.includes(h));
    expect(
      lost,
      'ANATOMY.md lists hooks that appear nowhere in frontend/src. ' +
        'Restore the hook, or remove it from ANATOMY.md and bump the contract (section 6).',
    ).toEqual([]);
  });

  it.each(BUILT)('$names are still built by $file', ({ file, code }) => {
    expect(readFileSync(new URL(file, frontend), 'utf8')).toContain(code);
  });

  it('allow only hooks the markup does not show', () => {
    const extra = built.filter((n) => inMarkup.has(n) || !hooks.includes(n));
    expect(extra, 'Remove these from BUILT.').toEqual([]);
  });
});
