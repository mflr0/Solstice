<script lang="ts">
  import { typeLetter, typeTone } from '../core/marks.js';
  import type { TypeRef } from '../core/types.js';
  import { session } from '../lib/session.svelte.js';
  import Highlight from './Highlight.svelte';
  import { typeIcon } from './type-icons.js';

  /** An issue type as its bare icon or letter, optionally followed by its name. */
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
    class="type-mark {tone}"
    data-type={tone}
    data-fill={type.fill || null}
    style:--type-color={type.color || null}
    aria-hidden={withName}
    >{#if glyph}<svg viewBox="0 0 16 16" aria-hidden="true"
        >{#each glyph as path, i (i)}<path d={path.d} class={path.part} />{/each}</svg
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
  .type-mark {
    --tone: var(--tone-task);
    --mark: var(--type-color, var(--tone));
    --mark-solid: var(--mark);
    --mark-on-tint: var(--mark);
    display: inline-grid;
    place-items: center;
    width: 1.125rem;
    height: 1.125rem;
    flex: none;
    color: var(--mark);
    font: 600 1rem/1 var(--font-mono);
  }

  .bug {
    --tone: var(--tone-bug);
  }

  .story {
    --tone: var(--tone-story);
  }

  .epic {
    --tone: var(--tone-epic);
  }

  .subtask {
    --tone: var(--tone-subtask);
  }

  .type-mark svg {
    width: 1rem;
    height: 1rem;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  /*
   * An epic without a fill of its own is filled. Most skins' faces stop below
   * 800, where 600 and 800 synthesise the same bold: the stroke keeps them apart.
   */
  .type-mark:is([data-fill='filled'], .epic:not([data-fill])) {
    color: var(--mark-solid);
    font-weight: 800;
    -webkit-text-stroke: 0.04em currentColor;
  }

  .type-mark:is([data-fill='filled'], .epic:not([data-fill])) path:not(.line, .detail) {
    fill: currentColor;
  }

  .type-mark:is([data-fill='filled'], .epic:not([data-fill])) path.detail {
    stroke: var(--paper);
  }

  .type-mark[data-fill='subtle'] {
    color: var(--mark-on-tint);
  }

  .type-mark[data-fill='subtle'] path:not(.line, .detail) {
    fill: color-mix(in srgb, var(--mark-on-tint) 18%, transparent);
  }

  /*
   * As PriorityMark: a colour too pale for a light paper (too deep for a dark
   * one) is taken to the lightness that keeps 3:1 against the papers. A tinted
   * glyph moves further from its own tint.
   */
  @supports (color: oklch(from red l c h)) {
    .type-mark {
      --mark: oklch(from var(--type-color, var(--tone)) min(l, 0.62) c h);
      --mark-solid: oklch(from var(--type-color, var(--tone)) min(l, 0.55) c h);
      --mark-on-tint: oklch(from var(--type-color, var(--tone)) min(l, 0.5) c h);
    }

    :global([data-scheme='dark']) .type-mark {
      --mark: oklch(from var(--type-color, var(--tone)) max(l, 0.62) c h);
      --mark-solid: oklch(from var(--type-color, var(--tone)) max(l, 0.62) c h);
      --mark-on-tint: oklch(from var(--type-color, var(--tone)) max(l, 0.75) c h);
    }
  }
</style>
