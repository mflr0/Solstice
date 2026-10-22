// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { createRawSnippet, flushSync, mount, unmount } from 'svelte';

import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { t, useMessages } from '../../src/i18n/t.js';
import FilterPopover from '../../src/ui/search/FilterPopover.svelte';

const children = createRawSnippet(() => ({ render: () => '' }));

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  useMessages(en);
});

/** What a filter's button reads, the caret aside. */
function reads(label: string, summary: string): string {
  component = mount(FilterPopover, {
    target: document.body,
    props: { label, summary, open: false, children },
  });
  flushSync();
  const button = document.querySelector('button.filter-button') as HTMLButtonElement;
  const name = button.querySelector('.filter-name') as HTMLElement;
  const value = button.querySelector('.filter-value') as HTMLElement;
  return `${name.textContent}${value.textContent}`;
}

describe('a filter button', () => {
  it('separates the label from the value the English way', () => {
    expect(reads(t.search.fields.status, 'Open')).toBe('Status: Open');
  });

  it('keeps the French space before the colon', () => {
    useMessages(fr);
    expect(reads(t.search.fields.status, 'Ouvert')).toBe('Statut\u00a0: Ouvert');
  });
});
