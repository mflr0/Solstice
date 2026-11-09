<script lang="ts">
  import { untrack } from 'svelte';
  import { addShare, removeShare } from '../../core/board-shares.js';
  import { paths } from '../../core/route.js';
  import type { FilterShare, IdName, Project, SavedFilter, UserRef } from '../../core/types.js';
  import { formatList } from '../../i18n/format.js';
  import { t } from '../../i18n/t.js';
  import { layer } from '../../lib/actions.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import Avatar from '../Avatar.svelte';
  import CopyField from '../CopyField.svelte';
  import Modal from '../Modal.svelte';
  import { shareChoices, type ShareChoice } from './share-choices.js';

  /**
   * Who sees one of the user's views: each change is saved at once. A project
   * share narrowed to a group is listed and can go, not be made here.
   */
  interface Props {
    filter: SavedFilter | null;
    onclose: () => void;
    onsaved: (filter: SavedFilter) => void;
  }
  const { filter, onclose, onsaved }: Props = $props();

  let groups = $state<IdName[]>([]);
  let projects = $state<Project[]>([]);
  let people = $state<UserRef[]>([]);
  let q = $state('');
  let picked = $state<ShareChoice | null>(null);
  let open = $state(false);
  let active = $state(0);
  let busy = $state(false);
  let error = $state('');
  let input: HTMLInputElement | undefined = $state();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const uid = $props.id();
  const listId = `share-choices-${uid}`;
  const optionId = (i: number) => `${listId}-${i}`;

  const shares = $derived(filter?.shares ?? []);
  const listed = $derived(shares.filter((s) => s.kind !== 'everyone'));
  const everyone = $derived(shares.some((s) => s.kind === 'everyone'));
  const choices = $derived(picked ? [] : shareChoices(q, people, groups, projects, shares));
  const expanded = $derived(open && choices.length > 0);

  $effect(() => {
    if (!filter) return;
    untrack(() => {
      q = '';
      picked = null;
      people = [];
      error = '';
      open = false;
      void loadChoices();
    });
  });

  async function loadChoices() {
    try {
      [groups, projects] = await Promise.all([api.shareableGroups(), api.projects()]);
    } catch (err) {
      error = errorMessage(err);
    }
  }

  async function findPeople() {
    const mine = ++seq;
    const term = q.trim();
    try {
      const found = term ? await api.users(term) : [];
      if (mine === seq) people = found.filter((u) => u.id !== filter?.owner?.id);
    } catch {
      if (mine === seq) people = [];
    }
  }

  function typed() {
    picked = null;
    open = true;
    active = 0;
    clearTimeout(timer);
    timer = setTimeout(() => void findPeople(), 180);
  }

  function choose(c: ShareChoice) {
    picked = c;
    q = c.share.name;
    open = false;
    input?.focus();
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' && choices.length) {
      e.preventDefault();
      open = true;
      active = Math.min(choices.length - 1, active + 1);
    } else if (e.key === 'ArrowUp' && expanded) {
      e.preventDefault();
      active = Math.max(0, active - 1);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const c = expanded ? choices[active] : undefined;
      if (c) choose(c);
      else if (picked) void share();
    } else if (e.key === 'Escape' && expanded) {
      // Close the list, not the dialog.
      e.preventDefault();
      e.stopPropagation();
      open = false;
    }
  }

  async function save(next: FilterShare[]): Promise<boolean> {
    if (!filter || busy) return false;
    busy = true;
    error = '';
    try {
      const out = await api.updateFilter(filter.id, { shares: next });
      if (out) onsaved(out);
      return true;
    } catch (err) {
      error = errorMessage(err);
      return false;
    } finally {
      busy = false;
    }
  }

  async function share() {
    if (!picked) return;
    if (await save(addShare(shares, picked.share))) {
      q = '';
      picked = null;
      people = [];
      input?.focus();
    }
  }

  function remove(s: FilterShare) {
    void save(removeShare(shares, s));
  }

  /** The box follows the saved shares: a refused save unticks it again. */
  async function toggleEveryone(box: HTMLInputElement) {
    const all: FilterShare = { kind: 'everyone' };
    if (!(await save(box.checked ? addShare(shares, all) : removeShare(shares, all))))
      box.checked = everyone;
  }

  function kindOf(s: FilterShare): string {
    if (s.kind === 'project')
      return s.group ? t.viewShare.kinds.projectGroup : t.viewShare.kinds.project;
    return s.kind === 'group' ? t.viewShare.kinds.group : t.viewShare.kinds.user;
  }
</script>

<Modal open={filter !== null} title={filter ? t.viewShare.title(filter.name) : ''} {onclose}>
  {#if filter}
    <div class="share">
      <p class="intro">{t.viewShare.intro}</p>

      <div class="add">
        <div class="search">
          <input
            bind:this={input}
            class="input"
            type="search"
            bind:value={q}
            oninput={typed}
            onkeydown={keydown}
            placeholder={t.viewShare.placeholder}
            aria-label={t.viewShare.search}
            autocomplete="off"
            role="combobox"
            aria-expanded={expanded}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={expanded ? optionId(active) : undefined}
          />
          {#if expanded}
            <ul
              class="popover choices"
              id={listId}
              role="listbox"
              aria-label={t.viewShare.search}
              popover="manual"
              use:layer={{ anchor: input, matchWidth: true, onoutside: () => (open = false) }}
            >
              {#each choices as c, i (`${c.share.kind}-${c.share.id}`)}
                <li id={optionId(i)} role="option" aria-selected={i === active}>
                  <button
                    type="button"
                    class="item"
                    class:active={i === active}
                    tabindex="-1"
                    onclick={() => choose(c)}
                    onmouseenter={() => (active = i)}
                  >
                    <span class="choice">{c.share.name}</span>
                    <span class="muted">{c.username ?? kindOf(c.share)}</span>
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <button
          type="button"
          class="btn btn-primary"
          disabled={!picked || busy}
          onclick={() => void share()}>{t.viewShare.share}</button
        >
      </div>

      <section aria-labelledby="share-who-{uid}">
        <h3 id="share-who-{uid}" class="heading">{t.viewShare.who}</h3>
        <ul class="list">
          {#if filter.owner}
            <li>
              <Avatar user={filter.owner} />
              <span class="name"
                >{filter.owner.name}
                {#if filter.owner.id === session.me?.id}<span class="muted">{t.viewShare.you}</span
                  >{/if}</span
              >
              <span class="muted role">{t.viewShare.owner}</span>
            </li>
          {/if}
          {#each listed as s (`${s.kind}-${s.id ?? 0}-${s.group ?? 0}`)}
            <li>
              <span class="name">{s.name ?? ''} <span class="muted">· {kindOf(s)}</span></span>
              <button
                type="button"
                class="btn btn-quiet btn-small"
                disabled={busy}
                aria-label={t.viewShare.remove(s.name ?? '')}
                onclick={() => remove(s)}>{t.common.remove}</button
              >
            </li>
          {/each}
        </ul>
        <label class="everyone">
          <input
            type="checkbox"
            checked={everyone}
            disabled={busy}
            onchange={(e) => void toggleEveryone(e.currentTarget)}
          />
          <span>{t.viewShare.everyone(t.appName)}</span>
        </label>
      </section>

      {#if filter.boards.length}
        <p class="boards" role="note">
          {t.viewShare.boards(filter.boards.length, formatList(filter.boards.map((b) => b.name)))}
        </p>
      {/if}

      <section aria-labelledby="share-link-{uid}">
        <h3 id="share-link-{uid}" class="heading">{t.viewShare.link}</h3>
        <CopyField
          value={`${location.origin}${paths.view(filter.id)}`}
          label={t.viewShare.linkLabel}
        />
        <p class="hint">{t.viewShare.linkHint}</p>
      </section>

      {#if error}<p class="error-text" role="alert">{error}</p>{/if}
    </div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn" onclick={onclose}>{t.viewShare.done}</button>
  {/snippet}
</Modal>

<style>
  .share {
    display: grid;
    gap: 16px;
    min-width: 0;
  }

  .intro,
  .hint,
  .boards {
    margin: 0;
    font-size: 0.8125rem;
    color: var(--ink-soft);
  }

  .hint {
    margin-top: 6px;
    font-size: 0.75rem;
    color: var(--muted);
  }

  .boards {
    padding: 8px 12px;
    border-left: 3px solid var(--accent);
    background: var(--paper-sunk);
    color: var(--ink);
  }

  .add {
    display: flex;
    gap: 8px;
  }

  .search {
    flex: 1;
    min-width: 0;
  }

  .search .input {
    width: 100%;
    box-sizing: border-box;
  }

  .choices {
    list-style: none;
    margin: 0;
    padding: 4px 0;
  }

  .item {
    display: flex;
    align-items: baseline;
    gap: 8px;
    width: 100%;
    text-align: left;
  }

  .choice {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .heading {
    margin: 0 0 4px;
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .list {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .list li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
    border-bottom: 1px solid var(--rule);
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .role {
    font-size: 0.8125rem;
  }

  .everyone {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
  }
</style>
