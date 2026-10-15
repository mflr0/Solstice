<script lang="ts">
  import { paths } from '../core/route.js';
  import { t } from '../i18n/fr.js';

  /**
   * Labels as chips. Typing then Enter, comma or space adds one (labels
   * cannot contain spaces); × removes one.
   */
  interface Props {
    labels: string[];
    onchange: (labels: string[]) => void;
    editable?: boolean | undefined;
  }
  const { labels, onchange, editable = true }: Props = $props();
  let draft = $state('');

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

<div class="labels">
  {#each labels as l (l)}
    <span class="chip">
      <a href={paths.search(`#${l}`)}>{l}</a>
      {#if editable}
        <button
          type="button"
          aria-label={`${t.common.remove} ${l}`}
          onclick={() => onchange(labels.filter((x) => x !== l))}>×</button
        >
      {/if}
    </span>
  {:else}
    {#if !editable}<span class="none">{t.common.noneF}</span>{/if}
  {/each}
  {#if editable}
    <input
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
    min-width: 70px;
    border: none;
    border-bottom: 1px dashed var(--rule-strong);
    background: transparent;
    padding: 2px 0;
    font: 12.5px var(--font-mono);
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
