// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { SavedFilter } from '../../src/core/types.js';
import { en } from '../../src/i18n/en.js';
import { fr } from '../../src/i18n/fr.js';
import { useMessages } from '../../src/i18n/t.js';
import SavedFilters from '../../src/ui/search/SavedFilters.svelte';

const filter = (id: number, name: string): SavedFilter => ({
  id,
  name,
  description: null,
  query: 'status = Open',
  owner: null,
  mine: true,
  favourite: false,
  canEdit: true,
  shares: [],
});

const filters = [filter(1, 'Open bugs'), filter(2, 'My work')];

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  useMessages(en);
});

function render(current: number | null, list = filters) {
  const onclose = vi.fn();
  const ondeletemany = vi.fn();
  component = mount(SavedFilters, {
    target: document.body,
    props: {
      filters: list,
      current,
      onstar: () => {},
      onclose,
      onstarmany: () => Promise.resolve(),
      ondeletemany,
    },
  });
  flushSync();
  return { onclose, ondeletemany };
}

function manage() {
  const button = [...document.querySelectorAll<HTMLButtonElement>('button.manage')][0]!;
  button.click();
  flushSync();
}

const button = (text: string) =>
  [...document.querySelectorAll<HTMLButtonElement>('.manage-actions button')].find((b) =>
    b.textContent!.startsWith(text),
  )!;
const itemBoxes = () => document.querySelectorAll<HTMLInputElement>('.pick input');
const allBox = () => document.querySelector<HTMLInputElement>('.pick-all input')!;

const closeButtons = () => document.querySelectorAll<HTMLButtonElement>('button.close');

describe('the saved filters column', () => {
  it('shows no close button without a current filter', () => {
    render(null);
    expect(closeButtons()).toHaveLength(0);
  });

  it('puts a close button naming the filter on the current item only', () => {
    render(1);
    const buttons = closeButtons();
    expect(buttons).toHaveLength(1);
    const item = buttons[0]!.closest('li') as HTMLElement;
    expect(item.querySelector('a')?.getAttribute('aria-current')).toBe('page');
    expect(buttons[0]!.getAttribute('aria-label')).toBe('Close “Open bugs”');
    expect(buttons[0]!.title).toBe('Close “Open bugs”');
  });

  it('names the filter in the label in French too', () => {
    useMessages(fr);
    render(1);
    expect(closeButtons()[0]!.getAttribute('aria-label')).toBe('Fermer « Open bugs »');
  });

  it('closes from the close button', () => {
    const { onclose } = render(1);
    closeButtons()[0]!.click();
    expect(onclose).toHaveBeenCalledOnce();
  });

  it('closes when the current name is clicked again', () => {
    const { onclose } = render(1);
    const link = document.querySelector('a[aria-current="page"]') as HTMLAnchorElement;
    const click = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 });
    link.dispatchEvent(click);
    expect(onclose).toHaveBeenCalledOnce();
    expect(click.defaultPrevented).toBe(true);
  });

  it('opens another filter without closing', () => {
    const { onclose } = render(1);
    const other = document.querySelector('a[href="/issues?filter=2"]') as HTMLAnchorElement;
    other.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
    expect(onclose).not.toHaveBeenCalled();
  });
});

describe('manage mode', () => {
  const shared = { ...filter(3, 'Team board'), mine: false, canEdit: false };

  it('toggles an item when it is clicked, without opening it', () => {
    render(null);
    manage();
    expect(document.querySelectorAll('.saved-item a')).toHaveLength(0);
    const name = document.querySelector('.pick .name') as HTMLElement;
    name.click();
    flushSync();
    expect(itemBoxes()[0]!.checked).toBe(true);
    name.click();
    flushSync();
    expect(itemBoxes()[0]!.checked).toBe(false);
  });

  it('names the filter in each checkbox', () => {
    render(null);
    manage();
    expect(itemBoxes()[0]!.getAttribute('aria-label')).toBe('Select “Open bugs”');
  });

  it('selects every item, then goes indeterminate when one is cleared', () => {
    render(null, [...filters, shared]);
    manage();
    allBox().click();
    flushSync();
    expect([...itemBoxes()].every((b) => b.checked)).toBe(true);
    expect(allBox().checked).toBe(true);
    expect(allBox().indeterminate).toBe(false);
    itemBoxes()[1]!.click();
    flushSync();
    expect(allBox().checked).toBe(false);
    expect(allBox().indeterminate).toBe(true);
  });

  it('counts only the filters one can delete', () => {
    const { ondeletemany } = render(null, [...filters, shared]);
    manage();
    allBox().click();
    flushSync();
    const del = button('Delete');
    expect(del.textContent).toBe('Delete (2)');
    del.click();
    expect(ondeletemany).toHaveBeenCalledWith([...filters, shared]);
  });

  it('leaves manage mode and clears the selection on Done', () => {
    render(null);
    manage();
    allBox().click();
    flushSync();
    button('Done').click();
    flushSync();
    expect(itemBoxes()).toHaveLength(0);
    manage();
    expect([...itemBoxes()].some((b) => b.checked)).toBe(false);
  });
});
