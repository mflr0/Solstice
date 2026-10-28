<script lang="ts">
  import { untrack } from 'svelte';
  import { hasProblems } from '../../core/admin-forms.js';
  import type { Category, Group, KeyCheck, RoleRow } from '../../core/admin-types.js';
  import {
    issueTypesProblem,
    keyProblem,
    normalizeKey,
    projectPayload,
    projectProblems,
    suggestKey,
    type AccessKind,
    type ProjectForm,
  } from '../../core/project-admin.js';
  import type { UserRef } from '../../core/types.js';
  import { t } from '../../i18n/t.js';
  import { adminApi } from '../../lib/admin-api.js';
  import { api, errorMessage } from '../../lib/api.js';
  import { session } from '../../lib/session.svelte.js';
  import Dropdown from '../Dropdown.svelte';
  import Modal from '../Modal.svelte';
  import UserPicker from '../UserPicker.svelte';

  /**
   * Creating a project: name, key (suggested from the name, checked as it is
   * typed), lead, category, starting workflow, who may work in it and which
   * issue types it offers. Used by the administration and by the projects
   * list, so it carries its own form styles.
   */
  interface Props {
    open: boolean;
    onclose: () => void;
    /** Called once the project exists; the page then takes the visitor to it. */
    oncreated: (project: { id: number; key: string }, name: string) => void;
  }
  const { open, onclose, oncreated }: Props = $props();

  const ACCESS: AccessKind[] = ['lead', 'everyone', 'group', 'role'];

  const blank = (): ProjectForm => {
    const me = session.me;
    return {
      name: '',
      key: '',
      template: 'software',
      lead: me ? { id: me.id, username: me.username, name: me.name, active: true } : null,
      category: null,
      access: 'lead',
      accessId: null,
      description: '',
      issueTypes: (session.meta?.issueTypes ?? []).map((i) => i.id),
      defaultAssignee: 'unassigned',
    };
  };

  let form = $state<ProjectForm>(blank());
  let keyEdited = $state(false);
  let tried = $state(false);
  let touched = $state<Record<string, boolean>>({});
  let error = $state('');
  let busy = $state(false);
  let categories = $state<Category[]>([]);
  let groups = $state<Group[]>([]);
  let roles = $state<RoleRow[]>([]);
  let keyState = $state<{ key: string; check: KeyCheck | null } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  const formId = 'project-form';

  const allTypes = $derived(session.meta?.issueTypes ?? []);
  const problems = $derived({
    ...projectProblems(form),
    ...(issueTypesProblem(form.issueTypes ?? [], allTypes)
      ? { types: 'typesRequired' as const }
      : {}),
  });
  const shown = (f: 'name' | 'key' | 'access' | 'types') => {
    const p = problems[f];
    return (tried || touched[f]) && p ? t.projectsAdmin.problems[p] : '';
  };
  /** The server's word on the key, once it answers for the key now typed. */
  const taken = $derived(
    keyState && keyState.key === form.key && keyState.check && !keyState.check.available
      ? (keyState.check.reason ?? '')
      : '',
  );
  const free = $derived(
    !keyProblem(form.key) && keyState?.key === form.key && keyState.check?.available === true,
  );
  const checking = $derived(!keyProblem(form.key) && !!form.key && keyState?.key !== form.key);

  $effect(() => {
    if (!open) return;
    form = untrack(blank);
    keyEdited = false;
    tried = false;
    touched = {};
    error = '';
    keyState = null;
    // The choices come from the server; the form works without them.
    adminApi
      .categories()
      .then((c) => (categories = c))
      .catch(() => (categories = []));
    adminApi
      .groups()
      .then((g) => (groups = g))
      .catch(() => (groups = []));
    adminApi
      .roles()
      .then((r) => (roles = r))
      .catch(() => (roles = []));
  });

  function nameTyped(e: Event & { currentTarget: HTMLInputElement }) {
    // Read the field itself: the binding may not have run yet.
    if (!keyEdited) {
      form.key = suggestKey(e.currentTarget.value);
      keyChanged();
    }
  }

  function keyTyped(e: Event & { currentTarget: HTMLInputElement }) {
    keyEdited = true;
    form.key = normalizeKey(e.currentTarget.value);
    e.currentTarget.value = form.key;
    keyChanged();
  }

  /** Asks the server whether the key is free, once typing pauses. */
  function keyChanged() {
    clearTimeout(timer);
    const key = form.key;
    if (keyProblem(key)) return;
    const mine = ++seq;
    timer = setTimeout(async () => {
      try {
        const check = await adminApi.checkKey(key);
        if (mine === seq) keyState = { key, check };
      } catch {
        // The server has the last word when the form is sent.
        if (mine === seq) keyState = { key, check: null };
      }
    }, 300);
  }

  function setLead(u: UserRef | null) {
    form.lead = u;
  }

  function toggleType(id: string, on: boolean) {
    const rest = (form.issueTypes ?? []).filter((x) => x !== id);
    form.issueTypes = allTypes.map((x) => x.id).filter((x) => (x === id ? on : rest.includes(x)));
  }

  function accessChanged() {
    form.accessId = null;
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    tried = true;
    error = '';
    if (hasProblems(problems) || taken) return;
    busy = true;
    try {
      const created = await adminApi.createProject(projectPayload(form));
      oncreated(created, form.name.trim());
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} title={t.projectsAdmin.createTitle} onclose={() => !busy && onclose()}>
  <form id={formId} class="project-form" onsubmit={submit} novalidate>
    <div class="grid wide-first">
      <label class="field">
        <span>{t.projects.name}</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input"
          bind:value={form.name}
          oninput={nameTyped}
          placeholder={t.projectsAdmin.namePlaceholder}
          autocomplete="off"
          autofocus
          aria-invalid={!!shown('name')}
          onblur={() => (touched.name = true)}
        />
        {#if shown('name')}<span class="field-error">{shown('name')}</span>{/if}
      </label>
      <label class="field">
        <span>{t.projects.key}</span>
        <input
          class="input mono"
          bind:value={form.key}
          oninput={keyTyped}
          autocomplete="off"
          spellcheck="false"
          maxlength="10"
          aria-invalid={!!(shown('key') || taken)}
          aria-describedby="project-key-note"
          onblur={() => (touched.key = true)}
        />
      </label>
    </div>
    <p class="note" id="project-key-note" aria-live="polite">
      {#if shown('key')}
        <span class="field-error">{shown('key')}</span>
      {:else if taken}
        <span class="field-error">{taken}</span>
      {:else if free}
        <span class="ok">{t.projectsAdmin.keyFree(form.key)}</span>
      {:else if checking}
        <span class="muted">{t.projectsAdmin.keyChecking}</span>
      {:else}
        <span class="muted">{t.projectsAdmin.keyHint}</span>
      {/if}
    </p>

    <div class="grid">
      <div class="field">
        <span>{t.projects.lead}</span>
        <UserPicker
          field
          value={form.lead}
          label={t.projects.lead}
          allowNone={false}
          search={(q) => api.users(q)}
          onpick={setLead}
        />
        <span class="hint">{t.projectsAdmin.leadHint}</span>
      </div>
      <div class="field">
        <span>{t.projectsAdmin.category}</span>
        <Dropdown
          label={t.projectsAdmin.category}
          value={form.category === null ? '' : String(form.category)}
          options={[
            { value: '', label: t.projectsAdmin.noCategory },
            ...categories.map((c) => ({ value: String(c.id), label: c.name })),
          ]}
          onchange={(v) => (form.category = v ? Number(v) : null)}
        />
      </div>
    </div>

    <div class="field">
      <span>{t.projectsAdmin.template}</span>
      <Dropdown
        label={t.projectsAdmin.template}
        value={form.template}
        options={[
          { value: 'software', label: t.projectsAdmin.templates.software },
          { value: 'simple', label: t.projectsAdmin.templates.simple },
        ]}
        onchange={(v) => (form.template = v as 'software' | 'simple')}
      />
    </div>

    <label class="field">
      <span>{t.projectsAdmin.description}</span>
      <textarea class="textarea" bind:value={form.description} rows="2"></textarea>
    </label>

    <div class="grid">
      <div class="field">
        <span>{t.projectsAdmin.access}</span>
        <Dropdown
          label={t.projectsAdmin.access}
          value={form.access}
          options={ACCESS.map((a) => ({ value: a, label: t.projectsAdmin.accessKinds[a] }))}
          onchange={(v) => {
            form.access = v as AccessKind;
            accessChanged();
          }}
        />
      </div>
      {#if form.access === 'group' || form.access === 'role'}
        <div class="field grid-end">
          <Dropdown
            label={t.projectsAdmin.accessKinds[form.access]}
            value={form.accessId === null ? '' : String(form.accessId)}
            options={(form.access === 'group' ? groups : roles).map((x) => ({
              value: String(x.id),
              label: x.name,
            }))}
            placeholder={form.access === 'group'
              ? t.projectsAdmin.chooseGroup
              : t.projectsAdmin.chooseRole}
            invalid={!!shown('access')}
            onblur={() => (touched.access = true)}
            onchange={(v) => (form.accessId = Number(v))}
          />
        </div>
      {/if}
    </div>
    {#if shown('access')}
      <p class="note"><span class="field-error">{shown('access')}</span></p>
    {:else}
      <p class="note muted">{t.projectsAdmin.accessHint}</p>
    {/if}

    {#if allTypes.length > 0}
      <fieldset class="choices">
        <legend class="field-label">{t.projectsAdmin.types}</legend>
        <div class="type-list">
          {#each allTypes as it (it.id)}
            <label class="check">
              <input
                class="checkbox"
                type="checkbox"
                checked={form.issueTypes?.includes(it.id)}
                onchange={(e) => toggleType(it.id, e.currentTarget.checked)}
              />
              <span>{it.name}</span>
            </label>
          {/each}
        </div>
        {#if shown('types')}
          <span class="field-error">{shown('types')}</span>
        {:else}
          <span class="hint">{t.projectsAdmin.typesHint}</span>
        {/if}
      </fieldset>
    {/if}

    <div class="field">
      <span>{t.projectsAdmin.assignee}</span>
      <Dropdown
        label={t.projectsAdmin.assignee}
        value={form.defaultAssignee ?? 'unassigned'}
        options={[
          { value: 'unassigned', label: t.projectsAdmin.assignees.unassigned },
          { value: 'lead', label: t.projectsAdmin.assignees.lead },
        ]}
        onchange={(v) => (form.defaultAssignee = v as 'unassigned' | 'lead')}
      />
    </div>

    {#if error}<p class="error-text" role="alert">{error}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-quiet" disabled={busy} onclick={onclose}
      >{t.common.cancel}</button
    >
    <button class="btn btn-primary" type="submit" form={formId} disabled={busy}
      >{busy ? t.common.saving : t.projectsAdmin.create}</button
    >
  {/snippet}
</Modal>

<style>
  .project-form {
    display: grid;
    gap: 14px;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
    align-items: start;
  }

  .grid.wide-first {
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: 13px;
    letter-spacing: 0.04em;
  }

  /* A line of explanation under a field, close to it. */
  .note {
    margin: -8px 0 0;
    font-size: 12.5px;
    line-height: 1.45;
  }

  .ok {
    color: var(--ink-soft);
  }

  .field-error {
    font-size: 12.5px;
    color: var(--accent-text);
  }

  .input[aria-invalid='true'],
  .field :global(.select[aria-invalid='true']) {
    border-color: var(--accent);
  }

  .grid-end {
    align-self: end;
  }

  fieldset.choices {
    display: grid;
    gap: 6px;
    margin: 0;
    padding: 0;
    border: none;
    min-width: 0;
  }

  fieldset.choices > legend {
    padding: 0;
    margin-bottom: 6px;
  }

  .type-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 6px 14px;
  }

  .check {
    display: flex;
    gap: 8px;
    align-items: baseline;
    line-height: 1.4;
  }

  .hint {
    font-size: 12.5px;
    color: var(--muted);
  }

  @media (max-width: 600px) {
    .grid,
    .grid.wide-first {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
