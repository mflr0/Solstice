<script lang="ts">
  /** A shortcut drawn as one key cap per key; screen readers get it as one label. */

  let { parts, label }: { parts: readonly string[]; label?: string } = $props();

  // Apple's modifier symbols come out tiny in most monospace fonts.
  const GLYPHS = /^[⌘⌥⇧⌃]$/;
</script>

<span class="caps"
  >{#if label}<span class="sr-only">{label}</span>{/if}<span class="row" aria-hidden="true"
    >{#each parts as part, i (i)}<kbd class:glyph={GLYPHS.test(part)}>{part}</kbd>{/each}</span
  ></span
>

<style>
  .caps,
  .row {
    display: inline-flex;
    align-items: center;
  }

  .row {
    gap: 3px;
  }

  kbd {
    display: inline-block;
    min-width: 1.5em;
    padding: 2px 5px 1px;
    border: 1px solid var(--rule-strong);
    border-bottom-width: 2px;
    border-radius: var(--radius-s);
    background: var(--paper-raised);
    color: var(--ink);
    font: 500 0.8125rem/1.25 var(--font-mono);
    text-align: center;
    white-space: nowrap;
  }

  .glyph {
    font-family: var(--font-body);
    font-size: 0.9375rem;
    line-height: 1.08;
  }
</style>
