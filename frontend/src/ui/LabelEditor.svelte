<script lang="ts">
  import { tick } from 'svelte';
  import { focusAfterRemoval } from '../core/focus.js';
  import { paths } from '../core/route.js';
  import { t } from '../i18n/t.js';
  import { refocus } from '../lib/actions.js';

  /**
   * Labels as chips. Typing then Enter, comma or space adds one (labels
   * cannot contain spaces); × removes one.
   */
  interface Props {
    labels: string[];
    /** A save that takes time returns a promise, so focus moves once the chip is gone. */
    onchange: (labels: string[]) => unknown;
    editable?: boolean | undefined;
    /** The input's id, so a `<label for>` around the editor focuses it. */
    id?: string | undefined;
  }
  const { labels, onchange, editable = true, id }: Props = $props();
  let draft = $state('');
  let root = $state<HTMLElement>();
  let input = $state<HTMLElement>();

  async function remove(label: string) {
    const next = focusAfterRemoval(labels, label);
    await onchange(labels.filter((x) => x !== label));
    await tick();
    const removes = root?.querySelectorAll<HTMLElement>('.chip button');
    refocus(next === undefined ? input : (removes?.[labels.indexOf(next)] ?? input));
  }

  function commit() {
    const l = draft.trim().replace(/\s+/g, '_');
    draft = '';
    if (l && !labels.includes(l)) onchange([...labels, l]);
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
      e.preventDefault();
      commit();
    } else if (e.key === 'Backspace' && draft === '' && labels.length > 0) {
      onchange(labels.slice(0, -1));
    }
  }
</script>

<div class="labels" bind:this={root}>
  {#each labels as l (l)}
    <span class="chip">
      <a href={paths.search(`#${l}`)}>{l}</a>
      {#if editable}
        <button type="button" aria-label={`${t.common.remove} ${l}`} onclick={() => void remove(l)}
          >×</button
        >
      {/if}
    </span>
  {:else}
    {#if !editable}<span class="none">{t.common.noneF}</span>{/if}
  {/each}
  {#if editable}
    <input
      bind:this={input}
      {id}
      class="add"
      bind:value={draft}
      onkeydown={keydown}
      onblur={commit}
      placeholder={labels.length ? '+' : t.issue.addLabel}
      aria-label={t.issue.addLabel}
    />
  {/if}
</div>

<style>
  .labels {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }

  .chip a {
    text-decoration: none;
  }

  .chip a:hover {
    text-decoration: underline;
  }

  .add {
    flex: 1;
    min-width: 4.375rem;
    border: none;
    border-bottom: 1px dashed var(--rule-strong);
    background: transparent;
    padding: 2px 0;
    font: 0.78125rem var(--font-mono);
  }

  .add:focus {
    outline: none;
    border-bottom-color: var(--accent);
  }

  .none {
    color: var(--muted);
    font-style: italic;
  }
</style>
