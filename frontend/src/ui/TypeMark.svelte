<script lang="ts">
  import { typeLetter, typeTone } from '../core/marks.js';
  import type { TypeRef } from '../core/types.js';
  import { session } from '../lib/session.svelte.js';
  import Highlight from './Highlight.svelte';
  import { typeIcon } from './type-icons.js';

  /** An issue type as a small box with its icon or letter, optionally followed by its name. */
  interface Props {
    type: TypeRef;
    withName?: boolean | undefined;
    /** Shows the name as stored, not as a seeded default in the viewer's language. */
    asStored?: boolean | undefined;
    /** A search whose matches the shown name marks. */
    search?: string | undefined;
  }
  const { type, withName = false, asStored = false, search = '' }: Props = $props();
  const tone = $derived(typeTone(type.name, type.subtask));
  const name = $derived(asStored ? type.name : session.typeName(type));
  const glyph = $derived(typeIcon(type.icon));
</script>

<span class="type" title={name}>
  <span
    class="box type-mark {tone}"
    data-type={tone}
    data-fill={type.fill || null}
    style:--type-color={type.color || null}
    aria-hidden={withName}
    >{#if glyph}<svg viewBox="0 0 16 16" aria-hidden="true"
        >{#each glyph as d, i (i)}<path {d} />{/each}</svg
      >{:else}{typeLetter(name)}{/if}</span
  >
  {#if withName}<span
      >{#if search}<Highlight text={name} {search} />{:else}{name}{/if}</span
    >{:else}<span class="sr-only">{name}</span>{/if}
</span>

<style>
  .type {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    vertical-align: middle;
  }

  /* The server's colour comes in --type-color; without one, the guessed tone. */
  .box {
    --tone: var(--tone-task);
    --mark: var(--type-color, var(--tone));
    --mark-solid: var(--mark);
    --mark-on-tint: var(--mark);
    display: inline-grid;
    place-items: center;
    width: 1.125rem;
    height: 1.125rem;
    flex: none;
    border: 1px solid var(--mark);
    border-radius: var(--radius-xs);
    color: var(--mark);
    background: var(--paper-raised);
    font: 600 0.65625rem/1 var(--font-mono);
  }

  .bug {
    --tone: var(--tone-bug);
  }

  .story {
    --tone: var(--tone-story);
  }

  .epic {
    --tone: var(--tone-epic);
    background: var(--mark);
    color: var(--paper);
  }

  .subtask {
    --tone: var(--tone-subtask);
    border-style: dashed;
  }

  .box svg {
    width: 0.75rem;
    height: 0.75rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .box[data-fill] {
    border-style: solid;
  }

  .box[data-fill='filled'] {
    border-color: var(--mark-solid);
    background: var(--mark-solid);
    color: #fff;
  }

  .box[data-fill='subtle'] {
    border-color: transparent;
    background: color-mix(in srgb, var(--mark) 18%, var(--paper-raised));
    color: var(--mark-on-tint);
  }

  .box[data-fill='outline'] {
    border-color: var(--mark);
    background: transparent;
    color: var(--mark);
  }

  /*
   * As PriorityMark: a colour too pale for a light paper (too deep for a dark
   * one) is taken to the lightness that keeps 3:1 against the papers. A filled
   * box stays deep enough in both schemes for its light glyph; a tinted one
   * moves its glyph further from the tint.
   */
  @supports (color: oklch(from red l c h)) {
    .box {
      --mark: oklch(from var(--type-color, var(--tone)) min(l, 0.62) c h);
      --mark-solid: oklch(from var(--type-color, var(--tone)) min(l, 0.55) c h);
      --mark-on-tint: oklch(from var(--type-color, var(--tone)) min(l, 0.5) c h);
    }

    :global([data-scheme='dark']) .box {
      --mark: oklch(from var(--type-color, var(--tone)) max(l, 0.62) c h);
      --mark-on-tint: oklch(from var(--type-color, var(--tone)) max(l, 0.75) c h);
    }
  }
</style>
