<script lang="ts">
  import { typeLetter, typeTone } from '../core/marks.js';
  import type { TypeRef } from '../core/types.js';
  import { session } from '../lib/session.svelte.js';

  /** An issue type as a small lettered box, optionally followed by its name. */
  interface Props {
    type: TypeRef;
    withName?: boolean | undefined;
    /** Shows the name as stored, not as a seeded default in the viewer's language. */
    asStored?: boolean | undefined;
  }
  const { type, withName = false, asStored = false }: Props = $props();
  const tone = $derived(typeTone(type.name, type.subtask));
  const name = $derived(asStored ? type.name : session.typeName(type));
</script>

<span class="type" title={name}>
  <span class="box type-mark {tone}" data-type={tone} aria-hidden={withName}
    >{typeLetter(name)}</span
  >
  {#if withName}<span>{name}</span>{:else}<span class="sr-only">{name}</span>{/if}
</span>

<style>
  .type {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    vertical-align: middle;
  }

  .box {
    --tone: var(--tone-task);
    display: inline-grid;
    place-items: center;
    width: 18px;
    height: 18px;
    flex: none;
    border: 1px solid var(--tone);
    border-radius: var(--radius-xs);
    color: var(--tone);
    background: var(--paper-raised);
    font: 600 10.5px/1 var(--font-mono);
  }

  .bug {
    --tone: var(--tone-bug);
  }

  .story {
    --tone: var(--tone-story);
  }

  .epic {
    --tone: var(--tone-epic);
    background: var(--tone);
    color: var(--paper);
  }

  .subtask {
    --tone: var(--tone-subtask);
    border-style: dashed;
  }
</style>
