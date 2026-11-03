<script lang="ts">
  import { bindingFromPress, formatShortcut, isRefused } from '../../core/shortcuts.js';
  import { t } from '../../i18n/t.js';
  import { mac } from '../../lib/ui.svelte.js';

  /** A button that records the next key press as a binding of one action. Escape cancels. */

  let {
    id,
    action,
    disabled = false,
    onrecord,
  }: {
    id: string;
    /** The action's name, as the page shows it. */
    action: string;
    disabled?: boolean;
    /** Takes a binding; false when the action already has it. */
    onrecord: (binding: string) => boolean;
  } = $props();

  let recording = $state(false);
  let message = $state('');
  let error = $state(false);
  // The press that ends a recording is released on the button: its keyup
  // must not click it (Space) and start a new one.
  let swallowKeyup = false;

  function say(text: string, isError = false) {
    message = text;
    error = isError;
  }

  function start() {
    if (recording) return;
    recording = true;
    say(t.keyboard.recording);
  }

  function stop() {
    recording = false;
    swallowKeyup = true;
  }

  function keydown(e: KeyboardEvent) {
    if (!recording) return;
    if (e.key === 'Tab') {
      recording = false;
      say(t.keyboard.cancelled);
      return;
    }
    // The press is for the recorder, not the page's shortcuts or the button.
    e.preventDefault();
    e.stopPropagation();
    if (e.repeat) return;
    if (e.key === 'Escape') {
      stop();
      say(t.keyboard.cancelled);
      return;
    }
    const binding = bindingFromPress(e, mac);
    if (!binding) return;
    stop();
    const shown = formatShortcut(binding, mac, t.editor.keys);
    if (isRefused(binding)) say(t.keyboard.refused(shown), true);
    else if (!onrecord(binding)) say(t.keyboard.already(shown), true);
    else say(t.keyboard.recorded(shown, action));
  }

  function keyup(e: KeyboardEvent) {
    if (!swallowKeyup) return;
    swallowKeyup = false;
    e.preventDefault();
  }

  function blur() {
    if (!recording) return;
    recording = false;
    say(t.keyboard.cancelled);
  }
</script>

<span class="recorder">
  <button
    {id}
    type="button"
    class="btn btn-small"
    aria-pressed={recording}
    {disabled}
    onclick={start}
    onkeydown={keydown}
    onkeyup={keyup}
    onblur={blur}>{recording ? t.keyboard.recording : t.keyboard.addKey}</button
  >
  <span role="status" class={error ? 'message error-text' : 'sr-only'}>{message}</span>
</span>

<style>
  .recorder {
    display: inline-flex;
    flex-wrap: wrap;
    gap: 4px 8px;
    align-items: center;
    min-width: 0;
  }

  button {
    height: auto;
    min-height: 1.5rem;
    white-space: normal;
    text-align: start;
  }

  .message {
    font-size: 0.75rem;
    overflow-wrap: anywhere;
  }
</style>
