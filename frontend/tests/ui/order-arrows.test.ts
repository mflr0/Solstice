// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount, type ComponentProps } from 'svelte';

import { t } from '../../src/i18n/t.js';
import OrderArrows from '../../src/ui/admin/OrderArrows.svelte';

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
});

function arrows(props: Partial<ComponentProps<typeof OrderArrows>> = {}) {
  const moves: number[] = [];
  component = mount(OrderArrows, {
    target: document.body,
    props: {
      id: 7,
      name: 'Bug',
      first: false,
      last: false,
      onmove: (by: -1 | 1) => moves.push(by),
      ...props,
    },
  });
  flushSync();
  const up = document.getElementById('order-up-7') as HTMLButtonElement;
  const down = document.getElementById('order-down-7') as HTMLButtonElement;
  return { up, down, moves };
}

describe('OrderArrows', () => {
  it('names the row in the labels', () => {
    const { up, down } = arrows();
    expect(up.getAttribute('aria-label')).toBe(t.catalogAdmin.moveUp('Bug'));
    expect(down.getAttribute('aria-label')).toBe(t.catalogAdmin.moveDown('Bug'));
  });

  it('moves the row up and down', () => {
    const { up, down, moves } = arrows();
    up.click();
    down.click();
    expect(moves).toEqual([-1, 1]);
  });

  it('cannot move the first row up', () => {
    const { up, down } = arrows({ first: true });
    expect([up.disabled, down.disabled]).toEqual([true, false]);
  });

  it('cannot move the last row down', () => {
    const { up, down } = arrows({ last: true });
    expect([up.disabled, down.disabled]).toEqual([false, true]);
  });

  it('disables both arrows while the list is saved', () => {
    const { up, down } = arrows({ disabled: true });
    expect([up.disabled, down.disabled]).toEqual([true, true]);
  });
});
