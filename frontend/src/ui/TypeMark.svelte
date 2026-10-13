<script lang="ts">
  import { typeLetter, typeTone } from '../core/marks.js';
  import type { TypeRef } from '../core/types.js';

  /** An issue type as a small lettered box, optionally followed by its name. */
  interface Props {
    type: TypeRef;
    withName?: boolean | undefined;
  }
  const { type, withName = false }: Props = $props();
  const tone = $derived(typeTone(type.name, type.subtask));
</script>

<span class="type" title={type.name}>
  <span class="box {tone}" aria-hidden={withName}>{typeLetter(type.name)}</span>
  {#if withName}<span>{type.name}</span>{:else}<span class="sr-only">{type.name}</span>{/if}
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
    border-radius: 2px;
    color: var(--tone);
    background: var(--paper-raised);
    font: 600 10.5px/1 var(--mono);
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
