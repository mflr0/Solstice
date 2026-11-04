<script lang="ts">
  import type { HTMLInputAttributes } from 'svelte/elements';
  import { t } from '../i18n/t.js';

  /** A password input with an eye button that shows or hides what was typed. */
  interface Props extends Omit<HTMLInputAttributes, 'type' | 'value'> {
    value: string;
    input?: HTMLInputElement | undefined;
    /** The setup wizard has its own catalogue, so it passes its own labels. */
    showLabel?: string;
    hideLabel?: string;
  }
  let {
    value = $bindable(),
    input = $bindable(),
    showLabel = t.common.showPassword,
    hideLabel = t.common.hidePassword,
    class: className = '',
    ...rest
  }: Props = $props();

  let visible = $state(false);
</script>

<span class="password-input">
  <input
    bind:this={input}
    bind:value
    {...rest}
    class="input {className}"
    type={visible ? 'text' : 'password'}
  />
  <button
    type="button"
    class="reveal"
    aria-pressed={visible}
    aria-label={visible ? hideLabel : showLabel}
    onclick={() => (visible = !visible)}
  >
    <svg viewBox="0 0 20 20" width="16" height="16" aria-hidden="true">
      <path
        d="M1.75 10S4.75 4.25 10 4.25 18.25 10 18.25 10 15.25 15.75 10 15.75 1.75 10 1.75 10Z"
      />
      <circle cx="10" cy="10" r="2.75" />
      {#if visible}<path d="M3.25 3.25l13.5 13.5" />{/if}
    </svg>
  </button>
</span>

<style>
  .password-input {
    position: relative;
    display: block;
  }

  .password-input .input {
    padding-right: 34px;
  }

  .password-input .input::-ms-reveal {
    display: none;
  }

  .reveal {
    position: absolute;
    top: 50%;
    right: 3px;
    transform: translateY(-50%);
    display: grid;
    place-items: center;
    width: 28px;
    height: 26px;
    padding: 0;
    border: 0;
    border-radius: var(--radius-xs);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }

  .reveal:hover {
    color: var(--ink);
  }

  .reveal:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .reveal svg {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.4;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
