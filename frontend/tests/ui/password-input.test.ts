// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import PasswordInput from '../../src/ui/PasswordInput.svelte';

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
});

function field(props: Record<string, unknown> = {}) {
  const form = document.createElement('form');
  document.body.append(form);
  component = mount(PasswordInput, { target: form, props: { value: 'secret', ...props } });
  flushSync();
  const input = form.querySelector('input');
  const button = form.querySelector('button');
  if (!input || !button) throw new Error('no field');
  return { form, input, button };
}

describe('PasswordInput', () => {
  it('starts hidden and toggles the input type', () => {
    const { input, button } = field();
    expect(input.type).toBe('password');
    expect(input.value).toBe('secret');
    expect(button.getAttribute('aria-pressed')).toBe('false');
    expect(button.getAttribute('aria-label')).toBe('Show password');

    button.click();
    flushSync();
    expect(input.type).toBe('text');
    expect(button.getAttribute('aria-pressed')).toBe('true');
    expect(button.getAttribute('aria-label')).toBe('Hide password');

    button.click();
    flushSync();
    expect(input.type).toBe('password');
  });

  it('does not submit the form', () => {
    const { form, button } = field();
    const submit = vi.fn((e: Event) => e.preventDefault());
    form.addEventListener('submit', submit);
    expect(button.type).toBe('button');
    button.click();
    expect(submit).not.toHaveBeenCalled();
  });

  it('forwards attributes and takes its own labels', () => {
    const { input, button } = field({
      class: 'mono',
      autocomplete: 'new-password',
      maxlength: 255,
      showLabel: 'Afficher',
      hideLabel: 'Masquer',
    });
    expect(input.classList.contains('input')).toBe(true);
    expect(input.classList.contains('mono')).toBe(true);
    expect(input.autocomplete).toBe('new-password');
    expect(input.maxLength).toBe(255);
    expect(button.getAttribute('aria-label')).toBe('Afficher');
  });
});
