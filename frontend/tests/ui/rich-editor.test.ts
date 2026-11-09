// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';

import { t } from '../../src/i18n/t.js';
import RichEditor from '../../src/ui/editor/RichEditor.svelte';

vi.mock('../../src/lib/api.js', () => ({
  api: {
    users: () => Promise.resolve([]),
    issues: () => Promise.resolve([]),
    upload: () => new Promise(() => {}),
  },
  errorMessage: String,
  isVisible: () => true,
}));

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
});

function open(uploadTo?: string) {
  component = mount(RichEditor, {
    target: document.body,
    props: { value: 'Some text', label: 'Description', uploadTo },
  });
  flushSync();
}

function attach(): HTMLButtonElement | null {
  return document.querySelector<HTMLButtonElement>(`button[aria-label="${t.editor.attach}"]`);
}

/** jsdom has no DragEvent: a plain event carrying the types a browser would give. */
function drag(target: Element, type: string, types: string[]): Event {
  const e = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(e, 'dataTransfer', { value: { types, files: [] } });
  target.dispatchEvent(e);
  flushSync();
  return e;
}

const overlay = () => document.querySelector('.editor-dropzone');

describe('RichEditor', () => {
  it('offers the attach button only when files can be uploaded', () => {
    open();
    expect(attach()).toBeNull();
    unmount(component!);
    open('ALPHA-1');
    expect(attach()).not.toBeNull();
    expect(attach()!.title).toBe(t.editor.attach);
  });

  it('keeps the attach button in the toolbar arrow-key order', () => {
    open('ALPHA-1');
    const button = attach()!;
    const before = button.previousElementSibling as HTMLButtonElement;
    before.focus();
    before.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(document.activeElement).toBe(button);
  });

  it('shows the drop overlay while files are over the editor', () => {
    open('ALPHA-1');
    const frame = document.querySelector('.editor')!;
    const text = document.querySelector('.editor-content')!;
    drag(frame, 'dragenter', ['Files']);
    expect(overlay()?.textContent).toContain(t.editor.dropFiles);
    // Crossing into a child: enter there first, then leave the parent.
    drag(text, 'dragenter', ['Files']);
    drag(frame, 'dragleave', ['Files']);
    expect(overlay()).not.toBeNull();
    drag(text, 'dragleave', ['Files']);
    expect(overlay()).toBeNull();

    drag(frame, 'dragenter', ['Files']);
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    flushSync();
    expect(overlay()).toBeNull();
  });

  it('ignores dragged text', () => {
    open('ALPHA-1');
    drag(document.querySelector('.editor')!, 'dragenter', ['text/plain', 'text/html']);
    expect(overlay()).toBeNull();
  });

  it('leaves dropped files to the page when nothing is uploaded here', () => {
    open();
    const frame = document.querySelector('.editor')!;
    drag(frame, 'dragenter', ['Files']);
    expect(overlay()).toBeNull();
    expect(drag(frame, 'dragover', ['Files']).defaultPrevented).toBe(false);
    expect(drag(frame, 'drop', ['Files']).defaultPrevented).toBe(false);
  });
});
