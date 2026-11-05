// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import type { TypeIcon, TypeRef } from '../../src/core/types.js';
import TypeMark from '../../src/ui/TypeMark.svelte';
import { TYPE_ICONS } from '../../src/ui/type-icons.js';

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
});

function mark(type: TypeRef): HTMLElement {
  const target = document.createElement('div');
  document.body.append(target);
  component = mount(TypeMark, { target, props: { type } });
  flushSync();
  const el = target.querySelector<HTMLElement>('.type-mark');
  if (!el) throw new Error('no mark');
  return el;
}

describe('TypeMark', () => {
  it('draws the letter and the guessed tone of an unstyled type', () => {
    const el = mark({ id: '1', name: 'Bug', subtask: false });
    expect(el.textContent).toBe('B');
    expect(el.querySelector('svg')).toBeNull();
    expect(el.dataset.type).toBe('bug');
    expect(el.dataset.fill).toBeUndefined();
    expect(el.style.getPropertyValue('--type-color')).toBe('');
  });

  it("draws the type's icon, colour and fill", () => {
    const el = mark({
      id: '1',
      name: 'Bug',
      subtask: false,
      icon: 'rocket',
      color: '#aa3300',
      fill: 'subtle',
    });
    expect(el.querySelectorAll('svg path')).toHaveLength(TYPE_ICONS.rocket.length);
    expect(el.textContent).toBe('');
    expect(el.dataset.type).toBe('bug');
    expect(el.dataset.fill).toBe('subtle');
    expect(el.style.getPropertyValue('--type-color')).toBe('#aa3300');
  });

  it('marks the detail paths of a filled icon, and not its outer paths', () => {
    const el = mark({ id: '1', name: 'Task', subtask: false, icon: 'task', fill: 'filled' });
    const paths = [...el.querySelectorAll('svg path')];
    expect(paths.map((p) => p.getAttribute('d'))).toEqual(TYPE_ICONS.task.map((p) => p.d));
    expect(paths.map((p) => p.classList.contains('detail'))).toEqual([false, true]);
  });

  it('names the icon it draws, so that a task icon can be filled by default', () => {
    expect(mark({ id: '1', name: 'Task', subtask: false, icon: 'task' }).dataset.icon).toBe('task');
    expect(mark({ id: '2', name: 'Task', subtask: false }).dataset.icon).toBeUndefined();
  });

  it('falls back to the letter for an icon this build does not draw', () => {
    const el = mark({ id: '1', name: 'Story', subtask: false, icon: 'unknown' as TypeIcon });
    expect(el.textContent).toBe('S');
  });
});
