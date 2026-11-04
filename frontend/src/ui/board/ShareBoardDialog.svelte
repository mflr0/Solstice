<script lang="ts">
  import { untrack } from 'svelte';
  import { addShare, removeShare, shareLabel, sharesLost } from '../../core/board-shares.js';
  import type { BoardDetail } from '../../core/board-types.js';
  import { paths } from '../../core/route.js';
  import type { FilterShare, IdName, Project, UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { notify } from '../../lib/ui.svelte.js';
  import CopyField from '../CopyField.svelte';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /** A board's link, and who sees it: its filter's shares, editable with can.share. */
  interface Props {
    open: boolean;
    detail: BoardDetail;
    shares: FilterShare[];
    onclose: () => void;
    onsaved: (shares: FilterShare[]) => void;
  }
  const { open, detail, shares, onclose, onsaved }: Props = $props();

  const b = $derived(detail.board);
  const canShare = $derived(detail.can.share);

  let draft = $state<FilterShare[]>([]);
  let error = $state('');
  let busy = $state(false);

  let kind = $state<FilterShare['kind']>('project');
  let groups = $state<IdName[]>([]);
  let projects = $state<Project[]>([]);
  let projectGroups = $state<IdName[]>([]);
  let groupId = $state('');
  let projectId = $state('');
  let projectGroupId = $state('');
  let person = $state<UserRef | null>(null);
  let projectSeq = 0;

  const kinds = $derived(
    (['everyone', 'group', 'project', 'user'] as const).filter(
      (k) =>
        !(k === 'everyone' && draft.some((s) => s.kind === 'everyone')) &&
        !(k === 'group' && groups.length === 0),
    ),
  );
  const lost = $derived(sharesLost(shares, draft));
  const ready = $derived(
    kind === 'everyone' ||
      (kind === 'group' && groupId !== '') ||
      (kind === 'project' && projectId !== '') ||
      (kind === 'user' && person !== null),
  );

  $effect(() => {
    if (!open) return;
    untrack(() => {
      draft = shares;
      error = '';
      kind = 'project';
      groupId = projectId = projectGroupId = '';
      person = null;
      projectGroups = [];
      if (canShare) void loadChoices();
    });
  });

  async function loadChoices() {
    try {
      [groups, projects] = await Promise.all([api.shareableGroups(), api.projects()]);
    } catch (err) {
      error = errorMessage(err);
    }
  }

  async function chooseProject(value: string) {
    projectId = value;
    projectGroupId = '';
    projectGroups = [];
    const mine = ++projectSeq;
    try {
      const found = await api.shareableGroups(Number(value));
      if (mine === projectSeq) projectGroups = found;
    } catch (err) {
      if (mine === projectSeq) error = errorMessage(err);
    }
  }

  /** What the add row describes; the name is the one the server gives this share. */
  function chosen(): FilterShare | null {
    if (kind === 'everyone') return { kind };
    if (kind === 'group') {
      const g = groups.find((x) => String(x.id) === groupId);
      return g ? { kind, id: g.id, name: g.name } : null;
    }
    if (kind === 'user') return person ? { kind, id: person.id, name: person.name } : null;
    const p = projects.find((x) => String(x.id) === projectId);
    if (!p) return null;
    const g = projectGroups.find((x) => String(x.id) === projectGroupId);
    return g
      ? { kind, id: p.id, group: g.id, name: `${p.key} · ${g.name}` }
      : { kind, id: p.id, name: p.key };
  }

  function add() {
    const share = chosen();
    if (!share) return;
    draft = addShare(draft, share);
    groupId = projectId = projectGroupId = '';
    projectGroups = [];
    person = null;
    if (share.kind === 'everyone') kind = 'project';
  }

  async function save() {
    if (busy) return;
    busy = true;
    error = '';
    try {
      const out = await api.setBoardShares(b.id, draft);
      notify(t.boards.shareSaved);
      onsaved(out.shares);
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} title={t.boards.share} {onclose}>
  <div class="share">
    <div class="field">
      <span>{t.boards.shareLink}</span>
      <CopyField value={`${location.origin}${paths.board(b.id)}`} label={t.boards.shareLink} />
    </div>

    <section aria-labelledby="share-who">
      <h3 id="share-who" class="who">{t.boards.shareWho}</h3>
      {#if draft.length === 0}
        <p class="none">{t.boards.shareNobody}</p>
      {:else}
        <ul class="list">
          {#each draft as share (`${share.kind}-${share.id ?? 0}-${share.group ?? 0}`)}
            <li>
              <span class="name">{shareLabel(share)}</span>
              {#if canShare}
                <button
                  type="button"
                  class="btn btn-quiet btn-small"
                  aria-label={t.boards.shareRemove(shareLabel(share))}
                  onclick={() => (draft = removeShare(draft, share))}>{t.common.remove}</button
                >
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if canShare}
      <div class="add">
        <Dropdown
          label={t.boards.shareKind}
          value={kind}
          options={kinds.map((k) => ({ value: k, label: t.boards.shareKinds[k] }))}
          onchange={(v) => (kind = v as FilterShare['kind'])}
        />
        {#if kind === 'group'}
          <Dropdown
            label={t.boards.shareKinds.group}
            value={groupId}
            placeholder={t.boards.sharePickGroup}
            options={groups.map((g) => ({ value: String(g.id), label: g.name }))}
            search={groups.length > 8}
            onchange={(v) => (groupId = v)}
          />
        {:else if kind === 'project'}
          <Dropdown
            label={t.boards.shareKinds.project}
            value={projectId}
            placeholder={t.boards.sharePickProject}
            options={projects.map((p) => ({
              value: String(p.id),
              label: `${p.key} · ${p.name}`,
            }))}
            search={projects.length > 8}
            onchange={(v) => void chooseProject(v)}
          />
          {#if projectGroups.length}
            <Dropdown
              label={t.boards.shareGroup}
              value={projectGroupId}
              options={[
                { value: '', label: t.boards.shareAnyGroup },
                ...projectGroups.map((g) => ({ value: String(g.id), label: g.name })),
              ]}
              onchange={(v) => (projectGroupId = v)}
            />
          {/if}
        {:else if kind === 'user'}
          <UserPicker
            value={person}
            label={t.boards.shareKinds.user}
            search={api.users}
            allowNone={false}
            noneLabel={t.boards.sharePickPerson}
            field
            onpick={(u) => (person = u)}
          />
        {/if}
        <button type="button" class="btn" disabled={!ready} onclick={add}
          >{t.boards.shareAdd}</button
        >
      </div>
      {#if lost}<p class="loss" role="status">{t.boards.shareLoss}</p>{/if}
    {/if}

    <p class="hint">
      <span>{t.boards.shareFilterBefore}</span><a href="/issues?filter={b.filterId}"
        >{b.filterName}</a
      ><span>{t.boards.shareFilterAfter}</span>
    </p>
    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </div>
  {#snippet footer()}
    {#if canShare}
      <button type="button" class="btn btn-quiet" onclick={onclose}>{t.common.cancel}</button>
      <button type="button" class="btn btn-primary" disabled={busy} onclick={() => void save()}
        >{busy ? t.common.saving : t.common.save}</button
      >
    {/if}
  {/snippet}
</Modal>

<style>
  .share {
    display: grid;
    gap: 16px;
    min-width: 0;
  }

  .who {
    margin: 0 0 6px;
    font-size: inherit;
  }

  .none,
  .list {
    margin: 0;
  }

  .list {
    display: grid;
    gap: 4px;
    padding: 0;
    list-style: none;
  }

  .list li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 2rem;
  }

  .name {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .loss {
    margin: 0;
    font-size: 0.8125rem;
  }

  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 0.8125rem;
  }
</style>
